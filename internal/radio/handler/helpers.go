package handler

import (
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// safeFilenameRe matches characters that are safe in Content-Disposition filenames.
var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// sanitiseTrack returns a map representation of a track with the absolute
// file-system path replaced by just the filename, preventing server path leaks.
func sanitiseTrack(t *playlist.Track) map[string]interface{} {
	return map[string]interface{}{
		"id":       t.ID,
		"title":    t.Title,
		"artist":   t.Artist,
		"album":    t.Album,
		"genre":    t.Genre,
		"year":     t.Year,
		"trackNum": t.TrackNum,
		"duration": t.Duration,
		"filePath": filepath.Base(t.FilePath),
		"format":   t.Format,
		"checksum": t.Checksum,
	}
}

// sanitiseTracks applies sanitiseTrack to a slice of tracks.
func sanitiseTracks(tracks []*playlist.Track) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tracks))
	for _, t := range tracks {
		result = append(result, sanitiseTrack(t))
	}
	return result
}

// isNotFound is a heuristic to distinguish "not found" errors from others.
func isNotFound(err error) bool {
	return err != nil && containsAny(err.Error(), "not found", "does not exist")
}

// isValidationError detects validation / bad-request type errors.
func isValidationError(err error) bool {
	return err != nil && containsAny(err.Error(), "invalid tag", "name is required", "must be one of")
}

// isForbidden detects path-traversal / forbidden errors.
func isForbidden(err error) bool {
	return err != nil && containsAny(err.Error(), "within the music directory", "forbidden")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}
