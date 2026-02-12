package playlist

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// TrackLibrary is the single source of truth for all known tracks. Every track
// in the system lives here; playlists hold pointers into the library so that
// metadata edits propagate everywhere automatically.
type TrackLibrary struct {
	mu     sync.RWMutex
	tracks map[string]*Track // keyed by checksum
	byID   map[int64]*Track  // secondary index by numeric ID
	nextID int64             // counter for assigning stable IDs
}

// NewTrackLibrary creates an empty TrackLibrary.
func NewTrackLibrary() *TrackLibrary {
	return &TrackLibrary{
		tracks: make(map[string]*Track),
		byID:   make(map[int64]*Track),
		nextID: 0,
	}
}

// allocateID returns the next unique track ID. Caller must hold the write lock.
func (lib *TrackLibrary) allocateID() int64 {
	lib.nextID++
	return lib.nextID
}

// Add inserts a track into the library. If a track with the same checksum
// already exists, the existing track is returned unchanged and added is false.
// Otherwise the track receives a new stable ID, is stored, and added is true.
func (lib *TrackLibrary) Add(t *Track) (existing *Track, added bool) {
	if t == nil || t.Checksum == "" {
		return t, false
	}

	lib.mu.Lock()
	defer lib.mu.Unlock()

	if ex, ok := lib.tracks[t.Checksum]; ok {
		return ex, false
	}

	t.ID = lib.allocateID()
	lib.tracks[t.Checksum] = t
	lib.byID[t.ID] = t
	return t, true
}

// AddOrUpdate inserts a track if it doesn't exist (by checksum). If it already
// exists, the file path is updated (the file may have been moved/renamed) but
// user-edited metadata is preserved. Returns the canonical track.
func (lib *TrackLibrary) AddOrUpdate(t *Track) *Track {
	if t == nil || t.Checksum == "" {
		return t
	}

	lib.mu.Lock()
	defer lib.mu.Unlock()

	if ex, ok := lib.tracks[t.Checksum]; ok {
		// Update file path in case the file moved on disk.
		if t.FilePath != "" && t.FilePath != ex.FilePath {
			ex.FilePath = t.FilePath
		}
		// Update format if it changed (e.g. re-encoded file with same content hash – unlikely but safe).
		if t.Format != "" {
			ex.Format = t.Format
		}
		return ex
	}

	t.ID = lib.allocateID()
	lib.tracks[t.Checksum] = t
	lib.byID[t.ID] = t
	return t
}

// BulkAdd adds multiple tracks to the library. Tracks whose checksums are
// already present are skipped. Returns the number of newly added tracks.
func (lib *TrackLibrary) BulkAdd(tracks []*Track) int {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	added := 0
	for _, t := range tracks {
		if t == nil || t.Checksum == "" {
			continue
		}
		if _, ok := lib.tracks[t.Checksum]; ok {
			continue
		}
		t.ID = lib.allocateID()
		lib.tracks[t.Checksum] = t
		lib.byID[t.ID] = t
		added++
	}
	return added
}

// Get returns the track with the given checksum, or nil if not found.
func (lib *TrackLibrary) Get(checksum string) *Track {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	return lib.tracks[checksum]
}

// GetByID returns the track with the given numeric ID, or nil if not found.
func (lib *TrackLibrary) GetByID(id int64) *Track {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	return lib.byID[id]
}

// GetByFilePath returns the first track matching the given file path, or nil.
func (lib *TrackLibrary) GetByFilePath(filePath string) *Track {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	for _, t := range lib.tracks {
		if t.FilePath == filePath {
			return t
		}
	}
	return nil
}

// Remove deletes the track with the given checksum from the library. Returns
// the removed track or nil if not found.
func (lib *TrackLibrary) Remove(checksum string) *Track {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	t, ok := lib.tracks[checksum]
	if !ok {
		return nil
	}
	delete(lib.tracks, checksum)
	delete(lib.byID, t.ID)
	return t
}

// RemoveByID deletes the track with the given numeric ID from the library.
// Returns the removed track or nil if not found.
func (lib *TrackLibrary) RemoveByID(id int64) *Track {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	t, ok := lib.byID[id]
	if !ok {
		return nil
	}
	delete(lib.tracks, t.Checksum)
	delete(lib.byID, id)
	return t
}

// Update modifies the mutable metadata fields of the track identified by the
// given ID. Only non-nil fields in the update are applied. Returns the updated
// track or an error if the track is not found.
func (lib *TrackLibrary) Update(id int64, upd TrackUpdate) (*Track, error) {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	t, ok := lib.byID[id]
	if !ok {
		return nil, fmt.Errorf("track %d not found in library", id)
	}

	if upd.Title != nil {
		t.Title = *upd.Title
	}
	if upd.Artist != nil {
		t.Artist = *upd.Artist
	}
	if upd.Album != nil {
		t.Album = *upd.Album
	}
	if upd.Genre != nil {
		t.Genre = *upd.Genre
	}
	if upd.Year != nil {
		t.Year = *upd.Year
	}
	if upd.TrackNum != nil {
		t.TrackNum = *upd.TrackNum
	}
	if upd.Duration != nil {
		t.Duration = *upd.Duration
	}

	return t, nil
}

// TrackUpdate holds optional field updates for a track. Nil fields are not
// applied.
type TrackUpdate struct {
	Title    *string `json:"title,omitempty"`
	Artist   *string `json:"artist,omitempty"`
	Album    *string `json:"album,omitempty"`
	Genre    *string `json:"genre,omitempty"`
	Year     *int    `json:"year,omitempty"`
	TrackNum *int    `json:"trackNum,omitempty"`
	Duration *int    `json:"duration,omitempty"`
}

