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

func (s *Server) syncIssue(ctx context.Context) error {
	for _, r := range s.collection.GetRecords() {
		if time.Now().Sub(time.Unix(r.GetMetadata().LastSyncTime, 0)) > time.Hour*24*7 && time.Now().Sub(time.Unix(r.GetMetadata().DateAdded, 0)) > time.Hour*24*7 {
			s.RaiseIssue(ctx, "Sync Issue", fmt.Sprintf("%v hasn't synced in a week!", r.GetRelease().Title), false)
		}
	}
	return nil
}

func (s *Server) pushSales(ctx context.Context) error {
	s.lastSalePush = time.Now()
	for _, val := range s.saleMap {
		if val.GetMetadata().SaleDirty && val.GetMetadata().Category == pb.ReleaseMetadata_LISTED_TO_SELL {
			s.salesPushes++
			err := s.retr.UpdateSalePrice(int(val.GetMetadata().SaleId), int(val.GetRelease().Id), "Very Good Plus (VG+)", float32(val.GetMetadata().SalePrice)/100)
			if err == nil {
				val.GetMetadata().SaleDirty = false
				break
			}
		}

		if val.GetMetadata().Category == pb.ReleaseMetadata_SOLD_OFFLINE {
			s.soldAdjust++
			err := s.retr.RemoveFromSale(int(val.GetMetadata().SaleId), int(val.GetRelease().Id))

			if err == nil {
				val.GetMetadata().SaleState = pbd.SaleState_SOLD
				val.GetMetadata().SaleDirty = false
				break
			}
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
			//Check that we can move this record
			val, err := s.quota.hasQuota(ctx, r.GetMetadata().GetMoveFolder())

			if err != nil {
				e, ok := status.FromError(err)
				if ok && e.Code() == codes.InvalidArgument {
					s.RaiseIssue(context.Background(), "Quota Problem", fmt.Sprintf("Error getting quota: %v for %v", err, r.GetRelease().Id), false)
				}
				return false, fmt.Sprintf("No Quota: %v", err)
			}

			if val.GetOverQuota() {
				if val.SpillFolder > 0 {
					r.GetMetadata().MoveFolder = val.SpillFolder
				} else {
					return false, "Over Quota"
				}
			}
		}

		if r.GetMetadata().MoveFolder != r.GetRelease().FolderId {
			err := s.mover.moveRecord(r, r.GetRelease().FolderId, r.GetMetadata().GetMoveFolder())
			if err != nil {
				return false, fmt.Sprintf("Move fail: %v (%v)", err, ctx)
			}

			s.retr.MoveToFolder(int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId), int(r.GetMetadata().GetMoveFolder()))
			r.GetRelease().FolderId = r.GetMetadata().MoveFolder
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
	if r.GetMetadata() == nil {
		r.Metadata = &pb.ReleaseMetadata{}
	}

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

func (s *Server) syncCollection(ctx context.Context) {
	startTime := time.Now()
	records := s.retr.GetCollection()
	for _, record := range records {
		found := false
		for _, r := range s.collection.GetRecords() {
			if r.GetRelease().InstanceId == record.InstanceId {
				found = true

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

				// Override if the rating doesn't match
				if r.Release.Rating != record.Rating {
					r.Release.Rating = record.Rating
				}

				r.GetMetadata().LastSyncTime = time.Now().Unix()
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
		if r.GetMetadata().SaleId > 0 && r.GetMetadata().SalePrice == 0 && !r.GetMetadata().SaleDirty {
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
	s.syncCollection(ctx)
	s.saveRecordCollection(ctx)
	return nil
}
