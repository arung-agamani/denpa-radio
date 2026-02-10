package playlist

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/dhowden/tag"
)

// lastTrackID is a global counter for generating unique track IDs.
var lastTrackID atomic.Int64

// nextTrackID returns the next unique track ID.
func nextTrackID() int64 {
	return lastTrackID.Add(1)
}

// SetLastTrackID sets the global track ID counter. This is used when loading
// persisted playlists so that newly created tracks don't collide with existing IDs.
func SetLastTrackID(id int64) {
	lastTrackID.Store(id)
}

// Track represents a single audio file with its metadata.
type Track struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Year     int    `json:"year,omitempty"`
	TrackNum int    `json:"trackNum,omitempty"`
	Duration int    `json:"duration"` // in seconds
	FilePath string `json:"filePath"`
	Format   string `json:"format"`
	Checksum string `json:"checksum"`
}

// SupportedFormats lists the audio file extensions that are recognized.
var SupportedFormats = []string{".mp3", ".wav", ".flac", ".aac", ".ogg"}

// IsSupportedFormat returns true if the file extension (including the dot) is
// a supported audio format.
func IsSupportedFormat(ext string) bool {
	lower := strings.ToLower(ext)
	for _, f := range SupportedFormats {
		if lower == f {
			return true
		}
	}
	return false
}

// NewTrackFromFile creates a Track by reading metadata and computing a checksum
// for the audio file at the given path. Returns an error if the file cannot be
// read or hashed.
func NewTrackFromFile(path string) (*Track, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	ext := strings.ToLower(filepath.Ext(absPath))
	filename := filepath.Base(absPath)
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Compute checksum
	checksum, err := computeChecksum(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute checksum for %s: %w", absPath, err)
	}

	track := &Track{
		ID:       nextTrackID(),
		Title:    nameWithoutExt,
		FilePath: absPath,
		Format:   strings.TrimPrefix(ext, "."),
		Checksum: checksum,
	}

	// Try to extract metadata from tags
	extractTrackMetadata(track, absPath)

	return track, nil
}

// NewTrackFromExisting creates a Track with all fields pre-populated. This is
// used when loading from persisted data where metadata is already known.
func NewTrackFromExisting(id int64, title, artist, album, genre string, year, trackNum, duration int, filePath, format, checksum string) *Track {
	return &Track{
		ID:       id,
		Title:    title,
		Artist:   artist,
		Album:    album,
		Genre:    genre,
		Year:     year,
		TrackNum: trackNum,
		Duration: duration,
		FilePath: filePath,
		Format:   format,
		Checksum: checksum,
	}
}

// computeChecksum returns the hex-encoded SHA-256 hash of the file at path.
func computeChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// extractTrackMetadata reads ID3/tag metadata from the file and populates the
// Track's metadata fields. If tags cannot be read the Track retains its
// filename-based defaults.
func extractTrackMetadata(track *Track, path string) {
	f, err := os.Open(path)
	if err != nil {
		slog.Warn("Could not open file for metadata", "path", path, "error", err)
		return
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		slog.Debug("Could not read tags", "path", path, "error", err)
		return
	}

	if m.Title() != "" {
		track.Title = m.Title()
	}
	if m.Artist() != "" {
		track.Artist = m.Artist()
	}
	if m.Album() != "" {
		track.Album = m.Album()
	}
	if m.Genre() != "" {
		track.Genre = m.Genre()
	}
	if m.Year() != 0 {
		track.Year = m.Year()
	}
	if num, _ := m.Track(); num != 0 {
		track.TrackNum = num
	}
}

// FileExists returns true if the track's file path points to an existing file.
func (t *Track) FileExists() bool {
	info, err := os.Stat(t.FilePath)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// VerifyChecksum recomputes the file checksum and returns true if it matches
// the stored checksum.
func (t *Track) VerifyChecksum() (bool, error) {
	current, err := computeChecksum(t.FilePath)
	if err != nil {
		return false, err
	}
	return current == t.Checksum, nil
}

// MaxTrackID walks a slice of tracks and returns the highest ID found.
// Returns 0 if the slice is empty.
func MaxTrackID(tracks []*Track) int64 {
	var max int64
	for _, t := range tracks {
		if t.ID > max {
			max = t.ID
		}
	}
	return max
}
