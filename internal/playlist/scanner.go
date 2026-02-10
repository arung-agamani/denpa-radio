package playlist

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ScanResult holds the outcome of scanning a music directory.
type ScanResult struct {
	// Tracks contains all discovered audio tracks, sorted by file path.
	Tracks []*Track
	// Errors maps file paths to errors encountered while processing them.
	// These are non-fatal; the scan continues past individual file failures.
	Errors map[string]error
}

// ScanMusicDirectory walks the given directory recursively and creates Track
// objects for every supported audio file found. Tracks are sorted by file path
// in the result. Individual file errors (checksum failures, unreadable files,
// etc.) are collected in ScanResult.Errors rather than aborting the whole scan.
func ScanMusicDirectory(musicDir string) (*ScanResult, error) {
	info, err := os.Stat(musicDir)
	if err != nil {
		return nil, fmt.Errorf("cannot access music directory %q: %w", musicDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", musicDir)
	}

	result := &ScanResult{
		Tracks: make([]*Track, 0),
		Errors: make(map[string]error),
	}

	err = filepath.Walk(musicDir, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// Record the error but keep walking.
			result.Errors[path] = walkErr
			slog.Warn("Error accessing path during scan", "path", path, "error", walkErr)
			return nil
		}

		if fi.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !IsSupportedFormat(ext) {
			return nil
		}

		track, err := NewTrackFromFile(path)
		if err != nil {
			result.Errors[path] = err
			slog.Warn("Failed to create track from file", "path", path, "error", err)
			return nil
		}

		result.Tracks = append(result.Tracks, track)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking music directory %q: %w", musicDir, err)
	}

	// Sort tracks by file path for deterministic ordering.
	sort.Slice(result.Tracks, func(i, j int) bool {
		return result.Tracks[i].FilePath < result.Tracks[j].FilePath
	})

	slog.Info("Music directory scan complete",
		"directory", musicDir,
		"tracks_found", len(result.Tracks),
		"errors", len(result.Errors),
	)

	return result, nil
}

// FindOrphanedTracks compares a fresh scan of the music directory against the
// tracks already present in the master playlist. It returns tracks that exist
// on disk but are not referenced by any playlist (matched by checksum).
func FindOrphanedTracks(musicDir string, master *MasterPlaylist) ([]*Track, error) {
	scanResult, err := ScanMusicDirectory(musicDir)
	if err != nil {
		return nil, err
	}

	// Build a set of checksums that are already in the master playlist.
	knownChecksums := make(map[string]bool)
	for _, track := range master.AllTracks() {
		knownChecksums[track.Checksum] = true
	}

	orphaned := make([]*Track, 0)
	for _, track := range scanResult.Tracks {
		if !knownChecksums[track.Checksum] {
			orphaned = append(orphaned, track)
		}
	}

	slog.Info("Orphaned track detection complete",
		"total_scanned", len(scanResult.Tracks),
		"orphaned", len(orphaned),
	)

	return orphaned, nil
}

// BuildDefaultPlaylist scans the music directory and creates a single playlist
// containing all discovered tracks. The playlist is tagged with the current
// time-of-day tag. This is used for first-run initialisation when no saved
// playlist exists.
func BuildDefaultPlaylist(musicDir string) (*Playlist, error) {
	scanResult, err := ScanMusicDirectory(musicDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan music directory: %w", err)
	}

	if len(scanResult.Tracks) == 0 {
		return nil, fmt.Errorf("no supported audio files found in %q", musicDir)
	}

	tag := CurrentTimeTag()
	pl := NewPlaylist("Default Playlist", tag)
	pl.AddTracks(scanResult.Tracks)

	slog.Info("Default playlist created",
		"name", pl.Name,
		"tag", pl.Tag,
		"tracks", pl.Count(),
	)

	return pl, nil
}

// ReconcileTracks compares the tracks in the master playlist against the files
// currently on disk. It removes tracks whose files have been deleted and
// returns newly discovered files as orphaned tracks. This is the core of the
// hot-reload feature.
func ReconcileTracks(musicDir string, master *MasterPlaylist) (orphaned []*Track, removedCount int, err error) {
	// First, remove tracks whose files no longer exist.
	removedCount = master.RemoveDeletedTracks()
	if removedCount > 0 {
		slog.Info("Removed tracks with missing files", "count", removedCount)
	}

	// Then find new files that aren't in any playlist.
	orphaned, err = FindOrphanedTracks(musicDir, master)
	if err != nil {
		return nil, removedCount, fmt.Errorf("failed to find orphaned tracks: %w", err)
	}

	return orphaned, removedCount, nil
}
