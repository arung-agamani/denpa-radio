package service

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// TrackService implements the business logic for track library operations.
type TrackService struct {
	master *playlist.MasterPlaylist
	store  *playlist.Store
	cfg    *config.Config
}

func NewTrackService(master *playlist.MasterPlaylist, store *playlist.Store, cfg *config.Config) *TrackService {
	return &TrackService{master: master, store: store, cfg: cfg}
}

func (s *TrackService) save() {
	if err := s.store.Save(s.master); err != nil {
		slog.Error("Failed to save playlist state", "error", err)
	}
}

// List returns all tracks from the library, or deduplicated from all playlists
// if the library is not initialised.
func (s *TrackService) List() []*playlist.Track {
	if s.master.Library != nil {
		return s.master.Library.List()
	}
	return s.master.AllTracksDeduped()
}

// GetByID returns a single track by its numeric ID.
func (s *TrackService) GetByID(id int64) (*playlist.Track, error) {
	if s.master.Library != nil {
		if t := s.master.Library.GetByID(id); t != nil {
			return t, nil
		}
	}
	for _, pl := range s.master.AllPlaylists() {
		if t, _, err := pl.FindTrackByID(id); err == nil {
			return t, nil
		}
	}
	return nil, fmt.Errorf("track %d not found", id)
}

// Search returns tracks matching the query string.
func (s *TrackService) Search(q string) ([]*playlist.Track, error) {
	if s.master.Library == nil {
		return nil, fmt.Errorf("track library not initialised")
	}
	return s.master.Library.Search(q), nil
}

// ListOrphaned returns tracks present on disk but not registered in any playlist.
func (s *TrackService) ListOrphaned() ([]*playlist.Track, error) {
	return playlist.FindOrphanedTracks(s.cfg.MusicDir, s.master)
}

// Update modifies the metadata of a library track by ID.
func (s *TrackService) Update(id int64, upd playlist.TrackUpdate) (*playlist.Track, error) {
	if s.master.Library == nil {
		return nil, fmt.Errorf("track library not initialised")
	}
	track, err := s.master.Library.Update(id, upd)
	if err != nil {
		return nil, err
	}
	slog.Info("Track metadata updated", "track_id", id, "title", track.Title)
	s.save()
	return track, nil
}

// Delete removes a track from the library and every playlist it appears in.
// Returns the number of playlist positions that were removed.
func (s *TrackService) Delete(id int64) (playlistRemovals int, err error) {
	if s.master.Library == nil {
		return 0, fmt.Errorf("track library not initialised")
	}
	track := s.master.Library.GetByID(id)
	if track == nil {
		return 0, fmt.Errorf("track %d not found in library", id)
	}
	playlistRemovals = s.master.RemoveTrackFromAll(track.Checksum)
	s.master.Library.RemoveByID(id)
	slog.Info("Track deleted from library",
		"track_id", id,
		"title", track.Title,
		"playlist_removals", playlistRemovals,
	)
	s.save()
	return playlistRemovals, nil
}

// Scan re-scans the music directory and registers newly discovered files in
// the library. Returns (newlyAdded, libraryTotal, error).
func (s *TrackService) Scan() (int, int, error) {
	if s.master.Library == nil {
		return 0, 0, fmt.Errorf("track library not initialised")
	}
	_, added, err := playlist.ScanIntoLibrary(s.cfg.MusicDir, s.master.Library)
	if err != nil {
		return 0, 0, err
	}
	s.save()
	return added, s.master.Library.Count(), nil
}

// LibraryTotal returns the number of tracks currently in the library.
func (s *TrackService) LibraryTotal() int {
	if s.master.Library == nil {
		return 0
	}
	return s.master.Library.Count()
}

// UploadResult holds the outcome of a successful track upload.
type UploadResult struct {
	Track *playlist.Track
	Added bool // true if this is a new track; false if a duplicate
}

// Upload saves the provided audio content to the music directory under the
// given filename, registers it in the track library, and persists state.
// Returns an error if the extension is unsupported or if any I/O fails.
func (s *TrackService) Upload(filename string, content io.Reader) (*UploadResult, error) {
	if s.master.Library == nil {
		return nil, fmt.Errorf("track library not initialised")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !playlist.IsSupportedFormat(ext) {
		return nil, fmt.Errorf("unsupported audio format %q; supported: %s",
			ext, strings.Join(playlist.SupportedFormats, ", "))
	}

	// Clean the filename to prevent path traversal.
	safe := filepath.Base(filepath.Clean(filename))
	dest := filepath.Join(s.cfg.MusicDir, safe)

	// Ensure the destination is within the music directory.
	absMusic, err := filepath.Abs(s.cfg.MusicDir)
	if err != nil {
		return nil, fmt.Errorf("could not resolve music directory: %w", err)
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return nil, fmt.Errorf("could not resolve destination path: %w", err)
	}
	if !strings.HasPrefix(absDest+string(filepath.Separator), absMusic+string(filepath.Separator)) {
		return nil, fmt.Errorf("destination path is outside the music directory")
	}

	// Write the file to disk.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	if _, err = io.Copy(out, content); err != nil {
		out.Close()
		os.Remove(dest)
		return nil, fmt.Errorf("failed to write file: %w", err)
	}
	if err = out.Close(); err != nil {
		os.Remove(dest)
		return nil, fmt.Errorf("failed to finalise file write: %w", err)
	}

	// Build track metadata from the newly written file.
	track, err := playlist.NewTrackFromFile(dest)
	if err != nil {
		os.Remove(dest)
		return nil, fmt.Errorf("failed to read audio metadata: %w", err)
	}

	canonical, added := s.master.Library.Add(track)

	if added {
		slog.Info("Track uploaded and registered in library",
			"file", safe,
			"track_id", canonical.ID,
			"title", canonical.Title,
		)
		s.save()
	} else {
		// Duplicate – remove the file we just wrote since the library already
		// knows this checksum (possibly under a different filename).
		os.Remove(dest)
		slog.Info("Uploaded file is a duplicate of an existing track",
			"file", safe,
			"existing_track_id", canonical.ID,
		)
	}

	return &UploadResult{Track: canonical, Added: added}, nil
}
