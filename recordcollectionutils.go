package main

import (
	"fmt"
	"log"
	"time"

	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

func (s *Server) syncCollection() {
	s.Log(fmt.Sprintf("Starting sync collection"))
	records := s.retr.GetCollection()
	log.Printf("RETRIEVED: %v", len(records))

	for _, record := range records {
		found := false
		for _, r := range s.collection.GetRecords() {
			if r.GetRelease().InstanceId == record.InstanceId {
				found = true
				proto.Merge(r.Release, &record)
			}
		}
		if !found {
			s.collection.Records = append(s.collection.Records, &pb.Record{Release: &record})
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
				proto.Merge(w, &want)
			}
		}
		if !found {
			s.collection.Wants = append(s.collection.Wants, &want)
		}
	}
}

func (s *Server) runSync() {
	//SyncWithDiscogs Syncs everything with discogs
	s.syncCollection()
	s.syncWantlist()
	s.saveRecordCollection()
}
