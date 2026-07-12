package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/repository"
)

// TrackService implements the business logic for track library operations.
type TrackService struct {
	tracks  repository.TrackRepository
	master  repository.MasterPlaylistRepository
	cfg     *config.Config
	encoder *ffmpeg.Encoder
}

func NewTrackService(tracks repository.TrackRepository, master repository.MasterPlaylistRepository, cfg *config.Config, encoder *ffmpeg.Encoder) *TrackService {
	return &TrackService{tracks: tracks, master: master, cfg: cfg, encoder: encoder}
}

// List returns all tracks from the library, or deduplicated from all playlists
// if the library is not initialised.
func (s *TrackService) List() []*playlist.Track {
	lib := s.master.Library()
	if lib != nil {
		return s.tracks.List()
	}
	return s.master.AllTracksDeduped()
}

// GetByID returns a single track by its numeric ID.
func (s *TrackService) GetByID(id int64) (*playlist.Track, error) {
	if t := s.tracks.GetByID(id); t != nil {
		return t, nil
	}
	for _, pl := range s.master.AllPlaylists() {
		if t, _, err := pl.FindTrackByID(id); err == nil {
			return t, nil
		}
	}
	return nil, apierror.ErrNotFound(fmt.Sprintf("track %d not found", id))
}

// Search returns tracks matching the query string.
func (s *TrackService) Search(q string) ([]*playlist.Track, error) {
	if s.master.Library() == nil {
		return nil, apierror.ErrInternal("track library not initialised")
	}
	return s.tracks.Search(q), nil
}

// ListOrphaned returns tracks present on disk but not registered in any playlist.
func (s *TrackService) ListOrphaned() ([]*playlist.Track, error) {
	lib := s.master.Library()
	if lib == nil {
		return nil, apierror.ErrInternal("track library not initialised")
	}
	return playlist.FindOrphanedTracksFromLibrary(s.cfg.MusicDir, lib)
}

// Update modifies the metadata of a library track by ID.
func (s *TrackService) Update(id int64, upd playlist.TrackUpdate) (*playlist.Track, error) {
	if s.master.Library() == nil {
		return nil, apierror.ErrInternal("track library not initialised")
	}
	track, err := s.tracks.Update(id, upd)
	if err != nil {
		return nil, err
	}
	slog.Info("Track metadata updated", "track_id", id, "title", track.Title)
	if err := s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return track, nil
}

// BatchUpdate applies metadata changes to all tracks matching the filter.
func (s *TrackService) BatchUpdate(filter playlist.TrackFilter, upd playlist.TrackUpdate) (int, error) {
	if s.master.Library() == nil {
		return 0, apierror.ErrInternal("track library not initialised")
	}
	updated, err := s.tracks.BatchUpdate(filter, upd)
	if err != nil {
		return 0, err
	}
	if updated > 0 {
		if err := s.master.Save(); err != nil {
			slog.Error("failed to save playlist state", "error", err)
		}
	}
	return updated, nil
}

