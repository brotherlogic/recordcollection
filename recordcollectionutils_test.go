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
	"github.com/prometheus/client_golang/prometheus/testutil"
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

type testGenerator struct {
	fail bool
	desc string
}

func (t *testGenerator) generate(ctx context.Context, address string, rec *pb.Record) (string, error) {
	if t.fail {
		return "", errors.New("Built to fail")
	}
	if t.desc != "" {
		return t.desc, nil
	}
	return "Generated Description", nil
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
	lastSaleNotes   string
	lastSalePrice   float32
	removedSaleID   int
	instanceInfo        map[int64]*godiscogs.InstanceInfo
	saleID              int64
	currentSaleStateErr    error
	currentSaleState       pbgd.SaleState
	getCurrentSaleCalls    int
	lastUpdatedSaleNotes   string
	lastUpdatedSaleStatus  string
	lastUpdatedSalePrice   float32
	updateSaleListingCalls int
	failUpdateSaleListing  bool
}

func (t *testSyncer) GetInstanceInfo(ctx context.Context, ID int32) (map[int64]*godiscogs.InstanceInfo, error) {
	if t.instanceInfo != nil {
		return t.instanceInfo, nil
	}
	return map[int64]*godiscogs.InstanceInfo{
		123: {
			PackageScore: "4",
		},
	}, nil
}

func (t *testSyncer) GetInventory(ctx context.Context) ([]*pbgd.ForSale, error) {
	if t.badInventory {
		return []*pbgd.ForSale{}, fmt.Errorf("Built to fail")
	}
	return []*pbgd.ForSale{&pbgd.ForSale{Id: 123, SaleId: 123}}, nil
}

func (t *testSyncer) ExpireSale(ctx context.Context, saleID int64, releaseID int, price float32) error {
	return nil
}

func (t *testSyncer) UpdateSalePrice(ctx context.Context, saleID int, releaseID int, condition, sleeve string, price float32) error {
	if t.failSalePrice {
		return fmt.Errorf("built to fail")
	}
	return nil
}

func (t *testSyncer) UpdateSaleListing(ctx context.Context, saleID int64, releaseID int, condition, sleeve, state, notes string, price float32) error {
	t.updateSaleListingCalls++
	if t.failUpdateSaleListing {
		return fmt.Errorf("built to fail")
	}
	t.lastUpdatedSaleNotes = notes
	t.lastUpdatedSaleStatus = state
	t.lastUpdatedSalePrice = price
	return nil
}

func (t *testSyncer) RemoveFromSale(ctx context.Context, saleID int, releaseID int) error {
	t.removedSaleID = saleID
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

func (t *testSyncer) SellRecord(ctx context.Context, releaseID int, price float32, state string, condition, sleeve string, weight int, notes string) (int64, error) {
	t.lastSaleNotes = notes
	t.lastSalePrice = price
	if t.saleID != 0 {
		return t.saleID, nil
	}
	return 1001, nil
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
	t.getCurrentSaleCalls++
	if t.currentSaleStateErr != nil {
		return pbgd.SaleState_NOT_FOR_SALE, t.currentSaleStateErr
	}
	if t.currentSaleState != 0 {
		return t.currentSaleState, nil
	}
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
	s.cacheRecord(context.Background(), r, true)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r, true)

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
	s.cacheRecord(context.Background(), r, true)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r, true)

	if r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been double cached: %v", r)
	}
}

func TestImageMergeWithFailScore(t *testing.T) {
	s := InitTestServer(".testImageMerge")
	s.scorer = &testScorer{fail: true}
	r := &pb.Record{Release: &pbgd.Release{Id: 4707982, InstanceId: 236418222}, Metadata: &pb.ReleaseMetadata{}}
	s.cacheRecord(context.Background(), r, true)

	if r.GetRelease().Title != "Future" || r.GetMetadata().LastCache == 0 {
		t.Fatalf("Record has not been recached %v", r)
	}

	r.Metadata.LastCache = 0
	s.cacheRecord(context.Background(), r, true)

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

func TestGodiscogsPackageScoreSupported(t *testing.T) {
	info := &godiscogs.InstanceInfo{
		PackageScore: "5",
	}
	if info.PackageScore != "5" {
		t.Errorf("PackageScore mismatch: got %v, want 5", info.PackageScore)
	}
}

func TestParsePackageScore(t *testing.T) {
	s := InitTestServer(".testParsePackageScore")

	tests := []struct {
		name         string
		raw          string
		currentScore int32
		instanceID   int64
		expected     int32
	}{
		{
			name:         "Discogs input 0 sets PackageScore = 0",
			raw:          "0",
			currentScore: -1,
			instanceID:   123,
			expected:     0,
		},
		{
			name:         "Discogs input 1 sets PackageScore = 1",
			raw:          "1",
			currentScore: -1,
			instanceID:   123,
			expected:     1,
		},
		{
			name:         "Discogs input 2 sets PackageScore = 2",
			raw:          "2",
			currentScore: -1,
			instanceID:   123,
			expected:     2,
		},
		{
			name:         "Discogs input 3 sets PackageScore = 3",
			raw:          "3",
			currentScore: -1,
			instanceID:   123,
			expected:     3,
		},
		{
			name:         "Discogs input 4 sets PackageScore = 4",
			raw:          "4",
			currentScore: -1,
			instanceID:   123,
			expected:     4,
		},
		{
			name:         "Discogs input 5 sets PackageScore = 5",
			raw:          "5",
			currentScore: -1,
			instanceID:   123,
			expected:     5,
		},
		{
			name:         "Discogs input empty sets PackageScore = -1",
			raw:          "",
			currentScore: 3,
			instanceID:   123,
			expected:     -1,
		},
		{
			name:         "Out-of-bounds low -2 calls s.RaiseIssue and retains previous score",
			raw:          "-2",
			currentScore: 3,
			instanceID:   123,
			expected:     3,
		},
		{
			name:         "Out-of-bounds low -2 with 0/unset returns -1",
			raw:          "-2",
			currentScore: 0,
			instanceID:   123,
			expected:     -1,
		},
		{
			name:         "Out-of-bounds high 6 calls s.RaiseIssue and retains previous score",
			raw:          "6",
			currentScore: 4,
			instanceID:   123,
			expected:     4,
		},
		{
			name:         "Out-of-bounds high 10 calls s.RaiseIssue and retains previous score",
			raw:          "10",
			currentScore: 2,
			instanceID:   123,
			expected:     2,
		},
		{
			name:         "Out-of-bounds high with 0/unset returns -1",
			raw:          "6",
			currentScore: 0,
			instanceID:   123,
			expected:     -1,
		},
		{
			name:         "Non-integer string poor calls s.RaiseIssue and retains previous score",
			raw:          "poor",
			currentScore: 5,
			instanceID:   123,
			expected:     5,
		},
		{
			name:         "Non-integer string N/A calls s.RaiseIssue and retains previous score",
			raw:          "N/A",
			currentScore: 1,
			instanceID:   123,
			expected:     1,
		},
		{
			name:         "Non-integer string with 0/unset returns -1",
			raw:          "N/A",
			currentScore: 0,
			instanceID:   123,
			expected:     -1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.parsePackageScore(tc.raw, tc.currentScore, tc.instanceID)
			if got != tc.expected {
				t.Errorf("s.parsePackageScore(%q, %v, %v) = %v; want %v", tc.raw, tc.currentScore, tc.instanceID, got, tc.expected)
			}
			gotFn := parsePackageScore(tc.raw, tc.currentScore, tc.instanceID, s)
			if gotFn != tc.expected {
				t.Errorf("parsePackageScore(%q, %v, %v, s) = %v; want %v", tc.raw, tc.currentScore, tc.instanceID, gotFn, tc.expected)
			}
		})
	}
}

