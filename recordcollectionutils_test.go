package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	godiscogs "github.com/brotherlogic/godiscogs"
	pbgd "github.com/brotherlogic/godiscogs/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
	pbro "github.com/brotherlogic/recordsorganiser/proto"
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

func (t *testMover) moveRecord(ctx context.Context, record *pb.Record, oldFolder, newFolder int32) error {
	if t.pass {
		return nil
	}

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
	badLoad         bool
	badInventory    bool
	count           int
}

func (t *testSyncer) GetInstanceInfo(ctx context.Context, ID int32) (map[int32]*godiscogs.InstanceInfo, error) {
	return make(map[int32]*godiscogs.InstanceInfo), nil
}

func (t *testSyncer) GetInventory(ctx context.Context) ([]*pbgd.ForSale, error) {
	if t.badInventory {
		return []*pbgd.ForSale{}, fmt.Errorf("Built to fail")
	}
	return []*pbgd.ForSale{&pbgd.ForSale{Id: 123, SaleId: 123}}, nil
}

func (t *testSyncer) ExpireSale(ctx context.Context, saleID int, releaseID int, price float32) error {
	return nil
}

func (t *testSyncer) UpdateSalePrice(ctx context.Context, saleID int, releaseID int, condition, sleeve string, price float32) error {
	if t.failSalePrice {
		return fmt.Errorf("built to fail")
	}
	return nil
}

func (t *testSyncer) RemoveFromSale(ctx context.Context, saleID int, releaseID int) error {
	return nil
}

func (t *testSyncer) GetCollection(ctx context.Context) []*pbgd.Release {
	return []*pbgd.Release{
		&pbgd.Release{FolderId: 12, InstanceId: 1, Id: 234, MasterId: 12, Title: "Magic", Formats: []*pbgd.Format{&pbgd.Format{Name: "blah"}}, Images: []*pbgd.Image{&pbgd.Image{Uri: "blahblahblah"}}},
		&pbgd.Release{FolderId: 12, InstanceId: 2, Id: 123, Title: "Johnson", MasterId: 12},
		&pbgd.Release{FolderId: 12, InstanceId: 3, Id: 1255, Title: "Johnson", MasterId: 123},
	}
}

func (t *testSyncer) GetWantlist(ctx context.Context) ([]*pbgd.Release, error) {
	return []*pbgd.Release{&pbgd.Release{Id: 255, Title: "Mirror"}}, nil
}

func (t *testSyncer) GetOrder(ctx context.Context, ID string) (map[int64]int32, time.Time, error) {
	return make(map[int64]int32), time.Now(), nil
}

func (t *testSyncer) GetRelease(ctx context.Context, id int32) (*pbgd.Release, error) {
	if id == 4707982 {
		return &pbgd.Release{Id: 4707982, Title: "Future", Images: []*pbgd.Image{&pbgd.Image{Type: "primary", Uri: "http://magic"}}}, nil
	}
	return &pbgd.Release{Id: id, Title: "On The Wall", Labels: []*pbgd.Label{&pbgd.Label{Name: "madeup", Id: 123}}}, nil
}

func (t *testSyncer) AddToFolder(ctx context.Context, id int32, folderID int32) (int, error) {
	t.count++
	return 200 + t.count, nil
}

func (t *testSyncer) SetRating(ctx context.Context, id int, rating int) error {
	if t.failOnRate {
		return errors.New("Set to fail")
	}
	t.setRatingCount = 1
	return nil
}

func (t *testSyncer) MoveToFolder(ctx context.Context, a, b, c, d int) (string, error) {
	t.moveRecordCount = 1
	return "ALL GOOD!", nil
}

func (t *testSyncer) DeleteInstance(ctx context.Context, folderID, releaseID, instanceID int) error {
	return fmt.Errorf("ALL GOOD!")
}

func (t *testSyncer) SellRecord(ctx context.Context, releaseID int, price float32, state string, condition, sleeve string, weight int) (int64, error) {
	return 0, nil
}
func (t *testSyncer) GetSalePrice(ctx context.Context, releaseID int) (float32, error) {
	return 15.5, nil
}
func (t *testSyncer) GetSaleState(ctx context.Context, releaseID int) pbgd.SaleState {
	return pbgd.SaleState_FOR_SALE
}

func (t *testSyncer) RemoveFromWantlist(ctx context.Context, releaseID int) error {
	t.updateWantCount++
	return nil
}

func (t *testSyncer) AddToWantlist(ctx context.Context, releaseID int) error {
	// Do nothing
	return nil
}