// List returns all tracks in the library as a slice, sorted by ID.
func (lib *TrackLibrary) List() []*Track {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	result := make([]*Track, 0, len(lib.tracks))
	// Iterate in ID order for deterministic output.
	for id := int64(1); id <= lib.nextID; id++ {
		if t, ok := lib.byID[id]; ok {
			result = append(result, t)
		}
	}
	return result
}

// Count returns the number of tracks in the library.
func (lib *TrackLibrary) Count() int {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	return len(lib.tracks)
}

// Contains returns true if a track with the given checksum is in the library.
func (lib *TrackLibrary) Contains(checksum string) bool {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	_, ok := lib.tracks[checksum]
	return ok
}

// Checksums returns all checksums currently in the library.
func (lib *TrackLibrary) Checksums() []string {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	result := make([]string, 0, len(lib.tracks))
	for cs := range lib.tracks {
		result = append(result, cs)
	}
	return result
}

// Search returns tracks whose title, artist, or album contain the query
// string (case-insensitive substring match).
func (lib *TrackLibrary) Search(query string) []*Track {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	if query == "" {
		return lib.listUnsafe()
	}

	var results []*Track
	for id := int64(1); id <= lib.nextID; id++ {
		t, ok := lib.byID[id]
		if !ok {
			continue
		}
		if containsFold(t.Title, query) ||
			containsFold(t.Artist, query) ||
			containsFold(t.Album, query) ||
			containsFold(t.Genre, query) {
			results = append(results, t)
		}
	}
	return results
}

// listUnsafe returns all tracks sorted by ID without locking. Caller must hold
// at least a read lock.
func (lib *TrackLibrary) listUnsafe() []*Track {
	result := make([]*Track, 0, len(lib.tracks))
	for id := int64(1); id <= lib.nextID; id++ {
		if t, ok := lib.byID[id]; ok {
			result = append(result, t)
		}
	}
	return result
}

// Resolve returns the canonical *Track pointers for the given checksums. Any
// checksum that is not found in the library is silently skipped, and a warning
// is logged. This is used when loading playlists to turn persisted checksum
// references back into live pointers.
func (lib *TrackLibrary) Resolve(checksums []string) []*Track {
	lib.mu.RLock()
	defer lib.mu.RUnlock()

	result := make([]*Track, 0, len(checksums))
	for _, cs := range checksums {
		t, ok := lib.tracks[cs]
		if !ok {
			slog.Warn("Library: track checksum not found during resolve, skipping",
				"checksum", cs,
			)
			continue
		}
		result = append(result, t)
	}
	return result
}

// RemoveStale removes all tracks from the library whose files no longer exist
// on disk. Returns the list of removed tracks.
func (lib *TrackLibrary) RemoveStale() []*Track {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	var removed []*Track
	for cs, t := range lib.tracks {
		if !t.FileExists() {
			removed = append(removed, t)
			delete(lib.tracks, cs)
			delete(lib.byID, t.ID)
		}
	}

	if len(removed) > 0 {
		slog.Info("Library: removed stale tracks", "count", len(removed))
	}

	return removed
}

// SyncNextID updates the internal ID counter to be at least as large as the
// maximum ID present in the library. This must be called after bulk-loading
// tracks with pre-assigned IDs (e.g. migration from old format).
func (lib *TrackLibrary) SyncNextID() {
	lib.mu.Lock()
	defer lib.mu.Unlock()

	for _, t := range lib.tracks {
		if t.ID > lib.nextID {
			lib.nextID = t.ID
		}
	}
}

// Import adds a track with a pre-assigned ID (used when loading from
// persisted data). If a track with the same checksum already exists, the
// existing one is returned. The internal ID counter is updated to stay above
// the imported ID.
func (lib *TrackLibrary) Import(t *Track) *Track {
	if t == nil || t.Checksum == "" {
		return t
	}

	lib.mu.Lock()
	defer lib.mu.Unlock()

	if ex, ok := lib.tracks[t.Checksum]; ok {
		return ex
	}

	lib.tracks[t.Checksum] = t
	lib.byID[t.ID] = t

	if t.ID > lib.nextID {
		lib.nextID = t.ID
	}

	return t
}

// NextID returns the current value of the internal ID counter (for diagnostics).
func (lib *TrackLibrary) NextID() int64 {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	return lib.nextID
}

// ---------------------------------------------------------------------------
// Serialisation helpers
// ---------------------------------------------------------------------------

// MarshalJSON serialises the library as a sorted array of tracks.
func (lib *TrackLibrary) MarshalJSON() ([]byte, error) {
	lib.mu.RLock()
	defer lib.mu.RUnlock()
	return json.Marshal(lib.listUnsafe())
}

// UnmarshalJSON deserialises an array of tracks into the library, rebuilding
// the internal indices.
func (lib *TrackLibrary) UnmarshalJSON(data []byte) error {
	var tracks []*Track
	if err := json.Unmarshal(data, &tracks); err != nil {
		return err
	}

	lib.mu.Lock()
	defer lib.mu.Unlock()

	lib.tracks = make(map[string]*Track, len(tracks))
	lib.byID = make(map[int64]*Track, len(tracks))
	lib.nextID = 0

	for _, t := range tracks {
		if t == nil || t.Checksum == "" {
			continue
		}
		lib.tracks[t.Checksum] = t
		lib.byID[t.ID] = t
		if t.ID > lib.nextID {
			lib.nextID = t.ID
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// containsFold is a simple case-insensitive substring check (avoids pulling
// in strings for a one-liner).
// ---------------------------------------------------------------------------

func containsFold(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Convert both to lower-case and check with a sliding window.
	sl := toLower(s)
	pl := toLower(substr)
	for i := 0; i <= len(sl)-len(pl); i++ {
		if sl[i:i+len(pl)] == pl {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
