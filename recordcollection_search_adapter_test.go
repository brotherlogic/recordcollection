package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	godiscogs "github.com/brotherlogic/godiscogs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	_ godiscogsSearcher = (*godiscogs.DiscogsRetriever)(nil)
	_ discogsSearcher   = (*godiscogsSearcherAdapter)(nil)
)

type fakeGodiscogsSearcher struct {
	gotCtx    context.Context
	gotParams godiscogs.SearchParams
	results   []*godiscogs.SearchResult
	err       error
}

func (f *fakeGodiscogsSearcher) Search(ctx context.Context, params godiscogs.SearchParams) ([]*godiscogs.SearchResult, error) {
	f.gotCtx = ctx
	f.gotParams = params
	return f.results, f.err
}

func TestGodiscogsSearcherAdapterMapsAllParamsAndForcesReleaseType(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-id")

	fake := &fakeGodiscogsSearcher{
		results: []*godiscogs.SearchResult{
			{
				ID:          12345,
				Title:       "Artist - Album",
				Year:        "2024",
				Formats:     []string{"Vinyl", "LP"},
				Country:     "US",
				URI:         "/release/12345-Artist-Album",
				ResourceURL: "https://api.discogs.com/releases/12345",
			},
		},
	}
	adapter := &godiscogsSearcherAdapter{retr: fake}

	in := searchParams{
		Query:        "ambient synth",
		Type:         "master",
		Title:        "Artist - Album",
		ReleaseTitle: "Album",
		Credit:       "Producer Name",
		Artist:       "Artist",
		ANV:          "Artist Var",
		Label:        "Warp",
		Genre:        "Electronic",
		Style:        "Ambient",
		Country:      "UK",
		Year:         "2024",
		Format:       "Vinyl",
		Catno:        "WARP001",
		Barcode:      "1234567890123",
		Track:        "Opening",
		Submitter:    "user1",
		Contributor:  "user2",
		PerPage:      25,
		Sort:         "year",
		SortOrder:    "desc",
	}

	got, err := adapter.Search(ctx, in)
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}

	if fake.gotCtx != ctx {
		t.Errorf("Search() did not forward context unchanged")
	}

	wantParams := godiscogs.SearchParams{
		Query:        "ambient synth",
		Type:         "release",
		Title:        "Artist - Album",
		ReleaseTitle: "Album",
		Credit:       "Producer Name",
		Artist:       "Artist",
		ANV:          "Artist Var",
		Label:        "Warp",
		Genre:        "Electronic",
		Style:        "Ambient",
		Country:      "UK",
		Year:         "2024",
		Format:       "Vinyl",
		Catno:        "WARP001",
		Barcode:      "1234567890123",
		Track:        "Opening",
		Submitter:    "user1",
		Contributor:  "user2",
		PerPage:      25,
		Sort:         "year",
		SortOrder:    "desc",
	}
	if !reflect.DeepEqual(fake.gotParams, wantParams) {
		t.Errorf("gotParams = %+v, want %+v", fake.gotParams, wantParams)
	}

	wantResults := []*searchResult{
		{
			ID:          12345,
			Title:       "Artist - Album",
			Year:        "2024",
			Formats:     []string{"Vinyl", "LP"},
			Country:     "US",
			URI:         "/release/12345-Artist-Album",
			ResourceURL: "https://api.discogs.com/releases/12345",
		},
	}
	if !reflect.DeepEqual(got, wantResults) {
		t.Errorf("got = %+v, want %+v", got, wantResults)
	}
}

func TestGodiscogsSearcherAdapterAlwaysForcesReleaseType(t *testing.T) {
	for _, inputType := range []string{"", "master", "artist", "label", "release"} {
		t.Run("type_"+inputType, func(t *testing.T) {
			fake := &fakeGodiscogsSearcher{}
			adapter := &godiscogsSearcherAdapter{retr: fake}

			_, err := adapter.Search(context.Background(), searchParams{
				Query: "test",
				Type:  inputType,
			})
			if err != nil {
				t.Fatalf("Search() unexpected error: %v", err)
			}
			if fake.gotParams.Type != "release" {
				t.Errorf("gotParams.Type = %q, want %q", fake.gotParams.Type, "release")
			}
		})
	}
}