// BatchUpdateCover saves the uploaded image and applies its path to all tracks
// matching the filter.
func (s *TrackService) BatchUpdateCover(filter playlist.TrackFilter, r io.Reader, filename string) (int, error) {
	if s.master.Library() == nil {
		return 0, apierror.ErrInternal("track library not initialised")
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	coverFilename := coverFilenameForFilter(filter) + ext
	coversDir := filepath.Join("data", "covers")
	if err := os.MkdirAll(coversDir, 0o755); err != nil {
		return 0, fmt.Errorf("failed to create covers directory: %w", err)
	}
	coverPath := filepath.Join(coversDir, coverFilename)

	f, err := os.Create(coverPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create cover file: %w", err)
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return 0, fmt.Errorf("failed to write cover file: %w", err)
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("failed to close cover file: %w", err)
	}

	updated, err := s.tracks.BatchSetCover(filter, coverPath)
	if err != nil {
		return 0, err
	}
	if updated > 0 {
		if err := s.master.Save(); err != nil {
			slog.Error("failed to save playlist state", "error", err)
		}
	}
	return updated, nil
}

type RefreshMetadataResult struct {
	Total   int `json:"total"`
	Probed  int `json:"probed"`
	Updated int `json:"updated"`
	Failed  int `json:"failed"`
}

// RefreshMetadata probes every library track via ffprobe to discover duration.
// Tracks that already have Duration > 0 are skipped. The store is saved once at
// the end.
func (s *TrackService) RefreshMetadata() (*RefreshMetadataResult, error) {
	if s.master.Library() == nil {
		return nil, apierror.ErrInternal("track library not initialised")
	}

	tracks := s.tracks.List()
	changed := false
	result := &RefreshMetadataResult{Total: len(tracks)}

	for _, t := range tracks {
		if t.Duration > 0 {
			continue
		}
		result.Probed++
		dur := ffmpeg.ProbeDuration(t.FilePath)
		if dur > 0 {
			t.Duration = dur
			changed = true
			result.Updated++
		} else {
			result.Failed++
		}
	}

	if changed {
		if err := s.master.Save(); err != nil {
			slog.Error("failed to save playlist state", "error", err)
		}
	}
	slog.Info("Metadata refresh complete",
		"total", result.Total,
		"probed", result.Probed,
		"updated", result.Updated,
		"failed", result.Failed,
	)
	return result, nil
}

func coverFilenameForFilter(filter playlist.TrackFilter) string {
	parts := make([]string, 0, 2)
	if filter.Artist != "" {
		parts = append(parts, sanitizeFileName(filter.Artist))
	}
	if filter.Album != "" {
		parts = append(parts, sanitizeFileName(filter.Album))
	}
	if filter.Genre != "" {
		parts = append(parts, sanitizeFileName(filter.Genre))
	}
	if len(parts) == 0 {
		return "cover"
	}
	return strings.Join(parts, "_")
}

func sanitizeFileName(s string) string {
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteByte(c)
		} else if c == ' ' || c == '.' {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}

// Delete removes a track from the library and every playlist it appears in.
// When deleteFromDisk is true the underlying audio file is also removed.
// Returns the number of playlist positions that were removed.
func (s *TrackService) Delete(id int64, deleteFromDisk bool) (playlistRemovals int, err error) {
	if s.master.Library() == nil {
		return 0, apierror.ErrInternal("track library not initialised")
	}
	track := s.tracks.GetByID(id)
	if track == nil {
	return 0, apierror.ErrNotFound(fmt.Sprintf("track %d not found in library", id))
}

	filePath := track.FilePath

	playlistRemovals = s.master.RemoveTrackFromAll(track.Checksum)
	s.tracks.RemoveByID(id)

	var fileDeleted bool
	if deleteFromDisk && filePath != "" {
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			slog.Warn("Failed to delete track file from disk",
				"path", filePath,
				"error", err,
			)
		} else {
			fileDeleted = true
		}
	}

	slog.Info("Track deleted from library",
		"track_id", id,
		"title", track.Title,
		"playlist_removals", playlistRemovals,
		"file_deleted", fileDeleted,
	)
	if err := s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return playlistRemovals, nil
}

// Scan re-scans the music directory and registers newly discovered files in
// the library. Returns (newlyAdded, libraryTotal, error).
func (s *TrackService) Scan() (int, int, error) {
	lib := s.master.Library()
	if lib == nil {
		return 0, 0, apierror.ErrInternal("track library not initialised")
	}
	_, added, err := playlist.ScanIntoLibrary(s.cfg.MusicDir, lib)
	if err != nil {
		return 0, 0, err
	}
	if err := s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return added, s.tracks.Count(), nil
}

// LibraryTotal returns the number of tracks currently in the library.
func (s *TrackService) LibraryTotal() int {
	return s.tracks.Count()
}

// UploadResult holds the outcome of a successful track upload.
type UploadResult struct {
	Track *playlist.Track
	Added bool // true if this is a new track; false if a duplicate
}

// UploadMeta holds optional metadata to apply to a freshly uploaded track.
// Empty strings are ignored; the embedded file tags are used as the fallback.
type UploadMeta struct {
	Title    string
	Artist   string
	Album    string
	Genre    string
	Optimize bool // when true, convert the uploaded file to OGG Vorbis
}

// sanitizeFilename replaces characters that are unsafe in cross-platform
// filenames (path separators, shell-special bytes, NUL) with underscores.
func sanitizeFilename(name string) string {
	const unsafe = `/\:*?"<>|`
	b := []byte(strings.TrimSpace(name))
	for i, ch := range b {
		if ch == 0 || strings.ContainsRune(unsafe, rune(ch)) {
			b[i] = '_'
		}
	}
	return string(b)
}

// uniqueDestPath returns a path under dir for filename that does not collide
// with an existing file. If filename is already free it is returned as-is;
// otherwise a numbered suffix is appended: "name (1).ext", "name (2).ext", …
func uniqueDestPath(dir, filename string) string {
	ext := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, ext)
	dest := filepath.Join(dir, filename)
	for i := 1; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			return dest
		}
		dest = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
	}
}

