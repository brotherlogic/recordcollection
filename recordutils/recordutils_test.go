package recordutils

import "testing"

import pbgd "github.com/brotherlogic/godiscogs"

func TestTrackExtract(t *testing.T) {
	r := &pbgd.Release{
		Title: "Testing",
		Tracklist: []*pbgd.Track{
			&pbgd.Track{Title: "Hello", Position: "1A"},
			&pbgd.Track{Title: "There", Position: "1B"},
		},
	}

	tracks := TrackExtract(r)

	if len(tracks) != 1 {
		t.Fatalf("Tracks not extracted")
	}

	if GetTitle(tracks[0]) != "Hello / There" {
		t.Errorf("Unable to get track title: %v", GetTitle(tracks[0]))
	}

}
