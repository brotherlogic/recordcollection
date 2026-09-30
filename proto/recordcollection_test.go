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

