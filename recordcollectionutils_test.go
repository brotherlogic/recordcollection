package main

import (
	"context"
	"errors"
	"log"
	"testing"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testMover struct {
	pass bool
}

func (t *testMover) moveRecord(instanceID, oldFolder, newFolder int32) error {
	if t.pass {
		return nil
	}

	log.Printf("HERE")
	return errors.New("Built to fail")
}

type testQuota struct {
	pass  bool
	spill int32
	fail  bool
}

func (t *testQuota) hasQuota(folder int32) (*pbro.QuotaResponse, error) {
	if t.pass {
		return &pbro.QuotaResponse{OverQuota: false}, nil
	}
	if t.fail {
		return &pbro.QuotaResponse{}, status.Error(codes.InvalidArgument, "Built to fail")
	}
	return &pbro.QuotaResponse{OverQuota: true, SpillFolder: t.spill}, nil
}

type testSyncer struct {
	setRatingCount  int
	moveRecordCount int
	failOnRate      bool
	updateWantCount int
}

func (t *testSyncer) GetCollection() []*pbd.Release {
	return []*pbd.Release{
		&pbd.Release{InstanceId: 1, Id: 234, MasterId: 12, Title: "Magic", Formats: []*pbd.Format{&pbd.Format{Name: "blah"}}, Images: []*pbd.Image{&pbd.Image{Uri: "blahblahblah"}}},
		&pbd.Release{InstanceId: 2, Id: 123, Title: "Johnson", MasterId: 12},
		&pbd.Release{InstanceId: 3, Id: 1255, Title: "Johnson", MasterId: 123},
	}
}

func (t *testSyncer) GetWantlist() ([]*pbd.Release, error) {
	return []*pbd.Release{&pbd.Release{Id: 255, Title: "Mirror"}}, nil
}

func (t *testSyncer) GetRelease(id int32) (*pbd.Release, error) {
	if id == 4707982 {
		return &pbd.Release{Id: 4707982, Title: "Future", Images: []*pbd.Image{&pbd.Image{Type: "primary", Uri: "http://magic"}}}, nil
	}
	return &pbd.Release{Id: 234, Title: "On The Wall", Labels: []*pbd.Label{&pbd.Label{Name: "madeup", Id: 123}}}, nil
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

func (t *testSyncer) DeleteInstance(a, b, c int) string {
	return "ALL GOOD!"
}

func (t *testSyncer) SellRecord(releaseID int, price float32, state string) {
	// Do Nothing
}
func (t *testSyncer) GetSalePrice(releaseID int) float32 {
	return 15.5
}

func (t *testSyncer) RemoveFromWantlist(releaseID int) {
	t.updateWantCount++
}

func (t *testSyncer) AddToWantlist(releaseID int) {
	// Do nothing
}

func TestSyncIssues(t *testing.T) {
	s := InitTestServer(".testsyncissues")
	s.collection.Records = append(s.collection.Records, &pb.Record{Metadata: &pb.ReleaseMetadata{LastSyncTime: 1}})
	s.syncIssue(context.Background())
}

func TestUpdateWantWithPush(t *testing.T) {
	s := InitTestServer(".testupdateWants")
	ts := &testSyncer{}
	s.retr = ts
	s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: &pbd.Release{Id: 123}, Metadata: &pb.WantMetadata{Active: true}})
	s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: &pbd.Release{Id: 12345}, Metadata: &pb.WantMetadata{Active: true}})

	s.UpdateWant(context.Background(), &pb.UpdateWantRequest{Update: &pb.Want{Release: &pbd.Release{Id: 123}}, Remove: true})
	s.pushWants(context.Background())

	if ts.updateWantCount != 1 {
		t.Errorf("Want not updated!")
	}

	s.pushWants(context.Background())
	if ts.updateWantCount != 1 {
		t.Errorf("More wants removed!")
	}
}

func TestGoodSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.runSync(context.Background())

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}
}

func TestCleanSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.runSync(context.Background())

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetRecords()[0].GetRelease().GetImages()) != 1 {
		t.Errorf("Wrong number of images in synced record: %v", s.collection.GetRecords()[0].GetRelease())
	}

	for i := 0; i < 10; i++ {
		s.runSync(context.Background())
	}

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}
	if len(s.collection.GetRecords()[0].GetRelease().GetImages()) != 1 {
		t.Errorf("Wrong number of images in synced record: %v", s.collection.GetRecords()[0].GetRelease())
	}
	if !s.collection.GetRecords()[0].GetMetadata().Others {
		t.Errorf("Sync has not set other correctly: %v", s.collection.GetRecords()[0])
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
	r := &pb.Record{Release: &pbd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{SetRating: 4}}
	s.cacheRecord(r)

	if r.GetMetadata().LastCache != 0 {
		t.Fatalf("Record has not been cached despite being dirty %v", r)
	}
}

func TestGoodMergeSync(t *testing.T) {
	s := InitTestServer(".testGoodMergeSync")
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}, Metadata: &pb.WantMetadata{Active: true}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Id: 234, InstanceId: 1}}}}
	s.runSync(context.Background())

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 3 {
		t.Errorf("Wrong number of records(%v): %v", len(s.collection.GetRecords()), s.collection.GetRecords())
	}
	if s.collection.GetRecords()[0].GetRelease().Title != "Magic" {
		t.Errorf("Incoming has not been merged: %v", s.collection.GetRecords()[0])
	}
	if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}
}

func TestGoodMergeSyncWithDirty(t *testing.T) {
	s := InitTestServer(".testGoodMergeSyncWithDirty")
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}, Metadata: &pb.WantMetadata{Active: true}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Rating: 5, Id: 234, InstanceId: 1}}}}
	s.runSync(context.Background())

	// Check that we have one record and one want
	if len(s.collection.GetRecords()) != 3 {
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
	s.runRecache(context.Background())
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{Id: 234}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 || r.GetRecords()[0].GetRelease().Title != "On The Wall" {
		t.Errorf("Error in reading records: %v", r)
	}

	log.Printf("WHAT = %v", r.GetRecords()[0])
}

func TestSimplePush(t *testing.T) {
	r := &pb.Record{Release: &pbd.Release{FolderId: 268147}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_DIGITAL, GoalFolder: 268147, Dirty: true, MoveFolder: 268147}}
	s := InitTestServer(".testsimplepush")
	v := s.pushRecord(r)
	if v {
		t.Fatalf("Push dirty record failed: %v", v)
	}
	if r.GetMetadata().MoveFolder != 0 {
		t.Errorf("Record move has not been reset")
	}
}

func TestBadPush(t *testing.T) {
	tRetr := &testSyncer{
		failOnRate: true,
	}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{}}, &pb.Record{Release: &pbd.Release{InstanceId: 1235, Id: 234}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})
	_, err = s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 1235}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush(context.Background())

	if tRetr.setRatingCount > 0 {
		t.Errorf("Update has not failed")
	}
}

func TestPushMove(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testpushmove")
	s.retr = tRetr
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{}}, &pb.Record{Release: &pbd.Release{InstanceId: 1235, Id: 23466}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})
	_, err = s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 1235}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush(context.Background())

	if tRetr.moveRecordCount != 1 {
		t.Errorf("Update has not run")
	}
}

func TestPushBadQuotaMove(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.quota = &testQuota{pass: false}
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234, FolderId: 23}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush(context.Background())

	if tRetr.moveRecordCount != 0 {
		t.Errorf("Update has run when it shouldn't")
	}
}

func TestPushBadHardQuotaMove(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.quota = &testQuota{pass: false, fail: true}
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234, FolderId: 23}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush(context.Background())

	if tRetr.moveRecordCount != 0 {
		t.Errorf("Update has run when it shouldn't")
	}
}

func TestPushBadMoveRecord(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.mover = &testMover{pass: false}
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234, FolderId: 23}, Metadata: &pb.ReleaseMetadata{}}}}
	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush(context.Background())

	if tRetr.moveRecordCount != 0 {
		t.Errorf("Update has run when it shouldn't")
	}
}

func TestPushBadQuotaMoveWithSpill(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.quota = &testQuota{pass: false, spill: 123}
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{InstanceId: 123, Id: 234, FolderId: 23}, Metadata: &pb.ReleaseMetadata{}}}}

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.runPush(context.Background())

	if tRetr.moveRecordCount != 1 {
		t.Errorf("Update has not run when it should've")
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

	s.runPush(context.Background())

	if tRetr.setRatingCount != 1 {
		t.Errorf("Update has not run")
	}
}
