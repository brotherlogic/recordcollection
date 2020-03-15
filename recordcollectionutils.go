package main

import (
	"fmt"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// RecacheDelay - recache everything every 30 days
	RecacheDelay = 60 * 60 * 24 * 30
)

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

			if rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_LISTED_TO_SELL && rec.GetMetadata().GetSaleId() == sale.GetSaleId() {
				matchCount++
				found = true
			}
		}

		if !found {
			s.Log(fmt.Sprintf("Sending off problem"))
			s.RaiseIssue(ctx, "Sale Error Found", fmt.Sprintf("%v is not found in collection", sale), false)
		}
	}
	s.Log(fmt.Sprintf("Matched %v", matchCount))

	return nil
}

func (s *Server) pushSale(ctx context.Context, val *pb.Record) (bool, error) {
	if val.GetMetadata().SaleDirty &&
		val.GetMetadata().NewSalePrice > 0 &&
		(val.GetMetadata().Category == pb.ReleaseMetadata_LISTED_TO_SELL ||
			val.GetMetadata().Category == pb.ReleaseMetadata_STALE_SALE) {

		if len(val.GetRelease().RecordCondition) == 0 {
			s.RaiseIssue(ctx, "Condition Issue", fmt.Sprintf("%v [%v] has no condition info", val.GetRelease().Title, val.GetRelease().Id), false)
			return false, fmt.Errorf("%v [%v/%v] has no condition info", val.GetRelease().Title, val.GetRelease().Id, val.GetRelease().InstanceId)
		}

		s.lastSale = int64(val.GetRelease().InstanceId)
		s.salesPushes++
		err := s.retr.UpdateSalePrice(int(val.GetMetadata().SaleId), int(val.GetRelease().Id), val.GetRelease().RecordCondition, val.GetRelease().SleeveCondition, float32(val.GetMetadata().NewSalePrice)/100)
		if err == nil {
			val.GetMetadata().SaleDirty = false
			val.GetMetadata().NewSalePrice = 0
			val.GetMetadata().LastSalePriceUpdate = time.Now().Unix()
		} else {
			// Unavailable is a valid response from a sales push
			if st, ok := status.FromError(err); !ok || st.Code() != codes.Unavailable {
				s.RaiseIssue(ctx, "Error pushing sale", fmt.Sprintf("Error on sale push for %v: %v", val.GetRelease().Id, err), false)
				return false, fmt.Errorf("PUSH ERROR FOR %v -> %v", val.GetRelease().Id, err)
			}
		}
		return true, err
	}

	if val.GetMetadata().Category == pb.ReleaseMetadata_SOLD_OFFLINE {
		s.soldAdjust++
		err := s.retr.RemoveFromSale(int(val.GetMetadata().SaleId), int(val.GetRelease().Id))

		if err == nil || fmt.Sprintf("%v", err) == "POST ERROR (STATUS CODE): 404, {\"message\": \"Item not found. It may have been deleted.\"}" {
			val.GetMetadata().SaleState = pbd.SaleState_SOLD
			val.GetMetadata().SaleDirty = false
			val.GetMetadata().LastUpdateTime = time.Now().Unix()
		}
		return true, err
	}

	//Nothing to do here
	val.GetMetadata().SaleDirty = false
	return false, nil
}

func (s *Server) pushSales(ctx context.Context) error {
	s.lastSalePush = time.Now()
	doneID := int32(-1)
	for _, id := range s.collection.SaleUpdates {
		val, err := s.loadRecord(ctx, id)
		if err != nil {
			return err
		}
		success, err := s.pushSale(ctx, val)
		s.Log(fmt.Sprintf("SALE PUSH %v -> %v, %v", id, success, err))
		if err != nil {
			return fmt.Errorf("Error pushing %v (%v): %v", val.GetRelease().InstanceId, val.GetMetadata().Category, err)
		}
		if success {
			s.saveRecord(ctx, val)
		}

		doneID = id
		break
	}

	sales := []int32{}
	for _, v := range s.collection.SaleUpdates {
		if doneID != v {
			sales = append(sales, v)
		}
	}
	s.collection.SaleUpdates = sales
	s.saveRecordCollection(ctx)
	return nil
}

func (s *Server) pushWants(ctx context.Context) error {
	for _, w := range s.collection.NewWants {
		if w.GetMetadata().Active {
			s.wantCheck = fmt.Sprintf("%v", w)
			if s.updateWant(w) {
				s.lastWantText = fmt.Sprintf("%v", w)
				s.lastWantUpdate = w.GetRelease().Id
				break
			}
		}
	}

	return s.saveRecordCollection(ctx)
}

func (s *Server) runPush(ctx context.Context) error {
	s.lastPushTime = time.Now()
	s.lastPushSize = len(s.collection.NeedsPush)
	s.lastPushDone = 0
	if len(s.collection.NeedsPush) > 0 {
		id := s.collection.NeedsPush[0]
		s.Log(fmt.Sprintf("Pushing %v", id))
		val, err := s.getRecord(ctx, id)
		if err != nil {
			return err
		}
		_, err = s.pushRecord(ctx, val)
		if err != nil {
			return err
		}
		s.lastPushDone++

		newPush := []int32{}
		for _, v := range s.collection.NeedsPush {
			if v != val.GetRelease().InstanceId {
				newPush = append(newPush, v)
			}
		}

		s.collection.NeedsPush = newPush
		s.saveRecordCollection(ctx)
	}

	s.lastPushLength = time.Now().Sub(s.lastPushTime)
	return nil
}

