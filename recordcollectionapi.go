package main

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	pbgd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
)

// DeleteRecord deletes a record
func (s *Server) DeleteRecord(ctx context.Context, request *pb.DeleteRecordRequest) (*pb.DeleteRecordResponse, error) {
	for i, r := range s.collection.GetRecords() {
		if r.GetRelease().InstanceId == request.InstanceId {
			s.retr.DeleteInstance(int(r.GetRelease().FolderId), int(r.GetRelease().Id), int(r.GetRelease().InstanceId))
			s.collection.Records = append(s.collection.Records[:i], s.collection.Records[i+1:]...)
		}
	}

	return &pb.DeleteRecordResponse{}, nil
}

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
		if request.Filter == nil || utils.FuzzyMatch(request.Filter, rec) {
			if request.GetStrip() {
				r := proto.Clone(rec).(*pb.Record)
				r.GetRelease().Images = make([]*pbgd.Image, 0)
				r.GetRelease().Formats = make([]*pbgd.Format, 0)
				response.Records = append(response.Records, r)
			} else {
				response.Records = append(response.Records, rec)
			}
			if request.GetForce() {
				s.cacheRecord(rec)
				s.Log(fmt.Sprintf("Cached %v into %v", rec, response))
			} else {
				s.cacheMutex.Lock()
				s.cacheMap[rec.GetRelease().Id] = rec
				s.cacheMutex.Unlock()
			}
			if rec.GetMetadata().GetDirty() {
				s.pushMutex.Lock()
				s.pushMap[rec.GetRelease().Id] = rec
				s.pushMutex.Unlock()
			}
		}
	}

	//Don't report if we're forcing
	if !request.GetForce() {
		s.LogFunction(fmt.Sprintf("GetRecords-%v", len(s.collection.GetRecords())), t)
	}

	response.InternalProcessingTime = time.Now().Sub(t).Nanoseconds() / 1000000
	return response, nil
}

// GetWants gets a bunch of records
func (s *Server) GetWants(ctx context.Context, request *pb.GetWantsRequest) (*pb.GetWantsResponse, error) {
	t := time.Now()
	response := &pb.GetWantsResponse{Wants: make([]*pb.Want, 0)}

	for _, rec := range s.collection.GetNewWants() {
		if request.Filter == nil || utils.FuzzyMatch(request.Filter, rec) {
			response.Wants = append(response.Wants, rec)
		}
	}

	s.LogFunction("GetWants", t)
	return response, nil
}

//UpdateWant updates the record
func (s *Server) UpdateWant(ctx context.Context, request *pb.UpdateWantRequest) (*pb.UpdateWantResponse, error) {
	t := time.Now()
	var want *pb.Want
	for _, rec := range s.collection.GetNewWants() {
		if rec.GetRelease().Id == request.GetUpdate().GetRelease().Id {
			proto.Merge(rec, request.GetUpdate())

			want = rec
		}
	}

	s.saveNeeded = true
	s.LogFunction("UpdateWant", t)
	return &pb.UpdateWantResponse{Updated: want}, nil
}

//UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {
	t := time.Now()
	var record *pb.Record
	for _, rec := range s.collection.GetRecords() {
		if rec.GetRelease().InstanceId == request.GetUpdate().GetRelease().InstanceId {
			s.LogMilestone("UpdateRecord", "FoundRecord", t)

			// If this is being sold - mark it for sale
			if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().Category == pb.ReleaseMetadata_SOLD && rec.GetMetadata().Category != pb.ReleaseMetadata_SOLD {
				price := s.retr.GetSalePrice(int(rec.GetRelease().Id))
				s.retr.SellRecord(int(rec.GetRelease().Id), price, "For Sale")
			}

			// Avoid increasing repeasted fields
			if len(request.GetUpdate().GetRelease().GetFormats()) > 0 {
				rec.GetRelease().Images = []*pbgd.Image{}
				rec.GetRelease().Artists = []*pbgd.Artist{}
				rec.GetRelease().Formats = []*pbgd.Format{}
				rec.GetRelease().Labels = []*pbgd.Label{}
			}

			proto.Merge(rec, request.GetUpdate())
			rec.GetMetadata().Dirty = true
			record = rec
			s.pushMutex.Lock()
			s.pushMap[rec.GetRelease().Id] = rec
			s.pushMutex.Unlock()
			s.LogMilestone("UpdateRecord", "UpdatedRecord", t)
		}
	}

	s.saveNeeded = true
	s.LogFunction(fmt.Sprintf("UpdateRecord", len(s.collection.GetRecords())), t)
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
		s.saveNeeded = true
	}

	return &pb.AddRecordResponse{Added: request.GetToAdd()}, err
}
