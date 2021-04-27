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
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return nil, err
	}

	//Remove the record from the maps
	delete(collection.InstanceToUpdate, request.InstanceId)
	delete(collection.InstanceToFolder, request.InstanceId)
	delete(collection.InstanceToMaster, request.InstanceId)
	delete(collection.InstanceToCategory, request.InstanceId)
	delete(collection.InstanceToId, request.InstanceId)
	delete(collection.InstanceToUpdateIn, request.InstanceId)

	betterDelete := []int32{}
	for _, val := range collection.NeedsPush {
		if val != request.InstanceId {
			betterDelete = append(betterDelete, val)
		}
	}

	rec, err := s.loadRecord(ctx, request.GetInstanceId(), false)
	if status.Convert(err).Code() == codes.OutOfRange {
		return &pb.DeleteRecordResponse{}, s.saveRecordCollection(ctx, collection)
	}
	if err != nil {
		return nil, err
	}

	res := s.retr.DeleteInstance(int(rec.GetRelease().GetFolderId()), int(rec.GetRelease().GetId()), int(request.GetInstanceId()))
	s.Log(fmt.Sprintf("Deleted from collection: %v", res))

	s.Log(fmt.Sprintf("Removed from push: %v -> %v given %v and %v", len(collection.NeedsPush), len(betterDelete), request.InstanceId, collection.NeedsPush))
	collection.NeedsPush = betterDelete

	err = s.saveRecordCollection(ctx, collection)
	if err != nil {
		return nil, err
	}

	return &pb.DeleteRecordResponse{}, s.deleteRecord(ctx, request.InstanceId)
}

// GetWants gets a bunch of records
func (s *Server) GetWants(ctx context.Context, request *pb.GetWantsRequest) (*pb.GetWantsResponse, error) {
	response := &pb.GetWantsResponse{Wants: make([]*pb.Want, 0)}

	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return nil, err
	}

	for _, rec := range collection.GetNewWants() {
		if request.Filter == nil || utils.FuzzyMatch(request.Filter, rec) == nil {
			response.Wants = append(response.Wants, rec)
		}
	}

	return response, nil
}

//UpdateWant updates the record
func (s *Server) UpdateWant(ctx context.Context, request *pb.UpdateWantRequest) (*pb.UpdateWantResponse, error) {
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return nil, err
	}

	var want *pb.Want
	found := false
	for _, rec := range collection.GetNewWants() {
		if rec.GetRelease().Id == request.GetUpdate().GetRelease().Id {
			found = true
			proto.Merge(rec, request.GetUpdate())
			if request.Remove {
				rec.ClearWant = true
			}
			if request.GetUpdate().EnableWant {
				rec.EnableWant = true
			}
			want = rec
		}
	}

	if !found {
		//s.retr.AddToWantlist(int(request.GetUpdate().GetRelease().Id))
		collection.NewWants = append(collection.NewWants, request.GetUpdate())
	}

	return &pb.UpdateWantResponse{Updated: want}, s.saveRecordCollection(ctx, collection)
}

//UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {

	if request.GetReason() == "" {
		return nil, fmt.Errorf("You must supply a reason")
	}
	if request.GetUpdate().GetRelease().GetId() > 0 {
		return nil, fmt.Errorf("You cannot do a record update like this")
	}
	s.Log(fmt.Sprintf("UpdateRecord %v", request))

	rec, err := s.loadRecord(ctx, request.GetUpdate().GetRelease().InstanceId, false)
	if err != nil {
		return nil, err
	}
	// Set the metadata if it's not
	if rec.GetMetadata() == nil {
		rec.Metadata = &pb.ReleaseMetadata{}
	}

	// If we've loaded the record correctly we're probably fine
	updates, err := s.loadUpdates(ctx, request.GetUpdate().GetRelease().InstanceId)
	code := status.Convert(err).Code()
	if code != codes.OK && code != codes.InvalidArgument {
		return nil, err
	}
	if code == codes.InvalidArgument {
		updates = &pb.Updates{Updates: []*pb.RecordUpdate{}}
	}
	updates.Updates = append(updates.Updates, &pb.RecordUpdate{Update: request.GetUpdate(), Reason: request.GetReason(), Time: time.Now().Unix()})
	err = s.saveUpdates(ctx, request.GetUpdate().GetRelease().InstanceId, updates)
	if err != nil {
		return nil, err
	}

	// Should be less than 1k
	if proto.Size(updates) > 100000 {
		s.RaiseIssue("Update size", fmt.Sprintf("%v has triggered a big update", request))
	}

	hasLabels := len(rec.GetRelease().GetLabels()) > 0

	// If this is being sold - mark it for sale
	if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().Category == pb.ReleaseMetadata_SOLD && rec.GetMetadata().Category != pb.ReleaseMetadata_SOLD {
		if !request.NoSell {
			s.Log(fmt.Sprintf("Running sale path"))
			time.Sleep(time.Second * 2)
			if len(rec.GetRelease().SleeveCondition) == 0 {
				s.cacheRecord(ctx, rec)
				if len(rec.GetRelease().SleeveCondition) == 0 {
					s.RaiseIssue(fmt.Sprintf("%v needs condition", rec.GetRelease().GetInstanceId()), "Yes")
					return nil, status.Errorf(codes.FailedPrecondition, "No Condition info")
				}
			}
			if s.disableSales {
				return nil, fmt.Errorf("Sales are disabled")
			}
			price, _ := s.retr.GetSalePrice(int(rec.GetRelease().Id))
			saleid := s.retr.SellRecord(int(rec.GetRelease().Id), price, "For Sale", rec.GetRelease().RecordCondition, rec.GetRelease().SleeveCondition)

			// Cancel changes in the update
			request.GetUpdate().GetMetadata().SaleId = 0
			request.GetUpdate().GetMetadata().SaleState = 0
			rec.GetMetadata().SaleId = int32(saleid)
			rec.GetMetadata().LastSalePriceUpdate = time.Now().Unix()
			rec.GetMetadata().SalePrice = int32(price * 100)

			// Preemptive save to ensure we get the saleid
			s.saveRecord(ctx, rec)
		}
	}

	// If this is a sale update - set the dirty flag
	if request.GetUpdate().GetMetadata().GetNewSalePrice() > 0 || request.GetUpdate().GetMetadata().GetExpireSale() {

		if rec.GetMetadata().SalePrice-request.GetUpdate().GetMetadata().NewSalePrice > 500 && request.GetUpdate().GetMetadata().NewSalePrice > 0 {
			return nil, fmt.Errorf("Price change from %v to %v (for %v) is too large", rec.GetMetadata().SalePrice, request.GetUpdate().GetMetadata().NewSalePrice, rec.GetRelease().InstanceId)
		}

		rec.GetMetadata().SaleDirty = true
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

	//Reset scores if needed and an explicit category update is made
	if (rec.GetRelease().GetRating() > 0 &&
		request.GetUpdate().GetMetadata().GetCategory() != pb.ReleaseMetadata_UNKNOWN) && (rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_HIGH_SCHOOL ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_FRESHMAN ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_SOPHMORE ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_GRADUATE ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_DISTINGUISHED ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_PROFESSOR ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_FRESHMAN ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_VALIDATE ||
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PREPARE_TO_SELL) {
		rec.GetMetadata().SetRating = -1
		rec.GetMetadata().Dirty = true
	}

	if rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_VALIDATE {
		rec.GetMetadata().LastValidate = time.Now().Unix()
		rec.GetMetadata().Dirty = true
	}

	//Reset the move folder
	if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().MoveFolder == -1 {
		rec.GetMetadata().MoveFolder = 0
	}

	s.testForLabels(ctx, rec, request, hasLabels)

	if !rec.GetMetadata().GetDirty() && (rec.GetMetadata().GetMoveFolder() != 0 || rec.GetMetadata().GetSetRating() != 0) {
		rec.GetMetadata().Dirty = true
	}

	//Reset the update in value
	rec.GetMetadata().LastUpdateIn = time.Now().Unix()
	err = s.saveRecord(ctx, rec)

	//Only add the fanout if we can
	if len(s.updateFanout) > 90 {
		return nil, status.Errorf(codes.ResourceExhausted, "Fanout is full, but we've saved: %v", err)
	}
	s.updateFanout <- &fo{
		iid:    rec.GetRelease().GetInstanceId(),
		origin: request.GetReason(),
	}
	updateFanout.Set(float64(len(s.updateFanout)))

	return &pb.UpdateRecordsResponse{Updated: rec}, err
}

func (s *Server) testForLabels(ctx context.Context, rec *pb.Record, request *pb.UpdateRecordRequest, hasLabels bool) {
	if len(rec.GetRelease().GetLabels()) == 0 && rec.GetMetadata().GetCategory() != pb.ReleaseMetadata_NO_LABELS && hasLabels {
		s.RaiseIssue("Label reduction", fmt.Sprintf("Update %v has reduced label count", request))
	}
}

