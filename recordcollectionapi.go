package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pbgd "github.com/brotherlogic/godiscogs/proto"
	qpb "github.com/brotherlogic/queue/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
	rfpb "github.com/brotherlogic/recordfanout/proto"
	google_protobuf "github.com/golang/protobuf/ptypes/any"
)

var (
	updateCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "recordcollection_update",
		Help: "Push Size",
	}, []string{"reason"})
)

func (s *Server) DeleteSale(ctx context.Context, req *pb.DeleteSaleRequest) (*pb.DeleteSaleResponse, error) {
	sales, err := s.retr.GetInventory(ctx)
	if err != nil {
		return nil, err
	}

	for _, sale := range sales {
		if sale.GetSaleId() == req.GetSaleId() {
			err = s.retr.ExpireSale(ctx, int64(sale.GetSaleId()), int(sale.GetId()), float32(sale.GetSalePrice())/100.0)
		}
	}

	return &pb.DeleteSaleResponse{}, nil
}

func (s *Server) GetInventory(ctx context.Context, req *pb.GetInventoryRequest) (*pb.GetInventoryResponse, error) {
	inventory, err := s.retr.GetInventory(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GetInventoryResponse{Items: inventory}, nil
}

// CommitRecord runs through the record process stuff
func (s *Server) CommitRecord(ctx context.Context, request *pb.CommitRecordRequest) (*pb.CommitRecordResponse, error) {
	added := false
	record, err := s.loadRecord(ctx, request.GetInstanceId(), false)
	updated := false
	if err != nil {
		if status.Convert(err).Code() == codes.OutOfRange {
			return &pb.CommitRecordResponse{}, nil
		}
		return nil, err
	}

	// Fast push on deletes
	if record.GetMetadata().GetDeleteSaleState() == pb.ReleaseMetadata_DELETE {
		_, err = s.DeleteSale(ctx, &pb.DeleteSaleRequest{SaleId: record.GetMetadata().GetSaleId()})
		if err != nil {
			return nil, err
		}
		record.GetMetadata().SaleId = -1
	}

	gUpdate := record.GetMetadata().GetNeedsGramUpdate()

	if record.GetMetadata().GetTransferIid() > 0 {
		s.CtxLog(ctx, fmt.Sprintf("Not commiting transferred record %v: (%v)", record.GetRelease().GetInstanceId(), record.GetMetadata()))
		return &pb.CommitRecordResponse{}, nil
	}

	if record.GetRelease().GetFolderId() == 812802 && record.GetMetadata().GetGoalFolder() == 6268933 && record.GetMetadata().GetLastListenTime() > 0 {
		if record.GetMetadata().GetFiledUnder() == pb.ReleaseMetadata_FILE_12_INCH {
			record.GetMetadata().GoalFolder = 242017
			updated = true
		} else {
			log.Fatalf("Unknown state: %v", record)
		}
	}

	updateReason := ""

	if record.GetMetadata().GetSellOffline() {
		// Remove the sale
		if record.GetMetadata().GetSaleId() > 0 {
			err := s.retr.RemoveFromSale(ctx, int(record.GetMetadata().GetSaleId()), int(record.GetRelease().GetId()))
			if err != nil {
				return nil, err
			}
		}
		record.GetMetadata().SoldPrice = 1
		record.GetMetadata().SoldDate = time.Now().Unix()
		record.GetMetadata().SellOffline = false

		updateReason = "SOLD_OFFLINE"
		updated = true
	}

	// Update the sale state
	if record.GetMetadata().GetSaleId() > 100 && (record.GetMetadata().GetSaleState() == pbgd.SaleState_NOT_FOR_SALE) {
		record.GetMetadata().SaleState = pbgd.SaleState_FOR_SALE
		updateReason += " FOR_SALE"
		updated = true
	}

	if record.GetMetadata().GetSaleId() == -1 {
		record.GetMetadata().SaleId = 0
		updateReason += " SALE_ID"
		updated = true
	}

	// Update keeper status
	if record.GetMetadata().GetKeep() == pb.ReleaseMetadata_RESET_TO_UNKNOWN {
		record.GetMetadata().Keep = pb.ReleaseMetadata_KEEP_UNKNOWN
	}

	// Update for sale records every 48 hours
	if (record.GetMetadata().GetSaleState() == pbgd.SaleState_EXPIRED || record.GetMetadata().GetSaleState() == pbgd.SaleState_FOR_SALE) && !gUpdate {
		// Queue up an update for a month from now
		upup := &rfpb.FanoutRequest{
			InstanceId: record.GetRelease().GetInstanceId(),
		}
		data, _ := proto.Marshal(upup)
		_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
			QueueName: "record_fanout",
			RunTime:   time.Now().Add(time.Hour * 24 * 7).Unix(),
			Payload:   &google_protobuf.Any{Value: data},
			Key:       fmt.Sprintf("%v", record.GetRelease().GetInstanceId()),
		})
		s.CtxLog(ctx, fmt.Sprintf("Updating %v for sale updates", record.GetRelease().GetInstanceId()))
		if err != nil {
			return nil, err
		}
		added = true

		// Update the sale price
		cp := record.GetMetadata().GetCurrentSalePrice()
		price := s.retr.GetCurrentSalePrice(ctx, record.GetMetadata().GetSaleId())
		if price > 0 {
			record.GetMetadata().CurrentSalePrice = int32(price * 100)
		}
		nss, err := s.retr.GetCurrentSaleState(ctx, record.GetMetadata().GetSaleId())
		if err == nil {
			s.CtxLog(ctx, fmt.Sprintf("SALE %v From %v -> %v (%v)", request.GetInstanceId(), record.GetMetadata().SaleState, nss, err))
			updated = (cp != record.GetMetadata().GetCurrentSalePrice() || nss != record.GetMetadata().GetSaleState())
			record.GetMetadata().SaleState = nss
			s.CtxLog(ctx, fmt.Sprintf("UPDATEDSALESTATE: [%v] => %v", cp, record.GetMetadata()))
		}
	}

	// Perform a discogs update if needed
	if record.GetMetadata().GetNeedsGramUpdate() || time.Since(time.Unix(record.GetMetadata().GetLastCache(), 0)) > time.Hour*24*30 ||
		(record.GetMetadata().GetFiledUnder() != pb.ReleaseMetadata_FILE_DIGITAL && (record.GetRelease().GetFolderId() == 812802 || record.GetRelease().GetFolderId() == 3386035) && record.GetRelease().GetRecordCondition() == "") ||
		(len(record.GetRelease().GetImages()) > 0 && strings.Contains(record.GetRelease().GetImages()[0].GetUri(), "img.discogs")) ||
		len(record.GetRelease().GetTracklist()) == 0 {
		err = s.cacheRecord(ctx, record, record.GetMetadata().GetNeedsGramUpdate())
		if err != nil {
			return nil, err
		}

		// Assume that caching pulls in the labels
		record.GetMetadata().NeedsGramUpdate = false
		updated = true
		updateReason += " RECACHE"
	}

	// Reset filed under
	if record.GetMetadata().GetFiledUnder() == -1 {
		record.GetMetadata().FiledUnder = pb.ReleaseMetadata_FILE_UNKNOWN
		updateReason += " FILED_UNDER"
		updated = true
	}

	// Adjust the sale price
	if time.Now().Sub(time.Unix(record.GetMetadata().GetSalePriceUpdate(), 0)) > time.Hour*24*7 {
		s.updateRecordSalePrice(ctx, record)
		updateReason += " SALE_PRICE"
		updated = true
	}

	if record.GetMetadata().GetTransferTo() > 0 && !gUpdate {
		trecord, err := s.transfer(ctx, record)
		if err != nil {
			return nil, err
		}
		record.GetMetadata().TransferIid = trecord.GetRelease().GetInstanceId()
		updateReason += " TRANSFER"
		updated = true

		err = s.saveRecord(ctx, trecord)
		if err != nil {
			return nil, err
		}

		// Run updates on the transferred record too
		upup := &rfpb.FanoutRequest{
			InstanceId: trecord.GetRelease().GetInstanceId(),
		}
		data, _ := proto.Marshal(upup)
		_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
			QueueName: "record_fanout",
			RunTime:   time.Now().Add(time.Second * 10).Unix(),
			Payload:   &google_protobuf.Any{Value: data},
			Key:       fmt.Sprintf("%v", record.GetRelease().GetInstanceId()),
		})
		s.CtxLog(ctx, fmt.Sprintf("Updating %v for transfer", record.GetRelease().GetInstanceId()))

		if err != nil {
			return nil, err
		}
		added = true

		res := s.retr.DeleteInstance(ctx, int(record.GetRelease().GetFolderId()), int(record.GetRelease().GetId()), int(request.GetInstanceId()))
		s.CtxLog(ctx, fmt.Sprintf("Deleted from collection: %v", res))
	}

	/*if time.Since(time.Unix(record.GetMetadata().GetSalePriceUpdate(), 0)) > time.Hour*24 {
		err = s.pushMetadata(ctx, record)
		s.CtxLog(ctx, fmt.Sprintf("Pushed Metadata for %v", record.GetRelease().GetInstanceId()))
		if err != nil {
			return nil, err
		}
	}*/

	// Finally push the record if we need to
	if record.GetMetadata().GetDirty() {
		pushed, err := s.pushRecord(ctx, record)
		if err != nil {
			return nil, err
		}
		if pushed {
			updated = true
		}
	}

	if record.GetMetadata().GetSaleDirty() {
		pushed, err := s.pushSale(ctx, record)
		if err != nil {
			return nil, err
		}
		if pushed {
			updated = true
		}
	}

	err = nil
	if updated {
		err = s.saveRecord(ctx, record)
		if err != nil {
			return nil, err
		}

		if !gUpdate {

			upup := &rfpb.FanoutRequest{
				InstanceId: record.GetRelease().GetInstanceId(),
			}
			data, _ := proto.Marshal(upup)
			_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
				QueueName: "record_fanout",
				RunTime:   time.Now().Add(time.Second * 10).Unix(),
				Payload:   &google_protobuf.Any{Value: data},
				Key:       fmt.Sprintf("%v", record.GetRelease().GetInstanceId()),
			})
			s.CtxLog(ctx, fmt.Sprintf("Updating %v because we updated it (%v)", record.GetRelease().GetInstanceId(), updateReason))
			queueResults.With(prometheus.Labels{"error": fmt.Sprintf("%v", err)}).Inc()
		}
	} else {
		if !gUpdate && !added {
			upup := &rfpb.FanoutRequest{
				InstanceId: record.GetRelease().GetInstanceId(),
			}
			data, _ := proto.Marshal(upup)
			_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
				QueueName:       "record_fanout",
				RunTime:         time.Now().Add(time.Hour * 24 * 7).Unix(),
				Payload:         &google_protobuf.Any{Value: data},
				Key:             fmt.Sprintf("%v", record.GetRelease().GetInstanceId()),
				AddIfNotPresent: true,
			})
			s.CtxLog(ctx, fmt.Sprintf("Updating %v in 24 hours because we did not update it (%v)", record.GetRelease().GetInstanceId(), updateReason))
		}
	}

	if gUpdate {
		return &pb.CommitRecordResponse{}, status.Errorf(codes.FailedPrecondition, "Auto fail on gram update")
	}
	return &pb.CommitRecordResponse{}, err
}

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

	rec, err := s.loadRecord(ctx, request.GetInstanceId(), false)
	if status.Convert(err).Code() == codes.OutOfRange {
		return &pb.DeleteRecordResponse{}, s.saveRecordCollection(ctx, collection)
	}
	if err != nil {
		return nil, err
	}

	res := s.retr.DeleteInstance(ctx, int(rec.GetRelease().GetFolderId()), int(rec.GetRelease().GetId()), int(request.GetInstanceId()))
	s.CtxLog(ctx, fmt.Sprintf("Deleted from collection: %v -> %v and %v and %v", res, int(rec.GetRelease().GetFolderId()), int(rec.GetRelease().GetId()), int(request.GetInstanceId())))

	err = s.saveRecordCollection(ctx, collection)
	if err != nil {
		return nil, err
	}

	return &pb.DeleteRecordResponse{}, s.deleteRecord(ctx, request.InstanceId)
}