func TestNewRecordInitializationDefaultsPackageScore(t *testing.T) {
	s := InitTestServer(".testNewRecordInit")
	resp, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{
		ToAdd: &pb.Record{
			Release: &pbgd.Release{Id: 12345, Title: "Test Album"},
			Metadata: &pb.ReleaseMetadata{
				Cost:       10,
				GoalFolder: 12,
			},
		},
	})
	if err != nil {
		t.Fatalf("AddRecord failed: %v", err)
	}
	if resp.GetAdded().GetMetadata().GetPackageScore() != -1 {
		t.Errorf("Expected PackageScore to default to -1, got %v", resp.GetAdded().GetMetadata().GetPackageScore())
	}
}

func TestCacheRecordExtractsPackageScore(t *testing.T) {
	s := InitTestServer(".testCacheRecordExtractsPackageScore")
	tRetr := &testSyncer{
		instanceInfo: map[int64]*godiscogs.InstanceInfo{
			9876: {
				PackageScore: "4",
			},
		},
	}
	s.retr = tRetr

	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         100,
			InstanceId: 9876,
			Title:      "Test Release",
		},
		Metadata: &pb.ReleaseMetadata{
			PackageScore: -1,
		},
	}

	err := s.cacheRecord(context.Background(), r, true)
	if err != nil {
		t.Fatalf("cacheRecord failed: %v", err)
	}

	if r.GetMetadata().GetPackageScore() != 4 {
		t.Errorf("PackageScore not extracted: got %v, want 4", r.GetMetadata().GetPackageScore())
	}
}

func TestCacheRecordPackageScoreEmpty(t *testing.T) {
	s := InitTestServer(".testCacheRecordPackageScoreEmpty")
	tRetr := &testSyncer{
		instanceInfo: map[int64]*godiscogs.InstanceInfo{
			9876: {
				PackageScore: "",
			},
		},
	}
	s.retr = tRetr

	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         100,
			InstanceId: 9876,
			Title:      "Test Release",
		},
		Metadata: &pb.ReleaseMetadata{
			PackageScore: 3,
		},
	}

	err := s.cacheRecord(context.Background(), r, true)
	if err != nil {
		t.Fatalf("cacheRecord failed: %v", err)
	}

	if r.GetMetadata().GetPackageScore() != -1 {
		t.Errorf("PackageScore not reset to -1: got %v, want -1", r.GetMetadata().GetPackageScore())
	}
}

func TestCacheRecordPackageScoreInvalidRetainsScore(t *testing.T) {
	s := InitTestServer(".testCacheRecordPackageScoreInvalidRetainsScore")
	tRetr := &testSyncer{
		instanceInfo: map[int64]*godiscogs.InstanceInfo{
			9876: {
				PackageScore: "invalid",
			},
		},
	}
	s.retr = tRetr

	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         100,
			InstanceId: 9876,
			Title:      "Test Release",
		},
		Metadata: &pb.ReleaseMetadata{
			PackageScore: 3,
		},
	}

	err := s.cacheRecord(context.Background(), r, true)
	if err != nil {
		t.Fatalf("cacheRecord failed: %v", err)
	}

	if r.GetMetadata().GetPackageScore() != 3 {
		t.Errorf("PackageScore should be retained on invalid: got %v, want 3", r.GetMetadata().GetPackageScore())
	}
}

func TestCacheRecordPackageScoreDefaultMockData(t *testing.T) {
	s := InitTestServer(".testCacheRecordPackageScoreDefaultMockData")
	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         100,
			InstanceId: 123,
			Title:      "Test Release",
		},
		Metadata: &pb.ReleaseMetadata{
			PackageScore: -1,
		},
	}

	err := s.cacheRecord(context.Background(), r, true)
	if err != nil {
		t.Fatalf("cacheRecord failed: %v", err)
	}

	if r.GetMetadata().GetPackageScore() != 4 {
		t.Errorf("PackageScore default mock data not extracted: got %v, want 4", r.GetMetadata().GetPackageScore())
	}
}

