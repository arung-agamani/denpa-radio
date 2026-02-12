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
//
// NOTE: The returned tracks have ID 0. Use ScanIntoLibrary to both scan and
// register tracks with stable IDs in a TrackLibrary.
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

// ScanIntoLibrary scans the music directory and adds all discovered tracks to
// the provided TrackLibrary. Tracks that already exist in the library (matched
// by checksum) are updated with the current file path but otherwise left
// unchanged, preserving any user-edited metadata.
//
// Returns the scan result (with the library-canonical Track pointers) and the
// number of newly added tracks.
func ScanIntoLibrary(musicDir string, lib *TrackLibrary) (*ScanResult, int, error) {
	scanResult, err := ScanMusicDirectory(musicDir)
	if err != nil {
		return nil, 0, err
	}

	added := 0
	for i, t := range scanResult.Tracks {
		canonical := lib.AddOrUpdate(t)
		scanResult.Tracks[i] = canonical
		if canonical == t {
			// AddOrUpdate returned the same pointer → it was newly added.
			added++
		}
	}

	slog.Info("Scan into library complete",
		"directory", musicDir,
		"total_scanned", len(scanResult.Tracks),
		"newly_added", added,
		"library_total", lib.Count(),
	)

	return scanResult, added, nil
}

// FindOrphanedTracks compares a fresh scan of the music directory against the
// tracks already present in the library. It returns tracks that exist on disk
// but are not yet in the library (matched by checksum).
func FindOrphanedTracks(musicDir string, master *MasterPlaylist) ([]*Track, error) {
	scanResult, err := ScanMusicDirectory(musicDir)
	if err != nil {
		return nil, err
	}

	// Build a set of checksums that are already known.
	knownChecksums := make(map[string]bool)

	if master.Library != nil {
		for _, cs := range master.Library.Checksums() {
			knownChecksums[cs] = true
		}
	} else {
		// Fallback: collect from playlists directly.
		for _, track := range master.AllTracks() {
			knownChecksums[track.Checksum] = true
		}
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

// FindOrphanedTracksFromLibrary returns tracks on disk that are not in the
// library. Unlike FindOrphanedTracks, this works directly against the library
// and does not require a MasterPlaylist.
func FindOrphanedTracksFromLibrary(musicDir string, lib *TrackLibrary) ([]*Track, error) {
	scanResult, err := ScanMusicDirectory(musicDir)
	if err != nil {
		return nil, err
	}

	orphaned := make([]*Track, 0)
	for _, track := range scanResult.Tracks {
		if !lib.Contains(track.Checksum) {
			orphaned = append(orphaned, track)
		}
	}

	slog.Info("Orphaned track detection (library) complete",
		"total_scanned", len(scanResult.Tracks),
		"orphaned", len(orphaned),
	)

	return orphaned, nil
}

// BuildDefaultPlaylist scans the music directory, adds all discovered tracks
// to the master playlist's library, and creates a single playlist containing
// all of them. The playlist is tagged with the current time-of-day tag. This
// is used for first-run initialisation when no saved playlist exists.
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

// BuildDefaultPlaylistWithLibrary scans the music directory, registers all
// discovered tracks in the provided library with stable IDs, and creates a
// playlist containing all of them. The playlist is linked to the library.
func BuildDefaultPlaylistWithLibrary(musicDir string, lib *TrackLibrary) (*Playlist, error) {
	scanResult, added, err := ScanIntoLibrary(musicDir, lib)
	if err != nil {
		return nil, fmt.Errorf("failed to scan music directory: %w", err)
	}

	if len(scanResult.Tracks) == 0 {
		return nil, fmt.Errorf("no supported audio files found in %q", musicDir)
	}

	_ = added

	tag := CurrentTimeTag()
	pl := NewPlaylist("Default Playlist", tag)
	pl.library = lib

	// Add canonical library pointers.
	for _, t := range scanResult.Tracks {
		pl.Tracks = append(pl.Tracks, t)
	}

	slog.Info("Default playlist created (with library)",
		"name", pl.Name,
		"tag", pl.Tag,
		"tracks", pl.Count(),
		"library_total", lib.Count(),
	)

	return pl, nil
}

// ReconcileTracks compares the tracks in the master playlist against the files
// currently on disk. It removes tracks whose files have been deleted (from both
// the library and playlists) and returns newly discovered files as orphaned
// tracks. This is the core of the hot-reload feature.
func ReconcileTracks(musicDir string, master *MasterPlaylist) (orphaned []*Track, removedCount int, err error) {
	// First, remove tracks whose files no longer exist.
	if master.Library != nil {
		// Remove stale tracks from library; this gives us the list of removed.
		stale := master.Library.RemoveStale()
		removedCount = len(stale)

		// Also remove those tracks from all playlists.
		for _, t := range stale {
			master.RemoveTrackFromAll(t.Checksum)
		}

		if removedCount > 0 {
			slog.Info("Removed stale tracks from library and playlists", "count", removedCount)
		}
	} else {
		// Fallback: remove from playlists directly (legacy path).
		removedCount = master.RemoveDeletedTracks()
		if removedCount > 0 {
			slog.Info("Removed tracks with missing files", "count", removedCount)
		}
	}

	// Then find new files that aren't in the library.
	orphaned, err = FindOrphanedTracks(musicDir, master)
	if err != nil {
		return nil, removedCount, fmt.Errorf("failed to find orphaned tracks: %w", err)
	}

	// Add orphaned tracks to the library so they get stable IDs.
	if master.Library != nil && len(orphaned) > 0 {
		for i, t := range orphaned {
			canonical := master.Library.AddOrUpdate(t)
			orphaned[i] = canonical
		}
		slog.Info("Added orphaned tracks to library", "count", len(orphaned))
	}

	return orphaned, removedCount, nil
}
