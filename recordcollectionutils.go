package main

import (
	"fmt"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
)

func (s *Server) runUpdateFanout() {
	for id := range s.updateFanout {
		s.repeatCount[id]++
		if s.repeatCount[id] > 10 {
			s.RaiseIssue(fmt.Sprintf("%v cannot be updated", id), fmt.Sprintf("Last error was %v", s.repeatError[id]))
		}

		s.Log(fmt.Sprintf("Running election for %v", id))
		time.Sleep(time.Second * 2)
		ecancel, err := s.ElectKey(fmt.Sprintf("%v", id))

		s.Log(fmt.Sprintf("Elected: %v, %v -> %v", err, id, s.fanoutServers))
		time.Sleep(time.Second * 2)

		if err != nil {
			s.repeatError[id] = err
			s.Log(fmt.Sprintf("Unable to elect: %v", err))
			updateFanoutFailure.With(prometheus.Labels{"server": "elect", "error": fmt.Sprintf("%v", err)}).Inc()
			s.updateFanout <- id
			ecancel()
			time.Sleep(time.Minute)
			continue
		}

		ctx, cancel := utils.ManualContext("rciu", "rciu", time.Minute, true)
		record, err := s.loadRecord(ctx, id)
		if err != nil {
			s.repeatError[id] = err
			s.Log(fmt.Sprintf("Unable to load: %v", err))
			updateFanoutFailure.With(prometheus.Labels{"server": "load", "error": fmt.Sprintf("%v", err)}).Inc()
			s.updateFanout <- id
			ecancel()
			time.Sleep(time.Minute)
			continue
		}

		// Perform a discogs update if needed
		if time.Now().Sub(time.Unix(record.GetMetadata().GetLastCache(), 0)) > time.Hour*24*30 ||
			time.Now().Sub(time.Unix(record.GetMetadata().GetLastInfoUpdate(), 0)) > time.Hour*24*30 {
			s.cacheRecord(ctx, record)
		}
		cancel()

		// Finally push the record if we need to
		if record.GetMetadata().GetDirty() {
			ctx, cancel := utils.ManualContext("rciu", "rciu", time.Minute, true)
			_, err = s.pushRecord(ctx, record)
			cancel()
			if err != nil {
				s.repeatError[id] = err
				s.Log(fmt.Sprintf("Unable to push: %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "push", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- id
				ecancel()
				time.Sleep(time.Minute)
				continue
			}
		}

		// Update the sale
		if record.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL {
			ctx, cancel := utils.ManualContext("rcu", "rcu", time.Minute, true)
			err := s.updateSale(ctx, record.GetRelease().GetInstanceId())
			if err == nil {
				record, err = s.loadRecord(ctx, id)
			}
			cancel()

			if err != nil {
				s.repeatError[id] = err
				s.Log(fmt.Sprintf("Unable to update record for sale: %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "updateSale", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- id
				ecancel()
				time.Sleep(time.Minute)
				continue
			}

		}

		// Push the sale
		if record.GetMetadata().GetSaleDirty() && record.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL {
			ctx, cancel := utils.ManualContext("rciu", "rciu", time.Minute, true)
			_, err = s.pushSale(ctx, record)
			cancel()
			time.Sleep(time.Second * 5)
			if err != nil {
				s.repeatError[id] = err
				s.Log(fmt.Sprintf("Unable to push sale : %v", err))
				updateFanoutFailure.With(prometheus.Labels{"server": "pushSale", "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- id
			}
		}

		for _, server := range s.fanoutServers {
			ctx, cancel := utils.ManualContext("rcfo", "rcfo", time.Minute, true)
			conn, err := s.FDialServer(ctx, server)

			if err != nil {
				s.repeatError[id] = err
				s.Log(fmt.Sprintf("Bad dial of %v -> %v", server, err))
				updateFanoutFailure.With(prometheus.Labels{"server": server, "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- id
				break
			}

			client := pb.NewClientUpdateServiceClient(conn)
			_, err = client.ClientUpdate(ctx, &pb.ClientUpdateRequest{InstanceId: id})
			if err != nil {
				s.repeatError[id] = err
				s.Log(fmt.Sprintf("Bad update of %v -> %v", server, err))
				updateFanoutFailure.With(prometheus.Labels{"server": server, "error": fmt.Sprintf("%v", err)}).Inc()
				s.updateFanout <- id
				conn.Close()
				break
			}

			conn.Close()
			cancel()
		}

		ecancel()
		updateFanout.Set(float64(len(s.updateFanout)))
		updateFanoutFailure.With(prometheus.Labels{"server": "none", "error": "nil"}).Inc()
		time.Sleep(time.Minute)
	}
}

func (s *Server) validateSales(ctx context.Context) error {
	sales, err := s.retr.GetInventory()
	if err != nil {
		return err
	}

	s.Log(fmt.Sprintf("Found %v sales", len(sales)))
	matchCount := 0
	for _, sale := range sales {
		found := false

		// This call will not fail
		recs, _ := s.QueryRecords(ctx, &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_ReleaseId{sale.GetId()}})

		for _, id := range recs.GetInstanceIds() {
			rec, err := s.getRecord(ctx, id)
			if err != nil {
				s.Log(fmt.Sprintf("Err: %v", err))
				return err
			}

			if (rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL || rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE) && rec.GetMetadata().GetSaleId() == sale.GetSaleId() {
				found = true
			}
		}

		if !found {
			s.Log(fmt.Sprintf("Sending off problem"))
			s.RaiseIssue("Sale Error Found", fmt.Sprintf("%v is not found in collection", sale))
			return fmt.Errorf("Found a sale problem")
		}
		matchCount++
	}
	s.Log(fmt.Sprintf("Matched %v", matchCount))

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

		err := s.retr.UpdateSalePrice(int(val.GetMetadata().SaleId), int(val.GetRelease().Id), val.GetRelease().RecordCondition, val.GetRelease().SleeveCondition, float32(val.GetMetadata().NewSalePrice)/100)
		time.Sleep(time.Second * 5)
		s.Log(fmt.Sprintf("Updated sale price: %v -> %v", val.GetRelease().GetInstanceId(), err))

		if err == nil {
			val.GetMetadata().SaleDirty = false
			val.GetMetadata().SalePrice = val.GetMetadata().NewSalePrice
			val.GetMetadata().NewSalePrice = 0
			val.GetMetadata().LastSalePriceUpdate = time.Now().Unix()
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

	if val.GetMetadata().SaleDirty && val.GetMetadata().GetExpireSale() && (val.GetMetadata().GetSaleState() == pbd.SaleState_FOR_SALE || val.GetMetadata().GetSaleState() < 0) {
		err := s.retr.ExpireSale(int(val.GetMetadata().SaleId), int(val.GetRelease().Id), float32(val.GetMetadata().SalePrice+1)/100)
		val.GetMetadata().ExpireSale = err != nil
		if err == nil {
			val.GetMetadata().SaleState = pbd.SaleState_EXPIRED
			val.GetMetadata().SaleDirty = false
		}
		s.Log(fmt.Sprintf("EXPIRE(%v): %v", val.GetRelease().GetInstanceId(), err))
		return true, err
	}

	if val.GetMetadata().Category == pb.ReleaseMetadata_SOLD_OFFLINE {
		err := s.retr.RemoveFromSale(int(val.GetMetadata().SaleId), int(val.GetRelease().Id))

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

func (s *Server) updateWant(w *pb.Want) bool {
	if w.ClearWant {
		s.Log(fmt.Sprintf("Removing from wantlist %v -> %v and %v", w.GetRelease().Id, w.ClearWant, w.GetMetadata().Active))
		s.retr.RemoveFromWantlist(int(w.GetRelease().Id))
		w.ClearWant = false
		w.GetMetadata().Active = false
		return true
	}

	if w.GetMetadata().Active {
		s.retr.AddToWantlist(int(w.GetRelease().Id))
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

			_, err = s.retr.MoveToFolder(int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId), int(r.GetMetadata().GetMoveFolder()))
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
		err := s.retr.SetRating(int(r.GetRelease().Id), max(0, int(r.GetMetadata().GetSetRating())))
		s.Log(fmt.Sprintf("Attempting to set rating on %v: %v", r.GetRelease().InstanceId, err))
		r.GetRelease().Rating = int32(max(0, int(r.GetMetadata().SetRating)))
		if r.GetMetadata().GetSetRating() > 0 {
			r.GetMetadata().LastListenTime = time.Now().Unix()
		}
	}
	r.GetMetadata().SetRating = 0

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

func (s *Server) cacheRecord(ctx context.Context, r *pb.Record) {
	// Don't recache a record that has a pending score
	if r.GetMetadata().GetSetRating() > 0 {
		return
	}

	//Add the record if it has not instance ID
	if r.GetRelease().InstanceId == 0 {
		inst, err := s.retr.AddToFolder(r.GetRelease().FolderId, r.GetRelease().Id)
		if err == nil {
			r.GetRelease().InstanceId = int32(inst)
		}
	}

	// Update the score of the record
	sc, err := s.scorer.GetScore(ctx, r.GetRelease().InstanceId)
	if err == nil {
		r.GetMetadata().OverallScore = sc
	}

	//Force a recache if the record has no title
	if time.Now().Unix()-r.GetMetadata().GetLastCache() > 60*60*24*30 || r.GetRelease().Title == "" {
		release, err := s.retr.GetRelease(r.GetRelease().Id)
		s.Log(fmt.Sprintf("%v leads to %v", release.Id, len(release.Tracklist)))
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
	}

	// Re pull the date_added
	mp, err := s.retr.GetInstanceInfo(r.GetRelease().GetId())
	if err == nil {
		r.GetMetadata().DateAdded = mp[r.GetRelease().GetInstanceId()]
		r.GetMetadata().LastInfoUpdate = time.Now().Unix()
	}

	s.saveRecord(ctx, r)
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

	// Records with others don't need to be stock checked
	if time.Now().Sub(time.Unix(r.GetMetadata().LastStockCheck, 0)) < time.Hour*24*30*6 || r.GetMetadata().Others {
		r.GetMetadata().NeedsStockCheck = false
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
	records := s.retr.GetCollection()
	for _, record := range records {
		foundInList := false
		for iid := range collection.InstanceToFolder {
			if iid == record.InstanceId {
				foundInList = true
				r, err := s.loadRecord(ctx, record.InstanceId)
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
	r, err := s.loadRecord(ctx, iid)
	if err == nil {
		if r.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL || r.GetMetadata().GetCategory() == pb.ReleaseMetadata_STALE_SALE {
			if r.GetMetadata().SaleId > 0 && !r.GetMetadata().SaleDirty {
				r.GetMetadata().SalePrice = int32(s.retr.GetCurrentSalePrice(int(r.GetMetadata().SaleId)) * 100)
			}
			if r.GetMetadata().SaleId > 0 && r.GetMetadata().SaleState != pbd.SaleState_SOLD {
				r.GetMetadata().SaleState = s.retr.GetCurrentSaleState(int(r.GetMetadata().SaleId))
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

	wants, _ := s.retr.GetWantlist()

	for _, want := range wants {
		found := false
		for _, w := range collection.GetNewWants() {
			if w.GetRelease().Id == want.Id {
				found = true
				proto.Merge(w.GetRelease(), want)
				w.GetMetadata().Active = true
			}
		}
		if !found {
			collection.NewWants = append(collection.NewWants, &pb.Want{Release: want, Metadata: &pb.WantMetadata{Active: true}})
		}
	}

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
	release, err := s.retr.GetRelease(r.GetRelease().Id)
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
