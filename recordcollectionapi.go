package main

import (
	"fmt"
	"time"

	pbgd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DeleteRecord deletes a record
func (s *Server) DeleteRecord(ctx context.Context, request *pb.DeleteRecordRequest) (*pb.DeleteRecordResponse, error) {
	s.collectionMutex.Lock()
	//Remove the record from the maps
	delete(s.collection.InstanceToUpdate, request.InstanceId)
	delete(s.collection.InstanceToFolder, request.InstanceId)
	delete(s.collection.InstanceToMaster, request.InstanceId)
	delete(s.collection.InstanceToCategory, request.InstanceId)
	delete(s.collection.InstanceToId, request.InstanceId)

	betterDelete := []int32{}
	for _, val := range s.collection.NeedsPush {
		if val != request.InstanceId {
			betterDelete = append(betterDelete, val)
		}
	}

	//Delete from the cache
	s.recordCacheMutex.Lock()
	delete(s.recordCache, request.InstanceId)
	s.recordCacheMutex.Unlock()

	s.collectionMutex.Unlock()
	s.Log(fmt.Sprintf("Removed from push: %v -> %v given %v and %v", len(s.collection.NeedsPush), len(betterDelete), request.InstanceId, s.collection.NeedsPush[0]))
	s.collection.NeedsPush = betterDelete

	s.saveRecordCollection(ctx)
	return &pb.DeleteRecordResponse{}, s.deleteRecord(ctx, request.InstanceId)
}

// GetWants gets a bunch of records
func (s *Server) GetWants(ctx context.Context, request *pb.GetWantsRequest) (*pb.GetWantsResponse, error) {
	response := &pb.GetWantsResponse{Wants: make([]*pb.Want, 0)}

	for _, rec := range s.collection.GetNewWants() {
		if request.Filter == nil || utils.FuzzyMatch(request.Filter, rec) == nil {
			response.Wants = append(response.Wants, rec)
		}
	}

	return response, nil
}

//UpdateWant updates the record
func (s *Server) UpdateWant(ctx context.Context, request *pb.UpdateWantRequest) (*pb.UpdateWantResponse, error) {
	var want *pb.Want
	found := false
	for _, rec := range s.collection.GetNewWants() {
		if rec.GetRelease().Id == request.GetUpdate().GetRelease().Id {
			found = true
			proto.Merge(rec, request.GetUpdate())
			if request.Remove {
				rec.ClearWant = true
			}
			rec.GetMetadata().Active = true
			want = rec
		}
	}

	if !found {
		s.retr.AddToWantlist(int(request.GetUpdate().GetRelease().Id))
	}

	s.saveNeeded = true
	return &pb.UpdateWantResponse{Updated: want}, nil
}

//UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {
	var record *pb.Record
	var err error

	rec, err := s.loadRecord(ctx, request.GetUpdate().GetRelease().InstanceId)
	if err != nil {
		return nil, err
	}

	// If this is being sold - mark it for sale
	if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().Category == pb.ReleaseMetadata_SOLD && rec.GetMetadata().Category != pb.ReleaseMetadata_SOLD {
		if !request.NoSell {
			if len(rec.GetRelease().SleeveCondition) == 0 {
				return nil, fmt.Errorf("No Condition info")
			}
			if s.disableSales {
				return nil, fmt.Errorf("Sales are disabled")
			}
			price, _ := s.retr.GetSalePrice(int(rec.GetRelease().Id))
			saleid := s.retr.SellRecord(int(rec.GetRelease().Id), price, "For Sale", rec.GetRelease().RecordCondition, rec.GetRelease().SleeveCondition)
			rec.GetMetadata().SaleId = int32(saleid)
			rec.GetMetadata().LastSalePriceUpdate = time.Now().Unix()
		}
	}

	// If this is a sale update - set the dirty flag
	if request.GetUpdate().GetMetadata().NewSalePrice > 0 || request.GetUpdate().GetMetadata().SaleDirty {

		if rec.GetMetadata().SalePrice-request.GetUpdate().GetMetadata().NewSalePrice > 500 && request.GetUpdate().GetMetadata().NewSalePrice > 0 {
			return nil, fmt.Errorf("Price change from %v to %v is too large", rec.GetMetadata().SalePrice, request.GetUpdate().GetMetadata().NewSalePrice)
		}

		rec.GetMetadata().SaleDirty = true
		s.collection.SaleUpdates = append(s.collection.SaleUpdates, rec.GetRelease().InstanceId)
	}

	// Avoid increasing repeasted fields
	if len(request.GetUpdate().GetRelease().GetImages()) > 0 {
		rec.GetRelease().Images = []*pbgd.Image{}
	}
	if len(request.GetUpdate().GetRelease().GetArtists()) > 0 {
		rec.GetRelease().Artists = []*pbgd.Artist{}
	}
	if len(request.GetUpdate().GetRelease().GetFormats()) > 0 {
		rec.GetRelease().Formats = []*pbgd.Format{}
	}
	if len(request.GetUpdate().GetRelease().GetLabels()) > 0 {
		rec.GetRelease().Labels = []*pbgd.Label{}
	}
	if len(request.GetUpdate().GetRelease().GetTracklist()) > 0 {
		rec.GetRelease().Tracklist = []*pbgd.Track{}
	}

	proto.Merge(rec, request.GetUpdate())

	//Reset the move folder
	if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().MoveFolder == -1 {
		rec.GetMetadata().MoveFolder = 0
	}

	rec.GetMetadata().LastUpdateTime = time.Now().Unix()
	rec.GetMetadata().Dirty = true
	record = rec
	s.collection.NeedsPush = append(s.collection.NeedsPush, rec.GetRelease().InstanceId)
	err = s.saveRecord(ctx, rec)

	s.saveNeeded = true
	return &pb.UpdateRecordsResponse{Updated: record}, err
}

