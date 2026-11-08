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

func TestProdGenerator_MediaFormat(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	mockServer := &mockSaleDescriptionServer{
		resp: &v1.GenerateDescriptionResponse{
			Description: "Generated Description",
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

	tests := []struct {
		name           string
		rec            *pb.Record
		expectedFormat v1.MediaFormat
	}{
		{
			name: "Vinyl from Release Formats (Vinyl/LP)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Kind of Blue",
					RecordCondition: "Near Mint (NM or M-)",
					SleeveCondition: "Very Good Plus (VG+)",
					Formats: []*pbgd.Format{
						{Name: "Vinyl", Descriptions: []string{"LP", "Album"}},
					},
				},
				Metadata: &pb.ReleaseMetadata{Notes: "Great copy"},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_VINYL,
		},
		{
			name: "Vinyl from Metadata FiledUnder (FILE_12_INCH)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Blue Train",
					RecordCondition: "Very Good Plus (VG+)",
					SleeveCondition: "Very Good Plus (VG+)",
				},
				Metadata: &pb.ReleaseMetadata{
					Notes:      "Clean playback",
					FiledUnder: pb.ReleaseMetadata_FILE_12_INCH,
				},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_VINYL,
		},
		{
			name: "Vinyl from Metadata FiledUnder (FILE_7_INCH)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Single 45",
					RecordCondition: "Very Good Plus (VG+)",
					SleeveCondition: "Very Good Plus (VG+)",
				},
				Metadata: &pb.ReleaseMetadata{
					Notes:      "Nice 7 inch",
					FiledUnder: pb.ReleaseMetadata_FILE_7_INCH,
				},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_VINYL,
		},
		{
			name: "CD from Release Formats (CD)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "OK Computer",
					RecordCondition: "Near Mint (NM or M-)",
					SleeveCondition: "Near Mint (NM or M-)",
					Formats: []*pbgd.Format{
						{Name: "CD", Descriptions: []string{"Album"}},
					},
				},
				Metadata: &pb.ReleaseMetadata{Notes: "Clean disc"},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_CD,
		},
		{
			name: "CD from Metadata FiledUnder (FILE_CD)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Kid A",
					RecordCondition: "Near Mint (NM or M-)",
					SleeveCondition: "Very Good Plus (VG+)",
				},
				Metadata: &pb.ReleaseMetadata{
					Notes:      "Includes booklet",
					FiledUnder: pb.ReleaseMetadata_FILE_CD,
				},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_CD,
		},
		{
			name: "Mixed from Release Formats (Vinyl + CD)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Deluxe Edition",
					RecordCondition: "Near Mint (NM or M-)",
					SleeveCondition: "Near Mint (NM or M-)",
					Formats: []*pbgd.Format{
						{Name: "Vinyl", Descriptions: []string{"LP"}},
						{Name: "CD", Descriptions: []string{"Album"}},
					},
				},
				Metadata: &pb.ReleaseMetadata{Notes: "Includes bonus CD"},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_MIXED,
		},
		{
			name: "Mixed from Metadata FiledUnder (FILE_BOXSET)",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Complete Box Set",
					RecordCondition: "Near Mint (NM or M-)",
					SleeveCondition: "Very Good Plus (VG+)",
				},
				Metadata: &pb.ReleaseMetadata{
					Notes:      "Complete with book",
					FiledUnder: pb.ReleaseMetadata_FILE_BOXSET,
				},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_MIXED,
		},
		{
			name: "Unspecified when no format or filing metadata",
			rec: &pb.Record{
				Release: &pbgd.Release{
					Title:           "Mystery Release",
					RecordCondition: "Very Good (VG)",
					SleeveCondition: "Very Good (VG)",
				},
				Metadata: &pb.ReleaseMetadata{Notes: "Solid condition"},
			},
			expectedFormat: v1.MediaFormat_MEDIA_FORMAT_UNSPECIFIED,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer.lastReq = nil
			_, err := gen.generate(context.Background(), lis.Addr().String(), tc.rec)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mockServer.lastReq == nil {
				t.Fatalf("mock server did not receive request")
			}
			if mockServer.lastReq.GetMediaFormat() != tc.expectedFormat {
				t.Errorf("expected MediaFormat %v, got %v", tc.expectedFormat, mockServer.lastReq.GetMediaFormat())
			}
		})
	}
}
