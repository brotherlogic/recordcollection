package main

import (
	"context"
	"log"
	"testing"
	"time"

	pbd "github.com/brotherlogic/godiscogs"
	pb "github.com/brotherlogic/recordcollection/proto"
)

const (
	//NumRecords - the number of records to test against
	NumRecords = 5000
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
