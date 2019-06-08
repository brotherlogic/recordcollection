package recordutils

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	pbgd "github.com/brotherlogic/godiscogs"
)

type TrackSet struct {
	tracks   []*pbgd.Track
	Position string
	Disk     string
	Format   string
}

func getPosition(t *pbgd.Track, lastTrack string, diskIncrement int) (string, string) {
	if t.Position == "Video" {
		return "0", t.Title
	}

	re := regexp.MustCompile("\\d+")
	if strings.Contains(t.Position, "-") {
		elems := strings.Split(t.Position, "-")
		return elems[0], elems[1]
	}

	if t.TrackType != pbgd.Track_TRACK {
		return "0", re.FindString(t.Title)
	}
	pos := re.FindString(t.Position)
	if pos == "" {
		pos = lastTrack
	}

	// Add the increment
	val, _ := strconv.Atoi(pos)
	return "1", fmt.Sprintf("%v", val+diskIncrement)
}

//TrackExtract extracts a trackset from a release
func TrackExtract(r *pbgd.Release) []*TrackSet {
	trackset := make([]*TrackSet, 0)

	multiFormat := false
	formatCounts := make(map[string]int)
	for _, form := range r.GetFormats() {
		formatCounts[form.GetName()]++
	}

	if len(formatCounts) > 1 {
		multiFormat = true
	}

	log.Printf("%v", formatCounts)

	diskIncrement := 0
	if multiFormat {
		diskIncrement--
	}

	lastTrack := ""
	for _, track := range r.Tracklist {
		found := false
		if track.TrackType == pbgd.Track_HEADING {
			diskIncrement++
		}

		log.Printf("INCREMENT %v from %v", diskIncrement, track.TrackType)

		for _, set := range trackset {
			disk, tr := getPosition(track, lastTrack, diskIncrement)
			if tr == set.Position && disk == set.Disk {
				set.tracks = append(set.tracks, track)
				found = true
			}
		}

		disk, tr := getPosition(track, lastTrack, diskIncrement)
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