// Upload saves the provided audio content to the music directory under the
// given filename, registers it in the track library, and persists state.
// Returns an error if the extension is unsupported or if any I/O fails.
//
// Collision avoidance: if a file with the chosen name already exists on disk
// (regardless of content) a numbered suffix is appended before writing, so
// uploading "audio.mp3" twice produces "audio.mp3" and "audio (1).mp3".
//
// Metadata: if meta.Title is non-empty it is used both as the saved filename
// (sanitised + original extension) and as the track's Title tag. Other meta
// fields override whatever is embedded in the file's audio tags.
func (s *TrackService) Upload(filename string, content io.Reader, meta UploadMeta) (*UploadResult, error) {
	if s.master.Library() == nil {
		return nil, apierror.ErrInternal("track library not initialised")
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if !playlist.IsSupportedFormat(ext) {
		return nil, apierror.ErrValidation(fmt.Sprintf("unsupported audio format %q; supported: %s",
			ext, strings.Join(playlist.SupportedFormats, ", ")))
	}

	// Determine the base name: prefer meta.Title (sanitized) over the original
	// filename so the on-disk name matches what the user typed.
	var baseName string
	if meta.Title != "" {
		sanitized := sanitizeFilename(meta.Title)
		if sanitized == "" {
			sanitized = "track"
		}
		baseName = sanitized + ext
	} else {
		baseName = filepath.Base(filepath.Clean(filename))
	}

	// Ensure the destination is within the music directory.
	absMusic, err := filepath.Abs(s.cfg.MusicDir)
	if err != nil {
		return nil, fmt.Errorf("could not resolve music directory: %w", err)
	}

	// Find a non-colliding destination path.
	dest := uniqueDestPath(absMusic, baseName)

	absDest, err := filepath.Abs(dest)
	if err != nil {
		return nil, fmt.Errorf("could not resolve destination path: %w", err)
	}
	if !strings.HasPrefix(absDest+string(filepath.Separator), absMusic+string(filepath.Separator)) {
		return nil, apierror.ErrForbidden("destination path is outside the music directory")
	}

	// Write the file to disk.
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
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

	// Optimise: convert to OGG Vorbis if requested and the source is not
	// already in OGG format.
	if meta.Optimize && s.encoder != nil && ext != ".ogg" {
		// Derive a unique OGG destination to avoid overwriting existing files.
		oggBaseName := strings.TrimSuffix(filepath.Base(dest), ext) + ".ogg"
		oggDest := uniqueDestPath(absMusic, oggBaseName)
		if err := s.encoder.ConvertToOGG(context.Background(), dest, oggDest); err != nil {
			os.Remove(dest)
			return nil, fmt.Errorf("OGG conversion failed: %w", err)
		}
		// Remove the original file after successful conversion.
		os.Remove(dest)
		dest = oggDest
		slog.Info("Uploaded file converted to OGG", "output", filepath.Base(dest))
	}

	// Build track metadata from the newly written file.
	track, err := playlist.NewTrackFromFile(dest)
	if err != nil {
		os.Remove(dest)
		return nil, fmt.Errorf("failed to read audio metadata: %w", err)
	}

	// Apply caller-supplied metadata overrides before registering in the library
	// so that whatever is stored is already correct.
	if meta.Title != "" {
		track.Title = meta.Title
	}
	if meta.Artist != "" {
		track.Artist = meta.Artist
	}
	if meta.Album != "" {
		track.Album = meta.Album
	}
	if meta.Genre != "" {
		track.Genre = meta.Genre
	}

	// If no user-supplied title was provided but the file contains embedded
	// metadata with a title, rename the on-disk file to match so the
	// filename is human-friendly (e.g. "Beautiful Song.ogg" instead of
	// "audio.ogg").
	currentExt := filepath.Ext(dest)
	currentStem := strings.TrimSuffix(filepath.Base(dest), currentExt)
	if meta.Title == "" && track.Title != "" && track.Title != currentStem {
		newBaseName := sanitizeFilename(track.Title) + currentExt
		newDest := uniqueDestPath(absMusic, newBaseName)
		if renameErr := os.Rename(dest, newDest); renameErr == nil {
			slog.Info("Renamed uploaded file to match metadata title",
				"old", filepath.Base(dest),
				"new", filepath.Base(newDest),
			)
			dest = newDest
			track.FilePath = dest
		} else {
			slog.Warn("Could not rename file to metadata title",
				"old", filepath.Base(dest),
				"target", newBaseName,
				"error", renameErr,
			)
		}
	}

	canonical, added := s.tracks.Add(track)

	if added {
		slog.Info("Track uploaded and registered in library",
			"file", filepath.Base(dest),
			"track_id", canonical.ID,
			"title", canonical.Title,
		)
		if err := s.master.Save(); err != nil {
			slog.Error("failed to save playlist state", "error", err)
		}
	} else {
		// Duplicate – remove the file we just wrote since the library already
		// knows this checksum (possibly under a different filename).
		os.Remove(dest)
		slog.Info("Uploaded file is a duplicate of an existing track",
			"file", filepath.Base(dest),
			"existing_track_id", canonical.ID,
		)
	}

	return &UploadResult{Track: canonical, Added: added}, nil
}