// AddRecord adds a record directly to the listening pile
func (s *Server) AddRecord(ctx context.Context, request *pb.AddRecordRequest) (*pb.AddRecordResponse, error) {
	//Reject the add if we don't have a cost or goal folder
	if request.GetToAdd().GetMetadata().GetCost() == 0 || request.GetToAdd().GetMetadata().GetGoalFolder() == 0 {
		return &pb.AddRecordResponse{}, fmt.Errorf("Unable to add - no cost or goal folder")
	}

	var err error
	instanceID := int(request.GetToAdd().GetRelease().InstanceId)
	if instanceID == 0 {
		instanceID, err = s.retr.AddToFolder(812802, request.GetToAdd().GetRelease().Id)
	}
	if err == nil {
		request.GetToAdd().Release.InstanceId = int32(instanceID)
		request.GetToAdd().GetMetadata().DateAdded = time.Now().Unix()
		s.collectionMutex.Lock()
		s.collection.InstanceToFolder[int32(instanceID)] = int32(812802)
		s.collectionMutex.Unlock()
		s.cacheRecord(ctx, request.GetToAdd())
		s.saveRecord(ctx, request.GetToAdd())
		s.saveNeeded = true
	}

	return &pb.AddRecordResponse{Added: request.GetToAdd()}, err
}

// QueryRecords gets a record using the new schema
func (s *Server) QueryRecords(ctx context.Context, req *pb.QueryRecordsRequest) (*pb.QueryRecordsResponse, error) {
	ids := make([]int32, 0)
	t := time.Now()
	s.collectionMutex.Lock()
	defer s.collectionMutex.Unlock()
	taken := time.Now().Sub(t)
	if taken > s.longest {
		s.longest = taken
	}
	switch x := req.Query.(type) {

	case *pb.QueryRecordsRequest_FolderId:
		for id, folder := range s.collection.InstanceToFolder {
			if folder == x.FolderId {
				ids = append(ids, id)
			}
		}

		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_UpdateTime:
		for id, updateTime := range s.collection.InstanceToUpdate {
			if updateTime >= x.UpdateTime {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_Category:
		for id, category := range s.collection.InstanceToCategory {
			if category == x.Category {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_MasterId:
		for id, masterID := range s.collection.InstanceToMaster {
			if masterID == x.MasterId {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_ReleaseId:
		for id, releaseID := range s.collection.GetInstanceToId() {
			if releaseID == x.ReleaseId {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	}

	return nil, fmt.Errorf("Bad request: %v", req)
}

// GetRecord gets a sigle record
func (s *Server) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordResponse, error) {
	rec, err := s.getRecord(ctx, req.InstanceId)

	if err != nil {
		st := status.Convert(err)
		if st.Code() != codes.DeadlineExceeded && st.Code() != codes.Unavailable {
			s.collectionMutex.Lock()
			s.RaiseIssue(ctx, "Record receive issue", fmt.Sprintf("%v cannot be found -> %v, [%v,%v,%v,%v] (%v)", req.InstanceId, err, s.collection.InstanceToFolder[req.InstanceId], s.collection.InstanceToMaster[req.InstanceId], s.collection.InstanceToCategory[req.InstanceId], s.collection.InstanceToUpdate[req.InstanceId], ctx), false)
			s.collectionMutex.Unlock()
		}

		return nil, fmt.Errorf("Could not locate %v -> %v", req.InstanceId, err)
	}

	return &pb.GetRecordResponse{Record: rec}, err
}
