package recordutils

import (
	"regexp"
	"strings"

	pbgd "github.com/brotherlogic/godiscogs"
)

type TrackSet struct {
	tracks   []*pbgd.Track
	position string
	disk     string
}

func getPosition(t *pbgd.Track) (string, string) {
	re := regexp.MustCompile("\\d+")
	if strings.Contains(t.Position, "-") {
		elems := strings.Split(t.Position, "-")
		return elems[0], re.FindString(t.Position)
	}
	return "1", re.FindString(t.Position)
}

//TrackExtract extracts a trackset from a release
func TrackExtract(r *pbgd.Release) []*TrackSet {
	trackset := make([]*TrackSet, 0)

	for _, track := range r.Tracklist {
		found := false
		for _, set := range trackset {
			disk, tr := getPosition(track)
			if tr == set.position && disk == set.disk {
				set.tracks = append(set.tracks, track)
				found = true
			}
		}

		disk, tr := getPosition(track)
		if !found {
			trackset = append(trackset, &TrackSet{disk: disk, tracks: []*pbgd.Track{track}, position: tr})
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
