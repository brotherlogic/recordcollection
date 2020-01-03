package main

import (
	"context"
	"os"
	"testing"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/keystore/client"
	pb "github.com/brotherlogic/recordcollection/proto"
)

func InitTestServer(folder string) *Server {
	s := Init()
	s.PrepServer()
	s.cacheWait = 0
	s.retr = &testSyncer{}
	s.mover = &testMover{pass: true}
	s.scorer = &testScorer{}

	// Create the record collection because we're not init'ing from a file
	s.collection = &pb.RecordCollection{}
	s.collection.InstanceToFolder = make(map[int32]int32)
	s.collection.InstanceToUpdate = make(map[int32]int64)
	s.collection.InstanceToCategory = make(map[int32]pb.ReleaseMetadata_Category)
	s.collection.InstanceToMaster = make(map[int32]int32)
	s.collection.InstanceToId = make(map[int32]int32)
	s.collection.InstanceToRecache = make(map[int32]int64)
	s.quota = &testQuota{pass: true}

	os.RemoveAll(folder)
	s.GoServer.KSclient = *keystoreclient.GetTestClient(folder)
	s.SkipLog = true
	s.SkipIssue = true

	return s
}

func TestAddRecord(t *testing.T) {
	s := InitTestServer(".testaddrecord")
	r, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 20}, Release: &pbd.Release{Id: 1234}}})

	if err != nil {
		t.Fatalf("Error in adding record: %v", err)
	}

	//Retrieve the record
	if r.GetAdded().GetRelease().InstanceId <= 0 {
		t.Errorf("Added record does not have an instance id: %v", r)
	}
}

func TestDeleteRecord(t *testing.T) {
	s := InitTestServer(".testaddrecord")
	s.collection.NeedsPush = []int32{int32(456), int32(123)}
	r, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 20}, Release: &pbd.Release{Id: 1234, InstanceId: 123}}})

	if err != nil {
		t.Fatalf("Error in adding record: %v", err)
	}

	//Retrieve the record
	if r.GetAdded().GetRelease().InstanceId <= 0 {
		t.Errorf("Added record does not have an instance id: %v", r)
	}

	_, err = s.DeleteRecord(context.Background(), &pb.DeleteRecordRequest{InstanceId: r.GetAdded().GetRelease().InstanceId})

	if err != nil {
		t.Fatalf("Error in deleting record: %v", err)
	}

	if len(s.collection.Records) != 0 {
		t.Errorf("Record has not been delete: %v", s.collection)
	}

	if len(s.collection.NeedsPush) != 1 {
		t.Errorf("Has not been removed from push: %v", s.collection.NeedsPush)
	}
}

func TestAddRecordSetsDateAdded(t *testing.T) {
	s := InitTestServer(".testaddrecord")
	r, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 20}, Release: &pbd.Release{Id: 1234}}})

	if err != nil {
		t.Fatalf("Error in adding record: %v", err)
	}

	if r.GetAdded().GetMetadata().GetDateAdded() == 0 {
		t.Errorf("Added record has not had date added set: %v", r)
	}
}

func TestAddRecordNoCost(t *testing.T) {
	s := InitTestServer(".testaddrecordnocost")
	r, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Metadata: &pb.ReleaseMetadata{GoalFolder: 20}, Release: &pbd.Release{Id: 1234}}})

	if err == nil {
		t.Fatalf("No cost add has not failed: %v", r)
	}
}

func TestUpdateRecords(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	_, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})
	if err != nil {
		t.Errorf("Error adding record: %v", err)
	}

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1, Formats: []*pbd.Format{&pbd.Format{Name: "12"}}, Images: []*pbd.Image{&pbd.Image{Uri: "blah"}}, Artists: []*pbd.Artist{&pbd.Artist{Name: "Dave"}}, Labels: []*pbd.Label{&pbd.Label{Name: "Daves Label"}}, Tracklist: []*pbd.Track{&pbd.Track{Title: "blah"}}}}})

	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 1})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if r == nil || r.GetRecord().Release.Title != "madeup2" {
		t.Errorf("Error in updating records: %v", r)
	}
}

func TestBadUpdateRecords(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1, Formats: []*pbd.Format{&pbd.Format{Name: "12"}}, Images: []*pbd.Image{&pbd.Image{Uri: "blah"}}, Artists: []*pbd.Artist{&pbd.Artist{Name: "Dave"}}, Labels: []*pbd.Label{&pbd.Label{Name: "Daves Label"}}, Tracklist: []*pbd.Track{&pbd.Track{Title: "blah"}}}}})

	if err == nil {
		t.Errorf("Update did not fail")
	}
}

func TestUpdateRecordsNoCondition(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_SOLD}, Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1, Formats: []*pbd.Format{&pbd.Format{Name: "12"}}, Images: []*pbd.Image{&pbd.Image{Uri: "blah"}}, Artists: []*pbd.Artist{&pbd.Artist{Name: "Dave"}}, Labels: []*pbd.Label{&pbd.Label{Name: "Daves Label"}}, Tracklist: []*pbd.Track{&pbd.Track{Title: "blah"}}}}})

	if err == nil {
		t.Errorf("Should have triggered condition issue")
	}
}