func TestReconcileDeletedSale(t *testing.T) {
	tests := []struct {
		name             string
		initialCategory  pb.ReleaseMetadata_Category
		initialSaleId    int64
		initialSalePrice int32
		initialCurrPrice int32
		initialSaleState pbgd.SaleState
		initialSaleDirty bool
		takenToStore     bool
		expectedCategory pb.ReleaseMetadata_Category
	}{
		{
			name:             "ListedToSell transitions to PrepareToSell and resets fields",
			initialCategory:  pb.ReleaseMetadata_LISTED_TO_SELL,
			initialSaleId:    12345,
			initialSalePrice: 2000,
			initialCurrPrice: 1900,
			initialSaleState: pbgd.SaleState_FOR_SALE,
			initialSaleDirty: true,
			expectedCategory: pb.ReleaseMetadata_PREPARE_TO_SELL,
		},
		{
			name:             "StaleSale transitions to PrepareToSell and resets fields",
			initialCategory:  pb.ReleaseMetadata_STALE_SALE,
			initialSaleId:    54321,
			initialSalePrice: 1500,
			initialCurrPrice: 1500,
			initialSaleState: pbgd.SaleState_FOR_SALE,
			initialSaleDirty: true,
			expectedCategory: pb.ReleaseMetadata_PREPARE_TO_SELL,
		},
		{
			name:             "ListedToSell with TakenToStore preserves category and resets fields",
			initialCategory:  pb.ReleaseMetadata_LISTED_TO_SELL,
			initialSaleId:    12346,
			initialSalePrice: 2000,
			initialCurrPrice: 1900,
			initialSaleState: pbgd.SaleState_FOR_SALE,
			initialSaleDirty: true,
			takenToStore:     true,
			expectedCategory: pb.ReleaseMetadata_LISTED_TO_SELL,
		},
		{
			name:             "StaleSale with TakenToStore preserves category and resets fields",
			initialCategory:  pb.ReleaseMetadata_STALE_SALE,
			initialSaleId:    54322,
			initialSalePrice: 1500,
			initialCurrPrice: 1500,
			initialSaleState: pbgd.SaleState_FOR_SALE,
			initialSaleDirty: true,
			takenToStore:     true,
			expectedCategory: pb.ReleaseMetadata_STALE_SALE,
		},
		{
			name:             "Non-sale category IN_COLLECTION is preserved and resets fields",
			initialCategory:  pb.ReleaseMetadata_IN_COLLECTION,
			initialSaleId:    999,
			initialSalePrice: 1000,
			initialCurrPrice: 1000,
			initialSaleState: pbgd.SaleState_FOR_SALE,
			initialSaleDirty: true,
			expectedCategory: pb.ReleaseMetadata_IN_COLLECTION,
		},
		{
			name:             "Non-sale category SOLD is preserved and resets fields",
			initialCategory:  pb.ReleaseMetadata_SOLD,
			initialSaleId:    888,
			initialSalePrice: 1200,
			initialCurrPrice: 1200,
			initialSaleState: pbgd.SaleState_SOLD,
			initialSaleDirty: true,
			expectedCategory: pb.ReleaseMetadata_SOLD,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := InitTestServer(fmt.Sprintf(".testReconcileDeletedSale_%d", i))
			ctx := context.Background()
			instanceID := int64(1000 + i)

			r := &pb.Record{
				Release: &pbgd.Release{
					Id:         int32(2000 + i),
					InstanceId: instanceID,
					Title:      "Reconciliation Test",
				},
				Metadata: &pb.ReleaseMetadata{
					Category:               tc.initialCategory,
					SaleId:                 tc.initialSaleId,
					SalePrice:              tc.initialSalePrice,
					CurrentSalePrice:       tc.initialCurrPrice,
					SaleState:              tc.initialSaleState,
					SaleDirty:              tc.initialSaleDirty,
					RefreshSaleDescription: true,
					TakenToStore:           tc.takenToStore,
				},
			}

			// Pre-save record in server storage
			err := s.saveRecord(ctx, r)
			if err != nil {
				t.Fatalf("Failed to pre-save record: %v", err)
			}

			metricBefore := testutil.ToFloat64(deletedSaleReconciled)

			// Call reconcileDeletedSale
			err = s.reconcileDeletedSale(ctx, r)
			if err != nil {
				t.Fatalf("reconcileDeletedSale failed: %v", err)
			}

			metricAfter := testutil.ToFloat64(deletedSaleReconciled)
			if metricAfter-metricBefore != 1 {
				t.Errorf("deletedSaleReconciled metric increment: got diff %v, want 1", metricAfter-metricBefore)
			}

			// Assert in-memory record modifications
			if r.GetMetadata().GetSaleId() != -1 {
				t.Errorf("SaleId: got %v, want -1", r.GetMetadata().GetSaleId())
			}
			if r.GetMetadata().GetSalePrice() != -1 {
				t.Errorf("SalePrice: got %v, want -1", r.GetMetadata().GetSalePrice())
			}
			if r.GetMetadata().GetCurrentSalePrice() != -1 {
				t.Errorf("CurrentSalePrice: got %v, want -1", r.GetMetadata().GetCurrentSalePrice())
			}
			if r.GetMetadata().GetSaleState() != pbgd.SaleState_NOT_FOR_SALE {
				t.Errorf("SaleState: got %v, want %v", r.GetMetadata().GetSaleState(), pbgd.SaleState_NOT_FOR_SALE)
			}
			if r.GetMetadata().GetSaleDirty() != false {
				t.Errorf("SaleDirty: got %v, want false", r.GetMetadata().GetSaleDirty())
			}
			if r.GetMetadata().GetRefreshSaleDescription() != false {
				t.Errorf("RefreshSaleDescription: got %v, want false", r.GetMetadata().GetRefreshSaleDescription())
			}
			if r.GetMetadata().GetCategory() != tc.expectedCategory {
				t.Errorf("Category: got %v, want %v", r.GetMetadata().GetCategory(), tc.expectedCategory)
			}

			// Assert record updates are saved to storage
			savedRecord, err := s.loadRecord(ctx, instanceID, false)
			if err != nil {
				t.Fatalf("Failed to load saved record: %v", err)
			}
			if savedRecord.GetMetadata().GetSaleId() != -1 {
				t.Errorf("Persisted SaleId: got %v, want -1", savedRecord.GetMetadata().GetSaleId())
			}
			if savedRecord.GetMetadata().GetSalePrice() != -1 {
				t.Errorf("Persisted SalePrice: got %v, want -1", savedRecord.GetMetadata().GetSalePrice())
			}
			if savedRecord.GetMetadata().GetCurrentSalePrice() != -1 {
				t.Errorf("Persisted CurrentSalePrice: got %v, want -1", savedRecord.GetMetadata().GetCurrentSalePrice())
			}
			if savedRecord.GetMetadata().GetSaleState() != pbgd.SaleState_NOT_FOR_SALE {
				t.Errorf("Persisted SaleState: got %v, want %v", savedRecord.GetMetadata().GetSaleState(), pbgd.SaleState_NOT_FOR_SALE)
			}
			if savedRecord.GetMetadata().GetSaleDirty() != false {
				t.Errorf("Persisted SaleDirty: got %v, want false", savedRecord.GetMetadata().GetSaleDirty())
			}
			if savedRecord.GetMetadata().GetRefreshSaleDescription() != false {
				t.Errorf("Persisted RefreshSaleDescription: got %v, want false", savedRecord.GetMetadata().GetRefreshSaleDescription())
			}
			if savedRecord.GetMetadata().GetCategory() != tc.expectedCategory {
				t.Errorf("Persisted Category: got %v, want %v", savedRecord.GetMetadata().GetCategory(), tc.expectedCategory)
			}
		})
	}
}

func TestUpdateSale_DeletedSale_ReconcilesToPrepareToSell(t *testing.T) {
	s := InitTestServer(".testUpdateSale_DeletedSale_ReconcilesToPrepareToSell")
	ctx := context.Background()
	ts := &testSyncer{
		currentSaleStateErr: status.Errorf(codes.NotFound, "Sale not found"),
	}
	s.retr = ts

	instanceID := int64(1001)
	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         1001,
			InstanceId: instanceID,
			Title:      "Deleted Sale Record",
		},
		Metadata: &pb.ReleaseMetadata{
			Category:         pb.ReleaseMetadata_LISTED_TO_SELL,
			SaleId:           555,
			SalePrice:        2000,
			CurrentSalePrice: 1900,
			SaleState:        pbgd.SaleState_FOR_SALE,
			SaleDirty:        true,
		},
	}
	err := s.saveRecord(ctx, r)
	if err != nil {
		t.Fatalf("Failed to save initial record: %v", err)
	}

	err = s.updateSale(ctx, int32(instanceID))
	if err != nil {
		t.Fatalf("updateSale failed unexpectedly: %v", err)
	}

	saved, err := s.loadRecord(ctx, instanceID, false)
	if err != nil {
		t.Fatalf("Failed to reload record: %v", err)
	}

	if saved.GetMetadata().GetSaleId() != -1 {
		t.Errorf("SaleId: got %v, want -1", saved.GetMetadata().GetSaleId())
	}
	if saved.GetMetadata().GetSalePrice() != -1 {
		t.Errorf("SalePrice: got %v, want -1", saved.GetMetadata().GetSalePrice())
	}
	if saved.GetMetadata().GetCurrentSalePrice() != -1 {
		t.Errorf("CurrentSalePrice: got %v, want -1", saved.GetMetadata().GetCurrentSalePrice())
	}
	if saved.GetMetadata().GetSaleState() != pbgd.SaleState_NOT_FOR_SALE {
		t.Errorf("SaleState: got %v, want %v", saved.GetMetadata().GetSaleState(), pbgd.SaleState_NOT_FOR_SALE)
	}
	if saved.GetMetadata().GetSaleDirty() != false {
		t.Errorf("SaleDirty: got %v, want false", saved.GetMetadata().GetSaleDirty())
	}
	if saved.GetMetadata().GetCategory() != pb.ReleaseMetadata_PREPARE_TO_SELL {
		t.Errorf("Category: got %v, want %v", saved.GetMetadata().GetCategory(), pb.ReleaseMetadata_PREPARE_TO_SELL)
	}
}

