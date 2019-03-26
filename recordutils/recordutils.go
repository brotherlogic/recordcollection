package recordutils

import (
	"regexp"
	"strings"

	pbgd "github.com/brotherlogic/godiscogs"
)

type TrackSet struct {
	tracks   []*pbgd.Track
	Position string
	Disk     string
}

func getPosition(t *pbgd.Track, lastTrack string) (string, string) {
	re := regexp.MustCompile("\\d+")
	if strings.Contains(t.Position, "-") {
		elems := strings.Split(t.Position, "-")
		return elems[0], re.FindString(t.Position)
	}

	if t.TrackType != pbgd.Track_TRACK {
		return "0", re.FindString(t.Title)
	}
	pos := re.FindString(t.Position)
	if pos == "" {
		pos = lastTrack
	}
	return "1", pos
}

//TrackExtract extracts a trackset from a release
func TrackExtract(r *pbgd.Release) []*TrackSet {
	trackset := make([]*TrackSet, 0)

	lastTrack := ""
	for _, track := range r.Tracklist {
		found := false
		for _, set := range trackset {
			disk, tr := getPosition(track, lastTrack)
			if tr == set.Position && disk == set.Disk {
				set.tracks = append(set.tracks, track)
				found = true
			}
		}

		disk, tr := getPosition(track, lastTrack)
		if disk == "0" {
			lastTrack = tr
		}
		if !found && disk != "0" {
			trackset = append(trackset, &TrackSet{Disk: disk, tracks: []*pbgd.Track{track}, Position: tr})
		}
	}

	return trackset
}

//GetTitle of trackset
func GetTitle(t *TrackSet) string {
	result := t.tracks[0].Title
	for _, tr := range t.tracks[1:] {
		result += " / " + tr.Title
	}
	return result
}
