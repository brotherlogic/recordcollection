package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/brotherlogic/keystore/client"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
)

func InitTestServer(folder string) *Server {
	s := Init()
	s.cacheWait = 0
	s.retr = &testSyncer{}

	// Create the record collection because we're not init'ing from a file
	s.collection = &pb.RecordCollection{}

	os.RemoveAll(folder)
	s.GoServer.KSclient = *keystoreclient.GetTestClient(folder)
	s.SkipLog = true
	return s
}

func TestGetCollection(t *testing.T) {
	s := InitTestServer(".testaddrecord")
	_, err := s.AddRecord(context.Background(), &pb.AddRecordRequest{ToAdd: &pb.Record{Metadata: &pb.ReleaseMetadata{Cost: 100, GoalFolder: 20}, Release: &pbd.Release{Id: 1234}}})

	if err != nil {
		t.Fatalf("Error in adding record: %v", err)
	}

	collection, err := s.GetRecordCollection(context.Background(), &pb.GetRecordCollectionRequest{})

	if err != nil {
		t.Fatalf("Error in getting record collection: %v", err)
	}

	if len(collection.GetInstanceIds()) != 1 {
		t.Errorf("Error getting collection: %v", collection)
	}
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

func TestGetRecords(t *testing.T) {
	s := InitTestServer(".testGetRecords")
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 2}})
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 124, Title: "madeup2", InstanceId: 3}})
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{}})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 2 {
		t.Errorf("Wrong number of records returned: (%v) %v", len(r.GetRecords()), r)
	}
}

func TestGetRecordsById(t *testing.T) {
	s := InitTestServer(".testGetRecords")
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 2}})
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{Id: 123}}})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 {
		t.Errorf("Wrong number of records returned: (%v) %v", len(r.GetRecords()), r)
	}
}

func TestGetRecordsByFolder(t *testing.T) {
	s := InitTestServer(".testGetRecordsByFolder")
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 2, FolderId: 70}})
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{FolderId: 70}}})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 {
		t.Errorf("Wrong number of records returned: (%v) %v", len(r.GetRecords()), r)
	}
}

func TestUpdateRecords(t *testing.T) {
	s := InitTestServer(".testUpdateRecords")
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1}, Metadata: &pb.ReleaseMetadata{}})

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1}}})

	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if r == nil || len(r.Records) != 1 || r.Records[0].Release.Title != "madeup2" {
		t.Errorf("Error in updating records: %v", r)
	}
}

func TestForceRecache(t *testing.T) {
	s := InitTestServer(".testforcerecache")
	s.cacheWait = time.Hour
	s.collection = &pb.RecordCollection{Wants: []*pbd.Release{&pbd.Release{Id: 255}}, Records: []*pb.Record{&pb.Record{Release: &pbd.Release{Id: 234}}}}

	s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{Id: 234}}})
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{Id: 234}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 || r.GetRecords()[0].GetRelease().Title == "On The Wall" {
		t.Errorf("Record has been recached: %v", r)
	}

	r, err = s.GetRecords(context.Background(), &pb.GetRecordsRequest{Force: true, Filter: &pb.Record{Release: &pbd.Release{Id: 234}}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 || r.GetRecords()[0].GetRelease().Title != "On The Wall" {
		t.Errorf("Record has not been recached: %v", r)
	}
}