func (s *Server) updateWant(w *pb.Want) bool {
	if w.GetRelease().Id == 766489 {
		s.wantUpdate = fmt.Sprintf("%v and %v", w.ClearWant, w.GetMetadata().Active)
	}
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
			s.Log(fmt.Sprintf("MOVED: %v", r))

			s.retr.MoveToFolder(int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId), int(r.GetMetadata().GetMoveFolder()))
			r.GetRelease().FolderId = r.GetMetadata().MoveFolder
			r.GetMetadata().LastMoveTime = time.Now().Unix()
		}
	}
	r.GetMetadata().MoveFolder = 0

	// Push the score
	if (r.GetMetadata().GetSetRating() > 0 || r.GetMetadata().GetSetRating() == -1) && r.GetRelease().Rating != r.GetMetadata().GetSetRating() {
		s.retr.SetRating(int(r.GetRelease().Id), max(0, int(r.GetMetadata().GetSetRating())))
		r.GetRelease().Rating = int32(max(0, int(r.GetMetadata().SetRating)))
		r.GetMetadata().LastListenTime = time.Now().Unix()
	}
	r.GetMetadata().SetRating = 0

	r.GetMetadata().Dirty = false

	//Ensure records get updated
	r.GetMetadata().LastUpdateTime = time.Now().Unix()
	s.saveRecord(ctx, r)
	return pushed, nil
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
			s.saveRecordCollection(ctx)
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

	s.saveRecord(ctx, r)
	s.saveRecordCollection(ctx)
}

func (s *Server) syncRecords(ctx context.Context, r *pb.Record, record *pbd.Release, num int64) {
	//Update record if releases don't match

	s.collectionMutex.Lock()
	s.collection.InstanceToFolder[record.InstanceId] = record.FolderId
	s.collectionMutex.Unlock()

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

	s.saveRecord(ctx, r)
}

func (s *Server) syncCollection(ctx context.Context, colNumber int64) error {
	startTime := time.Now()
	records := s.retr.GetCollection()
	for _, record := range records {
		foundInList := false
		s.collectionMutex.Lock()
		for iid := range s.collection.InstanceToFolder {
			if iid == record.InstanceId {
				foundInList = true
				s.collectionMutex.Unlock()
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
				s.collectionMutex.Lock()
			}
		}

		if !foundInList {
			nrec := &pb.Record{Release: record, Metadata: &pb.ReleaseMetadata{DateAdded: time.Now().Unix(), GoalFolder: record.FolderId}}
			s.collectionMutex.Unlock()
			s.saveRecord(ctx, nrec)
			s.collectionMutex.Lock()
		}

		s.collectionMutex.Unlock()
	}

	// Update sale info
	for iid, category := range s.collection.InstanceToCategory {
		s.updateSale(ctx, iid, category)
	}

	s.lastSyncTime = time.Now()
	s.lastSyncLength = time.Now().Sub(startTime)
	return s.saveRecordCollection(ctx)
}

func (s *Server) updateSale(ctx context.Context, iid int32, category pb.ReleaseMetadata_Category) {
	if category == pb.ReleaseMetadata_LISTED_TO_SELL {
		r, err := s.loadRecord(ctx, iid)
		if err == nil {
			if r.GetMetadata().SaleId > 0 && !r.GetMetadata().SaleDirty {
				r.GetMetadata().SalePrice = int32(s.retr.GetCurrentSalePrice(int(r.GetMetadata().SaleId)) * 100)
			}
			if r.GetMetadata().SaleId > 0 && r.GetMetadata().SaleState != pbd.SaleState_SOLD {
				r.GetMetadata().SaleState = s.retr.GetCurrentSaleState(int(r.GetMetadata().SaleId))
			}
			s.saveRecord(ctx, r)
		}
	}
}

func (s *Server) syncWantlist() {
	wants, _ := s.retr.GetWantlist()

	for _, want := range wants {
		found := false
		for _, w := range s.collection.GetNewWants() {
			if w.GetRelease().Id == want.Id {
				found = true
				proto.Merge(w.GetRelease(), want)
				w.GetMetadata().Active = true
			}
		}
		if !found {

			s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: want, Metadata: &pb.WantMetadata{Active: true}})
		}
	}
}

func (s *Server) runSyncWants(ctx context.Context) error {
	s.syncWantlist()
	s.saveRecordCollection(ctx)
	return nil
}

func (s *Server) runSync(ctx context.Context) error {
	err := s.syncCollection(ctx, s.collection.CollectionNumber+1)
	s.collection.CollectionNumber++
	s.saveRecordCollection(ctx)
	return err
}

func (s *Server) recache(ctx context.Context, r *pb.Record) error {
	// Don't recache a record that has a pending score
	if r.GetMetadata().GetSetRating() > 0 || r.GetMetadata().Dirty {
		s.collection.NeedsPush = append(s.collection.NeedsPush, r.GetRelease().InstanceId)
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
