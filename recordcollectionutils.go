package main

import (
	"fmt"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

const (
	// RecacheDelay - recache everything every 30 days
	RecacheDelay = 60 * 60 * 24 * 30
)

func (s *Server) runPush() {
	s.lastPushTime = time.Now()
	s.lastPushSize = len(s.pushMap)
	s.lastPushDone = 0
	save := len(s.pushMap) > 0
	for key, val := range s.pushMap {
		pushed := s.pushRecord(val)
		s.pushMutex.Lock()
		delete(s.pushMap, key)
		s.pushMutex.Unlock()
		s.lastPushDone++

		if pushed {
			break
		}
	}
	if save {
		s.saveRecordCollection()
	}
	s.lastPushLength = time.Now().Sub(s.lastPushTime)
}

func (s *Server) runRecache() {
	for key, val := range s.cacheMap {
		s.cacheRecord(val)
		delete(s.cacheMap, key)
		time.Sleep(s.cacheWait)
		break
	}
}

func (s *Server) pushRecord(r *pb.Record) bool {
	s.Log(fmt.Sprintf("PUSH: %v", r))
	pushed := (r.GetMetadata().GetSetRating() > 0 && r.GetRelease().Rating != r.GetMetadata().GetSetRating()) || (r.GetMetadata().GetMoveFolder() > 0 && r.GetRelease().FolderId != r.GetMetadata().GetMoveFolder())
	// Push the score
	if (r.GetMetadata().GetSetRating() > 0 || r.GetMetadata().GetSetRating() == -1) && r.GetRelease().Rating != r.GetMetadata().GetSetRating() {
		err := s.retr.SetRating(int(r.GetRelease().Id), max(0, int(r.GetMetadata().GetSetRating())))
		if err != nil {
			s.Log(fmt.Sprintf("RATING ERROR: %v", err))
		}
		r.GetRelease().Rating = int32(max(0, int(r.GetMetadata().SetRating)))
	}
	r.GetMetadata().SetRating = 0

	if r.GetMetadata().GetMoveFolder() > 0 && r.GetRelease().FolderId != r.GetMetadata().GetMoveFolder() {
		//Check that we can move this record
		val, err := s.quota.hasQuota(r.GetMetadata().GetMoveFolder())
		if err != nil || !val {
			s.Log(fmt.Sprintf("QUOTA DENIED: %v, %v", val, err))
		} else {
			resp := s.retr.MoveToFolder(int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId), int(r.GetMetadata().GetMoveFolder()))
			if len(resp) > 0 {
				s.Log(fmt.Sprintf("Moving record: %v", resp))
			}
			r.GetRelease().FolderId = r.GetMetadata().MoveFolder
			r.GetMetadata().MoveFolder = 0
		}
	}

	r.GetMetadata().Dirty = false
	s.Log(fmt.Sprintf("PUSHED: %v", r))
	return pushed
}

func (s *Server) cacheRecord(r *pb.Record) {
	if r.GetMetadata() == nil {
		r.Metadata = &pb.ReleaseMetadata{}
	}

	// Don't recache a dirty Record
	if r.GetMetadata().GetDirty() {
		return
	}

	//Force a recache if the record has no title
	if time.Now().Unix()-r.GetMetadata().GetLastCache() > 60*60*24*30 || r.GetRelease().Title == "" || len(r.GetRelease().GetFormats()) == 0 {
		release, err := s.retr.GetRelease(r.GetRelease().Id)
		if err == nil {

			//Clear repeated fields first
			r.GetRelease().Images = []*pbd.Image{}
			r.GetRelease().Artists = []*pbd.Artist{}
			r.GetRelease().Formats = []*pbd.Format{}
			r.GetRelease().Labels = []*pbd.Label{}

			proto.Merge(r.GetRelease(), release)

			r.GetMetadata().LastCache = time.Now().Unix()
			s.saveRecordCollection()
		}
	}
}

func (s *Server) syncCollection() {
	s.Log(fmt.Sprintf("Starting sync collection"))
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

	s.Log(fmt.Sprintf("Synced to %v", len(s.collection.GetRecords())))
	s.lastSyncTime = time.Now()
	s.saveRecordCollection()
}

func (s *Server) syncWantlist() {
	wants, _ := s.retr.GetWantlist()

	for _, want := range wants {
		found := false
		for _, w := range s.collection.GetNewWants() {
			if w.GetRelease().Id == want.Id {
				found = true
				proto.Merge(w.GetRelease(), want)
			}
		}
		if !found {
			s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: want, Metadata: &pb.WantMetadata{Active: true}})
		}
	}
}

func (s *Server) runSync() {
	s.syncCollection()
	s.syncWantlist()
	s.saveRecordCollection()
}
