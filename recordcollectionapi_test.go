package main

import (
	"context"
	"testing"

	"github.com/brotherlogic/goserver"
	"github.com/brotherlogic/keystore/client"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
)

func InitTestServer() *Server {
	s := &Server{GoServer: &goserver.GoServer{}, collection: &pb.RecordCollection{}}
	s.retr = &testSyncer{}
	s.GoServer.KSclient = *keystoreclient.GetTestClient(".testing/")
	s.SkipLog = true
	return s
}

func TestGetRecords(t *testing.T) {
	s := InitTestServer()
	s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: 123, Title: "madeup1", InstanceId: 2}})
	r, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{}})

	if err != nil {
		t.Errorf("Error in getting records: %v", err)
	}

	if len(r.GetRecords()) != 1 {
		t.Errorf("Wrong number of records returned: %v", r)
	}
}

func TestUpdateRecords(t *testing.T) {
	s := InitTestServer()
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
