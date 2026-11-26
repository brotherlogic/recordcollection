package proto

import (
	"testing"

	"google.golang.org/protobuf/proto"
)

func TestReleaseMetadata_PackageScore(t *testing.T) {
	meta := &ReleaseMetadata{
		PackageScore: 4,
	}
	if meta.GetPackageScore() != 4 {
		t.Fatalf("expected package_score 4, got %v", meta.GetPackageScore())
	}

	data, err := proto.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal ReleaseMetadata with package_score: %v", err)
	}

	meta2 := &ReleaseMetadata{}
	if err := proto.Unmarshal(data, meta2); err != nil {
		t.Fatalf("failed to unmarshal ReleaseMetadata: %v", err)
	}

	if meta2.GetPackageScore() != 4 {
		t.Fatalf("expected unmarshaled package_score 4, got %v", meta2.GetPackageScore())
	}
}

func TestReleaseMetadata_RippedQuality(t *testing.T) {
	meta := &ReleaseMetadata{
		RippedQuality: 85,
	}
	if meta.GetRippedQuality() != 85 {
		t.Fatalf("expected ripped_quality 85, got %v", meta.GetRippedQuality())
	}

	data, err := proto.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal ReleaseMetadata with ripped_quality: %v", err)
	}

	meta2 := &ReleaseMetadata{}
	if err := proto.Unmarshal(data, meta2); err != nil {
		t.Fatalf("failed to unmarshal ReleaseMetadata: %v", err)
	}

	if meta2.GetRippedQuality() != 85 {
		t.Fatalf("expected unmarshaled ripped_quality 85, got %v", meta2.GetRippedQuality())
	}
}

func TestReleaseMetadata_OutOfPlay(t *testing.T) {
	// Create ReleaseMetadata with OutOfPlay = proto.Bool(true), marshal, unmarshal, and assert GetOutOfPlay() == true.
	metaTrue := &ReleaseMetadata{
		OutOfPlay: proto.Bool(true),
	}
	if !metaTrue.GetOutOfPlay() {
		t.Fatalf("expected out_of_play true, got %v", metaTrue.GetOutOfPlay())
	}

	dataTrue, err := proto.Marshal(metaTrue)
	if err != nil {
		t.Fatalf("failed to marshal ReleaseMetadata with out_of_play true: %v", err)
	}

	metaTrueUnmarshaled := &ReleaseMetadata{}
	if err := proto.Unmarshal(dataTrue, metaTrueUnmarshaled); err != nil {
		t.Fatalf("failed to unmarshal ReleaseMetadata: %v", err)
	}

	if !metaTrueUnmarshaled.GetOutOfPlay() {
		t.Fatalf("expected unmarshaled out_of_play true, got %v", metaTrueUnmarshaled.GetOutOfPlay())
	}

	// Create ReleaseMetadata with OutOfPlay = proto.Bool(false), marshal, unmarshal, and assert GetOutOfPlay() == false.
	metaFalse := &ReleaseMetadata{
		OutOfPlay: proto.Bool(false),
	}
	if metaFalse.GetOutOfPlay() {
		t.Fatalf("expected out_of_play false, got %v", metaFalse.GetOutOfPlay())
	}

	dataFalse, err := proto.Marshal(metaFalse)
	if err != nil {
		t.Fatalf("failed to marshal ReleaseMetadata with out_of_play false: %v", err)
	}

	metaFalseUnmarshaled := &ReleaseMetadata{}
	if err := proto.Unmarshal(dataFalse, metaFalseUnmarshaled); err != nil {
		t.Fatalf("failed to unmarshal ReleaseMetadata: %v", err)
	}

	if metaFalseUnmarshaled.GetOutOfPlay() {
		t.Fatalf("expected unmarshaled out_of_play false, got %v", metaFalseUnmarshaled.GetOutOfPlay())
	}
}

func TestReleaseMetadata_RefreshSaleDescription(t *testing.T) {
	rec := &Record{
		Metadata: &ReleaseMetadata{
			RefreshSaleDescription: true,
		},
	}
	if !rec.GetMetadata().GetRefreshSaleDescription() {
		t.Fatalf("expected refresh_sale_description true, got %v", rec.GetMetadata().GetRefreshSaleDescription())
	}

	data, err := proto.Marshal(rec)
	if err != nil {
		t.Fatalf("failed to marshal Record with refresh_sale_description: %v", err)
	}

	recUnmarshaled := &Record{}
	if err := proto.Unmarshal(data, recUnmarshaled); err != nil {
		t.Fatalf("failed to unmarshal Record: %v", err)
	}

	if !recUnmarshaled.GetMetadata().GetRefreshSaleDescription() {
		t.Fatalf("expected unmarshaled refresh_sale_description true, got %v", recUnmarshaled.GetMetadata().GetRefreshSaleDescription())
	}
}

