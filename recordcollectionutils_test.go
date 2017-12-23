package main

import (
	"context"
	"testing"

	pbd "github.com/brotherlogic/godiscogs"
)
import pb "github.com/brotherlogic/recordcollection/proto"

type testSyncer struct {
	setRatingCount int
}

func (t *testSyncer) GetCollection() []*pbd.Release {
	return []*pbd.Release{&pbd.Release{Id: 234, Title: "Magic"}}
}

func (t *testSyncer) GetWantlist() ([]*pbd.Release, error) {
	return []*pbd.Release{&pbd.Release{Id: 255, Title: "Mirror"}}, nil
}

func (t *testSyncer) GetRelease(id int32) (*pbd.Release, error) {
	if id == 4707982 {
		return &pbd.Release{Id: 4707982, Title: "Future", Images: []*pbd.Image{&pbd.Image{Type: "primary", Uri: "http://magic"}}}, nil
	}
	return &pbd.Release{Id: 234, Title: "On The Wall"}, nil
}

func (t *testSyncer) AddToFolder(id int32, folderID int32) (int, error) {
	return 200, nil
}

func (t *testSyncer) SetRating(id int, rating int) error {
	t.setRatingCount = 1
	return nil
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

func TestImageMerge(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	r := &pb.Record{Release: &pbd.Release{Id: 4707982, InstanceId: 236418222}}
	s.cacheRecord(r)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(r)

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}

	if len(r.GetRelease().GetImages()) != 1 {
		t.Errorf("Image merge has failed: %v", r)
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

func TestRecache(t *testing.T) {
	s := InitTestServer(".testrecache")
	s.collection = &pb.RecordCollection{Wants: []*pbd.Release{&pbd.Release{Id: 255}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{Id: 234}}}}

	s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{Id: 234}}})
	s.runRecache()
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{Id: 234}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 || r.GetRecords()[0].GetRelease().Title != "On The Wall" {
		t.Errorf("Error in reading records: %v", r)
	}
}

func TestPush(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.collection = &pb.RecordCollection{Wants: []*pbd.Release{&pbd.Release{Id: 255}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{Id: 234}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{Id: 234, Rating: 5}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush()

	if tRetr.setRatingCount != 1 {
		t.Errorf("Update has not run")
	}
}
