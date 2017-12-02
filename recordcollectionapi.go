package main

import (
	"fmt"
	"log"
	"time"

	"golang.org/x/net/context"

	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

// GetRecords gets a bunch of records
func (s *Server) GetRecords(ctx context.Context, request *pb.GetRecordsRequest) (*pb.GetRecordsResponse, error) {
	t := time.Now()
	response := &pb.GetRecordsResponse{Records: make([]*pb.Record, 0)}

	for _, rec := range s.collection.GetRecords() {
		log.Printf("Comparing %v -> %v with %v", rec, request.Filter.GetRelease(), request.Filter.GetRelease() == nil)
		if request.Filter.GetRelease() == nil || (request.Filter.GetRelease().InstanceId > 0 && request.Filter.GetRelease().InstanceId == rec.GetRelease().InstanceId) {
			response.Records = append(response.Records, rec)
		} else if request.Filter.GetRelease().Id > 0 && request.Filter.GetRelease().Id == rec.GetRelease().Id {
			response.Records = append(response.Records, rec)
		} else if request.Filter.GetRelease().FolderId > 0 && request.Filter.GetRelease().FolderId == rec.GetRelease().FolderId {
			response.Records = append(response.Records, rec)
		}
	}

	s.LogFunction(fmt.Sprintf("GetRecords-%v", len(s.collection.GetRecords())), t)
	return response, nil
}

//UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {
	for _, rec := range s.collection.GetRecords() {
		if rec.GetRelease().InstanceId == request.GetUpdate().GetRelease().InstanceId {
			log.Printf("BEFORE: %v", rec)
			proto.Merge(rec, request.GetUpdate())
			log.Printf("AFTER: %v", rec)
		}
	}
	return nil, nil
}
