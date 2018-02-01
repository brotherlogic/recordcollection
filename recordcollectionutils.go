package main

import (
	"fmt"
	"log"
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
		s.pushRecord(val)
		s.pushMutex.Lock()
		delete(s.pushMap, key)
		s.pushMutex.Unlock()
		s.lastPushDone++
		break
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
	pushed := r.GetMetadata().GetSetRating() > 0 || r.GetMetadata().GetMoveFolder() > 0
	// Push the score
	if r.GetMetadata().GetSetRating() > 0 {
		err := s.retr.SetRating(int(r.GetRelease().Id), int(r.GetMetadata().GetSetRating()))
		if err != nil {
			s.Log(fmt.Sprintf("RATING ERROR: %v", err))
		}
		r.GetRelease().Rating = r.GetMetadata().SetRating
		r.GetMetadata().SetRating = 0
	}

	if r.GetMetadata().GetMoveFolder() > 0 {
		resp := s.retr.MoveToFolder(int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId), int(r.GetMetadata().GetMoveFolder()))
		if len(resp) > 0 {
			s.Log(fmt.Sprintf("Moving record: %v", resp))
		}
		r.GetRelease().FolderId = r.GetMetadata().MoveFolder
		r.GetMetadata().MoveFolder = 0
	}

	r.GetMetadata().Dirty = false
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

			// Reset to dirty if the scores don't match
			if release.Rating != r.GetRelease().Rating {
				r.GetMetadata().Dirty = true
				return
			}

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
				before := len(r.GetRelease().GetFormats())
				if len(record.GetFormats()) > 0 {
					r.GetRelease().Formats = []*pbd.Format{}
					r.GetRelease().Artists = []*pbd.Artist{}
					r.GetRelease().Labels = []*pbd.Label{}
				}

				if len(record.GetImages()) > 0 {
					r.GetRelease().Images = []*pbd.Image{}
				}

				now := len(r.GetRelease().Formats)
				proto.Merge(r.Release, record)
				then := len(r.GetRelease().Formats)

				// Override if the rating doesn't match
				if r.Release.Rating != record.Rating {
					r.Release.Rating = record.Rating
				}

				if r.GetRelease().Id == 3331113 {
					s.Log(fmt.Sprintf("TORU FROM %v to %v given %v and %v and %v", before, len(r.GetRelease().GetFormats()), now, then, record))
				}
				if before < len(r.GetRelease().GetFormats()) {
					log.Fatalf("Sync has grown formats? %v -> %v", r, record)
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
		for _, w := range s.collection.GetWants() {
			if w.Id == want.Id {
				found = true
				proto.Merge(w, want)
			}
		}
		if !found {
			s.collection.Wants = append(s.collection.Wants, want)
		}
	}
}

func (s *Server) runSync() {
	s.syncCollection()
	s.syncWantlist()
	s.saveRecordCollection()
}