// AddRecord adds a record directly to the listening pile
func (s *Server) AddRecord(ctx context.Context, request *pb.AddRecordRequest) (*pb.AddRecordResponse, error) {
	//Reject the add if we don't have a cost or goal folder
	if request.GetToAdd().GetMetadata().GetCost() == 0 || request.GetToAdd().GetMetadata().GetGoalFolder() == 0 {
		return &pb.AddRecordResponse{}, fmt.Errorf("Unable to add - no cost or goal folder")
	}

	s.Log(fmt.Sprintf("AddRecord %v", request))

	var err error
	instanceID := int(request.GetToAdd().GetRelease().InstanceId)
	if instanceID == 0 {
		instanceID, err = s.retr.AddToFolder(812802, request.GetToAdd().GetRelease().Id)
	}
	if err == nil {
		request.GetToAdd().Release.InstanceId = int32(instanceID)
		request.GetToAdd().GetRelease().FolderId = int32(812802)
		request.GetToAdd().GetMetadata().DateAdded = time.Now().Unix()
		s.updateFanout <- &fo{
			iid:    int32(instanceID),
			origin: "adding-record",
		}
		s.saveRecord(ctx, request.GetToAdd())
	}

	return &pb.AddRecordResponse{Added: request.GetToAdd()}, err
}

// QueryRecords gets a record using the new schema
func (s *Server) QueryRecords(ctx context.Context, req *pb.QueryRecordsRequest) (*pb.QueryRecordsResponse, error) {
	collection, err := s.readRecordCollection(ctx)
	if err != nil {
		return nil, err
	}

	ids := make([]int32, 0)
	switch x := req.Query.(type) {

	case *pb.QueryRecordsRequest_FolderId:
		for id, folder := range collection.GetInstanceToFolder() {
			if folder == x.FolderId {
				ids = append(ids, id)
			}
		}

		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_UpdateTime:
		for id, updateTime := range collection.InstanceToUpdate {
			if updateTime >= x.UpdateTime {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_Category:
		for id, category := range collection.GetInstanceToCategory() {
			if category == x.Category {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_MasterId:
		for id, masterID := range collection.InstanceToMaster {
			if masterID == x.MasterId {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_ReleaseId:
		for id, releaseID := range collection.GetInstanceToId() {
			if releaseID == x.ReleaseId {
				ids = append(ids, id)
			}
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil

	case *pb.QueryRecordsRequest_All:
		coll := s.retr.GetCollection()
		for _, rel := range coll {
			ids = append(ids, rel.GetInstanceId())
		}
		return &pb.QueryRecordsResponse{InstanceIds: ids}, nil
	}

	return nil, fmt.Errorf("Bad request: %v", req)
}

// GetRecord gets a sigle record
func (s *Server) GetRecord(ctx context.Context, req *pb.GetRecordRequest) (*pb.GetRecordResponse, error) {
	if req.GetInstanceId() == 0 && req.GetReleaseId() == 0 {
		return nil, fmt.Errorf("No such record exists!")
	}

	// Short cut if we're not asking for a specific release
	if req.GetReleaseId() > 0 {
		got, err := s.retr.GetRelease(req.GetReleaseId())
		if err != nil {
			return nil, err
		}
		return &pb.GetRecordResponse{Record: &pb.Record{Release: got}}, nil
	}

	rec, err := s.loadRecord(ctx, req.InstanceId, req.GetValidate())

	if err != nil {
		if req.GetForce() > 0 {
			rec := &pb.Record{Release: &pbgd.Release{Id: req.GetForce(), InstanceId: req.InstanceId}, Metadata: &pb.ReleaseMetadata{GoalFolder: 242017, Cost: 1}}
			return &pb.GetRecordResponse{Record: rec}, s.cacheRecord(ctx, rec)
		}

		st := status.Convert(err)
		if st.Code() != codes.DeadlineExceeded && st.Code() != codes.Unavailable && st.Code() != codes.Canceled && st.Code() != codes.OutOfRange && st.Code() != codes.NotFound {
			s.Log(fmt.Sprintf("Bad receive: %v", req))
			s.RaiseIssue("Record receive issue", fmt.Sprintf("%v cannot be found -> %v(%v)", req.InstanceId, err, ctx))
		}

		return nil, status.Errorf(st.Code(), fmt.Sprintf("Could not locate %v -> %v", req.InstanceId, err))
	}

	return &pb.GetRecordResponse{Record: rec}, err
}

//Trigger runs a local sync
func (s *Server) Trigger(ctx context.Context, req *pb.TriggerRequest) (*pb.TriggerResponse, error) {
	err := s.runSync(ctx)
	return nil, err
}

//GetUpdates to a record
func (s *Server) GetUpdates(ctx context.Context, req *pb.GetUpdatesRequest) (*pb.GetUpdatesResponse, error) {
	updates, err := s.loadUpdates(ctx, req.GetInstanceId())
	return &pb.GetUpdatesResponse{Updates: updates}, err
}
