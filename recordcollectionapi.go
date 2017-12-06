package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

// GetRecords gets a bunch of records
func (s *Server) GetRecords(ctx context.Context, request *pb.GetRecordsRequest) (*pb.GetRecordsResponse, error) {
	t := time.Now()
	response := &pb.GetRecordsResponse{Records: make([]*pb.Record, 0)}

	s.Log(fmt.Sprintf("Processing %v records", len(s.collection.GetRecords())))
	for _, rec := range s.collection.GetRecords() {
		if request.Filter.GetRelease() == nil || utils.FuzzyMatch(request.Filter, rec) {
			response.Records = append(response.Records, rec)
			s.cacheMap[rec.GetRelease().Id] = rec
		}
	}

	s.LogFunction(fmt.Sprintf("GetRecords-%v", len(s.collection.GetRecords())), t)
	return response, nil
}

//UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {
	for _, rec := range s.collection.GetRecords() {
		if rec.GetRelease().InstanceId == request.GetUpdate().GetRelease().InstanceId {
			proto.Merge(rec, request.GetUpdate())
		}
	}
	return nil, nil
}

// AddRecord adds a record
func (s *Server) AddRecord(ctx context.Context, request *pb.AddRecordRequest) (*pb.AddRecordResponse, error) {
	err := s.retr.AddToFolder(1, request.GetToAdd().GetRelease().Id)
	return &pb.AddRecordResponse{Added: request.GetToAdd()}, err
}
