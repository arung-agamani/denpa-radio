package playlist

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// ---------------------------------------------------------------------------
// On-disk formats
// ---------------------------------------------------------------------------

// storeDataV1 is the legacy on-disk format where playlists embed full track
// objects directly. This is only used for reading old data files during
// migration.
type storeDataV1 struct {
	Morning   []*Playlist `json:"morning"`
	Afternoon []*Playlist `json:"afternoon"`
	Evening   []*Playlist `json:"evening"`
	Night     []*Playlist `json:"night"`
}

// storePlaylistV2 is the per-playlist representation in the v2 format.
// Instead of embedding full Track objects it stores an ordered list of
// checksums that reference entries in the library.
type storePlaylistV2 struct {
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Tag                  TimeTag  `json:"tag"`
	TrackChecksums       []string `json:"trackChecksums"`
	CurrentTrackChecksum string   `json:"currentTrackChecksum,omitempty"`
}

// storeDataV2 is the current on-disk format.
type storeDataV2 struct {
	Version   int                           `json:"version"`
	Timezone  string                        `json:"timezone,omitempty"`
	Library   *TrackLibrary                 `json:"library"`
	Playlists map[string][]*storePlaylistV2 `json:"playlists"`
}

// Store handles loading and saving the MasterPlaylist to a JSON file on disk.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore creates a new Store that reads from and writes to the given file
// path. The parent directory is created automatically if it does not exist.
func NewStore(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create store directory %q: %w", dir, err)
	}

	return &Store{path: path}, nil
}

// Path returns the file path used by this store.
func (s *Store) Path() string {
	return s.path
}

// Exists returns true if the store file already exists on disk.
func (s *Store) Exists() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// ---------------------------------------------------------------------------
// Save
// ---------------------------------------------------------------------------

// Save serialises the MasterPlaylist (including its TrackLibrary) to JSON and
// writes it to disk atomically (write to temp file, then rename).
func (s *Store) Save(master *MasterPlaylist) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	master.mu.RLock()

	data := storeDataV2{
		Version:   2,
		Timezone:  master.Timezone(),
		Library:   master.Library,
		Playlists: make(map[string][]*storePlaylistV2),
	}

	for _, tag := range ValidTimeTags {
		pls := master.getPlaylistsUnsafe(tag)
		storePls := make([]*storePlaylistV2, 0, len(pls))
		for _, pl := range pls {
			sp := playlistToStoreV2(pl)
			storePls = append(storePls, sp)
		}
		data.Playlists[string(tag)] = storePls
	}

	master.mu.RUnlock()

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal master playlist: %w", err)
	}

	// Write to a temporary file in the same directory so the rename is atomic.
	dir := filepath.Dir(s.path)
	tmp, err := os.CreateTemp(dir, "playlist-*.json.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(jsonBytes); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("failed to rename temp file to %q: %w", s.path, err)
	}

	slog.Info("Playlist saved to disk", "path", s.path)
	return nil
}

// playlistToStoreV2 converts a runtime Playlist into the v2 on-disk
// representation (checksums only, no embedded tracks).
func playlistToStoreV2(pl *Playlist) *storePlaylistV2 {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	checksums := make([]string, len(pl.Tracks))
	for i, t := range pl.Tracks {
		checksums[i] = t.Checksum
	}

	return &storePlaylistV2{
		ID:                   pl.ID,
		Name:                 pl.Name,
		Tag:                  pl.Tag,
		TrackChecksums:       checksums,
		CurrentTrackChecksum: pl.CurrentTrackChecksum,
	}
}

// ---------------------------------------------------------------------------
// Load
// ---------------------------------------------------------------------------

// Load reads the JSON file from disk and reconstructs a MasterPlaylist. It
// transparently handles both v1 (legacy) and v2 (current) formats, migrating
// v1 data on the fly.
func (s *Store) Load() (*MasterPlaylist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read playlist file %q: %w", s.path, err)
	}

	// Peek at the JSON to determine the format version.
	var versionProbe struct {
		Version int `json:"version"`
	}
	_ = json.Unmarshal(raw, &versionProbe)

	if versionProbe.Version >= 2 {
		return s.loadV2(raw)
	}
	return s.loadV1(raw)
}

