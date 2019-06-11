package recordutils

import (
	"fmt"
	"regexp"

	pbgd "github.com/brotherlogic/godiscogs"
)

type TrackSet struct {
	tracks   []*pbgd.Track
	Position string
	Disk     string
	Format   string
}

func shouldMerge(t1, t2 *TrackSet) bool {
	matcher := regexp.MustCompile("^[a-z]")
	if matcher.MatchString(t1.tracks[0].Position) && matcher.MatchString(t2.tracks[0].Position) {
		return true
	}

	cdJoin := regexp.MustCompile("^\\d[A-Z]")
	if cdJoin.MatchString(t1.tracks[0].Position) && cdJoin.MatchString(t2.tracks[0].Position) {
		if t1.tracks[0].Position[0] == t2.tracks[0].Position[0] {
			return true
		}
	}

	return false
}

//TrackExtract extracts a trackset from a release
func TrackExtract(r *pbgd.Release) []*TrackSet {
	trackset := make([]*TrackSet, 0)

	multiFormat := false
	formatCounts := make(map[string]int)
	for _, form := range r.GetFormats() {
		if form.GetName() != "Box Set" {
			formatCounts[form.GetName()]++
		}
	}

	if len(formatCounts) > 1 {
		multiFormat = true
	}

	disk := 1
	if multiFormat {
		disk = 0
	}

	currTrack := 1
	if multiFormat {
		currTrack = 0
	}

	currFormat := r.GetFormats()[0].Name
	for _, track := range r.Tracklist {
		if track.TrackType == pbgd.Track_HEADING {
			disk++
			currTrack = 1
			currFormat = track.Title
		} else if track.TrackType == pbgd.Track_TRACK {
			if track.Position != "Video" {
				trackset = append(trackset, &TrackSet{Format: currFormat, Disk: fmt.Sprintf("%v", disk), tracks: []*pbgd.Track{track}, Position: fmt.Sprintf("%v", currTrack)})
				currTrack++
			}
		}
	}

	//Perform la merge
	found := true
	for found {
		found = false
		for i := range trackset[1:] {
			if shouldMerge(trackset[i], trackset[i+1]) {
				trackset[i].tracks = append(trackset[i].tracks, trackset[i+1].tracks...)
				trackset = append(trackset[:i+1], trackset[i+2:]...)
				found = true
				break
			}
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
