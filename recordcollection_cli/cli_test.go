package main

import (
	"bytes"
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
	queryRecordsFn func(ctx context.Context, req *pbrc.QueryRecordsRequest, opts ...grpc.CallOption) (*pbrc.QueryRecordsResponse, error)
	getRecordFn    func(ctx context.Context, req *pbrc.GetRecordRequest, opts ...grpc.CallOption) (*pbrc.GetRecordResponse, error)
}

func (m *mockRecordCollectionClient) UpdateRecord(ctx context.Context, req *pbrc.UpdateRecordRequest, opts ...grpc.CallOption) (*pbrc.UpdateRecordsResponse, error) {
	if m.updateRecordFn != nil {
		return m.updateRecordFn(ctx, req, opts...)
	}
	return nil, errors.New("updateRecordFn not implemented")
}

func (m *mockRecordCollectionClient) QueryRecords(ctx context.Context, req *pbrc.QueryRecordsRequest, opts ...grpc.CallOption) (*pbrc.QueryRecordsResponse, error) {
	if m.queryRecordsFn != nil {
		return m.queryRecordsFn(ctx, req, opts...)
	}
	return nil, errors.New("queryRecordsFn not implemented")
}

func (m *mockRecordCollectionClient) GetRecord(ctx context.Context, req *pbrc.GetRecordRequest, opts ...grpc.CallOption) (*pbrc.GetRecordResponse, error) {
	if m.getRecordFn != nil {
		return m.getRecordFn(ctx, req, opts...)
	}
	return nil, errors.New("getRecordFn not implemented")
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

func TestPrintSleeveBoxsets_FiltersCorrectly(t *testing.T) {
	records := map[int64]*pbrc.Record{
		101: {
			Release:  &pbgd.Release{InstanceId: 101, Title: "Box Set 1"},
			Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_IN_COLLECTION, Sleeve: pbrc.ReleaseMetadata_BOX_SET},
		},
		102: {
			Release:  &pbgd.Release{InstanceId: 102, Title: "Regular 1"},
			Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_LISTED_TO_SELL, Sleeve: pbrc.ReleaseMetadata_VINYL_STORAGE_DOUBLE_FLAP},
		},
		103: {
			Release:  &pbgd.Release{InstanceId: 103, Title: "Box Set 2"},
			Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_STAGED, Sleeve: pbrc.ReleaseMetadata_BOX_SET},
		},
		104: {
			Release:  &pbgd.Release{InstanceId: 104, Title: "Regular 2"},
			Metadata: &pbrc.ReleaseMetadata{Category: pbrc.ReleaseMetadata_UNLISTENED, Sleeve: pbrc.ReleaseMetadata_SLEEVE_UNKNOWN},
		},
	}

	mock := &mockRecordCollectionClient{
		queryRecordsFn: func(ctx context.Context, req *pbrc.QueryRecordsRequest, opts ...grpc.CallOption) (*pbrc.QueryRecordsResponse, error) {
			return &pbrc.QueryRecordsResponse{
				InstanceIds: []int64{101, 102, 103, 104},
			}, nil
		},
		getRecordFn: func(ctx context.Context, req *pbrc.GetRecordRequest, opts ...grpc.CallOption) (*pbrc.GetRecordResponse, error) {
			rec, ok := records[req.GetInstanceId()]
			if !ok {
				return nil, errors.New("record not found")
			}
			return &pbrc.GetRecordResponse{Record: rec}, nil
		},
	}

	buf := &bytes.Buffer{}
	err := printSleeveBoxsets(context.Background(), mock, buf)
	if err != nil {
		t.Fatalf("printSleeveBoxsets unexpected error: %v", err)
	}

	output := buf.String()
	expectedLines := []string{
		"IN_COLLECTION 101",
		"STAGED 103",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, but got: %q", expected, output)
		}
	}

	if strings.Contains(output, "102") {
		t.Errorf("output should not contain instance 102 (not a box set): %q", output)
	}
	if strings.Contains(output, "104") {
		t.Errorf("output should not contain instance 104 (not a box set): %q", output)
	}
}

func TestPrintSleeveBoxsets_QueryError(t *testing.T) {
	mock := &mockRecordCollectionClient{
		queryRecordsFn: func(ctx context.Context, req *pbrc.QueryRecordsRequest, opts ...grpc.CallOption) (*pbrc.QueryRecordsResponse, error) {
			return nil, errors.New("query failed")
		},
	}

	buf := &bytes.Buffer{}
	err := printSleeveBoxsets(context.Background(), mock, buf)
	if err == nil {
		t.Fatalf("printSleeveBoxsets expected error on QueryRecords failure, got nil")
	}
}

func TestPrintSleeveBoxsets_GetRecordError(t *testing.T) {
	mock := &mockRecordCollectionClient{
		queryRecordsFn: func(ctx context.Context, req *pbrc.QueryRecordsRequest, opts ...grpc.CallOption) (*pbrc.QueryRecordsResponse, error) {
			return &pbrc.QueryRecordsResponse{
				InstanceIds: []int64{101},
			}, nil
		},
		getRecordFn: func(ctx context.Context, req *pbrc.GetRecordRequest, opts ...grpc.CallOption) (*pbrc.GetRecordResponse, error) {
			return nil, errors.New("get record failed")
		},
	}

	buf := &bytes.Buffer{}
	err := printSleeveBoxsets(context.Background(), mock, buf)
	if err == nil {
		t.Fatalf("printSleeveBoxsets expected error on GetRecord failure, got nil")
	}
}