func (t *testSyncer) GetCurrentSalePrice(ctx context.Context, saleID int64) float32 {
	return 12.34
}

func (t *testSyncer) GetCurrentSaleState(ctx context.Context, saleID int64) (pbgd.SaleState, error) {
	return pbgd.SaleState_FOR_SALE, nil
}

func TestUpdateWantWithPush(t *testing.T) {
	s := InitTestServer(".testupdateWants")
	ts := &testSyncer{}
	s.retr = ts
	//s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: &pbgd.Release{Id: 123}, Metadata: &pb.WantMetadata{Active: true}})
	//s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: &pbgd.Release{Id: 12345}, Metadata: &pb.WantMetadata{Active: true}})

	s.UpdateWant(context.Background(), &pb.UpdateWantRequest{Update: &pb.Want{ReleaseId: 123}, Remove: true})
	//s.pushWants(context.Background())

	if ts.updateWantCount != 1 {
		t.Errorf("Want not updated!")
	}

	//s.pushWants(context.Background())
	if ts.updateWantCount != 1 {
		t.Errorf("More wants removed!")
	}
}

func TestGoodSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	//s.collection.InstanceToCategory[int32(12)] = pb.ReleaseMetadata_LISTED_TO_SELL
	s.runSync(context.Background())

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.InstanceToFolder)
	}*/

	s.runSyncWants(context.Background())
	s.runSyncWants(context.Background())

	/*if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}*/
}

func TestGoodSyncWithBadLoad(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	//s.collection.InstanceToFolder[int32(1)] = int32(12)
	s.runSync(context.Background())

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.InstanceToFolder)
	}*/

	s.runSyncWants(context.Background())

	/*if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}*/
}

func TestGoodSyncWithBadLoadTimeout(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.TimeoutLoad = true
	//s.collection.InstanceToFolder[int32(1)] = int32(12)
	s.runSync(context.Background())

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 1 {
		t.Errorf("Wrong number of records: %v", s.collection.InstanceToFolder)
	}*/

	s.runSyncWants(context.Background())

	/*if len(s.collection.GetNewWants()) != 1 {
		t.Errorf("Wrong number of wants: %v", s.collection.GetNewWants())
	}*/
}

func TestCleanSync(t *testing.T) {
	s := InitTestServer(".testGoodSync")
	s.runSync(context.Background())

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.InstanceToFolder)
	}*/

	for i := 0; i < 10; i++ {
		s.runSync(context.Background())
	}

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.GetRecords())
	}*/
}

func TestImageMerge(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	r := &pb.Record{Release: &pbgd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r, "For Testing")

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r, "For Testing")

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}

	if len(r.GetRelease().GetImages()) != 1 {
		t.Errorf("Image merge has failed: %v", r)
	}
}

func TestInstanceIdCache(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	r := &pb.Record{Release: &pbgd.Release{Id: 4707982}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r, "For Testing")

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r, "For Testing")

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}
}

func TestImageMergeWithFailScore(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	s.scorer = &testScorer{fail: true}
	r := &pb.Record{Release: &pbgd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r, "For Testing")

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r, "For Testing")

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}

	if len(r.GetRelease().GetImages()) != 1 {
		t.Errorf("Image merge has failed: %v", r)
	}
}

func TestGoodMergeSync(t *testing.T) {
	s := InitTestServer(".testGoodMergeSync")
	//s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbgd.Release{Id: 255}, Metadata: &pb.WantMetadata{Active: true}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbgd.Release{Id: 234, InstanceId: 1}}}, InstanceToFolder: make(map[int32]int32)}
	s.runSync(context.Background())
	s.runSyncWants(context.Background())

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 3 {
		t.Errorf("Wrong number of records(%v): %v", len(s.collection.InstanceToFolder), s.collection.InstanceToFolder)
	}*/
}

func TestGoodMergeSyncWithDirty(t *testing.T) {
	s := InitTestServer(".testGoodMergeSyncWithDirty")
	//s.collection = &pb.RecordCollection{NewWants: []*pb.Want{&pb.Want{Release: &pbgd.Release{Id: 255}, Metadata: &pb.WantMetadata{Active: true}}}, Records: []*pb.Record{&pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbgd.Release{Rating: 5, Id: 234, InstanceId: 1}}}, InstanceToFolder: make(map[int32]int32)}
	s.runSync(context.Background())

	// Check that we have one record and one want
	/*if len(s.collection.InstanceToFolder) != 3 {
		t.Errorf("Wrong number of records: %v", s.collection.InstanceToFolder)
	}*/
}

