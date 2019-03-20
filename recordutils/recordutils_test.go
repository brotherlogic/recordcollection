package recordutils

import (
	"io/ioutil"
	"testing"

	pbgd "github.com/brotherlogic/godiscogs"
	"github.com/golang/protobuf/proto"
)

func TestTrackExtract(t *testing.T) {
	r := &pbgd.Release{
		Title: "Testing",
		Tracklist: []*pbgd.Track{
			&pbgd.Track{Title: "Hello", Position: "1A", TrackType: pbgd.Track_TRACK},
			&pbgd.Track{Title: "There", Position: "1B", TrackType: pbgd.Track_TRACK},
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

func TestRunExtract(t *testing.T) {
	data, _ := ioutil.ReadFile("testdata/1018055.file")

	release := &pbgd.Release{}
	proto.Unmarshal(data, release)

	tracks := TrackExtract(release)

	if len(tracks) != 13 {
		t.Errorf("Wrong number of tracks: %v", len(tracks))
	}

	for _, tr := range tracks {
		if tr.Position == "9" {
			if GetTitle(tr) != "Town Called Crappy / Solicitor In Studio" {
				t.Errorf("Bad title: %v", GetTitle(tr))
			}
		}
	}

}
