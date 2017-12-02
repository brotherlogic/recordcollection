package main

import (
	"fmt"
	"log"
	"time"

	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

const (
	// RecacheDelay - recache everything every 30 days
	RecacheDelay = 60 * 60 * 24 * 30
)

func (s *Server) runRecache() {
	for key, val := range s.cacheMap {
		log.Printf("CACHE: %v", val)
		s.cacheRecord(val)
		delete(s.cacheMap, key)
		time.Sleep(s.cacheWait)
	}
}

func (s *Server) cacheRecord(r *pb.Record) {
	if time.Now().Unix()-r.GetMetdata().GetLastCache() > 60*60*24*30 {
		release, err := s.retr.GetRelease(r.GetRelease().Id)
		s.Log(fmt.Sprintf("RECACHE: %v", release))
		if err == nil {
			proto.Merge(r.GetRelease(), release)
			log.Printf("NOW: %v", r.GetRelease())
			r.GetMetdata().LastCache = time.Now().Unix()
			s.saveRecordCollection()
		}
	}
}

func (s *Server) syncCollection() {
	s.Log(fmt.Sprintf("Starting sync collection"))
	records := s.retr.GetCollection()
	log.Printf("RETRIEVED: %v %v", len(records), s.collection.GetRecords())

	for _, record := range records {
		found := false
		for _, r := range s.collection.GetRecords() {
			if r.GetRelease().InstanceId == record.InstanceId {
				found = true
				proto.Merge(r.Release, record)
			}
		}

		if !found {
			s.collection.Records = append(s.collection.Records, &pb.Record{Release: record})
		}
	}

	log.Printf("SYNCED: %v", len(s.collection.GetRecords()))
	s.Log(fmt.Sprintf("Synced to %v", len(s.collection.GetRecords())))
	s.lastSyncTime = time.Now()
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
	log.Printf("RUNNING SYNC")
	s.syncCollection()
	s.syncWantlist()
	s.saveRecordCollection()
}
