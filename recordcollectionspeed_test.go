package main

import (
	"context"
	"io/ioutil"
	"log"
	"testing"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

const (
	//NumRecords - the number of records to test against
	NumRecords = 10000
)

func TestGetRecordsSpeed(t *testing.T) {
	s := InitTestServer(".testing")

	for i := 0; i < NumRecords; i++ {
		s.collection.Records = append(s.collection.Records, &pb.Record{Release: &pbd.Release{Id: int32(i), Title: "madeup1", InstanceId: int32(i + 1)}})
	}

	ts := time.Now()
	s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{}})
	diff := time.Now().Sub(ts)

	if diff.Seconds() >= 1.0 {
		t.Errorf("Get Records is taking over a second: %v", diff)
	} else {
		log.Printf("Took: %v", diff)
	}
}

func TestGetRecordsComplex(t *testing.T) {
	s := InitTestServer(".testing")

	s.collection = &pb.RecordCollection{}
	data, err := ioutil.ReadFile("testdata/bigrc.data")
	if err != nil {
		t.Fatalf("Error reading file: %v", err)
	}
	proto.Unmarshal(data, s.collection)

	ts := time.Now()
	v, err := s.GetRecords(context.Background(), &pb.GetRecordsRequest{Filter: &pb.Record{Release: &pbd.Release{FolderId: 1}}})
	diff := time.Now().Sub(ts)

	if err != nil {
		log.Fatalf("Error in getting records: %v", err)
	}

	if len(v.GetRecords()) != 15 {
		t.Errorf("Wrong number of records returned: %v", len(v.GetRecords()))
	}

	if diff.Seconds() >= 1.0 {
		t.Errorf("Get Records is taking over a second: %v", diff)
	} else {
		log.Printf("Took: %v", diff)
	}
}