func TestSimplePush(t *testing.T) {
	r := &pb.Record{Release: &pbgd.Release{FolderId: 268147}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_DIGITAL, GoalFolder: 268147, Dirty: true, MoveFolder: 268147}}
	s := InitTestServer(".testsimplepush")
	v, _ := s.pushRecord(context.Background(), r)
	if v {
		t.Fatalf("Push dirty record failed: %v", v)
	}
	if r.GetMetadata().MoveFolder != 0 {
		t.Errorf("Record move has not been reset")
	}
}

func TestFailPush(t *testing.T) {
	r := &pb.Record{Release: &pbgd.Release{FolderId: 268147}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_DIGITAL, GoalFolder: 268148, Dirty: true, MoveFolder: 268150}}
	s := InitTestServer(".testsimplepush")
	s.mover = &testMover{pass: false}
	v, _ := s.pushRecord(context.Background(), r)
	if v {
		t.Fatalf("Push dirty record failed: %v", v)
	}
}

func TestBadPush(t *testing.T) {
	tRetr := &testSyncer{
		failOnRate: true,
	}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title1", InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})
	_, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title2", InstanceId: 1235, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	if err != nil {
		t.Fatalf("Error in adding record: %v", err)
	}

	_, err = s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})
	_, err = s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 1235}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	//s.runPush(context.Background())

	if tRetr.setRatingCount > 0 {
		t.Errorf("Update has not failed")
	}
}

func TestPushMove(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testpushmove")
	s.retr = tRetr
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title1", InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})
	_, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title2", InstanceId: 1235, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	if err != nil {
		t.Fatalf("Error in adding record: %v", err)
	}

	_, err = s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})
	_, err = s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 1235}, Metadata: &pb.ReleaseMetadata{SetRating: 3}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.CommitRecord(context.Background(), &pb.CommitRecordRequest{InstanceId: 123})

	//s.runPush(context.Background())
	//s.runPush(context.Background())

	if tRetr.moveRecordCount != 1 {
		t.Errorf("Update has not run: %v", tRetr.moveRecordCount)
	}
}

func TestPushBadMoveRecord(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.mover = &testMover{pass: false}
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title1", InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})
	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.CommitRecord(context.Background(), &pb.CommitRecordRequest{InstanceId: 123})

	if tRetr.moveRecordCount != 0 {
		t.Errorf("Update has run when it shouldn't")
	}
}

func TestPushBadQuotaMoveWithSpill(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.quota = &testQuota{pass: false, spill: 123}
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title1", InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{MoveFolder: 26}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.CommitRecord(context.Background(), &pb.CommitRecordRequest{InstanceId: 123})

	if tRetr.moveRecordCount != 1 {
		t.Errorf("Update has not run when it should've")
	}
}