// GetWants gets a bunch of records
func (s *Server) GetWants(ctx context.Context, request *pb.GetWantsRequest) (*pb.GetWantsResponse, error) {
	response := &pb.GetWantsResponse{Wants: make([]*pb.Want, 0)}

	wants, err := s.retr.GetWantlist(ctx)
	if err != nil {
		return nil, err
	}

	for _, w := range wants {
		response.Wants = append(response.Wants,
			&pb.Want{ReleaseId: w.GetId()})
	}

	return response, nil
}

// UpdateWant updates the record
func (s *Server) UpdateWant(ctx context.Context, request *pb.UpdateWantRequest) (*pb.UpdateWantResponse, error) {
	var err error
	if request.GetRemove() {
		err = s.retr.RemoveFromWantlist(ctx, int(request.GetUpdate().GetReleaseId()))
	} else {
		err = s.retr.AddToWantlist(ctx, int(request.GetUpdate().GetReleaseId()))
	}
	return &pb.UpdateWantResponse{}, err
}

// UpdateRecord updates the record
func (s *Server) UpdateRecord(ctx context.Context, request *pb.UpdateRecordRequest) (*pb.UpdateRecordsResponse, error) {
	if request.GetReason() == "" {
		return nil, fmt.Errorf("you must supply a reason")
	}

	updateCount.With(prometheus.Labels{"reason": request.GetReason()}).Inc()

	if request.GetReason() != "Tripping gram update" &&
		request.GetReason() != "ping_from_gramophile" {
		s.RaiseIssue("Update", fmt.Sprintf("%v", request))
	}

	// Run a sale udpate
	if request.GetUpdate().GetMetadata().GetSaleId() > 0 && request.GetUpdate().GetRelease().GetInstanceId() == 0 {
		collection, err := s.readRecordCollection(ctx)
		if err != nil {
			return nil, err
		}
		for iid, cat := range collection.GetInstanceToCategory() {
			if cat == pb.ReleaseMetadata_LISTED_TO_SELL {
				r, err := s.loadRecord(ctx, iid, false)
				if err != nil {
					if status.Code(err) != codes.OutOfRange {
						return nil, err
					}
				}
				if r.GetMetadata().GetSaleId() == request.GetUpdate().GetMetadata().GetSaleId() {
					upup := &rfpb.FanoutRequest{
						InstanceId: r.GetRelease().GetInstanceId(),
					}
					data, _ := proto.Marshal(upup)
					_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
						QueueName: "record_fanout",
						RunTime:   time.Now().Add(time.Minute).Unix(),
						Payload:   &google_protobuf.Any{Value: data},
						Key:       fmt.Sprintf("%v", r.GetRelease().GetInstanceId()),
					})
					return &pb.UpdateRecordsResponse{}, err
				}
			}
		}
		return nil, status.Errorf(codes.NotFound, "Could not find sale: %v", request)
	}

	if request.GetUpdate().GetRelease().GetId() > 0 {
		// Allow release id adjustment
		if request.GetUpdate().GetRelease().GetInstanceId() > 0 {
			rec, err := s.loadRecord(ctx, request.GetUpdate().GetRelease().InstanceId, false)
			if err != nil {
				return nil, err
			}
			rec.Release.Id = request.GetUpdate().Release.Id
			return &pb.UpdateRecordsResponse{}, s.saveRecord(ctx, rec)
		}
		return nil, fmt.Errorf("you cannot do a record update like this")
	}

	s.CtxLog(ctx, fmt.Sprintf("UpdateRecord %v", request))

	rec, err := s.loadRecord(ctx, request.GetUpdate().GetRelease().InstanceId, false)
	if err != nil {
		return nil, err
	}

	// We are limited in what we can do to records that are in the box
	if rec.GetMetadata().GetBoxState() != pb.ReleaseMetadata_BOX_UNKNOWN && rec.GetMetadata().GetBoxState() != pb.ReleaseMetadata_OUT_OF_BOX {
		if request.GetUpdate().GetMetadata().GetNewBoxState() != pb.ReleaseMetadata_OUT_OF_BOX &&
			request.GetUpdate().GetMetadata().GetMoveFolder() != 3282985 &&
			request.GetUpdate().GetMetadata().GetMoveFolder() != 3291655 &&
			request.GetUpdate().GetMetadata().GetMoveFolder() != 3291970 &&
			request.GetUpdate().GetMetadata().GetMoveFolder() != 3299890 &&
			request.GetUpdate().GetMetadata().GetMoveFolder() != 3358141 &&
			request.GetUpdate().GetMetadata().GetSetRating() == 0 {
			if request.GetUpdate().GetMetadata().GetLastCleanDate() == 0 &&
				request.GetUpdate().GetMetadata().GetRecordWidth() == 0 {
				if request.GetUpdate().GetMetadata().GetLastStockCheck() == 0 {
					if request.GetUpdate().GetMetadata().GetDateArrived() == 0 {
						if request.GetUpdate().GetMetadata().GetGoalFolder() == 0 {
							if request.GetUpdate().GetMetadata().GetFiledUnder() >= 0 {
								s.CtxLog(ctx, fmt.Sprintf("Update %v failed because of the box situation", request))
								return nil, status.Errorf(codes.FailedPrecondition, "You cannot do %v to a given boxed record", request)
							}
						}
					}
				}
			}
		}
	}

	// Set the metadata if it's not
	if rec.GetMetadata() == nil {
		rec.Metadata = &pb.ReleaseMetadata{}
	}

	// Adjust keeper if we keeping a record that's staged to sell that we've marked as not a keeper
	if rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_STAGED_TO_SELL && request.GetUpdate().GetMetadata().GetSetRating() == 5 {
		if rec.GetMetadata().GetKeep() == pb.ReleaseMetadata_NOT_KEEPER {
			rec.GetMetadata().Keep = pb.ReleaseMetadata_KEEP_UNKNOWN
		}
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
		s.RaiseIssue("Update size", fmt.Sprintf("%v has triggered a big update -> %v: %v", request, proto.Size(updates), updates))
	}

	hasLabels := len(rec.GetRelease().GetLabels()) > 0

	// If this is being sold - mark it for sale
	if request.GetUpdate().GetMetadata() != nil && request.GetUpdate().GetMetadata().Category == pb.ReleaseMetadata_SOLD && rec.GetMetadata().Category != pb.ReleaseMetadata_SOLD {
		if !request.NoSell {
			s.CtxLog(ctx, fmt.Sprintf("Running sale path"))
			time.Sleep(time.Second * 2)
			if len(rec.GetRelease().SleeveCondition) == 0 {
				s.cacheRecord(ctx, rec, true)
				if len(rec.GetRelease().SleeveCondition) == 0 {
					s.RaiseIssue(fmt.Sprintf("%v needs condition", rec.GetRelease().GetInstanceId()), "Yes")
					return nil, status.Errorf(codes.FailedPrecondition, "No Condition info")
				}
			}
			if s.disableSales {
				return nil, fmt.Errorf("Sales are disabled")
			}
			price, _ := s.retr.GetSalePrice(ctx, int(rec.GetRelease().Id))
			//230 is approx weight of packaging
			saleid, err := s.retr.SellRecord(ctx, int(rec.GetRelease().Id), price, "For Sale", rec.GetRelease().RecordCondition, rec.GetRelease().SleeveCondition, int(rec.GetMetadata().GetWeightInGrams())+230)
			s.CtxLog(ctx, fmt.Sprintf("Sale return %v and %v => %v", saleid, err, status.Code(err)))
			if err != nil {
				if status.Code(err) == codes.FailedPrecondition {
					// This item is not allowed to be sold, so set to saleid of 1 to filter it out
					saleid = 1
				} else {
					return nil, err
				}
			}
			//saleid := 100

			// Cancel changes in the update
			request.GetUpdate().GetMetadata().SaleId = 0
			request.GetUpdate().GetMetadata().SaleState = 0
			rec.GetMetadata().SaleId = saleid
			rec.GetMetadata().LastSalePriceUpdate = time.Now().Unix()
			rec.GetMetadata().SalePrice = int32(price * 100)
			if saleid > 100 {
				rec.GetMetadata().SaleState = pbgd.SaleState_FOR_SALE
			}
			// Preemptive save to ensure we get the saleid
			s.CtxLog(ctx, fmt.Sprintf("Got sale id (%v): %v", saleid, rec.GetMetadata()))
			s.saveRecord(ctx, rec)
		}
	}

	// If this is a sale update - set the dirty flag
	if request.GetUpdate().GetMetadata().GetNewSalePrice() > 0 || request.GetUpdate().GetMetadata().GetExpireSale() {

		if rec.GetMetadata().SalePrice-request.GetUpdate().GetMetadata().NewSalePrice > 500 && request.GetUpdate().GetMetadata().NewSalePrice > 0 {
			return nil, fmt.Errorf("Price change from %v to %v (for %v) is much too large", rec.GetMetadata().SalePrice, request.GetUpdate().GetMetadata().NewSalePrice, rec.GetRelease().InstanceId)
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

	// Merge in the update
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
		rec.GetMetadata().GetCategory() == pb.ReleaseMetadata_PRE_IN_COLLECTION ||
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
	s.CtxLog(ctx, fmt.Sprintf("Saving with metada: %v [%v]-> %v", rec.GetRelease().GetInstanceId(), err, rec.GetMetadata()))

	if request.GetReason() != "Tripping gram update" {
		upup := &rfpb.FanoutRequest{
			InstanceId: rec.GetRelease().GetInstanceId(),
		}
		data, _ := proto.Marshal(upup)
		_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
			QueueName: "record_fanout",
			RunTime:   time.Now().Add(time.Minute).Unix(),
			Payload:   &google_protobuf.Any{Value: data},
			Key:       fmt.Sprintf("%v", rec.GetRelease().GetInstanceId()),
		})
		s.CtxLog(ctx, fmt.Sprintf("Updating %v for some unknown reason (%v) but :%v:", rec.GetRelease().GetInstanceId(), request, request.GetReason()))
	}

	queueResults.With(prometheus.Labels{"error": fmt.Sprintf("%v", err)}).Inc()

	return &pb.UpdateRecordsResponse{Updated: rec}, err
}