func TestUpdateSale_DeletedSale_StaleSale_ReconcilesToPrepareToSell(t *testing.T) {
	s := InitTestServer(".testUpdateSale_DeletedSale_StaleSale_ReconcilesToPrepareToSell")
	ctx := context.Background()
	ts := &testSyncer{
		currentSaleStateErr: status.Errorf(codes.NotFound, "Sale not found"),
	}
	s.retr = ts

	instanceID := int64(1002)
	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         1002,
			InstanceId: instanceID,
			Title:      "Stale Sale Record",
		},
		Metadata: &pb.ReleaseMetadata{
			Category:         pb.ReleaseMetadata_STALE_SALE,
			SaleId:           666,
			SalePrice:        1500,
			CurrentSalePrice: 1500,
			SaleState:        pbgd.SaleState_FOR_SALE,
			SaleDirty:        true,
		},
	}
	err := s.saveRecord(ctx, r)
	if err != nil {
		t.Fatalf("Failed to save initial record: %v", err)
	}

	err = s.updateSale(ctx, int32(instanceID))
	if err != nil {
		t.Fatalf("updateSale failed unexpectedly: %v", err)
	}

	saved, err := s.loadRecord(ctx, instanceID, false)
	if err != nil {
		t.Fatalf("Failed to reload record: %v", err)
	}

	if saved.GetMetadata().GetSaleId() != -1 {
		t.Errorf("SaleId: got %v, want -1", saved.GetMetadata().GetSaleId())
	}
	if saved.GetMetadata().GetSalePrice() != -1 {
		t.Errorf("SalePrice: got %v, want -1", saved.GetMetadata().GetSalePrice())
	}
	if saved.GetMetadata().GetCurrentSalePrice() != -1 {
		t.Errorf("CurrentSalePrice: got %v, want -1", saved.GetMetadata().GetCurrentSalePrice())
	}
	if saved.GetMetadata().GetSaleState() != pbgd.SaleState_NOT_FOR_SALE {
		t.Errorf("SaleState: got %v, want %v", saved.GetMetadata().GetSaleState(), pbgd.SaleState_NOT_FOR_SALE)
	}
	if saved.GetMetadata().GetSaleDirty() != false {
		t.Errorf("SaleDirty: got %v, want false", saved.GetMetadata().GetSaleDirty())
	}
	if saved.GetMetadata().GetCategory() != pb.ReleaseMetadata_PREPARE_TO_SELL {
		t.Errorf("Category: got %v, want %v", saved.GetMetadata().GetCategory(), pb.ReleaseMetadata_PREPARE_TO_SELL)
	}
}

func TestUpdateSale_DeletedSale_NonSaleCategoryPreserved(t *testing.T) {
	categories := []pb.ReleaseMetadata_Category{
		pb.ReleaseMetadata_IN_COLLECTION,
		pb.ReleaseMetadata_SOLD,
	}

	for i, cat := range categories {
		t.Run(cat.String(), func(t *testing.T) {
			s := InitTestServer(fmt.Sprintf(".testUpdateSale_DeletedSale_NonSale_%d", i))
			ctx := context.Background()
			ts := &testSyncer{
				currentSaleStateErr: status.Errorf(codes.NotFound, "Sale not found"),
			}
			s.retr = ts

			instanceID := int64(1003 + i)
			r := &pb.Record{
				Release: &pbgd.Release{
					Id:         int32(1003 + i),
					InstanceId: instanceID,
					Title:      "Non Sale Category Record",
				},
				Metadata: &pb.ReleaseMetadata{
					Category:         cat,
					SaleId:           777,
					SalePrice:        1000,
					CurrentSalePrice: 1000,
					SaleState:        pbgd.SaleState_FOR_SALE,
					SaleDirty:        true,
				},
			}
			err := s.saveRecord(ctx, r)
			if err != nil {
				t.Fatalf("Failed to save initial record: %v", err)
			}

			err = s.updateSale(ctx, int32(instanceID))
			if err != nil {
				t.Fatalf("updateSale failed unexpectedly: %v", err)
			}

			saved, err := s.loadRecord(ctx, instanceID, false)
			if err != nil {
				t.Fatalf("Failed to reload record: %v", err)
			}

			if saved.GetMetadata().GetSaleId() != -1 {
				t.Errorf("SaleId: got %v, want -1", saved.GetMetadata().GetSaleId())
			}
			if saved.GetMetadata().GetSalePrice() != -1 {
				t.Errorf("SalePrice: got %v, want -1", saved.GetMetadata().GetSalePrice())
			}
			if saved.GetMetadata().GetCurrentSalePrice() != -1 {
				t.Errorf("CurrentSalePrice: got %v, want -1", saved.GetMetadata().GetCurrentSalePrice())
			}
			if saved.GetMetadata().GetSaleState() != pbgd.SaleState_NOT_FOR_SALE {
				t.Errorf("SaleState: got %v, want %v", saved.GetMetadata().GetSaleState(), pbgd.SaleState_NOT_FOR_SALE)
			}
			if saved.GetMetadata().GetSaleDirty() != false {
				t.Errorf("SaleDirty: got %v, want false", saved.GetMetadata().GetSaleDirty())
			}
			if saved.GetMetadata().GetCategory() != cat {
				t.Errorf("Category: got %v, want %v", saved.GetMetadata().GetCategory(), cat)
			}
		})
	}
}