// loadV2 handles the current format.
func (s *Store) loadV2(raw []byte) (*MasterPlaylist, error) {
	var data storeDataV2
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse v2 playlist file %q: %w", s.path, err)
	}

	lib := data.Library
	if lib == nil {
		lib = NewTrackLibrary()
	}

	master := NewMasterPlaylistWithLibrary(lib)

	// Restore persisted timezone.
	if data.Timezone != "" {
		if err := master.SetTimezone(data.Timezone); err != nil {
			slog.Warn("Ignoring invalid persisted timezone", "timezone", data.Timezone, "error", err)
		}
	}

	for _, tag := range ValidTimeTags {
		storePls, ok := data.Playlists[string(tag)]
		if !ok {
			continue
		}
		for _, sp := range storePls {
			pl := storeV2ToPlaylist(sp, tag, lib)
			master.setPlaylistsUnsafe(tag, append(master.getPlaylistsUnsafe(tag), pl))
		}
	}

	// Sync the playlist ID counter.
	syncPlaylistIDCounter(master)

	slog.Info("Playlist loaded from disk (v2)",
		"path", s.path,
		"timezone", data.Timezone,
		"library_tracks", lib.Count(),
		"morning", len(master.Morning),
		"afternoon", len(master.Afternoon),
		"evening", len(master.Evening),
		"night", len(master.Night),
	)

	return master, nil
}

// storeV2ToPlaylist converts a v2 on-disk playlist back into a runtime
// Playlist, resolving track checksums from the library.
func storeV2ToPlaylist(sp *storePlaylistV2, tag TimeTag, lib *TrackLibrary) *Playlist {
	tracks := lib.Resolve(sp.TrackChecksums)

	pl := &Playlist{
		ID:                   sp.ID,
		Name:                 sp.Name,
		Tag:                  tag,
		Tracks:               tracks,
		CurrentTrackChecksum: sp.CurrentTrackChecksum,
		library:              lib,
	}

	// Restore the current index from the checksum if possible.
	if sp.CurrentTrackChecksum != "" {
		for i, t := range pl.Tracks {
			if t.Checksum == sp.CurrentTrackChecksum {
				pl.currentIndex = i
				break
			}
		}
	}

	return pl
}

// loadV1 handles the legacy format where playlists embed full track objects.
// It migrates the data by extracting all tracks into a TrackLibrary and
// converting playlists to use library references.
func (s *Store) loadV1(raw []byte) (*MasterPlaylist, error) {
	var data storeDataV1
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse v1 playlist file %q: %w", s.path, err)
	}

	slog.Info("Migrating v1 playlist format to v2", "path", s.path)

	// Build the library from all tracks found across all playlists.
	lib := NewTrackLibrary()
	allPlaylists := [][]*Playlist{data.Morning, data.Afternoon, data.Evening, data.Night}
	for _, pls := range allPlaylists {
		for _, pl := range pls {
			if pl.Tracks == nil {
				pl.Tracks = make([]*Track, 0)
			}
			for _, t := range pl.Tracks {
				lib.Import(t)
			}
		}
	}
	lib.SyncNextID()

	master := NewMasterPlaylistWithLibrary(lib)

	// Rebuild playlists with library references.
	restorePlaylistsV1(data.Morning, TagMorning, lib)
	restorePlaylistsV1(data.Afternoon, TagAfternoon, lib)
	restorePlaylistsV1(data.Evening, TagEvening, lib)
	restorePlaylistsV1(data.Night, TagNight, lib)

	master.Morning = nonNilPlaylists(data.Morning)
	master.Afternoon = nonNilPlaylists(data.Afternoon)
	master.Evening = nonNilPlaylists(data.Evening)
	master.Night = nonNilPlaylists(data.Night)

	// Sync the playlist ID counter.
	syncPlaylistIDCounter(master)

	slog.Info("Migration complete",
		"library_tracks", lib.Count(),
		"morning", len(master.Morning),
		"afternoon", len(master.Afternoon),
		"evening", len(master.Evening),
		"night", len(master.Night),
	)

	return master, nil
}

