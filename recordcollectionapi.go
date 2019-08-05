package main

import (
	"fmt"
	"time"

	pbgd "github.com/brotherlogic/godiscogs"
	"github.com/brotherlogic/goserver/utils"
	pb "github.com/brotherlogic/recordcollection/proto"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
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

	pushLockTime := int64(0)
	for _, rec := range s.collection.GetRecords() {
		if request.Filter == nil || utils.FuzzyMatch(request.Filter, rec) {
			if request.GetStrip() {
				r := proto.Clone(rec).(*pb.Record)
				r.GetRelease().Images = make([]*pbgd.Image, 0)
				r.GetRelease().Formats = make([]*pbgd.Format, 0)
				response.Records = append(response.Records, r)
			} else if request.GetMoveStrip() {
				cleanRecord := &pb.Record{Metadata: &pb.ReleaseMetadata{}, Release: &pbgd.Release{}}

				cleanRecord.GetMetadata().Category = rec.GetMetadata().Category
				cleanRecord.GetRelease().FolderId = rec.GetRelease().FolderId
				cleanRecord.GetMetadata().MoveFolder = rec.GetMetadata().MoveFolder
				cleanRecord.GetRelease().Formats = rec.GetRelease().Formats
				cleanRecord.GetRelease().Id = rec.GetRelease().Id
				cleanRecord.GetRelease().InstanceId = rec.GetRelease().InstanceId
				cleanRecord.GetRelease().Rating = rec.GetRelease().Rating
				cleanRecord.GetMetadata().GoalFolder = rec.GetMetadata().GoalFolder
				cleanRecord.GetMetadata().FilePath = rec.GetMetadata().FilePath
				cleanRecord.GetMetadata().CdPath = rec.GetMetadata().CdPath
				cleanRecord.GetMetadata().Dirty = rec.GetMetadata().Dirty

				response.Records = append(response.Records, cleanRecord)
			} else {
				response.Records = append(response.Records, rec)
			}

			if rec.GetMetadata().GetDirty() {
				st := time.Now()
				s.pushMutex.Lock()
				took := time.Now().Sub(st).Nanoseconds() / 10000
				if took >= pushLockTime {
					pushLockTime = took
				}
				s.pushMap[rec.GetRelease().Id] = rec
				s.pushMutex.Unlock()
			}
		}
	}

	response.InternalProcessingTime = time.Now().Sub(t).Nanoseconds() / 1000000
	return response, nil
}

// GetWants gets a bunch of records
func (s *Server) GetWants(ctx context.Context, request *pb.GetWantsRequest) (*pb.GetWantsResponse, error) {
	response := &pb.GetWantsResponse{Wants: make([]*pb.Want, 0)}

	for _, rec := range s.collection.GetNewWants() {
		if request.Filter == nil || utils.FuzzyMatch(request.Filter, rec) {
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
	for _, rec := range s.collection.GetRecords() {
		if rec.GetRelease().InstanceId == request.GetUpdate().GetRelease().InstanceId {

			// If this is being sold - mark it for sale
			if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().Category == pb.ReleaseMetadata_SOLD && rec.GetMetadata().Category != pb.ReleaseMetadata_SOLD {
				if !request.NoSell {
					if len(rec.GetRelease().SleeveCondition) == 0 {
						return nil, fmt.Errorf("No Condition info")
					}
					price, _ := s.retr.GetSalePrice(int(rec.GetRelease().Id))
					saleid := s.retr.SellRecord(int(rec.GetRelease().Id), price, "For Sale", rec.GetRelease().RecordCondition, rec.GetRelease().SleeveCondition)
					rec.GetMetadata().SaleId = int32(saleid)
				}
			}

			// If this is a sale update - set the dirty flag
			if rec.GetMetadata().SalePrice != request.GetUpdate().GetMetadata().SalePrice {
				request.GetUpdate().GetMetadata().SaleDirty = true
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

			rec.GetMetadata().Dirty = true
			record = rec
			s.pushMutex.Lock()
			s.pushMap[rec.GetRelease().Id] = rec
			s.pushMutex.Unlock()
		}
	}

	s.saveNeeded = true
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
		s.cacheRecord(ctx, request.GetToAdd())
		s.saveNeeded = true
	}

	return &pb.AddRecordResponse{Added: request.GetToAdd()}, err
}
