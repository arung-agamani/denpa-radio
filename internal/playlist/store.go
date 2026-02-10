package playlist

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// storeData is the on-disk JSON representation of the entire playlist state.
type storeData struct {
	Morning   []*Playlist `json:"morning"`
	Afternoon []*Playlist `json:"afternoon"`
	Evening   []*Playlist `json:"evening"`
	Night     []*Playlist `json:"night"`
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

// Save serialises the MasterPlaylist to JSON and writes it to disk atomically
// (write to temp file, then rename). This prevents data corruption if the
// process crashes mid-write.
func (s *Store) Save(master *MasterPlaylist) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	master.mu.RLock()
	data := storeData{
		Morning:   master.Morning,
		Afternoon: master.Afternoon,
		Evening:   master.Evening,
		Night:     master.Night,
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

// Load reads the JSON file from disk and reconstructs a MasterPlaylist. It
// also updates the global ID counters so that subsequently created tracks and
// playlists receive unique IDs.
func (s *Store) Load() (*MasterPlaylist, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("failed to read playlist file %q: %w", s.path, err)
	}

	var data storeData
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("failed to parse playlist file %q: %w", s.path, err)
	}

	// Rebuild internal state that is not persisted (mutex, currentIndex, etc.)
	restorePlaylists(data.Morning, TagMorning)
	restorePlaylists(data.Afternoon, TagAfternoon)
	restorePlaylists(data.Evening, TagEvening)
	restorePlaylists(data.Night, TagNight)

	master := &MasterPlaylist{
		Morning:   data.Morning,
		Afternoon: data.Afternoon,
		Evening:   data.Evening,
		Night:     data.Night,
	}

	// Ensure nil slices become empty slices for consistency.
	if master.Morning == nil {
		master.Morning = make([]*Playlist, 0)
	}
	if master.Afternoon == nil {
		master.Afternoon = make([]*Playlist, 0)
	}
	if master.Evening == nil {
		master.Evening = make([]*Playlist, 0)
	}
	if master.Night == nil {
		master.Night = make([]*Playlist, 0)
	}

	// Update global ID counters to avoid collisions.
	syncIDCounters(master)

	slog.Info("Playlist loaded from disk",
		"path", s.path,
		"morning", len(master.Morning),
		"afternoon", len(master.Afternoon),
		"evening", len(master.Evening),
		"night", len(master.Night),
	)

	return master, nil
}

// ExportPlaylist serialises a single Playlist to JSON bytes suitable for
// sharing or backup.
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
// imported playlist receives a new unique ID to avoid collisions.
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

	// Update ID counters to account for imported track IDs.
	var maxTID int64
	for _, t := range pl.Tracks {
		if t.ID > maxTID {
			maxTID = t.ID
		}
	}
	if current := lastTrackID.Load(); maxTID > current {
		SetLastTrackID(maxTID)
	}

	slog.Info("Playlist imported",
		"name", pl.Name,
		"tag", pl.Tag,
		"tracks", len(pl.Tracks),
	)

	return &pl, nil
}

// ExportMasterPlaylist serialises the entire MasterPlaylist to JSON bytes.
func ExportMasterPlaylist(master *MasterPlaylist) ([]byte, error) {
	master.mu.RLock()
	data := storeData{
		Morning:   master.Morning,
		Afternoon: master.Afternoon,
		Evening:   master.Evening,
		Night:     master.Night,
	}
	master.mu.RUnlock()

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal master playlist: %w", err)
	}
	return jsonBytes, nil
}

// ImportMasterPlaylist reads JSON bytes and returns a fully reconstructed
// MasterPlaylist.
func ImportMasterPlaylist(data []byte) (*MasterPlaylist, error) {
	var sd storeData
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, fmt.Errorf("failed to parse master playlist data: %w", err)
	}

	restorePlaylists(sd.Morning, TagMorning)
	restorePlaylists(sd.Afternoon, TagAfternoon)
	restorePlaylists(sd.Evening, TagEvening)
	restorePlaylists(sd.Night, TagNight)

	master := &MasterPlaylist{
		Morning:   sd.Morning,
		Afternoon: sd.Afternoon,
		Evening:   sd.Evening,
		Night:     sd.Night,
	}

	if master.Morning == nil {
		master.Morning = make([]*Playlist, 0)
	}
	if master.Afternoon == nil {
		master.Afternoon = make([]*Playlist, 0)
	}
	if master.Evening == nil {
		master.Evening = make([]*Playlist, 0)
	}
	if master.Night == nil {
		master.Night = make([]*Playlist, 0)
	}

	syncIDCounters(master)

	return master, nil
}

// restorePlaylists walks a slice of playlists and restores internal runtime
// state (e.g. currentIndex from CurrentTrackChecksum, nil track slices).
func restorePlaylists(playlists []*Playlist, tag TimeTag) {
	for _, pl := range playlists {
		pl.Tag = tag

		if pl.Tracks == nil {
			pl.Tracks = make([]*Track, 0)
		}

		// Restore currentIndex from the persisted checksum.
		if pl.CurrentTrackChecksum != "" {
			for i, t := range pl.Tracks {
				if t.Checksum == pl.CurrentTrackChecksum {
					pl.currentIndex = i
					break
				}
			}
		}
	}
}

// syncIDCounters scans the master playlist for the highest track and playlist
// IDs, then updates the global counters so that new objects get unique IDs.
func syncIDCounters(master *MasterPlaylist) {
	var maxTrack int64
	var maxPlaylist int64

	for _, tag := range ValidTimeTags {
		for _, pl := range master.getPlaylistsUnsafe(tag) {
			if pl.ID > maxPlaylist {
				maxPlaylist = pl.ID
			}
			for _, t := range pl.Tracks {
				if t.ID > maxTrack {
					maxTrack = t.ID
				}
			}
		}
	}

	SetLastTrackID(maxTrack)
	SetLastPlaylistID(maxPlaylist)

	slog.Debug("ID counters synced",
		"max_track_id", maxTrack,
		"max_playlist_id", maxPlaylist,
	)
}
