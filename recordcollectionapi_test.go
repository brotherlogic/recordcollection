package main

import (
	"context"
	"os"
	"testing"

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

func TestGetRecords(t *testing.T) {
	s := InitTestServer(".testGetRecords")
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 2}})
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{}})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 {
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
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 1}})

	s.UpdateRecord(context.Background(), &pb.UpdateRecordRequest{Update: &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup2", InstanceId: 1}}})

	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{}})

	if err != nil {
		t.Fatalf("Error in getting records: %v", err)
	}

	if r == nil || len(r.Records) != 1 || r.Records[0].Release.Title != "madeup2" {
		t.Errorf("Error in updating records: %v", r)
	}
}