func TestGodiscogsSearcherAdapterResultMappingEdgeCases(t *testing.T) {
	cases := []struct {
		name      string
		gdResults []*godiscogs.SearchResult
		want      []*searchResult
	}{
		{
			name:      "empty_slice",
			gdResults: []*godiscogs.SearchResult{},
			want:      []*searchResult{},
		},
		{
			name:      "nil_slice",
			gdResults: nil,
			want:      []*searchResult{},
		},
		{
			name: "multiple_partial_and_nil_elements",
			gdResults: []*godiscogs.SearchResult{
				{
					ID:          101,
					Title:       "First Release",
					Year:        "1999",
					Formats:     []string{"File", "FLAC"},
					Country:     "DE",
					URI:         "/release/101",
					ResourceURL: "https://api.discogs.com/releases/101",
				},
				nil,
				{
					ID:    202,
					Title: "Partial Release",
				},
			},
			want: []*searchResult{
				{
					ID:          101,
					Title:       "First Release",
					Year:        "1999",
					Formats:     []string{"File", "FLAC"},
					Country:     "DE",
					URI:         "/release/101",
					ResourceURL: "https://api.discogs.com/releases/101",
				},
				{
					ID:    202,
					Title: "Partial Release",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeGodiscogsSearcher{results: tc.gdResults}
			adapter := &godiscogsSearcherAdapter{retr: fake}

			got, err := adapter.Search(context.Background(), searchParams{Query: "test"})
			if err != nil {
				t.Fatalf("Search() unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("Search() returned nil slice, want non-nil slice")
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestGodiscogsSearcherAdapterPropagatesErrorsUnchanged(t *testing.T) {
	plainErr := errors.New("raw underlying error")
	cases := []struct {
		name    string
		err     error
		wantErr error
	}{
		{
			name:    "resource_exhausted",
			err:     status.Error(codes.ResourceExhausted, "rate limited"),
			wantErr: status.Error(codes.ResourceExhausted, "rate limited"),
		},
		{
			name:    "failed_precondition",
			err:     status.Error(codes.FailedPrecondition, "unauthorized"),
			wantErr: status.Error(codes.FailedPrecondition, "unauthorized"),
		},
		{
			name:    "unavailable",
			err:     status.Error(codes.Unavailable, "discogs down"),
			wantErr: status.Error(codes.Unavailable, "discogs down"),
		},
		{
			name:    "deadline_exceeded",
			err:     status.Error(codes.DeadlineExceeded, "timed out"),
			wantErr: status.Error(codes.DeadlineExceeded, "timed out"),
		},
		{
			name:    "canceled",
			err:     status.Error(codes.Canceled, "canceled"),
			wantErr: status.Error(codes.Canceled, "canceled"),
		},
		{
			name:    "plain_error",
			err:     plainErr,
			wantErr: plainErr,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeGodiscogsSearcher{err: tc.err}
			adapter := &godiscogsSearcherAdapter{retr: fake}

			got, err := adapter.Search(context.Background(), searchParams{Query: "test"})
			if got != nil {
				t.Errorf("got = %+v, want nil on error", got)
			}
			if err != tc.wantErr && !errors.Is(err, tc.wantErr) && status.Code(err) != status.Code(tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
			if status.Code(err) != status.Code(tc.wantErr) || err.Error() != tc.wantErr.Error() {
				t.Errorf("err = %v (code %v), want %v (code %v)", err, status.Code(err), tc.wantErr, status.Code(tc.wantErr))
			}
		})
	}
}

func TestGodiscogsSearcherAdapterNilGuards(t *testing.T) {
	cases := []struct {
		name    string
		adapter *godiscogsSearcherAdapter
	}{
		{name: "nil_adapter", adapter: nil},
		{name: "nil_retriever", adapter: &godiscogsSearcherAdapter{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.adapter.Search(context.Background(), searchParams{Query: "test"})
			if got != nil {
				t.Errorf("got = %+v, want nil", got)
			}
			if status.Code(err) != codes.Unimplemented {
				t.Errorf("status.Code(err) = %v, want %v", status.Code(err), codes.Unimplemented)
			}
		})
	}
}
