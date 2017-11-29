package main

import "testing"
import pbd "github.com/brotherlogic/godiscogs"
import pb "github.com/brotherlogic/recordcollection/proto"

type testSyncer struct{}

func (t *testSyncer) GetCollection() []pbd.Release {
	return []pbd.Release{pbd.Release{Id: 234, Title: "Magic"}}
}

func (t *testSyncer) GetWantlist() ([]pbd.Release, error) {
	return []pbd.Release{pbd.Release{Id: 255, Title: "Mirror"}}, nil
}

func TestGoodSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.runSync()

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetWants())
	}
}

func TestGoodMergeSync(t *testing.T) {
	s := InitTestServer(".testGoodMergeSync")
	s.collection = &pb.RecordCollection{Wants: []*pbd.Release{&pbd.Release{Id: 255}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{Id: 234}}}}
	s.runSync()

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if s.collection.GetRecords()[0].GetRelease().Title != "Magic" {
		t.Errorf("Incoming has not been merged: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetWants())
	}
}
