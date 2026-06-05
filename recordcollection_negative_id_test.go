package main

import (
	"context"
	"testing"

	pb "github.com/brotherlogic/recordcollection/proto"
	pbgd "github.com/brotherlogic/godiscogs/proto"
)

func TestNegativeIdMigration(t *testing.T) {
	s := InitTestServer(".test_negative_id")
	
	// Create a record with a positive ID that maps to a negative int32
	// e.g. 2147937650 in uint32 is -2147029646
	var originalPositiveId int64 = 2147937650

	// Simulate saving it under the negative ID in the keystore directly to mock the old behavior
	oldRecord := &pb.Record{
		Release: &pbgd.Release{
			InstanceId: originalPositiveId, // The struct had int64 or int32, but let's assume it had the negative one
		},
		Metadata: &pb.ReleaseMetadata{
			InstanceId: int64(int32(originalPositiveId)), // set to negative initially
		},
	}
	// We must use int64(int32(originalPositiveId)) which evaluates to -2147029646 to save it under the old key
	s.KSclient.Save(context.Background(), "/github.com/brotherlogic/recordcollection/records/-2147029646", oldRecord)

	// Now try to load the record using the positive ID. 
	// loadRecord should fallback to the negative ID, fix it, resave under positive ID, and delete negative ID.
	req := &pb.GetRecordRequest{
		InstanceId: originalPositiveId,
	}

	res, err := s.GetRecord(context.Background(), req)
	if err != nil {
		t.Fatalf("Failed to GetRecord: %v", err)
	}

	if res.GetRecord().GetRelease().GetInstanceId() != originalPositiveId {
		t.Errorf("Expected instance ID %v, got %v", originalPositiveId, res.GetRecord().GetRelease().GetInstanceId())
	}

	if res.GetRecord().GetMetadata().GetInstanceId() != originalPositiveId {
		t.Errorf("Expected metadata instance ID %v, got %v", originalPositiveId, res.GetRecord().GetMetadata().GetInstanceId())
	}

	// Verify the new positive ID exists in keystore
	newRecord := &pb.Record{}
	_, _, err = s.KSclient.Read(context.Background(), "/github.com/brotherlogic/recordcollection/records/2147937650", newRecord)
	if err != nil {
		t.Errorf("Failed to read new positive key from keystore: %v", err)
	}

}

func TestNegativeIdNotInCacheAfterRead(t *testing.T) {
	s := InitTestServer(".test_negative_id_cache")

	// Build a RecordCollection with negative instance IDs in all cache maps
	dirty := &pb.RecordCollection{
		InstanceToFolder:              map[int64]int32{-1: 10, 100: 10},
		InstanceToCategory:            map[int64]pb.ReleaseMetadata_Category{-2: pb.ReleaseMetadata_UNLISTENED, 200: pb.ReleaseMetadata_UNLISTENED},
		InstanceToUpdate:              map[int64]int64{-3: 999, 300: 999},
		InstanceToUpdateIn:            map[int64]int64{-4: 888, 400: 888},
		InstanceToMaster:              map[int64]int32{-5: 50, 500: 50},
		InstanceToId:                  map[int64]int32{-6: 60, 600: 60},
		InstanceToRecache:             map[int64]int64{-7: 777, 700: 777},
		InstanceToLastSalePriceUpdate: map[int64]int64{-8: 666, 800: 666},
	}
	s.KSclient.Save(context.Background(), KEY, dirty)

	collection, err := s.readRecordCollection(context.Background())
	if err != nil {
		t.Fatalf("readRecordCollection failed: %v", err)
	}

	// Check every map – no key should be negative
	for k := range collection.GetInstanceToFolder() {
		if k < 0 {
			t.Errorf("InstanceToFolder still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToCategory() {
		if k < 0 {
			t.Errorf("InstanceToCategory still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToUpdate() {
		if k < 0 {
			t.Errorf("InstanceToUpdate still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToUpdateIn() {
		if k < 0 {
			t.Errorf("InstanceToUpdateIn still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToMaster() {
		if k < 0 {
			t.Errorf("InstanceToMaster still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToId() {
		if k < 0 {
			t.Errorf("InstanceToId still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToRecache() {
		if k < 0 {
			t.Errorf("InstanceToRecache still has negative key %v", k)
		}
	}
	for k := range collection.GetInstanceToLastSalePriceUpdate() {
		if k < 0 {
			t.Errorf("InstanceToLastSalePriceUpdate still has negative key %v", k)
		}
	}

	// Positive keys must be preserved
	if _, ok := collection.GetInstanceToFolder()[100]; !ok {
		t.Errorf("InstanceToFolder lost its positive key 100")
	}
}
