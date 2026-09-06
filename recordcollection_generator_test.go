package main

import (
	"context"
	"net"
	"testing"

	pbgd "github.com/brotherlogic/godiscogs/proto"
	pb "github.com/brotherlogic/recordcollection/proto"
	v1 "github.com/brotherlogic/recordcollection/proto/v1"
	"google.golang.org/grpc"
)

type mockSaleDescriptionServer struct {
	v1.UnimplementedSaleDescriptionServiceServer
	lastReq *v1.GenerateDescriptionRequest
	resp    *v1.GenerateDescriptionResponse
	err     error
}

func (m *mockSaleDescriptionServer) GenerateDescription(ctx context.Context, req *v1.GenerateDescriptionRequest) (*v1.GenerateDescriptionResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestProdGenerator_UseLocalModel(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	mockServer := &mockSaleDescriptionServer{
		resp: &v1.GenerateDescriptionResponse{
			Description: "Test Description with local model",
		},
	}

	grpcServer := grpc.NewServer()
	v1.RegisterSaleDescriptionServiceServer(grpcServer, mockServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			return
		}
	}()
	defer grpcServer.Stop()

	gen := &prodGenerator{}
	rec := &pb.Record{
		Release: &pbgd.Release{
			Id:              12345,
			Title:           "Dark Side of the Moon",
			RecordCondition: "Very Good Plus (VG+)",
			SleeveCondition: "Near Mint (NM or M-)",
			Artists: []*pbgd.Artist{
				{Name: "Pink Floyd"},
			},
		},
		Metadata: &pb.ReleaseMetadata{
			Notes: "Original UK pressing",
		},
	}

	desc, err := gen.generate(context.Background(), lis.Addr().String(), rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if desc != "Test Description with local model" {
		t.Errorf("expected 'Test Description with local model', got %q", desc)
	}

	if mockServer.lastReq == nil {
		t.Fatalf("mock server did not receive request")
	}

	if !mockServer.lastReq.GetUseLocalModel() {
		t.Errorf("expected UseLocalModel to be true, got %v", mockServer.lastReq.GetUseLocalModel())
	}
}