// restorePlaylistsV1 processes legacy playlists by setting their tag, linking
// them to the library, and resolving tracks to canonical library pointers.
func restorePlaylistsV1(playlists []*Playlist, tag TimeTag, lib *TrackLibrary) {
	for _, pl := range playlists {
		pl.Tag = tag

		if pl.Tracks == nil {
			pl.Tracks = make([]*Track, 0)
		}

		// Replace embedded track objects with canonical library pointers.
		pl.ResolveFromLibrary(lib)
	}
}

// nonNilPlaylists returns the slice as-is if non-nil, or an empty slice.
func nonNilPlaylists(pls []*Playlist) []*Playlist {
	if pls == nil {
		return make([]*Playlist, 0)
	}
	return pls
}

// syncPlaylistIDCounter scans all playlists in the master and updates the
// global playlist ID counter so that new playlists get unique IDs.
func syncPlaylistIDCounter(master *MasterPlaylist) {
	var maxPlaylist int64

	for _, tag := range ValidTimeTags {
		for _, pl := range master.getPlaylistsUnsafe(tag) {
			if pl.ID > maxPlaylist {
				maxPlaylist = pl.ID
			}
		}
	}

	SetLastPlaylistID(maxPlaylist)

	slog.Debug("Playlist ID counter synced", "max_playlist_id", maxPlaylist)
}

// ---------------------------------------------------------------------------
// Export / Import helpers
// ---------------------------------------------------------------------------

// ExportPlaylist serialises a single Playlist to JSON bytes suitable for
// sharing or backup. The exported data includes full track objects so it can
// be imported independently.
func ExportPlaylist(pl *Playlist) ([]byte, error) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	data, err := json.MarshalIndent(pl, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal playlist: %w", err)
	}
	return data, nil
}

// ExportPlaylistToFile writes a single Playlist to a JSON file at the given
// path.
func ExportPlaylistToFile(pl *Playlist, path string) error {
	data, err := ExportPlaylist(pl)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create export directory %q: %w", dir, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write export file %q: %w", path, err)
	}

	slog.Info("Playlist exported", "name", pl.Name, "path", path)
	return nil
}

// ImportPlaylist reads a JSON file and returns the deserialised Playlist.
// The imported playlist receives a new unique ID to avoid collisions with
// existing playlists.
func ImportPlaylist(path string) (*Playlist, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read import file %q: %w", path, err)
	}

	return ImportPlaylistFromBytes(raw)
}

// ImportPlaylistFromBytes deserialises a Playlist from JSON bytes. The
// imported playlist receives a new unique ID to avoid collisions. If a
// TrackLibrary is provided, the imported tracks are added to the library and
// the playlist is linked to it.
func ImportPlaylistFromBytes(data []byte) (*Playlist, error) {
	var pl Playlist
	if err := json.Unmarshal(data, &pl); err != nil {
		return nil, fmt.Errorf("failed to parse playlist data: %w", err)
	}

	// Assign a fresh ID so it doesn't clash with existing playlists.
	pl.ID = nextPlaylistID()

	// Ensure tracks slice is not nil.
	if pl.Tracks == nil {
		pl.Tracks = make([]*Track, 0)
	}

	// Restore currentIndex from checksum if applicable.
	if pl.CurrentTrackChecksum != "" {
		for i, t := range pl.Tracks {
			if t.Checksum == pl.CurrentTrackChecksum {
				pl.currentIndex = i
				break
			}
		}
	}

	slog.Info("Playlist imported",
		"name", pl.Name,
		"tag", pl.Tag,
		"tracks", len(pl.Tracks),
	)

	return &pl, nil
}