func TestUpdateRecordWithSalePrice(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	rec := &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893, SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{SaleId: 1234, SalePrice: 1234, Category: pb.ReleaseMetadata_LISTED_TO_SELL, LastCache: time.Now().Unix(), Cost: 100, GoalFolder: 100}}
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: rec})
	s.saleMap[1234] = rec

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{NewSalePrice: 1235}, Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893}}})
	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 177077893})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || !r.GetRecord().GetMetadata().SaleDirty {
		t.Errorf("Error in updating records: %v", r)
	}

	err = s.pushSales(context.Background())
	if err != nil {
		t.Errorf("Error pushing sales: %v", err)
	}

	r, err = s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 177077893})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || r.GetRecord().GetMetadata().SaleDirty {
		t.Errorf("Error in updating sale prices records: %v", r)
	}

}

func TestUpdateRecordWithNoPriceChangeSalePrice(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	rec := &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893, SleeveCondition: "blah", RecordCondition: "blah"}, Metadata: &pb.ReleaseMetadata{SaleId: 1234, SalePrice: 1234, Category: pb.ReleaseMetadata_LISTED_TO_SELL, Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: rec})
	s.saleMap[1234] = rec

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{SaleDirty: true}, Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893}}})
	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 177077893})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || !r.GetRecord().GetMetadata().SaleDirty {
		t.Errorf("Error in updating records: %v", r)
	}

	err = s.pushSales(context.Background())
	if err != nil {
		t.Fatalf("Error in pushing sales: %v", err)
	}

	r, err = s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 177077893})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || r.GetRecord().GetMetadata().SaleDirty {
		t.Errorf("Error in updating sale prices records: %v", r)
	}

}

func TestUpdateRecordWithNoPriceChangeSalePriceWithoutCOndition(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	rec := &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893}, Metadata: &pb.ReleaseMetadata{SaleId: 1234, SalePrice: 1234, Category: pb.ReleaseMetadata_LISTED_TO_SELL, GoalFolder: 100, Cost: 100, LastCache: time.Now().Unix()}}
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: rec})
	s.saleMap[1234] = rec

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{NewSalePrice: 100}, Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893}}})
	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 177077893})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || !r.GetRecord().GetMetadata().SaleDirty {
		t.Errorf("Error in updating records: %v", r)
	}

	err = s.pushSales(context.Background())

	if err == nil {
		t.Errorf("No error in pushing sales")
	}
}

func TestRemoveRecordFromSale(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	rec := &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 177077893}, Metadata: &pb.ReleaseMetadata{SaleId: 1234, SalePrice: 1234, Category: pb.ReleaseMetadata_SOLD_OFFLINE, SaleDirty: true, Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: rec})
	s.collection.SaleUpdates = append(s.collection.SaleUpdates, int32(177077893))
	s.collection.SaleUpdates = append(s.collection.SaleUpdates, int32(10))

	err := s.pushSales(context.Background())

	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 177077893})

	if err != nil || r == nil || r.GetRecord().GetMetadata().SaleDirty {
		t.Errorf("Error in updating sale prices records: %v", r)
	}

}

func TestUpdateRecordNullFolder(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{MoveFolder: -1}, Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1, Formats: []*pbd.Format{&pbd.Format{Name: "12"}}}}})

	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 1})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || r.GetRecord().Release.Title != "madeup2" || r.GetRecord().GetMetadata().MoveFolder != 0 {
		t.Errorf("Error in updating records: %v", r)
	}
}

func TestDoUpdateRecordsForSale(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1, RecordCondition: "Blah", SleeveCondition: "Blah"}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_SOLD}, Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1, Formats: []*pbd.Format{&pbd.Format{Name: "12"}}}}})

	r, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 1})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if r == nil || r.GetRecord().Release.Title != "madeup2" {
		t.Errorf("Error in updating records: %v", r)
	}
}

func TestUpdateRecordsForSaleSellingIsDisabled(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	s.disableSales = true
	s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1, RecordCondition: "Blah", SleeveCondition: "Blah"}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100, LastCache: time.Now().Unix()}}})

	_, err := s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Metadata: &pb.ReleaseMetadata{Category: pb.ReleaseMetadata_SOLD}, Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1, Formats: []*pbd.Format{&pbd.Format{Name: "12"}}}}})

	if err == nil {
		t.Errorf("Disabling sales did not cause error")
	}
}