func TestUpdateSale_TransientError_Propagates(t *testing.T) {
	s := InitTestServer(".testUpdateSale_TransientError_Propagates")
	ctx := context.Background()
	ts := &testSyncer{
		currentSaleStateErr: status.Errorf(codes.ResourceExhausted, "Rate limited"),
	}
	s.retr = ts

	instanceID := int64(1010)
	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         1010,
			InstanceId: instanceID,
			Title:      "Transient Error Record",
		},
		Metadata: &pb.ReleaseMetadata{
			Category:         pb.ReleaseMetadata_LISTED_TO_SELL,
			SaleId:           999,
			SalePrice:        2000,
			CurrentSalePrice: 2000,
			SaleState:        pbgd.SaleState_FOR_SALE,
			SaleDirty:        true,
		},
	}
	err := s.saveRecord(ctx, r)
	if err != nil {
		t.Fatalf("Failed to save initial record: %v", err)
	}

	err = s.updateSale(ctx, int32(instanceID))
	if err == nil {
		t.Fatalf("Expected error from updateSale, got nil")
	}
	if status.Code(err) != codes.ResourceExhausted {
		t.Errorf("Error code: got %v, want %v", status.Code(err), codes.ResourceExhausted)
	}

	saved, err := s.loadRecord(ctx, instanceID, false)
	if err != nil {
		t.Fatalf("Failed to reload record: %v", err)
	}

	if saved.GetMetadata().GetSaleId() != 999 {
		t.Errorf("SaleId: got %v, want 999 (should not be modified on transient error)", saved.GetMetadata().GetSaleId())
	}
	if saved.GetMetadata().GetCategory() != pb.ReleaseMetadata_LISTED_TO_SELL {
		t.Errorf("Category: got %v, want %v", saved.GetMetadata().GetCategory(), pb.ReleaseMetadata_LISTED_TO_SELL)
	}
}

func TestUpdateSale_IdempotentOnResetRecord(t *testing.T) {
	s := InitTestServer(".testUpdateSale_IdempotentOnResetRecord")
	ctx := context.Background()
	ts := &testSyncer{
		currentSaleStateErr: status.Errorf(codes.NotFound, "Sale not found"),
	}
	s.retr = ts

	instanceID := int64(1020)
	r := &pb.Record{
		Release: &pbgd.Release{
			Id:         1020,
			InstanceId: instanceID,
			Title:      "Already Reset Record",
		},
		Metadata: &pb.ReleaseMetadata{
			Category:         pb.ReleaseMetadata_PREPARE_TO_SELL,
			SaleId:           -1,
			SalePrice:        -1,
			CurrentSalePrice: -1,
			SaleState:        pbgd.SaleState_NOT_FOR_SALE,
			SaleDirty:        false,
		},
	}
	err := s.saveRecord(ctx, r)
	if err != nil {
		t.Fatalf("Failed to save initial record: %v", err)
	}

	err = s.updateSale(ctx, int32(instanceID))
	if err != nil {
		t.Fatalf("updateSale failed unexpectedly: %v", err)
	}

	if ts.getCurrentSaleCalls != 0 {
		t.Errorf("getCurrentSaleCalls: got %v, want 0", ts.getCurrentSaleCalls)
	}
}

func TestCacheRecord_AutoFlagsRefreshOnInputChange(t *testing.T) {
	tests := []struct {
		name           string
		category       pb.ReleaseMetadata_Category
		saleID         int64
		saleState      pbgd.SaleState
		initialRecCond string
		initialSlvCond string
		initialNotes   string
		newRecCond     string
		newSlvCond     string
		newNotes       string
		wantFlagged    bool
	}{
		{
			name:           "ActiveListing_NotesChanged",
			category:       pb.ReleaseMetadata_LISTED_TO_SELL,
			saleID:         500,
			saleState:      pbgd.SaleState_FOR_SALE,
			initialRecCond: "VG+",
			initialSlvCond: "VG+",
			initialNotes:   "Original notes",
			newRecCond:     "VG+",
			newSlvCond:     "VG+",
			newNotes:       "Updated notes",
			wantFlagged:    true,
		},
		{
			name:           "ActiveListing_RecordConditionChanged",
			category:       pb.ReleaseMetadata_LISTED_TO_SELL,
			saleID:         500,
			saleState:      pbgd.SaleState_FOR_SALE,
			initialRecCond: "VG+",
			initialSlvCond: "VG+",
			initialNotes:   "Original notes",
			newRecCond:     "NM",
			newSlvCond:     "VG+",
			newNotes:       "Original notes",
			wantFlagged:    true,
		},
		{
			name:           "StaleSale_SleeveConditionChanged",
			category:       pb.ReleaseMetadata_STALE_SALE,
			saleID:         500,
			saleState:      pbgd.SaleState_EXPIRED,
			initialRecCond: "VG+",
			initialSlvCond: "VG+",
			initialNotes:   "Original notes",
			newRecCond:     "VG+",
			newSlvCond:     "VG",
			newNotes:       "Original notes",
			wantFlagged:    true,
		},
		{
			name:           "ActiveListing_UnchangedInputs",
			category:       pb.ReleaseMetadata_LISTED_TO_SELL,
			saleID:         500,
			saleState:      pbgd.SaleState_FOR_SALE,
			initialRecCond: "VG+",
			initialSlvCond: "VG+",
			initialNotes:   "Original notes",
			newRecCond:     "VG+",
			newSlvCond:     "VG+",
			newNotes:       "Original notes",
			wantFlagged:    false,
		},
		{
			name:           "UnlistedRecord_NotesChanged",
			category:       pb.ReleaseMetadata_IN_COLLECTION,
			saleID:         0,
			saleState:      pbgd.SaleState_NOT_FOR_SALE,
			initialRecCond: "VG+",
			initialSlvCond: "VG+",
			initialNotes:   "Original notes",
			newRecCond:     "NM",
			newSlvCond:     "NM",
			newNotes:       "Updated notes",
			wantFlagged:    false,
		},
		{
			name:           "SoldRecord_NotesChanged",
			category:       pb.ReleaseMetadata_LISTED_TO_SELL,
			saleID:         500,
			saleState:      pbgd.SaleState_SOLD,
			initialRecCond: "VG+",
			initialSlvCond: "VG+",
			initialNotes:   "Original notes",
			newRecCond:     "NM",
			newSlvCond:     "NM",
			newNotes:       "Updated notes",
			wantFlagged:    false,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := InitTestServer(fmt.Sprintf(".testCacheRecord_AutoFlagsRefresh_%d", i))
			ctx := context.Background()
			instanceID := int64(4000 + i)

			s.retr = &testSyncer{
				instanceInfo: map[int64]*godiscogs.InstanceInfo{
					instanceID: {
						RecordCondition: tc.newRecCond,
						SleeveCondition: tc.newSlvCond,
						Notes:           tc.newNotes,
					},
				},
			}

			r := &pb.Record{
				Release: &pbgd.Release{
					Id:              int32(4000 + i),
					InstanceId:      instanceID,
					Title:           "Cache Refresh Test",
					RecordCondition: tc.initialRecCond,
					SleeveCondition: tc.initialSlvCond,
					Tracklist:       []*pbgd.Track{{Title: "Track 1"}},
				},
				Metadata: &pb.ReleaseMetadata{
					Category:  tc.category,
					SaleId:    tc.saleID,
					SaleState: tc.saleState,
					Notes:     tc.initialNotes,
					LastCache: time.Now().Unix(),
				},
			}

			if err := s.cacheRecord(ctx, r, true); err != nil {
				t.Fatalf("cacheRecord failed: %v", err)
			}

			if r.GetMetadata().GetRefreshSaleDescription() != tc.wantFlagged {
				t.Errorf("RefreshSaleDescription: got %v, want %v", r.GetMetadata().GetRefreshSaleDescription(), tc.wantFlagged)
			}
			if r.GetMetadata().GetSaleDirty() != tc.wantFlagged {
				t.Errorf("SaleDirty: got %v, want %v", r.GetMetadata().GetSaleDirty(), tc.wantFlagged)
			}
		})
	}
}

