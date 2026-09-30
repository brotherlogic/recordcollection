package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	pbgd "github.com/brotherlogic/godiscogs/proto"
	pbrc "github.com/brotherlogic/recordcollection/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type mockRecordCollectionClient struct {
	pbrc.RecordCollectionServiceClient
	updateRecordFn func(ctx context.Context, req *pbrc.UpdateRecordRequest, opts ...grpc.CallOption) (*pbrc.UpdateRecordsResponse, error)
}

func (m *mockRecordCollectionClient) UpdateRecord(ctx context.Context, req *pbrc.UpdateRecordRequest, opts ...grpc.CallOption) (*pbrc.UpdateRecordsResponse, error) {
	if m.updateRecordFn != nil {
		return m.updateRecordFn(ctx, req, opts...)
	}
	return nil, errors.New("updateRecordFn not implemented")
}

func TestParseOutOfPlayArgs_MissingArgs(t *testing.T) {
	testCases := [][]string{
		{"recordcollection_cli"},
		{"recordcollection_cli", "out_of_play"},
		{"recordcollection_cli", "out_of_play", "123"},
	}

	for _, args := range testCases {
		_, _, err := parseOutOfPlayArgs(args)
		if err == nil {
			t.Errorf("parseOutOfPlayArgs(%v) expected error for missing args, got nil", args)
		}
		if !strings.Contains(err.Error(), "Usage:") {
			t.Errorf("parseOutOfPlayArgs(%v) error %q does not contain usage instructions", args, err.Error())
		}
	}
}

func TestParseOutOfPlayArgs_InvalidInstanceID(t *testing.T) {
	args := []string{"recordcollection_cli", "out_of_play", "not-a-number", "true"}
	_, _, err := parseOutOfPlayArgs(args)
	if err == nil {
		t.Fatalf("parseOutOfPlayArgs(%v) expected error for invalid instance ID, got nil", args)
	}
	if !strings.Contains(err.Error(), "invalid instance ID") {
		t.Errorf("parseOutOfPlayArgs(%v) error %q expected to mention invalid instance ID", args, err.Error())
	}
}

func TestParseOutOfPlayArgs_InvalidBoolean(t *testing.T) {
	args := []string{"recordcollection_cli", "out_of_play", "12345", "maybe"}
	_, _, err := parseOutOfPlayArgs(args)
	if err == nil {
		t.Fatalf("parseOutOfPlayArgs(%v) expected error for invalid boolean, got nil", args)
	}
	if !strings.Contains(err.Error(), "invalid boolean") {
		t.Errorf("parseOutOfPlayArgs(%v) error %q expected to mention invalid boolean", args, err.Error())
	}
}

func TestParseOutOfPlayArgs_ValidTrue(t *testing.T) {
	args := []string{"recordcollection_cli", "out_of_play", "12345", "true"}
	iid, val, err := parseOutOfPlayArgs(args)
	if err != nil {
		t.Fatalf("parseOutOfPlayArgs(%v) unexpected error: %v", args, err)
	}
	if iid != 12345 {
		t.Errorf("parseOutOfPlayArgs(%v) got iid %d, want 12345", args, iid)
	}
	if !val {
		t.Errorf("parseOutOfPlayArgs(%v) got val false, want true", args)
	}
}

func TestParseOutOfPlayArgs_ValidFalse(t *testing.T) {
	args := []string{"recordcollection_cli", "out_of_play", "98765", "false"}
	iid, val, err := parseOutOfPlayArgs(args)
	if err != nil {
		t.Fatalf("parseOutOfPlayArgs(%v) unexpected error: %v", args, err)
	}
	if iid != 98765 {
		t.Errorf("parseOutOfPlayArgs(%v) got iid %d, want 98765", args, iid)
	}
	if val {
		t.Errorf("parseOutOfPlayArgs(%v) got val true, want false", args)
	}
}

func TestBuildOutOfPlayRequest(t *testing.T) {
	req := buildOutOfPlayRequest(54321, true)
	if req.GetReason() != "CLI-out-of-play" {
		t.Errorf("buildOutOfPlayRequest Reason = %q, want %q", req.GetReason(), "CLI-out-of-play")
	}
	if req.GetUpdate().GetRelease().GetInstanceId() != 54321 {
		t.Errorf("buildOutOfPlayRequest InstanceId = %d, want %d", req.GetUpdate().GetRelease().GetInstanceId(), 54321)
	}
	if !req.GetUpdate().GetMetadata().GetOutOfPlay() {
		t.Errorf("buildOutOfPlayRequest OutOfPlay = false, want true")
	}

	reqFalse := buildOutOfPlayRequest(54321, false)
	if reqFalse.GetUpdate().GetMetadata().GetOutOfPlay() {
		t.Errorf("buildOutOfPlayRequest OutOfPlay = true, want false")
	}
}

func TestRunOutOfPlay_Success(t *testing.T) {
	called := false
	mock := &mockRecordCollectionClient{
		updateRecordFn: func(ctx context.Context, req *pbrc.UpdateRecordRequest, opts ...grpc.CallOption) (*pbrc.UpdateRecordsResponse, error) {
			called = true
			if req.GetReason() != "CLI-out-of-play" {
				t.Errorf("unexpected reason: %v", req.GetReason())
			}
			if req.GetUpdate().GetRelease().GetInstanceId() != 123 {
				t.Errorf("unexpected instance id: %v", req.GetUpdate().GetRelease().GetInstanceId())
			}
			if !req.GetUpdate().GetMetadata().GetOutOfPlay() {
				t.Errorf("unexpected out_of_play: %v", req.GetUpdate().GetMetadata().GetOutOfPlay())
			}
			return &pbrc.UpdateRecordsResponse{
				Updated: &pbrc.Record{
					Release: &pbgd.Release{
						InstanceId: 123,
					},
					Metadata: &pbrc.ReleaseMetadata{
						OutOfPlay: proto.Bool(true),
					},
				},
			}, nil
		},
	}

	args := []string{"recordcollection_cli", "out_of_play", "123", "true"}
	err := runOutOfPlay(context.Background(), mock, args)
	if err != nil {
		t.Fatalf("runOutOfPlay failed: %v", err)
	}
	if !called {
		t.Errorf("UpdateRecord was not called")
	}
}

func TestRunOutOfPlay_UpdateError(t *testing.T) {
	mock := &mockRecordCollectionClient{
		updateRecordFn: func(ctx context.Context, req *pbrc.UpdateRecordRequest, opts ...grpc.CallOption) (*pbrc.UpdateRecordsResponse, error) {
			return nil, errors.New("rpc network failure")
		},
	}

	args := []string{"recordcollection_cli", "out_of_play", "123", "true"}
	err := runOutOfPlay(context.Background(), mock, args)
	if err == nil {
		t.Fatalf("runOutOfPlay expected error on RPC failure, got nil")
	}
	if !strings.Contains(err.Error(), "Failed to update record 123") {
		t.Errorf("unexpected error message: %v", err)
	}
}