// ImportPlaylistIntoLibrary imports a playlist and integrates its tracks into
// the provided library. Tracks that already exist in the library (by checksum)
// are resolved to the canonical library pointer; new tracks are added.
func ImportPlaylistIntoLibrary(data []byte, lib *TrackLibrary) (*Playlist, error) {
	pl, err := ImportPlaylistFromBytes(data)
	if err != nil {
		return nil, err
	}

	if lib == nil {
		return pl, nil
	}

	// Add tracks to library and resolve to canonical pointers.
	for i, t := range pl.Tracks {
		canonical := lib.AddOrUpdate(t)
		pl.Tracks[i] = canonical
	}
	pl.library = lib

	return pl, nil
}

// ExportMasterPlaylist serialises the entire MasterPlaylist to JSON bytes.
// This uses the v2 format (library + checksum references).
func ExportMasterPlaylist(master *MasterPlaylist) ([]byte, error) {
	master.mu.RLock()

	data := storeDataV2{
		Version:   2,
		Timezone:  master.Timezone(),
		Library:   master.Library,
		Playlists: make(map[string][]*storePlaylistV2),
	}

	for _, tag := range ValidTimeTags {
		pls := master.getPlaylistsUnsafe(tag)
		storePls := make([]*storePlaylistV2, 0, len(pls))
		for _, pl := range pls {
			storePls = append(storePls, playlistToStoreV2(pl))
		}
		data.Playlists[string(tag)] = storePls
	}

	master.mu.RUnlock()

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal master playlist: %w", err)
	}
	return jsonBytes, nil
}

// ImportMasterPlaylist reads JSON bytes and returns a fully reconstructed
// MasterPlaylist. Supports both v1 and v2 formats.
func ImportMasterPlaylist(data []byte) (*MasterPlaylist, error) {
	// Peek at the version.
	var versionProbe struct {
		Version int `json:"version"`
	}
	_ = json.Unmarshal(data, &versionProbe)

	if versionProbe.Version >= 2 {
		var sd storeDataV2
		if err := json.Unmarshal(data, &sd); err != nil {
			return nil, fmt.Errorf("failed to parse v2 master playlist data: %w", err)
		}

		lib := sd.Library
		if lib == nil {
			lib = NewTrackLibrary()
		}

		master := NewMasterPlaylistWithLibrary(lib)

		for _, tag := range ValidTimeTags {
			storePls, ok := sd.Playlists[string(tag)]
			if !ok {
				continue
			}
			for _, sp := range storePls {
				pl := storeV2ToPlaylist(sp, tag, lib)
				master.setPlaylistsUnsafe(tag, append(master.getPlaylistsUnsafe(tag), pl))
			}
		}

		syncPlaylistIDCounter(master)
		return master, nil
	}

	// V1 fallback.
	var sd storeDataV1
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("failed to parse v1 master playlist data: %w", err)
	}

	lib := NewTrackLibrary()
	allPlaylists := [][]*Playlist{sd.Morning, sd.Afternoon, sd.Evening, sd.Night}
	for _, pls := range allPlaylists {
		for _, pl := range pls {
			if pl.Tracks == nil {
				pl.Tracks = make([]*Track, 0)
			}
			for _, t := range pl.Tracks {
				lib.Import(t)
			}
		}
	}
	lib.SyncNextID()

	restorePlaylistsV1(sd.Morning, TagMorning, lib)
	restorePlaylistsV1(sd.Afternoon, TagAfternoon, lib)
	restorePlaylistsV1(sd.Evening, TagEvening, lib)
	restorePlaylistsV1(sd.Night, TagNight, lib)

	master := NewMasterPlaylistWithLibrary(lib)
	master.Morning = nonNilPlaylists(sd.Morning)
	master.Afternoon = nonNilPlaylists(sd.Afternoon)
	master.Evening = nonNilPlaylists(sd.Evening)
	master.Night = nonNilPlaylists(sd.Night)

	syncPlaylistIDCounter(master)
	return master, nil
}