func TestSyncRecords_AutoFlagsRefreshOnConditionChange(t *testing.T) {
	tests := []struct {
		name               string
		category           pb.ReleaseMetadata_Category
		saleID             int64
		saleState          pbgd.SaleState
		initialRecCond     string
		initialSlvCond     string
		incomingRecCond    string
		incomingSlvCond    string
		wantRefreshFlagged bool
		wantSaleDirty      bool
	}{
		{
			name:               "ActiveListing_RecordConditionChanged",
			category:           pb.ReleaseMetadata_LISTED_TO_SELL,
			saleID:             500,
			saleState:          pbgd.SaleState_FOR_SALE,
			initialRecCond:     "VG+",
			initialSlvCond:     "VG+",
			incomingRecCond:    "NM",
			incomingSlvCond:    "VG+",
			wantRefreshFlagged: true,
			wantSaleDirty:      true,
		},
		{
			name:               "StaleSale_SleeveConditionChanged",
			category:           pb.ReleaseMetadata_STALE_SALE,
			saleID:             500,
			saleState:          pbgd.SaleState_EXPIRED,
			initialRecCond:     "VG+",
			initialSlvCond:     "VG+",
			incomingRecCond:    "VG+",
			incomingSlvCond:    "VG",
			wantRefreshFlagged: true,
			wantSaleDirty:      true,
		},
		{
			name:               "ActiveListing_ConditionsUnchanged",
			category:           pb.ReleaseMetadata_LISTED_TO_SELL,
			saleID:             500,
			saleState:          pbgd.SaleState_FOR_SALE,
			initialRecCond:     "VG+",
			initialSlvCond:     "VG+",
			incomingRecCond:    "VG+",
			incomingSlvCond:    "VG+",
			wantRefreshFlagged: false,
			wantSaleDirty:      false,
		},
		{
			name:               "UnlistedRecord_RecordConditionChanged",
			category:           pb.ReleaseMetadata_IN_COLLECTION,
			saleID:             0,
			saleState:          pbgd.SaleState_NOT_FOR_SALE,
			initialRecCond:     "VG",
			initialSlvCond:     "VG",
			incomingRecCond:    "VG+",
			incomingSlvCond:    "VG+",
			wantRefreshFlagged: false,
			wantSaleDirty:      false,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := InitTestServer(fmt.Sprintf(".testSyncRecords_AutoFlagsRefresh_%d", i))
			ctx := context.Background()
			instanceID := int64(5000 + i)

			r := &pb.Record{
				Release: &pbgd.Release{
					Id:              int32(5000 + i),
					InstanceId:      instanceID,
					Title:           "Sync Refresh Test",
					RecordCondition: tc.initialRecCond,
					SleeveCondition: tc.initialSlvCond,
				},
				Metadata: &pb.ReleaseMetadata{
					Category:  tc.category,
					SaleId:    tc.saleID,
					SaleState: tc.saleState,
				},
			}

			incoming := &pbgd.Release{
				Id:              int32(5000 + i),
				InstanceId:      instanceID,
				RecordCondition: tc.incomingRecCond,
				SleeveCondition: tc.incomingSlvCond,
			}

			s.syncRecords(ctx, r, incoming, 1)

			if r.GetMetadata().GetRefreshSaleDescription() != tc.wantRefreshFlagged {
				t.Errorf("RefreshSaleDescription: got %v, want %v", r.GetMetadata().GetRefreshSaleDescription(), tc.wantRefreshFlagged)
			}
			if r.GetMetadata().GetSaleDirty() != tc.wantSaleDirty {
				t.Errorf("SaleDirty: got %v, want %v", r.GetMetadata().GetSaleDirty(), tc.wantSaleDirty)
			}
		})
	}
}

func TestTestSyncer_UpdateSaleListing(t *testing.T) {
	ts := &testSyncer{}
	var s saver = ts

	err := s.UpdateSaleListing(context.Background(), 1001, 2002, "VG+", "VG", "For Sale", "Updated description", 25.50)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if ts.updateSaleListingCalls != 1 {
		t.Errorf("updateSaleListingCalls: got %d, want 1", ts.updateSaleListingCalls)
	}
	if ts.lastUpdatedSaleNotes != "Updated description" {
		t.Errorf("lastUpdatedSaleNotes: got %q, want %q", ts.lastUpdatedSaleNotes, "Updated description")
	}
	if ts.lastUpdatedSaleStatus != "For Sale" {
		t.Errorf("lastUpdatedSaleStatus: got %q, want %q", ts.lastUpdatedSaleStatus, "For Sale")
	}
	if ts.lastUpdatedSalePrice != 25.50 {
		t.Errorf("lastUpdatedSalePrice: got %v, want 25.50", ts.lastUpdatedSalePrice)
	}

	ts.failUpdateSaleListing = true
	err = s.UpdateSaleListing(context.Background(), 1001, 2002, "VG+", "VG", "Expired", "Another description", 19.99)
	if err == nil {
		t.Fatalf("Expected error when failUpdateSaleListing is true, got nil")
	}
	if ts.updateSaleListingCalls != 2 {
		t.Errorf("updateSaleListingCalls: got %d, want 2", ts.updateSaleListingCalls)
	}
}

