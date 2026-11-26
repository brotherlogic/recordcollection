package recordcollection_client

import (
	"context"
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/brotherlogic/recordcollection/proto"
)

func TestSearchReleases_ErrorCode(t *testing.T) {
	tests := []struct {
		name      string
		errorCode codes.Code
		withHooks bool
	}{
		{
			name:      "resource exhausted",
			errorCode: codes.ResourceExhausted,
		},
		{
			name:      "unavailable overrides search hooks",
			errorCode: codes.Unavailable,
			withHooks: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			funcCalled := false
			c := &RecordCollectionClient{
				Test:      true,
				ErrorCode: tc.errorCode,
			}
			if tc.withHooks {
				c.SearchResults = []*pb.SearchReleaseResult{{ReleaseId: 99, Title: "Ignored"}}
				c.SearchFunc = func(ctx context.Context, req *pb.SearchReleasesRequest) (*pb.SearchReleasesResponse, error) {
					funcCalled = true
					return &pb.SearchReleasesResponse{}, nil
				}
			}

			resp, err := c.SearchReleases(context.Background(), &pb.SearchReleasesRequest{Artist: "Artist"})
			if resp != nil {
				t.Fatalf("expected nil response on error, got %+v", resp)
			}
			if status.Code(err) != tc.errorCode {
				t.Fatalf("expected status code %v, got %v (err=%v)", tc.errorCode, status.Code(err), err)
			}
			if funcCalled {
				t.Fatalf("expected SearchFunc not to be called when ErrorCode is set")
			}
		})
	}
}

func TestSearchReleases_StaticSearchResults(t *testing.T) {
	t.Run("returns configured static results", func(t *testing.T) {
		expected := []*pb.SearchReleaseResult{
			{
				ReleaseId: 12345,
				Title:     "Artist - Album",
				Year:      "2024",
				Formats:   []string{"File", "FLAC"},
				Country:   "UK",
				Url:       "https://www.discogs.com/release/12345",
			},
		}
		c := &RecordCollectionClient{
			Test:          true,
			SearchResults: expected,
		}

		resp, err := c.SearchReleases(context.Background(), &pb.SearchReleasesRequest{
			Artist:       "Artist",
			ReleaseTitle: "Album",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatalf("expected non-nil response")
		}
		if !reflect.DeepEqual(resp.GetResults(), expected) {
			t.Fatalf("got results %+v, want %+v", resp.GetResults(), expected)
		}
	})

	t.Run("returns empty results when unset", func(t *testing.T) {
		c := &RecordCollectionClient{
			Test: true,
		}

		resp, err := c.SearchReleases(context.Background(), &pb.SearchReleasesRequest{Q: "anything"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil {
			t.Fatalf("expected non-nil response")
		}
		if len(resp.GetResults()) != 0 {
			t.Fatalf("expected empty results, got %+v", resp.GetResults())
		}
	})
}

func TestSearchReleases_SearchFunc(t *testing.T) {
	t.Run("multi-call fallback scenario", func(t *testing.T) {
		var calls []*pb.SearchReleasesRequest
		expectedMatch := &pb.SearchReleaseResult{
			ReleaseId: 67890,
			Title:     "Boards of Canada - Music Has the Right to Children",
			Year:      "1998",
			Formats:   []string{"Vinyl"},
			Country:   "UK",
			Url:       "https://www.discogs.com/release/67890",
		}

		c := &RecordCollectionClient{
			Test: true,
			SearchResults: []*pb.SearchReleaseResult{
				{ReleaseId: 111, Title: "Should be overridden by SearchFunc"},
			},
			SearchFunc: func(ctx context.Context, req *pb.SearchReleasesRequest) (*pb.SearchReleasesResponse, error) {
				calls = append(calls, req)
				if req.GetFormat() == "File" {
					return &pb.SearchReleasesResponse{}, nil
				}
				return &pb.SearchReleasesResponse{
					Results: []*pb.SearchReleaseResult{expectedMatch},
				}, nil
			},
		}

		firstReq := &pb.SearchReleasesRequest{
			Artist:       "Boards of Canada",
			ReleaseTitle: "Music Has the Right to Children",
			Format:       "File",
		}
		firstResp, err := c.SearchReleases(context.Background(), firstReq)
		if err != nil {
			t.Fatalf("first call unexpected error: %v", err)
		}
		if len(firstResp.GetResults()) != 0 {
			t.Fatalf("expected empty first response, got %+v", firstResp.GetResults())
		}

		secondReq := &pb.SearchReleasesRequest{
			Artist:       "Boards of Canada",
			ReleaseTitle: "Music Has the Right to Children",
		}
		secondResp, err := c.SearchReleases(context.Background(), secondReq)
		if err != nil {
			t.Fatalf("second call unexpected error: %v", err)
		}
		if len(secondResp.GetResults()) != 1 || !reflect.DeepEqual(secondResp.GetResults()[0], expectedMatch) {
			t.Fatalf("got second response %+v, want [%+v]", secondResp.GetResults(), expectedMatch)
		}

		if len(calls) != 2 {
			t.Fatalf("expected 2 SearchFunc calls, got %d", len(calls))
		}
		if calls[0].GetFormat() != "File" || calls[0].GetArtist() != "Boards of Canada" || calls[0].GetReleaseTitle() != "Music Has the Right to Children" {
			t.Fatalf("unexpected first request: %+v", calls[0])
		}
		if calls[1].GetFormat() != "" || calls[1].GetArtist() != "Boards of Canada" || calls[1].GetReleaseTitle() != "Music Has the Right to Children" {
			t.Fatalf("unexpected second request: %+v", calls[1])
		}
	})

	t.Run("propagates SearchFunc error", func(t *testing.T) {
		expectedErr := status.Error(codes.Unavailable, "upstream down")
		c := &RecordCollectionClient{
			Test: true,
			SearchFunc: func(ctx context.Context, req *pb.SearchReleasesRequest) (*pb.SearchReleasesResponse, error) {
				return nil, expectedErr
			},
		}

		resp, err := c.SearchReleases(context.Background(), &pb.SearchReleasesRequest{Q: "test"})
		if resp != nil {
			t.Fatalf("expected nil response on error, got %+v", resp)
		}
		if status.Code(err) != codes.Unavailable || err.Error() != expectedErr.Error() {
			t.Fatalf("got err %v, want %v", err, expectedErr)
		}
	})
}