func (s *Server) testForLabels(ctx context.Context, rec *pb.Record, request *pb.UpdateRecordRequest, hasLabels bool) {
	if len(rec.GetRelease().GetLabels()) == 0 && rec.GetMetadata().GetCategory() != pb.ReleaseMetadata_NO_LABELS && hasLabels {
		s.RaiseIssue("Label reduction", fmt.Sprintf("Update %v has reduced label count", request))
	}
}

func (s *Server) transfer(ctx context.Context, rec *pb.Record) (*pb.Record, error) {
	s.CtxLog(ctx, "Transferring")

	// Add a record with the transfer id
	nmeta := proto.Clone(rec.GetMetadata()).(*pb.ReleaseMetadata)
	trecord, err := s.AddRecord(ctx, &pb.AddRecordRequest{
		ToAdd: &pb.Record{
			Release:  &pbgd.Release{Id: rec.GetMetadata().GetTransferTo()},
			Metadata: nmeta}})
	if err != nil {
		return nil, err
	}

	// Remove the transfer bit from the trecord
	trecord.GetAdded().GetMetadata().TransferTo = 0
	trecord.GetAdded().GetMetadata().TransferFrom = rec.GetRelease().GetInstanceId()

	s.CtxLog(ctx, fmt.Sprintf("TRANSFER: %v", trecord))

	return trecord.GetAdded(), nil
}