func TestPushSale_RefreshSaleDescription_Success(t *testing.T) {
	t.Run("ForSale_ExistingSalePrice", func(t *testing.T) {
		s := InitTestServer(".testPushSale_RefreshSaleDescription_ForSale")
		ctx := context.Background()
		ts := &testSyncer{}
		s.retr = ts
		s.generator = &testGenerator{desc: "Refreshed Description"}

		instanceID := int64(6001)
		r := &pb.Record{
			Release: &pbgd.Release{
				Id:              6001,
				InstanceId:      instanceID,
				Title:           "Refresh For Sale Album",
				RecordCondition: "Near Mint (NM or M-)",
				SleeveCondition: "Very Good Plus (VG+)",
			},
			Metadata: &pb.ReleaseMetadata{
				Category:               pb.ReleaseMetadata_LISTED_TO_SELL,
				SaleId:                 500,
				SalePrice:              2500,
				SaleState:              pbgd.SaleState_FOR_SALE,
				SaleDirty:              true,
				RefreshSaleDescription: true,
				SaleDescription:        "Old description",
				Notes:                  "Clean copy",
			},
		}
		if err := s.saveRecord(ctx, r); err != nil {
			t.Fatalf("saveRecord failed: %v", err)
		}

		pushed, err := s.pushSale(ctx, r)
		if err != nil {
			t.Fatalf("pushSale failed: %v", err)
		}
		if !pushed {
			t.Errorf("Expected pushed = true, got false")
		}
		if ts.updateSaleListingCalls != 1 {
			t.Errorf("updateSaleListingCalls: got %d, want 1", ts.updateSaleListingCalls)
		}
		if ts.lastUpdatedSaleNotes != "Refreshed Description" {
			t.Errorf("lastUpdatedSaleNotes: got %q, want %q", ts.lastUpdatedSaleNotes, "Refreshed Description")
		}
		if ts.lastUpdatedSaleStatus != "For Sale" {
			t.Errorf("lastUpdatedSaleStatus: got %q, want %q", ts.lastUpdatedSaleStatus, "For Sale")
		}
		if ts.lastUpdatedSalePrice != 25.0 {
			t.Errorf("lastUpdatedSalePrice: got %v, want 25.0", ts.lastUpdatedSalePrice)
		}
		if r.GetMetadata().GetSaleDescription() != "Refreshed Description" {
			t.Errorf("SaleDescription: got %q, want %q", r.GetMetadata().GetSaleDescription(), "Refreshed Description")
		}
		if r.GetMetadata().GetRefreshSaleDescription() {
			t.Errorf("Expected RefreshSaleDescription = false, got true")
		}
		if r.GetMetadata().GetSaleDirty() {
			t.Errorf("Expected SaleDirty = false, got true")
		}

		saved, err := s.loadRecord(ctx, instanceID, false)
		if err != nil {
			t.Fatalf("loadRecord failed: %v", err)
		}
		if saved.GetMetadata().GetSaleDescription() != "Refreshed Description" {
			t.Errorf("Persisted SaleDescription: got %q, want %q", saved.GetMetadata().GetSaleDescription(), "Refreshed Description")
		}
		if saved.GetMetadata().GetRefreshSaleDescription() {
			t.Errorf("Expected persisted RefreshSaleDescription = false, got true")
		}
		if saved.GetMetadata().GetSaleDirty() {
			t.Errorf("Expected persisted SaleDirty = false, got true")
		}
	})

	t.Run("Expired_WithNewSalePrice", func(t *testing.T) {
		s := InitTestServer(".testPushSale_RefreshSaleDescription_Expired")
		ctx := context.Background()
		ts := &testSyncer{}
		s.retr = ts
		s.generator = &testGenerator{desc: "Refreshed Expired Description"}

		instanceID := int64(6002)
		r := &pb.Record{
			Release: &pbgd.Release{
				Id:              6002,
				InstanceId:      instanceID,
				Title:           "Refresh Expired Album",
				RecordCondition: "Very Good Plus (VG+)",
				SleeveCondition: "Very Good (VG)",
			},
			Metadata: &pb.ReleaseMetadata{
				Category:               pb.ReleaseMetadata_STALE_SALE,
				SaleId:                 501,
				SalePrice:              2500,
				NewSalePrice:           2200,
				SaleState:              pbgd.SaleState_EXPIRED,
				SaleDirty:              true,
				RefreshSaleDescription: true,
				SaleDescription:        "Old description",
				Notes:                  "Slight shelf wear",
			},
		}
		if err := s.saveRecord(ctx, r); err != nil {
			t.Fatalf("saveRecord failed: %v", err)
		}

		pushed, err := s.pushSale(ctx, r)
		if err != nil {
			t.Fatalf("pushSale failed: %v", err)
		}
		if !pushed {
			t.Errorf("Expected pushed = true, got false")
		}
		if ts.updateSaleListingCalls != 1 {
			t.Errorf("updateSaleListingCalls: got %d, want 1", ts.updateSaleListingCalls)
		}
		if ts.lastUpdatedSaleNotes != "Refreshed Expired Description" {
			t.Errorf("lastUpdatedSaleNotes: got %q, want %q", ts.lastUpdatedSaleNotes, "Refreshed Expired Description")
		}
		if ts.lastUpdatedSaleStatus != "Expired" {
			t.Errorf("lastUpdatedSaleStatus: got %q, want %q", ts.lastUpdatedSaleStatus, "Expired")
		}
		if ts.lastUpdatedSalePrice != 22.0 {
			t.Errorf("lastUpdatedSalePrice: got %v, want 22.0", ts.lastUpdatedSalePrice)
		}
		if r.GetMetadata().GetSalePrice() != 2200 {
			t.Errorf("SalePrice: got %d, want 2200", r.GetMetadata().GetSalePrice())
		}
		if r.GetMetadata().GetNewSalePrice() != 0 {
			t.Errorf("NewSalePrice: got %d, want 0", r.GetMetadata().GetNewSalePrice())
		}
		if r.GetMetadata().GetLastSalePriceUpdate() == 0 {
			t.Errorf("Expected LastSalePriceUpdate to be set, got 0")
		}
		if r.GetMetadata().GetRefreshSaleDescription() {
			t.Errorf("Expected RefreshSaleDescription = false, got true")
		}
		if r.GetMetadata().GetSaleDirty() {
			t.Errorf("Expected SaleDirty = false, got true")
		}
	})
}

func TestPushSale_RefreshSaleDescription_MissingInputs(t *testing.T) {
	tests := []struct {
		name            string
		recordCondition string
		sleeveCondition string
		notes           string
	}{
		{
			name:            "MissingNotes",
			recordCondition: "VG+",
			sleeveCondition: "VG+",
			notes:           "",
		},
		{
			name:            "MissingRecordCondition",
			recordCondition: "",
			sleeveCondition: "VG+",
			notes:           "Clean copy",
		},
		{
			name:            "MissingSleeveCondition",
			recordCondition: "VG+",
			sleeveCondition: "",
			notes:           "Clean copy",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := InitTestServer(fmt.Sprintf(".testPushSale_RefreshMissingInputs_%d", i))
			ctx := context.Background()
			ts := &testSyncer{}
			s.retr = ts
			s.generator = &testGenerator{desc: "Should Not Be Used"}

			r := &pb.Record{
				Release: &pbgd.Release{
					Id:              int32(6100 + i),
					InstanceId:      int64(6100 + i),
					Title:           "Missing Inputs Album",
					RecordCondition: tc.recordCondition,
					SleeveCondition: tc.sleeveCondition,
				},
				Metadata: &pb.ReleaseMetadata{
					Category:               pb.ReleaseMetadata_LISTED_TO_SELL,
					SaleId:                 500,
					SalePrice:              2000,
					SaleState:              pbgd.SaleState_FOR_SALE,
					SaleDirty:              true,
					RefreshSaleDescription: true,
					SaleDescription:        "Existing Description",
					Notes:                  tc.notes,
				},
			}

			pushed, err := s.pushSale(ctx, r)
			if status.Code(err) != codes.FailedPrecondition {
				t.Errorf("Expected codes.FailedPrecondition, got %v (%v)", status.Code(err), err)
			}
			if pushed {
				t.Errorf("Expected pushed = false, got true")
			}
			if ts.updateSaleListingCalls != 0 {
				t.Errorf("updateSaleListingCalls: got %d, want 0", ts.updateSaleListingCalls)
			}
			if r.GetMetadata().GetSaleDescription() != "Existing Description" {
				t.Errorf("SaleDescription: got %q, want %q", r.GetMetadata().GetSaleDescription(), "Existing Description")
			}
			if !r.GetMetadata().GetRefreshSaleDescription() {
				t.Errorf("Expected RefreshSaleDescription to remain true, got false")
			}
			if !r.GetMetadata().GetSaleDirty() {
				t.Errorf("Expected SaleDirty to remain true, got false")
			}
		})
	}
}

