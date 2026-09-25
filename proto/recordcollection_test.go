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
