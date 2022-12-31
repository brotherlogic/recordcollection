package main

import (
	"fmt"
	"strings"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	pbgd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	// RecacheDelay - recache everything every 30 days
	RecacheDelay = 60 * 60 * 24 * 30
)

var (
	backlogCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "recordcollection_backlog",
		Help: "Push Size",
	}, []string{"source"})

	updateFanout = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "recordcollection_updatefanout",
		Help: "Push Size",
	})

	updateFanoutFailure = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recordcollection_updatefanoutfailure",
		Help: "Push Size",
	}, []string{"error", "server"})

	loopLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "recordcollection_loop_latency",
		Help:    "The latency of server requests",
		Buckets: []float64{.005 * 1000, .01 * 1000, .025 * 1000, .05 * 1000, .1 * 1000, .25 * 1000, .5 * 1000, 1 * 1000, 2.5 * 1000, 5 * 1000, 10 * 1000, 30 * 1000, 60 * 1000, 120 * 1000, 240 * 1000},
	}, []string{"method"})
)

func (s *Server) runUpdateFanout(ctx context.Context) {
	for fid := range s.updateFanout {
		id := fid.iid
		s.CtxLog(ctx, fmt.Sprintf("Running fanout for %+v", fid))

		s.repeatCount[id]++
		if s.repeatCount[id] > 10 {
			//s.RaiseIssue(fmt.Sprintf("%v cannot be updated", id), fmt.Sprintf("Last error was %v", s.repeatError[id]))
		}

		t := time.Now()
		loopLatency.With(prometheus.Labels{"method": "elect"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))

		ctx, cancel := utils.ManualContext("rciu", time.Minute)

		t = time.Now()
		record, err := s.loadRecord(ctx, id, false)
		loopLatency.With(prometheus.Labels{"method": "load"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))

		if err != nil {
			// Ignore out of range errors - these are deleted records
			if status.Convert(err).Code() != codes.OutOfRange {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Unable to load: %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "load", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
			}

			//We get an Invalid argument when we've failed to save out an added record
			if status.Convert(err).Code() == codes.InvalidArgument {
				record = &pb.Record{Release: &pbgd.Release{InstanceId: id}}
			} else {
				cancel()
				time.Sleep(time.Minute)
				s.CtxLog(ctx, fmt.Sprintf("Skipping %v because it's %v", id, err))
				continue
			}
		}

		t = time.Now()
		err = s.syncWantlist(ctx)
		if err != nil {
			s.CtxLog(ctx, fmt.Sprintf("Error pulling wantlist: %v", err))
			updateFanoutFailure.With(prometheus.Labels{"server": "syncwants", "error": fmt.Sprintf("%v", err)}).Inc()
		}
		loopLatency.With(prometheus.Labels{"method": "syncwant"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))

		// Perform a discogs update if needed
		if time.Now().Sub(time.Unix(record.GetMetadata().GetLastCache(), 0)) > time.Hour*24*30 ||
			time.Now().Sub(time.Unix(record.GetMetadata().GetLastInfoUpdate(), 0)) > time.Hour*24*30 ||
			record.GetRelease().GetRecordCondition() == "" {
			t = time.Now()
			s.cacheRecord(ctx, record)
			loopLatency.With(prometheus.Labels{"method": "cache"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
		}

		if time.Now().Sub(time.Unix(record.GetMetadata().GetSalePriceUpdate(), 0)) > time.Hour*24*7 {
			t = time.Now()
			s.updateRecordSalePrice(ctx, record)
			loopLatency.With(prometheus.Labels{"method": "saleprice"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
		}

		// Push the metadata every week
		if time.Now().Sub(time.Unix(record.GetMetadata().GetSalePriceUpdate(), 0)) > time.Hour*24 {
			t = time.Now()
			err = s.pushMetadata(ctx, record)
			loopLatency.With(prometheus.Labels{"method": "pushmeta"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
			if err != nil {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Unable to push: %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "push", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
				cancel()
				time.Sleep(time.Minute)
				continue
			}
		}

		// Finally push the record if we need to
		if record.GetMetadata().GetDirty() {
			ctx, cancel2 := utils.ManualContext("rciu", time.Minute)
			t = time.Now()
			_, err = s.pushRecord(ctx, record)
			loopLatency.With(prometheus.Labels{"method": "push"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
			if err != nil {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Unable to push: %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "push", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
				cancel2()
				cancel()
				time.Sleep(time.Minute)
				continue
			}
		}
		cancel()

		// Update the sale
		if record.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL || record.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE {
			ctx, cancel := utils.ManualContext("rcu", time.Minute)
			t = time.Now()
			err := s.updateSale(ctx, record.GetRelease().GetInstanceId())
			loopLatency.With(prometheus.Labels{"method": "updatesale"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
			if err == nil {
				record, err = s.loadRecord(ctx, id, false)
			}
			cancel()

			if err != nil {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Unable to update record for sale: %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "updateSale", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
				time.Sleep(time.Minute)
				continue
			}

		}

		// Push the sale (only if we're listed to sell and the record is for sale)
		if record.GetMetadata().GetSaleDirty() &&
			(record.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL || record.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE) &&
			record.GetMetadata().GetSaleState() != pbd.SaleState_SOLD {
			ctx, cancel := utils.ManualContext("rciu", time.Minute)
			t = time.Now()
			_, err = s.pushSale(ctx, record)
			loopLatency.With(prometheus.Labels{"method": "cache"}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
			cancel()
			if err != nil {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Unable to push sale : %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "pushSale", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
				time.Sleep(time.Minute)
				continue
			}
		}

		failed := false
		for _, server := range s.fanoutServers {
			t = time.Now()
			ctx, cancel := utils.ManualContext("rcfo", time.Minute*30)
			conn, err := s.FDialServer(ctx, server)

			if err != nil {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Bad dial of %v -> %v", server, err))
				updateFanoutFailure.With(prometheus.Labels{"server": server, "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
				failed = true
				break
			}

			client := pb.NewClientUpdateServiceClient(conn)
			_, err = client.ClientUpdate(ctx, &pb.ClientUpdateRequest{InstanceId: id})
			loopLatency.With(prometheus.Labels{"method": "update-" + server}).Observe(float64(time.Now().Sub(t).Nanoseconds() / 1000000))
			if err != nil {
				s.repeatError[id] = err
				s.CtxLog(ctx, fmt.Sprintf("Bad update of (%v) %v -> %v", id, server, err))
				updateFanoutFailure.With(prometheus.Labels{"server": server, "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- fid
				conn.Close()
				failed = true
				break
			}

			conn.Close()
			cancel()
		}

		if !failed {
			t = time.Now()

			//Attemp to update the record
			ctx, cancel = utils.ManualContext("rc-fw", time.Minute)
			record, err = s.loadRecord(ctx, id, false)
			if err == nil {
				record.GetMetadata().LastUpdateTime = time.Now().Unix()
				s.saveRecord(ctx, record)
			}
			s.CtxLog(ctx, fmt.Sprintf("Ran fanout for %v at %v with %v", id, time.Now(), err))

			updateFanout.Set(float64(len(s.updateFanout)))
			updateFanoutFailure.With(prometheus.Labels{"server": "none", "error": "nil"}).Inc()
		}

		time.Sleep(time.Second)
	}
}

func (s *Server) validateSales(ctx context.Context) error {
	sales, err := s.retr.GetInventory(ctx)
	if err != nil {
		return err
	}

	s.CtxLog(ctx, fmt.Sprintf("Found %v sales", len(sales)))
	matchCount := 0
	for _, sale := range sales {
		found := false

		// This call will not fail
		recs, _ := s.QueryRecords(ctx, &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_ReleaseId{sale.GetId()}})

		s.CtxLog(ctx, fmt.Sprintf("Found %v results (%v)", len(recs.GetInstanceIds()), sale.GetId()))

		for _, id := range recs.GetInstanceIds() {
			rec, err := s.getRecord(ctx, id)
			if err != nil {
				s.CtxLog(ctx, fmt.Sprintf("Err: %v", err))
				return err
			}

			if (rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL || rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE) && rec.GetMetadata().GetSaleId() == sale.GetSaleId() {
				found = true
			}
		}

		if !found {
			s.CtxLog(ctx, fmt.Sprintf("Sending off problem"))
			s.RaiseIssue("Sale Error Found", fmt.Sprintf("%v is not found in collection", sale))
			return fmt.Errorf("Found a sale problem")
		}
		matchCount++
	}
	s.CtxLog(ctx, fmt.Sprintf("Matched %v", matchCount))

	// Searching LISTED and STALE
	for _, folder := range []int32{488127, 1708299} {
		recs, _ := s.QueryRecords(ctx, &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_FolderId{folder}})
		for _, id := range recs.GetInstanceIds() {
			rec, err := s.getRecord(ctx, id)
			if err != nil {
				return err
			}

			seen := false
			for _, sale := range sales {
				if sale.GetSaleId() == rec.GetMetadata().GetSaleId() {
					seen = true
				}
			}
			if !seen {
				s.RaiseIssue("Sale Missing", fmt.Sprintf("%v is missing the sale", id))
				return fmt.Errorf("Found a sale problem")
			}
		}
	}

	return nil
}

func (s *Server) pushSale(ctx context.Context, val *pb.Record) (bool, error) {

	if val.GetMetadata().SaleDirty && !val.GetMetadata().GetExpireSale() && val.GetMetadata().NewSalePrice > 0 &&
		(val.GetMetadata().Category == pb.ReleaseMetadata_LISTED_TO_SELL ||
			val.GetMetadata().Category == pb.ReleaseMetadata_STALE_SALE) {

		if len(val.GetRelease().RecordCondition) == 0 {
			s.RaiseIssue("Condition Issue", fmt.Sprintf("%v [%v] has no condition info", val.GetRelease().Title, val.GetRelease().Id))
			return false, fmt.Errorf("%v [%v/%v] has no condition info", val.GetRelease().Title, val.GetRelease().Id, val.GetRelease().InstanceId)
		}

		err := s.retr.UpdateSalePrice(ctx, int(val.GetMetadata().SaleId), int(val.GetRelease().Id), val.GetRelease().RecordCondition, val.GetRelease().SleeveCondition, float32(val.GetMetadata().NewSalePrice)/100)
		time.Sleep(time.Second * 5)
		s.CtxLog(ctx, fmt.Sprintf("Updated sale price: %v -> %v", val.GetRelease().GetInstanceId(), err))

		if err == nil {
			// Only trip the time if the price has actually changed
			if val.GetMetadata().GetSalePrice() != val.GetMetadata().GetNewSalePrice() {
				val.GetMetadata().LastSalePriceUpdate = time.Now().Unix()
			}
			val.GetMetadata().SaleDirty = false
			val.GetMetadata().SalePrice = val.GetMetadata().NewSalePrice
			val.GetMetadata().NewSalePrice = 0
			err = s.saveRecord(ctx, val)
		} else {
			// Unavailable is a valid response from a sales push, as is Failed precondition when we try and update a sold item
			if st, ok := status.FromError(err); !ok || (st.Code() != codes.Unavailable && st.Code() != codes.FailedPrecondition) {
				// Force a record refresh
				val.GetMetadata().LastUpdateTime = time.Now().Unix()
				s.RaiseIssue("Error pushing sale", fmt.Sprintf("Error on sale push for %v: %v", val.GetRelease().Id, err))
				return true, nil
			}
		}
		return true, err
	}

	if val.GetMetadata().GetExpireSale() && val.GetMetadata().GetSaleState() == pbd.SaleState_EXPIRED {
		val.GetMetadata().ExpireSale = false
		return true, s.saveRecord(ctx, val)
	}

	if val.GetMetadata().SaleDirty && val.GetMetadata().GetExpireSale() && (val.GetMetadata().GetSaleState() == pbd.SaleState_FOR_SALE || val.GetMetadata().GetSaleState() < 0) {
		err := s.retr.ExpireSale(ctx, int(val.GetMetadata().SaleId), int(val.GetRelease().Id), float32(val.GetMetadata().SalePrice+1)/100)
		val.GetMetadata().ExpireSale = err != nil
		if err == nil {
			val.GetMetadata().SaleState = pbd.SaleState_EXPIRED
			val.GetMetadata().SaleDirty = false
		}
		s.CtxLog(ctx, fmt.Sprintf("EXPIRE(%v): %v", val.GetRelease().GetInstanceId(), err))
		return true, err
	}

	if val.GetMetadata().Category == pb.ReleaseMetadata_SOLD_OFFLINE {
		err := s.retr.RemoveFromSale(ctx, int(val.GetMetadata().SaleId), int(val.GetRelease().Id))

		if err == nil || fmt.Sprintf("%v", err) == "POST ERROR (STATUS CODE): 404, {\"message\": \"Item not found. It may have been deleted.\"}" {
			val.GetMetadata().SaleState = pbd.SaleState_SOLD
			val.GetMetadata().SaleDirty = false
			val.GetMetadata().LastUpdateTime = time.Now().Unix()
		}
		return true, err
	}

	//Handle hanging clause
	if val.GetMetadata().GetExpireSale() && val.GetMetadata().GetSaleState() == pbd.SaleState_EXPIRED {
		val.GetMetadata().ExpireSale = false
	}

	//Nothing to do here
	val.GetMetadata().SaleDirty = false
	return false, nil
}

func (s *Server) updateWant(ctx context.Context, w *pb.Want) bool {
	if w.GetReleaseId() == 0 {
		return false
	}
	if w.ClearWant {
		s.CtxLog(ctx, fmt.Sprintf("Removing from the wantlist %v -> %v", w.GetReleaseId(), w.ClearWant))
		s.retr.RemoveFromWantlist(ctx, int(w.GetReleaseId()))
		w.ClearWant = false
		return true
	}

	return false
}

func (s *Server) pushRecord(ctx context.Context, r *pb.Record) (bool, error) {
	pushed := (r.GetMetadata().GetSetRating() > 0 && r.GetRelease().Rating != r.GetMetadata().GetSetRating()) || (r.GetMetadata().GetMoveFolder() > 0 && r.GetMetadata().GetMoveFolder() != r.GetRelease().FolderId)

	if r.GetMetadata().GetMoveFolder() > 0 {
		if r.GetMetadata().MoveFolder != r.GetRelease().FolderId {
			err := s.mover.moveRecord(ctx, r, r.GetRelease().FolderId, r.GetMetadata().GetMoveFolder())
			if r.GetRelease().FolderId != 1 && err != nil {
				return false, fmt.Errorf("Move fail %v -> %v: %v (%v)", r.GetRelease().FolderId, r.GetMetadata().GetMoveFolder(), err, ctx)
			}

			_, err = s.retr.MoveToFolder(ctx, int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId), int(r.GetMetadata().GetMoveFolder()))
			if err != nil {
				s.RaiseIssue("Move Failure", fmt.Sprintf("%v -> %v", r.GetRelease().GetInstanceId(), err))

				//We need to clear the move to allow it to change
				r.GetMetadata().MoveFolder = 0
				r.GetMetadata().Dirty = false
				s.saveRecord(ctx, r)

				return false, err
			}
			r.GetRelease().FolderId = r.GetMetadata().MoveFolder
			r.GetMetadata().LastMoveTime = time.Now().Unix()
		}
	}
	r.GetMetadata().MoveFolder = 0

	// Push the score
	if (r.GetMetadata().GetSetRating() > 0 || r.GetMetadata().GetSetRating() == -1) && r.GetRelease().Rating != r.GetMetadata().GetSetRating() {
		err := s.retr.SetRating(ctx, int(r.GetRelease().Id), max(0, int(r.GetMetadata().GetSetRating())))
		s.CtxLog(ctx, fmt.Sprintf("Attempting to set rating on %v: %v", r.GetRelease().InstanceId, err))
		r.GetRelease().Rating = int32(max(0, int(r.GetMetadata().SetRating)))
		if r.GetMetadata().GetSetRating() > 0 {
			r.GetMetadata().LastListenTime = time.Now().Unix()
		}
	}
	r.GetMetadata().SetRating = 0

	// Update the boxness
	if r.GetMetadata().GetNewBoxState() != pb.ReleaseMetadata_BOX_UNKNOWN &&
		r.GetMetadata().GetBoxState() != r.GetMetadata().GetNewBoxState() {
		r.GetMetadata().BoxState = r.GetMetadata().GetNewBoxState()
		r.GetMetadata().NewBoxState = pb.ReleaseMetadata_BOX_UNKNOWN

		r.GetMetadata().SaleId = 0
		r.GetMetadata().SaleState = pbd.SaleState_EXPIRED
		if r.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL ||
			r.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE ||
			r.GetMetadata().GetCategory() == pb.ReleaseMetadata_SOLD_ARCHIVE ||
			r.GetMetadata().GetCategory() == pb.ReleaseMetadata_PREPARE_TO_SELL {
			r.GetMetadata().Category = pb.ReleaseMetadata_PRE_IN_COLLECTION
		}
	}

	r.GetMetadata().Dirty = false

	//Ensure records get updated
	r.GetMetadata().LastUpdateTime = time.Now().Unix()
	return pushed, s.saveRecord(ctx, r)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Server) cacheRecord(ctx context.Context, r *pb.Record) error {
	s.CtxLog(ctx, fmt.Sprintf("Updating cache for : %v (%v)", r.GetRelease().GetTitle(), r.GetRelease().GetRecordCondition()))
	// Don't recache a record that has a pending score
	if r.GetMetadata().GetSetRating() > 0 {
		return nil
	}

	//Add the record if it has not instance ID
	if r.GetRelease().InstanceId == 0 {
		inst, err := s.retr.AddToFolder(ctx, r.GetRelease().FolderId, r.GetRelease().Id)
		if err == nil {
			r.GetRelease().InstanceId = int32(inst)
		} else {
			return err
		}
	}

	//Force a recache if the record has no title or condition; or if it has the old image format
	if time.Now().Unix()-r.GetMetadata().GetLastCache() > 60*60*24*30 || r.GetRelease().Title == "" ||
		(len(r.GetRelease().GetImages()) > 0 && strings.Contains(r.GetRelease().GetImages()[0].GetUri(), "img.discogs")) {
		release, err := s.retr.GetRelease(ctx, r.GetRelease().Id)
		s.CtxLog(ctx, fmt.Sprintf("Retreived release for re-cache: %v", err))
		if err == nil {

			//Clear repeated fields first
			r.GetRelease().Images = []*pbd.Image{}
			r.GetRelease().Artists = []*pbd.Artist{}
			r.GetRelease().Formats = []*pbd.Format{}
			r.GetRelease().Labels = []*pbd.Label{}
			r.GetRelease().Tracklist = []*pbd.Track{}
			r.GetRelease().DigitalVersions = []int32{}
			r.GetRelease().OtherVersions = []int32{}

			s.CtxLog(ctx, fmt.Sprintf("Merged %v", release))
			proto.Merge(r.GetRelease(), release)

			r.GetMetadata().LastCache = time.Now().Unix()
			r.GetMetadata().LastUpdateTime = time.Now().Unix()
		} else {
			return err
		}
	}

	// Re pull the date_added
	mp, err := s.retr.GetInstanceInfo(ctx, r.GetRelease().GetId())
	if err == nil && mp[r.GetRelease().GetInstanceId()] != nil {
		s.CtxLog(ctx, fmt.Sprintf("Updating info (%v): %+v", r.GetRelease().GetInstanceId(), mp[r.GetRelease().GetInstanceId()]))
		r.GetMetadata().DateAdded = mp[r.GetRelease().GetInstanceId()].DateAdded
		r.GetRelease().RecordCondition = mp[r.GetRelease().GetInstanceId()].RecordCondition
		r.GetRelease().SleeveCondition = mp[r.GetRelease().GetInstanceId()].SleeveCondition
		r.GetMetadata().LastInfoUpdate = time.Now().Unix()
	} else {
		return err
	}

	return s.saveRecord(ctx, r)
}

func (s *Server) syncRecords(ctx context.Context, r *pb.Record, record *pbd.Release, num int64) {
	//Update record if releases don't match
	hasCondition := len(r.GetRelease().RecordCondition) > 0

	//Clear repeated fields first to prevent growth, but images come from
	//a hard sync so ignore that
	if len(record.GetFormats()) > 0 {
		r.GetRelease().Formats = []*pbd.Format{}
		r.GetRelease().Artists = []*pbd.Artist{}
		r.GetRelease().Labels = []*pbd.Label{}
	}

	if len(record.GetImages()) > 0 {
		r.GetRelease().Images = []*pbd.Image{}
	}
	if len(record.GetTracklist()) > 0 {
		r.GetRelease().Tracklist = []*pbd.Track{}
	}

	proto.Merge(r.Release, record)

	// Set sale dirty if the condition is new
	if !hasCondition && len(r.Release.RecordCondition) > 0 {
		r.Metadata.SaleDirty = true
	}

	//Make a goal folder adjustment
	if r.GetRelease().GetFolderId() == 1782105 &&
		(r.GetMetadata().GetGoalFolder() == 0 || r.GetMetadata().GetGoalFolder() == 268147) {
		r.GetMetadata().GoalFolder = 1782105
	}

	s.saveRecord(ctx, r)
}

func (s *Server) syncCollection(ctx context.Context, colNumber int64) error {
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return err
	}
	records := s.retr.GetCollection(ctx)
	for _, record := range records {
		foundInList := false
		for iid := range collection.InstanceToFolder {
			if iid == record.InstanceId {
				foundInList = true
				r, err := s.loadRecord(ctx, record.InstanceId, false)
				if err == nil {
					s.syncRecords(ctx, r, record, colNumber)
				} else {
					// If we can't find the record, need to resync
					if status.Convert(err).Code() == codes.NotFound {
						foundInList = false
					} else {
						return err
					}
				}
			}
		}

		if !foundInList {
			nrec := &pb.Record{Release: record, Metadata: &pb.ReleaseMetadata{DateAdded: time.Now().Unix(), GoalFolder: record.FolderId}}
			s.saveRecord(ctx, nrec)
		}

	}

	return s.saveRecordCollection(ctx, collection)
}

func (s *Server) updateSale(ctx context.Context, iid int32) error {
	r, err := s.loadRecord(ctx, iid, false)
	if err == nil {
		if r.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL || r.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE {
			if r.GetMetadata().SaleId > 1 && !r.GetMetadata().SaleDirty {
				r.GetMetadata().SalePrice = int32(s.retr.GetCurrentSalePrice(ctx, int(r.GetMetadata().SaleId)) * 100)
			}
			if r.GetMetadata().SaleId > 1 && r.GetMetadata().SaleState != pbd.SaleState_SOLD {
				r.GetMetadata().SaleState = s.retr.GetCurrentSaleState(ctx, int(r.GetMetadata().SaleId))
			}
			return s.saveRecord(ctx, r)
		}
	}
	return err
}

func (s *Server) syncWantlist(ctx context.Context) error {
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return err
	}

	wants, err := s.retr.GetWantlist(ctx)
	if err != nil {
		return err
	}

	for _, want := range wants {

		found := false
		for _, w := range collection.GetNewWants() {
			if w.GetReleaseId() == want.Id {
				found = true
			}
		}

		if !found {
			collection.NewWants = append(collection.NewWants, &pb.Want{ReleaseId: want.GetId()})
		}
	}

	var nw []*pb.Want
	for _, w := range collection.GetNewWants() {
		found := false
		for _, want := range wants {
			if w.GetReleaseId() == want.Id {
				found = true
			}
		}

		if found {
			nw = append(nw, w)
		}
	}
	collection.NewWants = nw

	return s.saveRecordCollection(ctx, collection)
}

func (s *Server) runSyncWants(ctx context.Context) error {
	return s.syncWantlist(ctx)
}

func (s *Server) runSync(ctx context.Context) error {
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return err
	}
	err = s.syncCollection(ctx, collection.CollectionNumber+1)
	collection.CollectionNumber++
	s.saveRecordCollection(ctx, collection)
	return err
}

func (s *Server) pushMetadata(ctx context.Context, record *pb.Record) error {
	info := &pb.StoredMetadata{
		Width: int32(record.GetMetadata().GetRecordWidth()),
	}
	str, _ := proto.Marshal(info)
	fmt.Sprintf("%v", str)
	return nil
	//return s.retr.AddNotes(ctx, record.GetRelease().GetInstanceId(), string(str))
}

func (s *Server) recache(ctx context.Context, r *pb.Record) error {
	// Don't recache a record that has a pending score
	if r.GetMetadata().GetSetRating() > 0 || r.GetMetadata().Dirty {
		return fmt.Errorf("%v has pending score or is dirty", r.GetRelease().InstanceId)
	}

	// Update the score of the record
	sc, err := s.scorer.GetScore(ctx, r.GetRelease().InstanceId)
	if err == nil {
		r.GetMetadata().OverallScore = sc
	}

	//Force a recache if the record has no title
	release, err := s.retr.GetRelease(ctx, r.GetRelease().Id)
	if err == nil {

		//Clear repeated fields first
		r.GetRelease().Images = []*pbd.Image{}
		r.GetRelease().Artists = []*pbd.Artist{}
		r.GetRelease().Formats = []*pbd.Format{}
		r.GetRelease().Labels = []*pbd.Label{}
		r.GetRelease().Tracklist = []*pbd.Track{}

		proto.Merge(r.GetRelease(), release)

		r.GetMetadata().LastCache = time.Now().Unix()
		r.GetMetadata().LastUpdateTime = time.Now().Unix()
	}

	return nil
}