func TestSearchReleasesRequest_RoundTrip(t *testing.T) {
	req := &SearchReleasesRequest{
		Q:            "boards of canada",
		Title:        "Music Has the Right to Children",
		ReleaseTitle: "Music Has the Right to Children",
		Credit:       "Marcus Eoin",
		Artist:       "Boards of Canada",
		Anv:          "BOC",
		Label:        "Warp Records",
		Genre:        "Electronic",
		Style:        "IDM",
		Country:      "UK",
		Year:         "1998",
		Format:       "Vinyl",
		Catno:        "WARPLP55",
		Barcode:      "5021603055117",
		Track:        "Roygbiv",
		Submitter:    "user123",
		Contributor:  "contrib456",
		PerPage:      25,
		Sort:         "year",
		SortOrder:    "asc",
	}

	if req.ProtoReflect().Descriptor().Fields().ByName("type") != nil {
		t.Fatalf("SearchReleasesRequest must not define a 'type' field")
	}

	data, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal SearchReleasesRequest: %v", err)
	}

	got := &SearchReleasesRequest{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("failed to unmarshal SearchReleasesRequest: %v", err)
	}

	if !proto.Equal(req, got) {
		t.Fatalf("SearchReleasesRequest round-trip mismatch: got %+v, want %+v", got, req)
	}
	if got.GetQ() != req.Q || got.GetArtist() != req.Artist || got.GetPerPage() != req.PerPage || got.GetSortOrder() != req.SortOrder {
		t.Fatalf("getter mismatch after unmarshal: got q=%q artist=%q per_page=%d sort_order=%q", got.GetQ(), got.GetArtist(), got.GetPerPage(), got.GetSortOrder())
	}
}

func TestSearchReleaseResult_RoundTrip(t *testing.T) {
	res := &SearchReleaseResult{
		ReleaseId: 123456,
		Title:     "Boards Of Canada - Music Has The Right To Children",
		Year:      "1998",
		Formats:   []string{"File", "FLAC", "Album"},
		Country:   "UK",
		Url:       "https://www.discogs.com/release/123456",
	}

	data, err := proto.Marshal(res)
	if err != nil {
		t.Fatalf("failed to marshal SearchReleaseResult: %v", err)
	}

	got := &SearchReleaseResult{}
	if err := proto.Unmarshal(data, got); err != nil {
		t.Fatalf("failed to unmarshal SearchReleaseResult: %v", err)
	}

	if !proto.Equal(res, got) {
		t.Fatalf("SearchReleaseResult round-trip mismatch: got %+v, want %+v", got, res)
	}
	if len(got.GetFormats()) != 3 {
		t.Fatalf("expected 3 formats, got %d (%v)", len(got.GetFormats()), got.GetFormats())
	}
	if got.GetReleaseId() != 123456 || got.GetUrl() != res.Url {
		t.Fatalf("getter mismatch after unmarshal: got release_id=%d url=%q", got.GetReleaseId(), got.GetUrl())
	}
}

func TestSearchReleasesResponse_RoundTrip(t *testing.T) {
	t.Run("multiple results", func(t *testing.T) {
		resp := &SearchReleasesResponse{
			Results: []*SearchReleaseResult{
				{
					ReleaseId: 1001,
					Title:     "Artist One - Album One",
					Year:      "2020",
					Formats:   []string{"Vinyl", "LP"},
					Country:   "US",
					Url:       "https://www.discogs.com/release/1001",
				},
				{
					ReleaseId: 1002,
					Title:     "Artist Two - Album Two",
					Year:      "2021",
					Formats:   []string{"File", "MP3"},
					Country:   "DE",
					Url:       "https://www.discogs.com/release/1002",
				},
			},
		}

		data, err := proto.Marshal(resp)
		if err != nil {
			t.Fatalf("failed to marshal SearchReleasesResponse: %v", err)
		}

		got := &SearchReleasesResponse{}
		if err := proto.Unmarshal(data, got); err != nil {
			t.Fatalf("failed to unmarshal SearchReleasesResponse: %v", err)
		}

		if !proto.Equal(resp, got) {
			t.Fatalf("SearchReleasesResponse round-trip mismatch: got %+v, want %+v", got, resp)
		}
		if len(got.GetResults()) != 2 {
			t.Fatalf("expected 2 results, got %d", len(got.GetResults()))
		}
	})

	t.Run("empty results", func(t *testing.T) {
		for _, resp := range []*SearchReleasesResponse{
			{Results: []*SearchReleaseResult{}},
			{},
		} {
			data, err := proto.Marshal(resp)
			if err != nil {
				t.Fatalf("failed to marshal empty SearchReleasesResponse: %v", err)
			}

			got := &SearchReleasesResponse{}
			if err := proto.Unmarshal(data, got); err != nil {
				t.Fatalf("failed to unmarshal empty SearchReleasesResponse: %v", err)
			}

			if len(got.GetResults()) != 0 {
				t.Fatalf("expected 0 results, got %d", len(got.GetResults()))
			}
		}
	})
}