// AddRecord adds a record directly to the listening pile
func (s *Server) AddRecord(ctx context.Context, request *pb.AddRecordRequest) (*pb.AddRecordResponse, error) {
	if request.GetToAdd().GetMetadata().GetLastUpdateIn() == 0 {
		request.GetToAdd().GetMetadata().LastUpdateIn = 1
	}

	//Reject the add if we don't have a cost or goal folder
	if request.GetToAdd().GetMetadata().GetCost() == 0 || request.GetToAdd().GetMetadata().GetGoalFolder() == 0 {
		return &pb.AddRecordResponse{}, fmt.Errorf("Unable to add - no cost or goal folder (%v)", request)
	}

	s.CtxLog(ctx, fmt.Sprintf("AddRecord %v", request))

	var err error
	instanceID := int(request.GetToAdd().GetRelease().InstanceId)
	if instanceID == 0 {
		instanceID, err = s.retr.AddToFolder(ctx, 3380098, request.GetToAdd().GetRelease().Id)
	}
	if err == nil {
		request.GetToAdd().Release.InstanceId = int32(instanceID)
		if request.GetToAdd().GetRelease().GetFolderId() == 0 {
			request.GetToAdd().GetRelease().FolderId = int32(3380098)
		}
		if request.GetToAdd().GetMetadata().GetDateAdded() == 0 {
			request.GetToAdd().GetMetadata().DateAdded = time.Now().Unix()
		}

		err := s.saveRecord(ctx, request.GetToAdd())
		s.CtxLog(ctx, fmt.Sprintf("Saved record: %v", err))
	}

	upup := &rfpb.FanoutRequest{
		InstanceId: int32(instanceID),
	}
	data, _ := proto.Marshal(upup)
	_, err = s.queueClient.AddQueueItem(ctx, &qpb.AddQueueItemRequest{
		QueueName:     "record_fanout",
		RunTime:       time.Now().Unix(),
		Payload:       &google_protobuf.Any{Value: data},
		Key:           fmt.Sprintf("%v", instanceID),
		RequireUnique: true,
	})
	s.CtxLog(ctx, fmt.Sprintf("Updating because we just added %v", instanceID))
	queueResults.With(prometheus.Labels{"error": fmt.Sprintf("%v", err)}).Inc()

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
		coll := s.retr.GetCollection(ctx)
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
		return nil, fmt.Errorf("no such record exists (from %v)", req)
	}

	// Short cut if we're not asking for a specific release
	if req.GetReleaseId() > 0 {
		got, err := s.retr.GetRelease(ctx, req.GetReleaseId())
		if err != nil {
			return nil, err
		}
		return &pb.GetRecordResponse{Record: &pb.Record{Release: got}}, nil
	}

	rec, err := s.loadRecord(ctx, req.InstanceId, req.GetValidate())
	if rec.GetRelease().GetInstanceId() == 365221500 {
		s.CtxLog(ctx, fmt.Sprintf("READ %v", rec))
	}

	if err != nil {

		if req.GetForce() > 0 {
			rec := &pb.Record{Release: &pbgd.Release{Id: req.GetForce(), InstanceId: req.InstanceId}, Metadata: &pb.ReleaseMetadata{GoalFolder: 242017, Cost: 1}}
			return &pb.GetRecordResponse{Record: rec}, s.cacheRecord(ctx, rec, true)
		}

		st := status.Convert(err)
		if st.Code() == codes.OutOfRange {
			config, err := s.readRecordCollection(ctx)
			if err != nil {
				return nil, err
			}

			delete(config.InstanceToFolder, req.GetInstanceId())
			delete(config.InstanceToUpdate, req.GetInstanceId())
			delete(config.InstanceToUpdateIn, req.GetInstanceId())
			delete(config.InstanceToCategory, req.GetInstanceId())
			delete(config.InstanceToMaster, req.GetInstanceId())
			delete(config.InstanceToId, req.GetInstanceId())
			delete(config.InstanceToRecache, req.GetInstanceId())
			delete(config.InstanceToLastSalePriceUpdate, req.GetInstanceId())

			err = s.saveRecordCollection(ctx, config)
			if err != nil {
				return nil, err
			}
		}

		return nil, status.Errorf(st.Code(), fmt.Sprintf("Could not locate %v -> %v", req.InstanceId, err))
	}

	if rec.GetMetadata().GetTransferIid() > 0 {
		return s.GetRecord(ctx, &pb.GetRecordRequest{InstanceId: rec.GetMetadata().GetTransferIid()})
	}

	return &pb.GetRecordResponse{Record: rec}, err
}

// Trigger runs a local sync
func (s *Server) Trigger(ctx context.Context, req *pb.TriggerRequest) (*pb.TriggerResponse, error) {
	err := s.runSync(ctx)
	return nil, err
}

// GetUpdates to a record
func (s *Server) GetUpdates(ctx context.Context, req *pb.GetUpdatesRequest) (*pb.GetUpdatesResponse, error) {
	updates, err := s.loadUpdates(ctx, req.GetInstanceId())
	return &pb.GetUpdatesResponse{Updates: updates}, err
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	rMap, t, err := s.retr.GetOrder(ctx, req.GetId())
	if err != nil {
		return nil, err
	}

	resp := &pb.GetOrderResponse{
		SaleDate:       t.Unix(),
		ListingToPrice: make(map[int64]int32),
	}
	for ID, price := range rMap {
		resp.ListingToPrice[ID] = price
	}

	return resp, nil
}

func (s *Server) GetPrice(ctx context.Context, req *pb.GetPriceRequest) (*pb.GetPriceResponse, error) {
	time.Sleep(time.Second * 5)
	price, err := s.retr.GetSalePrice(ctx, int(req.GetId()))
	return &pb.GetPriceResponse{Price: price}, err
}
