package main

import (
	"context"
	"errors"
	"testing"

	pbd "github.com/brotherlogic/godiscogs"

	pb "github.com/brotherlogic/recordcollection/proto"
)

type testSyncer struct {
	setRatingCount  int
	moveRecordCount int
	failOnRate      bool
}

func (t *testSyncer) GetCollection() []*pbd.Release {
	return []*pbd.Release{&pbd.Release{Id: 234, Title: "Magic", Formats: []*pbd.Format{&pbd.Format{Name: "blah"}}, Images: []*pbd.Image{&pbd.Image{Uri: "blahblahblah"}}}}
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
	if t.failOnRate {
		return errors.New("Set to fail")
	}
	t.setRatingCount = 1
	return nil
}

func (t *testSyncer) MoveToFolder(a, b, c, d int) string {
	t.moveRecordCount = 1
	return "ALL GOOD!"
}

func TestGoodSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.runSync()

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}
}

func TestCleanSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.runSync()

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetRecords()[0].GetRelease().GetImages()) != 1 {
		t.Errorf("Wrong number of images in synced record: %v", s.collection.GetRecords()[0].GetRelease())
	}

	for i := 0; i < 10; i++ {
		s.runSync()
	}

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetRecords()[0].GetRelease().GetImages()) != 1 {
		t.Errorf("Wrong number of images in synced record: %v", s.collection.GetRecords()[0].GetRelease())
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

func TestDirtyMerge(t *testing.T) {
	s := InitTestServer(".testDirtyMerge")
	r := &pb.Record{Release: &pbd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{Dirty: true}}
	s.cacheRecord(r)

	if r.GetMetadata().LastCache != 0 {
		t.Fatalf("Record has not been cached despite being dirty %v", r)
	}
}

func TestDirtyAttemptToMerge(t *testing.T) {
	s := InitTestServer(".testDirtyMerge")
	r := &pb.Record{Release: &pbd.Release{Id: 4707982, InstanceId: 236418222, Rating: 4}, Metadata: &pb.ReleaseMetadata{Dirty: false}}
	s.cacheRecord(r)

	if r.GetMetadata().LastCache != 0 || !r.GetMetadata().Dirty {
		t.Fatalf("Record has beed recached even though it should be dirty %v", r)
	}
}

func TestGoodMergeSync(t *testing.T) {
	s := InitTestServer(".testGoodMergeSync")
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Id: 234}}}}
	s.runSync()

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if s.collection.GetRecords()[0].GetRelease().Title != "Magic" {
		t.Errorf("Incoming has not been merged: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}
}

func TestGoodMergeSyncWithDirty(t *testing.T) {
	s := InitTestServer(".testGoodMergeSyncWithDirty")
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Rating: 5, Id: 234}}}}
	s.runSync()

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if s.collection.GetRecords()[0].GetRelease().Title != "Magic" {
		t.Errorf("Incoming has not been merged: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}
}

func TestRecache(t *testing.T) {
	s := InitTestServer(".testrecache")
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{Id: 234}}}}

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

func TestBadPush(t *testing.T) {
	tRetr := &testSyncer{
		failOnRate: true,
	}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush()

	if tRetr.setRatingCount > 0 {
		t.Errorf("Update has not failed")
	}
}

func TestPushMove(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234, FolderId: 23}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush()

	if tRetr.moveRecordCount != 1 {
		t.Errorf("Update has not run")
	}
}

func TestPushRating(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234, FolderId: 23}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{SetRating: 4}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush()

	if tRetr.setRatingCount != 1 {
		t.Errorf("Update has not run")
	}
}
