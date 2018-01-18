package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

// GetRecordCollection gets the full collection
func (s *Server) GetRecordCollection(ctx context.Context, request *pb.GetRecordCollectionRequest) (*pb.GetRecordCollectionResponse, error) {
	resp := &pb.GetRecordCollectionResponse{InstanceIds: make([]int32, 0)}
	for _, r := range s.collection.GetRecords() {
		resp.InstanceIds = append(resp.InstanceIds, r.GetRelease().InstanceId)
	}
	return resp, nil
}

// GetRecords gets a bunch of records
func (s *Server) GetRecords(ctx context.Context, request *pb.GetRecordsRequest) (*pb.GetRecordsResponse, error) {
	t := time.Now()
	response := &pb.GetRecordsResponse{Records: make([]*pb.Record, 0)}

	for _, rec := range s.collection.GetRecords() {
		if request.Filter.GetRelease() == nil || utils.FuzzyMatch(request.Filter, rec) {
			response.Records = append(response.Records, rec)
			if request.GetForce() {
				s.cacheRecord(rec)
			} else {
				s.cacheMap[rec.GetRelease().Id] = rec
			}
		}
	}

	s.LogFunction(fmt.Sprintf("GetRecords-%v", len(s.collection.GetRecords())), t)
	return response, nil
}

//UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {
	t := time.Now()
	var record *pb.Record
	for _, rec := range s.collection.GetRecords() {
		if rec.GetRelease().InstanceId == request.GetUpdate().GetRelease().InstanceId {
			proto.Merge(rec, request.GetUpdate())
			rec.GetMetadata().Dirty = true
			record = rec
			s.pushMutex.Lock()
			s.pushMap[rec.GetRelease().Id] = rec
			s.pushMutex.Unlock()
		}
	}
	s.LogFunction(fmt.Sprintf("UpdateRecord-%v", len(s.collection.GetRecords())), t)
	s.saveRecordCollection()
	return &pb.UpdateRecordsResponse{Updated: record}, nil
}

// AddRecord adds a record directly to the listening pile
func (s *Server) AddRecord(ctx context.Context, request *pb.AddRecordRequest) (*pb.AddRecordResponse, error) {
	//Reject the add if we don't have a cost or goal folder
	if request.GetToAdd().GetMetadata().GetCost() == 0 || request.GetToAdd().GetMetadata().GetGoalFolder() == 0 {
		return &pb.AddRecordResponse{}, fmt.Errorf("Unable to add - no cost or goal folder")
	}

	instanceID, err := s.retr.AddToFolder(812802, request.GetToAdd().GetRelease().Id)
	if err == nil {
		request.GetToAdd().Release.InstanceId = int32(instanceID)
		request.GetToAdd().GetMetadata().DateAdded = time.Now().Unix()
		s.collection.Records = append(s.collection.Records, request.GetToAdd())
		s.cacheRecord(request.GetToAdd())
		s.saveRecordCollection()
	}

	return &pb.AddRecordResponse{Added: request.GetToAdd()}, err
}
