package recordutils

import (
	"regexp"

	pbgd "github.com/brotherlogic/godiscogs"
)

type TrackSet struct {
	tracks   []*pbgd.Track
	position string
	disk     string
}

func getPosition(t *pbgd.Track) string {
	re := regexp.MustCompile("\\d+")
	return re.FindString(t.Position)
}

//TrackExtract extracts a trackset from a release
func TrackExtract(r *pbgd.Release) []*TrackSet {
	trackset := make([]*TrackSet, 0)

	for _, track := range r.Tracklist {
		found := false
		for _, set := range trackset {
			if getPosition(track) == set.position {
				set.tracks = append(set.tracks, track)
				found = true
			}
		}

		if !found {
			trackset = append(trackset, &TrackSet{tracks: []*pbgd.Track{track}, position: getPosition(track)})
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
