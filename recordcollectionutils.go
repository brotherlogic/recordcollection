package main

import (
	"fmt"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver/utils"
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

func (s *Server) syncIssue(ctx context.Context) error {
	for _, r := range s.collection.GetRecords() {
		if time.Now().Sub(time.Unix(r.GetMetadata().LastSyncTime, 0)) > time.Hour*24*7 && time.Now().Sub(time.Unix(r.GetMetadata().DateAdded, 0)) > time.Hour*24*7 {
			s.RaiseIssue(ctx, "Sync Issue", fmt.Sprintf("%v [%v] hasn't synced in a week!", r.GetRelease().Title, r.GetRelease().InstanceId), false)
		}
	}
	return nil
}

func (s *Server) pushSale(ctx context.Context, val *pb.Record) (bool, error) {
	if val.GetMetadata().SaleDirty &&
		(val.GetMetadata().Category == pb.ReleaseMetadata_LISTED_TO_SELL ||
			val.GetMetadata().Category == pb.ReleaseMetadata_STALE_SALE) {

		// Adjust sale price if needed
		if val.GetMetadata().SalePrice == 0 {
			val.GetMetadata().SalePrice = val.GetMetadata().CurrentSalePrice
		}
		if len(val.GetRelease().RecordCondition) == 0 {
			s.RaiseIssue(ctx, "Condition Issue", fmt.Sprintf("%v [%v] has no condition info", val.GetRelease().Title, val.GetRelease().Id), false)
			return false, fmt.Errorf("%v [%v/%v] has no condition info", val.GetRelease().Title, val.GetRelease().Id, val.GetRelease().InstanceId)
		}

		s.lastSale = int64(val.GetRelease().InstanceId)
		s.salesPushes++
		err := s.retr.UpdateSalePrice(int(val.GetMetadata().SaleId), int(val.GetRelease().Id), val.GetRelease().RecordCondition, val.GetRelease().SleeveCondition, float32(val.GetMetadata().SalePrice)/100)
		if err == nil {
			val.GetMetadata().SaleDirty = false
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
		}
		return true, err
	}

	return false, nil
}

func (s *Server) pushSales(ctx context.Context) error {
	s.lastSalePush = time.Now()
	for _, val := range s.collection.GetRecords() {
		success, err := s.pushSale(ctx, val)
		if err != nil {
			return fmt.Errorf("Error pushing %v (%v): %v", val.GetRelease().InstanceId, val.GetMetadata().Category, err)
		}
		if success {
			return nil
		}
	}
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

	s.saveRecordCollection(ctx)
	return nil
}

func (s *Server) runPush(ctx context.Context) error {
	s.lastPushTime = time.Now()
	s.lastPushSize = len(s.pushMap)
	s.lastPushDone = 0
	save := len(s.pushMap) > 0
	for key, val := range s.pushMap {
		pushed, resp := s.pushRecord(ctx, val)
		s.pushMutex.Lock()
		delete(s.pushMap, key)
		s.pushMutex.Unlock()
		s.lastPushDone++

		if pushed {
			break
		} else {
			val.GetMetadata().MoveFailure = resp
		}
	}
	if save {
		s.saveRecordCollection(ctx)
	}

	s.nextPush = nil
	for _, val := range s.pushMap {
		s.nextPush = val
		break
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

func (s *Server) pushRecord(ctx context.Context, r *pb.Record) (bool, string) {
	pushed := (r.GetMetadata().GetSetRating() > 0 && r.GetRelease().Rating != r.GetMetadata().GetSetRating()) || (r.GetMetadata().GetMoveFolder() > 0 && r.GetMetadata().GetMoveFolder() != r.GetRelease().FolderId)

	if r.GetMetadata().GetMoveFolder() > 0 {
		if r.GetMetadata().MoveFolder != r.GetRelease().FolderId {
			err := s.mover.moveRecord(r, r.GetRelease().FolderId, r.GetMetadata().GetMoveFolder())
			if r.GetRelease().FolderId != 1 && err != nil {
				return false, fmt.Sprintf("Move fail %v -> %v: %v (%v)", r.GetRelease().FolderId, r.GetMetadata().GetMoveFolder(), err, ctx)
			}

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
	return pushed, ""
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
	if time.Now().Unix()-r.GetMetadata().GetLastCache() > 60*60*24*30 || r.GetRelease().Title == "" || len(r.GetRelease().GetFormats()) == 0 {
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
		}
	}

	s.saveRecordCollection(ctx)
}

func (s *Server) syncRecords(r *pb.Record, record *pbd.Release, num int64) {
	//Update record if releases don't match
	if !utils.FuzzyMatch(r.GetRelease(), record) {
		s.Log(fmt.Sprintf("Release mismatch"))
	}

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

	// Override if the rating doesn't match
	if r.Release.Rating != record.Rating {
		r.Release.Rating = record.Rating
	}

	// Records with others don't need to be stock checked
	if r.GetMetadata().Others {
		r.GetMetadata().NeedsStockCheck = false
	}
	if time.Now().Sub(time.Unix(r.GetMetadata().LastStockCheck, 0)) < time.Hour*24*30*6 {
		r.GetMetadata().NeedsStockCheck = false
	}

	r.GetMetadata().LastSyncTime = time.Now().Unix()

}

func (s *Server) syncCollection(ctx context.Context, colNumber int64) {
	startTime := time.Now()
	records := s.retr.GetCollection()
	for _, record := range records {
		found := false
		for _, r := range s.collection.GetRecords() {
			if r.GetRelease().InstanceId == record.InstanceId {
				found = true
				s.syncRecords(r, record, colNumber)
			}
		}

		if !found {
			s.collection.Records = append(s.collection.Records, &pb.Record{Release: record, Metadata: &pb.ReleaseMetadata{DateAdded: time.Now().Unix()}})
		}
	}

	otherMap := make(map[int32]int32)
	for _, r := range s.collection.Records {
		if r.GetRelease().MasterId > 0 {
			if _, ok := otherMap[r.GetRelease().MasterId]; !ok {
				otherMap[r.GetRelease().MasterId] = 1
			} else {
				otherMap[r.GetRelease().MasterId] = 2
			}
		}
	}
	for _, r := range s.collection.Records {
		if otherMap[r.GetRelease().MasterId] > 1 {
			r.GetMetadata().Others = true
		} else {
			r.GetMetadata().Others = false
		}
	}

	// Update sale info
	for _, r := range s.collection.Records {
		if r.GetMetadata().SaleId > 0 && !r.GetMetadata().SaleDirty {
			r.GetMetadata().SalePrice = int32(s.retr.GetCurrentSalePrice(int(r.GetMetadata().SaleId)) * 100)
		}
		if r.GetMetadata().SaleId > 0 && r.GetMetadata().SaleState != pbd.SaleState_SOLD {
			r.GetMetadata().SaleState = s.retr.GetCurrentSaleState(int(r.GetMetadata().SaleId))

		}
	}

	s.lastSyncTime = time.Now()
	s.lastSyncLength = time.Now().Sub(startTime)
	s.saveRecordCollection(ctx)
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
	s.syncCollection(ctx, s.collection.CollectionNumber+1)
	s.collection.CollectionNumber++
	s.saveRecordCollection(ctx)
	return nil
}
