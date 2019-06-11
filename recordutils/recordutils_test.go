package recordutils

import (
	"io/ioutil"
	"log"
	"testing"

	pbgd "github.com/brotherlogic/godiscogs"
	pbrc "github.com/brotherlogic/recordcollection/proto"
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

func TestTrackExtractWithVideo(t *testing.T) {
	r := &pbgd.Release{
		Title: "Testing",
		Tracklist: []*pbgd.Track{
			&pbgd.Track{Title: "Hello", Position: "1", TrackType: pbgd.Track_TRACK},
			&pbgd.Track{Title: "There", Position: "Video", TrackType: pbgd.Track_TRACK},
		},
	}

	tracks := TrackExtract(r)

	if len(tracks) != 1 {
		t.Fatalf("Tracks not extracted")
	}

	if GetTitle(tracks[0]) != "Hello" {
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

func TestRunExtractTatay(t *testing.T) {
	data, _ := ioutil.ReadFile("testdata/565473.file")

	release := &pbgd.Release{}
	proto.Unmarshal(data, release)

	tracks := TrackExtract(release)

	if len(tracks) != 13 {
		t.Errorf("Wrong number of tracks: %v", len(tracks))
	}

	found := false
	for _, tr := range tracks {
		if tr.Position == "13" {
			found = true
			if GetTitle(tr) != "Anna Apera / Gegin Nos / Silff Ffenest / Backward Dog" {
				t.Errorf("Bad title: %v", GetTitle(tr))
			}
		}
	}

	if !found {
		t.Errorf("Track 13 was not found")
	}

}

func TestRunExtractLiveVariousYears(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/1997688.file")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	record := &pbrc.Record{}
	proto.Unmarshal(data, record)

	tracks := TrackExtract(record.GetRelease())

	if len(tracks) != 14 {
		t.Errorf("Wrong number of tracks: %v, from %v", len(tracks), len(record.GetRelease().Tracklist))
	}
}

func TestRunExtractSplitDecisionBand(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/10313832.data")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	record := &pbrc.Record{}
	proto.Unmarshal(data, record)

	tracks := TrackExtract(record.GetRelease())

	if len(tracks) != 24 {
		t.Errorf("Wrong number of tracks: %v, from %v", len(tracks), len(record.GetRelease().Tracklist))
		for i, t := range tracks {
			log.Printf("%v. %v", i, len(t.tracks))
			for j, tr := range t.tracks {
				log.Printf(" %v. %v", j, tr.Title)
			}
		}
	}

	if tracks[23].Format != "CD" {
		t.Errorf("Format was not extracted %+v", tracks[23])
	}
}

func TestRunExtractSunRa(t *testing.T) {
	data, err := ioutil.ReadFile("testdata/1075530.data")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	record := &pbrc.Record{}
	proto.Unmarshal(data, record)

	tracks := TrackExtract(record.GetRelease())

	if len(tracks) != 12 {
		t.Errorf("Wrong number of tracks: %v, from %v", len(tracks), len(record.GetRelease().Tracklist))
		for i, t := range tracks {
			log.Printf("%v. %v", i, len(t.tracks))
			for j, tr := range t.tracks {
				log.Printf(" %v. %v", j, tr.Title)
			}
		}
	}

	if tracks[6].Disk != "2" {
		t.Errorf("Disk was poorly pulled%+v -> %+v", tracks[6], tracks[6].tracks[0])
	}
}