func TestUpdateWants(t *testing.T) {
	s := InitTestServer(".testUpdateWant")
	s.collection.NewWants = append(s.collection.NewWants, &pb.Want{Release: &pbd.Release{Id: 123, Title: "madeup1"}, Metadata: &pb.WantMetadata{Active: true}})

	_, err := s.UpdateWant(context.Background(), &pb.UpdateWantRequest{Update: &pb.Want{Release: &pbd.Release{Id: 123, Title: "madeup2"}}})
	if err != nil {
		t.Fatalf("Error updating want")
	}

	r, err := s.GetWants(context.Background(), &pb.GetWantsRequest{Filter: &pb.Want{Release: &pbd.Release{}}})

	if err != nil {
		t.Fatalf("Error in getting wants: %v", err)
	}

	if r == nil || len(r.Wants) != 1 || r.Wants[0].GetRelease().Title != "madeup2" {
		t.Errorf("Error in updating wants: %v", r)
	}
}

func TestAddWant(t *testing.T) {
	s := InitTestServer(".testUpdateWant")

	_, err := s.UpdateWant(context.Background(), &pb.UpdateWantRequest{Update: &pb.Want{Release: &pbd.Release{Id: 123, Title: "madeup2"}}})
	if err != nil {
		t.Fatalf("Error updating want")
	}
}

func TestQueryRecordsBad(t *testing.T) {
	s := InitTestServer(".testqueryrecords")

	_, err := s.QueryRecords(context.Background(), &pb.QueryRecordsRequest{})

	if err == nil {
		t.Errorf("No error on empty query")
	}
}

func TestQueryRecordsWithFolderId(t *testing.T) {
	s := InitTestServer(".testqueryrecords")
	s.collection.InstanceToFolder[12] = 12

	q, err := s.QueryRecords(context.Background(), &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_FolderId{12}})

	if err != nil {
		t.Errorf("Error on query: %v", err)
	}

	if len(q.GetInstanceIds()) != 1 {
		t.Errorf("Wrong number of results: %v", q)
	}
}

func TestQueryRecordsWithUpdateTime(t *testing.T) {
	s := InitTestServer(".testqueryrecords")
	s.collection.InstanceToUpdate[12] = 14

	q, err := s.QueryRecords(context.Background(), &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_UpdateTime{12}})

	if err != nil {
		t.Errorf("Error on query: %v", err)
	}

	if len(q.GetInstanceIds()) != 1 {
		t.Errorf("Wrong number of results: %v", q)
	}
}

func TestQueryRecordsWithCategory(t *testing.T) {
	s := InitTestServer(".testqueryrecords")
	s.collection.InstanceToCategory[12] = pb.ReleaseMetadata_PRE_DISTINGUISHED

	q, err := s.QueryRecords(context.Background(), &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_Category{pb.ReleaseMetadata_PRE_DISTINGUISHED}})

	if err != nil {
		t.Errorf("Error on query: %v", err)
	}

	if len(q.GetInstanceIds()) != 1 {
		t.Errorf("Wrong number of results: %v", q)
	}
}

func TestQueryRecordsWithMasterId(t *testing.T) {
	s := InitTestServer(".testqueryrecords")
	_, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{InstanceId: 100, MasterId: 100}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100}}})
	if err != nil {
		t.Fatalf("Error adding record: %v", err)
	}

	q, err := s.QueryRecords(context.Background(), &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_MasterId{100}})

	if err != nil {
		t.Errorf("Error on query: %v", err)
	}

	if len(q.GetInstanceIds()) != 1 {
		t.Errorf("Wrong number of results: %v", q)
	}
}

func TestQueryRecordsWithReleaseId(t *testing.T) {
	s := InitTestServer(".testqueryrecords")
	_, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Release: &pbd.Release{InstanceId: 100, Id: 100}, Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 100}}})
	if err != nil {
		t.Fatalf("Error adding record: %v", err)
	}

	q, err := s.QueryRecords(context.Background(), &pb.QueryRecordsRequest{Query: &pb.QueryRecordsRequest_ReleaseId{100}})

	if err != nil {
		t.Errorf("Error on query: %v", err)
	}

	if len(q.GetInstanceIds()) != 1 {
		t.Errorf("Wrong number of results: %v", q)
	}
}

func TestGetRecord(t *testing.T) {
	s := InitTestServer(".testgetrecord")
	s.saveRecord(context.Background(), &pb.Record{Release: &pbd.Release{Id: 12345, InstanceId: 1234}, Metadata: &pb.ReleaseMetadata{GoalFolder: 12}})

	q, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 1234})
	if err != nil {
		t.Errorf("Error on get record: %v", err)
	}

	if q.GetRecord().GetRelease().GetId() != 12345 {
		t.Errorf("Bad pull on get record")
	}
}

func TestGetRecordFail(t *testing.T) {
	s := InitTestServer(".testgetrecord")

	q, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 1234})
	if err == nil {
		t.Errorf("Managed to receive no such record: %v", q)
	}
}

func TestGetRecordCacheMiss(t *testing.T) {
	s := InitTestServer(".testcachemiss")

	q, err := s.GetRecord(context.Background(), &pb.GetRecordRequest{InstanceId: 12})

	if err == nil {
		t.Errorf("Error in getting record: %v", q)
	}
}