func TestPushSale_RefreshSaleDescription_GeneratorOrDiscogsFailure(t *testing.T) {
	t.Run("GeneratorFailure", func(t *testing.T) {
		s := InitTestServer(".testPushSale_RefreshGenFail")
		ctx := context.Background()
		ts := &testSyncer{}
		s.retr = ts
		s.generator = &testGenerator{fail: true}

		r := &pb.Record{
			Release: &pbgd.Release{
				Id:              6201,
				InstanceId:      6201,
				Title:           "Gen Failure Album",
				RecordCondition: "VG+",
				SleeveCondition: "VG+",
			},
			Metadata: &pb.ReleaseMetadata{
				Category:               pb.ReleaseMetadata_LISTED_TO_SELL,
				SaleId:                 500,
				SalePrice:              2000,
				SaleState:              pbgd.SaleState_FOR_SALE,
				SaleDirty:              true,
				RefreshSaleDescription: true,
				SaleDescription:        "Existing Description",
				Notes:                  "Clean copy",
			},
		}

		pushed, err := s.pushSale(ctx, r)
		if err == nil {
			t.Fatalf("Expected generator error, got nil")
		}
		if pushed {
			t.Errorf("Expected pushed = false, got true")
		}
		if ts.updateSaleListingCalls != 0 {
			t.Errorf("updateSaleListingCalls: got %d, want 0", ts.updateSaleListingCalls)
		}
		if r.GetMetadata().GetSaleDescription() != "Existing Description" {
			t.Errorf("SaleDescription: got %q, want %q", r.GetMetadata().GetSaleDescription(), "Existing Description")
		}
		if !r.GetMetadata().GetRefreshSaleDescription() {
			t.Errorf("Expected RefreshSaleDescription to remain true, got false")
		}
		if !r.GetMetadata().GetSaleDirty() {
			t.Errorf("Expected SaleDirty to remain true, got false")
		}
	})

	t.Run("DiscogsUpdateFailure", func(t *testing.T) {
		s := InitTestServer(".testPushSale_RefreshDiscogsFail")
		ctx := context.Background()
		ts := &testSyncer{failUpdateSaleListing: true}
		s.retr = ts
		s.generator = &testGenerator{desc: "New Description"}

		r := &pb.Record{
			Release: &pbgd.Release{
				Id:              6202,
				InstanceId:      6202,
				Title:           "Discogs Failure Album",
				RecordCondition: "VG+",
				SleeveCondition: "VG+",
			},
			Metadata: &pb.ReleaseMetadata{
				Category:               pb.ReleaseMetadata_LISTED_TO_SELL,
				SaleId:                 500,
				SalePrice:              2000,
				SaleState:              pbgd.SaleState_FOR_SALE,
				SaleDirty:              true,
				RefreshSaleDescription: true,
				SaleDescription:        "Existing Description",
				Notes:                  "Clean copy",
			},
		}

		pushed, err := s.pushSale(ctx, r)
		if err == nil {
			t.Fatalf("Expected UpdateSaleListing error, got nil")
		}
		if pushed {
			t.Errorf("Expected pushed = false, got true")
		}
		if ts.updateSaleListingCalls != 1 {
			t.Errorf("updateSaleListingCalls: got %d, want 1", ts.updateSaleListingCalls)
		}
		if r.GetMetadata().GetSaleDescription() != "Existing Description" {
			t.Errorf("SaleDescription: got %q, want %q", r.GetMetadata().GetSaleDescription(), "Existing Description")
		}
		if !r.GetMetadata().GetRefreshSaleDescription() {
			t.Errorf("Expected RefreshSaleDescription to remain true, got false")
		}
		if !r.GetMetadata().GetSaleDirty() {
			t.Errorf("Expected SaleDirty to remain true, got false")
		}
	})
}

func TestCacheRecord_UpdatesCostFromDiscogsPurchasePrice(t *testing.T) {
	tests := []struct {
		name          string
		initialCost   int32
		purchasePrice int32
		wantCost      int32
	}{
		{
			name:          "InitialZeroCost_UpdatesFromPositivePurchasePrice",
			initialCost:   0,
			purchasePrice: 2150,
			wantCost:      2150,
		},
		{
			name:          "ExistingNonZeroCost_OverwrittenByPositivePurchasePrice",
			initialCost:   21500,
			purchasePrice: 2150,
			wantCost:      2150,
		},
		{
			name:          "ExistingNonZeroCost_PreservedWhenPurchasePriceZero",
			initialCost:   21500,
			purchasePrice: 0,
			wantCost:      21500,
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := InitTestServer(fmt.Sprintf(".testCacheRecord_PurchasePrice_%d", i))
			ctx := context.Background()
			instanceID := int64(7000 + i)

			s.retr = &testSyncer{
				instanceInfo: map[int64]*godiscogs.InstanceInfo{
					instanceID: {
						PurchasePrice:   tc.purchasePrice,
						RecordCondition: "Near Mint (NM or M-)",
						SleeveCondition: "Near Mint (NM or M-)",
						Notes:           "Like new",
						FolderId:        242017,
					},
				},
			}

			r := &pb.Record{
				Release: &pbgd.Release{
					Id:              int32(7000 + i),
					InstanceId:      instanceID,
					Title:           "Marjassa",
					RecordCondition: "Near Mint (NM or M-)",
					SleeveCondition: "Near Mint (NM or M-)",
					Tracklist:       []*pbgd.Track{{Title: "Marjassa Pt1."}},
				},
				Metadata: &pb.ReleaseMetadata{
					Cost:      tc.initialCost,
					LastCache: time.Now().Unix(),
				},
			}

			if err := s.cacheRecord(ctx, r, true); err != nil {
				t.Fatalf("cacheRecord failed: %v", err)
			}

			if r.GetMetadata().GetCost() != tc.wantCost {
				t.Errorf("in-memory Cost: got %d, want %d", r.GetMetadata().GetCost(), tc.wantCost)
			}

			saved, err := s.loadRecord(ctx, instanceID, false)
			if err != nil {
				t.Fatalf("loadRecord failed: %v", err)
			}
			if saved.GetMetadata().GetCost() != tc.wantCost {
				t.Errorf("persisted Cost: got %d, want %d", saved.GetMetadata().GetCost(), tc.wantCost)
			}
		})
	}
}







