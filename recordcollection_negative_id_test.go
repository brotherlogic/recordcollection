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
