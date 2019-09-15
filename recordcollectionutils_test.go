package main

import (
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type testScorer struct {
	fail bool
}

func (t *testScorer) GetScore(ctx context.Context, instanceID int32) (float32, error) {
	if t.fail {
		return -1, errors.New("Built to fail")
	}
	return 2.5, nil
}

type testMover struct {
	pass bool
}

func (t *testMover) moveRecord(record *pb.Record, oldFolder, newFolder int32) error {
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

func (t *testQuota) hasQuota(ctx context.Context, folder int32) (*pbro.QuotaResponse, error) {
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
	failSalePrice   bool
}

func (t *testSyncer) UpdateSalePrice(saleID int, releaseID int, condition, sleeve string, price float32) error {
	if t.failSalePrice {
		return fmt.Errorf("built to fail")
	}
	return nil
}

func (t *testSyncer) RemoveFromSale(saleID int, releaseID int) error {
	return nil
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

func (t *testSyncer) SellRecord(releaseID int, price float32, state string, condition, sleeve string) int {
	return 0
}
func (t *testSyncer) GetSalePrice(releaseID int) (float32, error) {
	return 15.5, nil
}
func (t *testSyncer) GetSaleState(releaseID int) pbd.SaleState {
	return pbd.SaleState_FOR_SALE
}

func (t *testSyncer) RemoveFromWantlist(releaseID int) {
	t.updateWantCount++
}

func (t *testSyncer) AddToWantlist(releaseID int) {
	// Do nothing
}

func (t *testSyncer) GetCurrentSalePrice(saleID int) float32 {
	return 12.34
}

func (t *testSyncer) GetCurrentSaleState(saleID int) pbd.SaleState {
	return pbd.SaleState_FOR_SALE
}

func TestSyncIssues(t *testing.T) {
	s := InitTestServer(".testsyncissues")
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123}, Metadata: &pb.ReleaseMetadata{LastSyncTime: 1}})
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

	s.runSyncWants(context.Background())

	if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}
}

func TestSaleSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	rec := &pb.Record{Release: &pbd.Release{Id: 123}, Metadata: &pb.ReleaseMetadata{SaleId: 123}}
	s.collection.Records = append(s.collection.Records, rec)
	s.runSync(context.Background())

	if rec.GetMetadata().SalePrice == 0 {
		t.Errorf("Price has not synced: %v", rec)
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
	r := &pb.Record{Release: &pbd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r)

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}

	if len(r.GetRelease().GetImages()) != 1 {
		t.Errorf("Image merge has failed: %v", r)
	}
}

func TestInstanceIdCache(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	r := &pb.Record{Release: &pbd.Release{Id: 4707982}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r)

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}
}

func TestImageMergeWithFailScore(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	s.scorer = &testScorer{fail: true}
	r := &pb.Record{Release: &pbd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r)

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
	s.cacheRecord(context.Background(), r)

	if r.GetMetadata().LastCache != 0 {
		t.Fatalf("Record has not been cached despite being dirty %v", r)
	}
}

func TestGoodMergeSync(t *testing.T) {
	s := InitTestServer(".testGoodMergeSync")
	s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbd.Release{Id: 255}, Metadata: &pb.WantMetadata{Active: true}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Id: 234, InstanceId: 1}}}}
	s.runSync(context.Background())
	s.runSyncWants(context.Background())

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

func TestSimplePush(t *testing.T) {
	r := &pb.Record{Release: &pbd.Release{FolderId: 268147}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_DIGITAL, GoalFolder: 268147, Dirty: true, MoveFolder: 268147}}
	s := InitTestServer(".testsimplepush")
	v, _ := s.pushRecord(context.Background(), r)
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

func TestBasic(t *testing.T) {
	s := InitTestServer(".madeup")
	s.updateWant(&pb.Want{Release: &pbd.Release{Id: 766489}, Metadata: &pb.WantMetadata{}})
}

func TestPushSaleWithAdjust(t *testing.T) {
	s := InitTestServer(".saleadjust")

	record := &pb.Record{Release: &pbd.Release{}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_LISTED_TO_SELL, CurrentSalePrice: 100, SaleDirty: true}}
	s.pushSale(context.Background(), record)

	if record.GetMetadata().SalePrice != 100 {
		t.Errorf("Sale price was not adjusted: %v", record)
	}
}

func TestPushSaleWithFail(t *testing.T) {
	s := InitTestServer(".saleadjust")
	s.retr = &testSyncer{failSalePrice: true}

	record := &pb.Record{Release: &pbd.Release{SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_LISTED_TO_SELL, CurrentSalePrice: 100, SaleDirty: true}}
	_, err := s.pushSale(context.Background(), record)
	if err == nil {
		t.Errorf("Sale push did not fail")
	}

}

func TestPushSaleBasic(t *testing.T) {
	s := InitTestServer(".saleadjust")

	record := &pb.Record{Release: &pbd.Release{SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_POSTDOC, CurrentSalePrice: 100, SaleDirty: true}}
	_, err := s.pushSale(context.Background(), record)
	if err != nil {
		t.Errorf("Sale push failed: %v", err)
	}

}

func TestSyncRecordTracklist(t *testing.T) {
	s := InitTestServer(".syncrecord")

	record := &pb.Record{Release: &pbd.Release{Tracklist: []*pbd.Track{&pbd.Track{Title: "One"}}}, Metadata: &pb.ReleaseMetadata{NeedsStockCheck: true, LastStockCheck: time.Now().Unix()}}
	s.syncRecords(context.Background(), record, &pbd.Release{Tracklist: []*pbd.Track{&pbd.Track{Title: "Two"}, &pbd.Track{Title: "Three"}}, RecordCondition: "blah", SleeveCondition: "alsoblah"}, int64(12))

	if len(record.GetRelease().GetTracklist()) != 2 {
		t.Errorf("Tracklisting not updated correctly")
	}

	if !record.GetMetadata().SaleDirty {
		t.Errorf("Sale dirty not set")
	}

	if record.GetMetadata().NeedsStockCheck {
		t.Errorf("Stock check not updated")
	}

}

func TestPushSaleBasicWithNone(t *testing.T) {
	s := InitTestServer(".saleadjustnone")
	err := s.pushSales(context.Background())
	if err != nil {
		t.Errorf("Sale push failed: %v", err)
	}

}