func TestPushRating(t *testing.T) {
	tRetr := &testSyncer{}
	s := InitTestServer(".testrecache")
	s.retr = tRetr
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbgd.Release{Title: "title1", InstanceId: 123, Id: 234}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Reason: "test", Update: &pb.Record{Release: &pbgd.Release{InstanceId: 123}, Metadata: &pb.ReleaseMetadata{SetRating: 4}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	s.CommitRecord(context.Background(), &pb.CommitRecordRequest{InstanceId: 123})

	if tRetr.setRatingCount != 1 {
		t.Errorf("Update has not run")
	}
}

func TestBasic(t *testing.T) {
	s := InitTestServer(".madeup")
	s.updateWant(context.Background(), &pb.Want{ReleaseId: 766489})
}

func TestPushSaleWithFail(t *testing.T) {
	s := InitTestServer(".saleadjust")
	s.retr = &testSyncer{failSalePrice: true}

	record := &pb.Record{Release: &pbgd.Release{SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_LISTED_TO_SELL, NewSalePrice: 100, SaleDirty: true}}
	_, err := s.pushSale(context.Background(), record)
	if err != nil {
		t.Errorf("Sale push failed: %v", err)
	}

}

func TestPushSaleBasic(t *testing.T) {
	s := InitTestServer(".saleadjust")

	record := &pb.Record{Release: &pbgd.Release{SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_POSTDOC, CurrentSalePrice: 100, SaleDirty: true}}
	_, err := s.pushSale(context.Background(), record)
	if err != nil {
		t.Errorf("Sale push failed: %v", err)
	}

}

func TestPushSaleExpire(t *testing.T) {
	s := InitTestServer(".saleadjust")

	record := &pb.Record{Release: &pbgd.Release{SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_POSTDOC, CurrentSalePrice: 100, SaleDirty: true, ExpireSale: true, SaleState: pbgd.SaleState_FOR_SALE}}
	_, err := s.pushSale(context.Background(), record)
	if err != nil {
		t.Errorf("Sale push failed: %v", err)
	}

}

func TestSyncRecordTracklist(t *testing.T) {
	s := InitTestServer(".syncrecord")

	record := &pb.Record{Release: &pbgd.Release{Rating: 12, Tracklist: []*pbgd.Track{&pbgd.Track{Title: "One"}}}, Metadata: &pb.ReleaseMetadata{NeedsStockCheck: true, LastStockCheck: time.Now().Unix()}}
	s.syncRecords(context.Background(), record, &pbgd.Release{Rating: 24, FolderId: 12, Tracklist: []*pbgd.Track{&pbgd.Track{Title: "Two"}, &pbgd.Track{Title: "Three"}}, RecordCondition: "blah", SleeveCondition: "alsoblah"}, int64(12))

	if len(record.GetRelease().GetTracklist()) != 2 {
		t.Errorf("Tracklisting not updated correctly")
	}

	if !record.GetMetadata().SaleDirty {
		t.Errorf("Sale dirty not set")
	}

}

func TestRecache(t *testing.T) {
	s := InitTestServer(".runrecache")
	record := &pb.Record{Release: &pbgd.Release{Rating: 12, Tracklist: []*pbgd.Track{&pbgd.Track{Title: "One"}}}, Metadata: &pb.ReleaseMetadata{NeedsStockCheck: true, LastStockCheck: time.Now().Unix()}}
	s.syncRecords(context.Background(), record, &pbgd.Release{Rating: 24, FolderId: 12, Tracklist: []*pbgd.Track{&pbgd.Track{Title: "Two"}, &pbgd.Track{Title: "Three"}}, RecordCondition: "blah", SleeveCondition: "alsoblah"}, int64(12))

	s.recache(context.Background(), record)
}

func TestRecacheWithPendingScore(t *testing.T) {
	s := InitTestServer(".runrecache")
	record := &pb.Record{Release: &pbgd.Release{Rating: 12, Tracklist: []*pbgd.Track{&pbgd.Track{Title: "One"}}}, Metadata: &pb.ReleaseMetadata{NeedsStockCheck: true, LastStockCheck: time.Now().Unix(), SetRating: 12}}
	s.syncRecords(context.Background(), record, &pbgd.Release{Rating: 24, FolderId: 12, Tracklist: []*pbgd.Track{&pbgd.Track{Title: "Two"}, &pbgd.Track{Title: "Three"}}, RecordCondition: "blah", SleeveCondition: "alsoblah"}, int64(12))

	err := s.recache(context.Background(), record)

	if err == nil {
		t.Errorf("recache failed")
	}
}

func TestUpdateSale(t *testing.T) {
	s := InitTestServer(".testupdatesale")
	//s.recordCache[int32(1234)] = &pb.Record{Metadata: &pb.ReleaseMetadata{SaleId: 12}}
	s.updateSale(context.Background(), int32(1234))
}

func TestValidateSales(t *testing.T) {
	s := InitTestServer(".testValidateSales")
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{
		Release:  &pbgd.Release{Title: "title1", InstanceId: 123, Id: 123},
		Metadata: &pb.ReleaseMetadata{LastUpdateIn: 1, Cost: 100, SaleId: 123, GoalFolder: 100, LastCache: time.Now().Unix(), Category: pb.ReleaseMetadata_LISTED_TO_SELL}}})
	err := s.validateSales(context.Background())
	if err != nil {
		t.Errorf("Bad validation: %v", err)
	}
}

func TestValidateSalesNotFound(t *testing.T) {
	s := InitTestServer(".testValidateSales")

	err := s.validateSales(context.Background())
	if err == nil {
		t.Errorf("Bad validation: %v", err)
	}
}

func TestValidateSalesBadget(t *testing.T) {
	s := InitTestServer(".testValidateSales")
	s.retr = &testSyncer{badInventory: true}
	err := s.validateSales(context.Background())
	if err == nil {
		t.Errorf("Validation did not fail")
	}
}
func TestValidateSalesBadLoad(t *testing.T) {
	s := InitTestServer(".testValidateSales")
	//s.collection.InstanceToId[1234] = 123
	err := s.validateSales(context.Background())
	if err == nil {
		t.Errorf("Validation did not fail")
	}
}
