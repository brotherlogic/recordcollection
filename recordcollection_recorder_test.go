package main

import (
	"context"
	"net"
	"testing"

	rpb "github.com/brotherlogic/recorder/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockQualityServer struct {
	rpb.UnimplementedQualityServiceServer
	lastReq *rpb.GetQualityRequest
	score   int32
	err     error
}

func (m *mockQualityServer) GetQuality(ctx context.Context, req *rpb.GetQualityRequest) (*rpb.GetQualityResponse, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &rpb.GetQualityResponse{Score: m.score}, nil
}

func TestProdQualityClient_GetQuality_Success(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	mockServer := &mockQualityServer{score: 85}
	grpcServer := grpc.NewServer()
	rpb.RegisterQualityServiceServer(grpcServer, mockServer)

	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	client := &prodQualityClient{}
	score, err := client.getQuality(context.Background(), lis.Addr().String(), 12345)
	if err != nil {
		t.Fatalf("unexpected error from getQuality: %v", err)
	}
	if score != 85 {
		t.Errorf("expected score 85, got %d", score)
	}
	if mockServer.lastReq == nil || mockServer.lastReq.GetReleaseId() != 12345 {
		t.Errorf("expected release id 12345 in request, got %v", mockServer.lastReq)
	}
}

func TestProdQualityClient_GetQuality_RPCError(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	mockServer := &mockQualityServer{err: status.Errorf(codes.Internal, "boom")}
	grpcServer := grpc.NewServer()
	rpb.RegisterQualityServiceServer(grpcServer, mockServer)

	go func() {
		_ = grpcServer.Serve(lis)
	}()
	defer grpcServer.Stop()

	client := &prodQualityClient{}
	_, err = client.getQuality(context.Background(), lis.Addr().String(), 12345)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if status.Code(err) != codes.Unavailable {
		t.Errorf("expected Unavailable error code, got %v", status.Code(err))
	}
}

func TestServer_Init_RecorderClientWired(t *testing.T) {
	s := Init()
	if s.recorderClient == nil {
		t.Fatalf("expected recorderClient to be wired in Init(), got nil")
	}
	if _, ok := s.recorderClient.(*prodQualityClient); !ok {
		t.Errorf("expected recorderClient to be *prodQualityClient, got %T", s.recorderClient)
	}
}
