package playlist

import (
	"errors"
	"math/rand/v2"
	"sync"
)

// TimeTag represents a time-of-day category for playlist scheduling.
type TimeTag string

const (
	TagMorning   TimeTag = "morning"
	TagAfternoon TimeTag = "afternoon"
	TagEvening   TimeTag = "evening"
	TagNight     TimeTag = "night"
)

// ValidTimeTags contains all valid TimeTag values.
var ValidTimeTags = []TimeTag{TagMorning, TagAfternoon, TagEvening, TagNight}

// IsValidTimeTag returns true if the given string is a valid TimeTag.
func IsValidTimeTag(s string) bool {
	for _, t := range ValidTimeTags {
		if string(t) == s {
			return true
		}
	}
	return false
}

// lastPlaylistID is a global counter for generating unique playlist IDs.
var (
	playlistIDMu   sync.Mutex
	lastPlaylistID int64
)

// nextPlaylistID returns the next unique playlist ID.
func nextPlaylistID() int64 {
	playlistIDMu.Lock()
	defer playlistIDMu.Unlock()
	lastPlaylistID++
	return lastPlaylistID
}

// SetLastPlaylistID sets the global playlist ID counter. Used when loading
// persisted playlists so that newly created playlists don't collide with
// existing IDs.
func SetLastPlaylistID(id int64) {
	playlistIDMu.Lock()
	defer playlistIDMu.Unlock()
	lastPlaylistID = id
}

// Playlist represents an ordered queue of tracks with a time-of-day tag.
// Tracks are pointers into a shared TrackLibrary so that metadata edits in the
// library are automatically visible everywhere.
type Playlist struct {
	mu                   sync.RWMutex
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Tag                  TimeTag  `json:"tag"`
	Tracks               []*Track `json:"tracks"`
	CurrentTrackChecksum string   `json:"currentTrackChecksum,omitempty"`
	currentIndex         int
	library              *TrackLibrary // optional reference; when set, tracks are validated against it
}

// SetLibrary associates this playlist with a TrackLibrary. When set, AddTrack
// and AddTrackByChecksum will validate that the track exists in the library and
// store a pointer to the canonical library entry.
func (p *Playlist) SetLibrary(lib *TrackLibrary) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.library = lib
}

// Library returns the TrackLibrary associated with this playlist, or nil.
func (p *Playlist) Library() *TrackLibrary {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.library
}

// TrackChecksums returns the ordered list of track checksums. This is used
// when persisting playlists so that only references (not full track data) are
// stored on disk.
func (p *Playlist) TrackChecksums() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cs := make([]string, len(p.Tracks))
	for i, t := range p.Tracks {
		cs[i] = t.Checksum
	}
	return cs
}

// relocateCursorUnsafe re-computes currentIndex from CurrentTrackChecksum
// after any structural mutation to the Tracks slice. Must be called with
// p.mu held for writing.
//
// Semantics of currentIndex: it is the index of the track that will be
// returned by the NEXT call to Next(). CurrentTrackChecksum holds the
// checksum of the track that was last returned by Next() (the one currently
// being streamed). Therefore, after finding the currently-playing track at
// position i, the cursor must be placed at (i+1) % len so that playback
// continues with the track after it.
func (p *Playlist) relocateCursorUnsafe() {
	if len(p.Tracks) == 0 {
		p.currentIndex = 0
		p.CurrentTrackChecksum = ""
		return
	}

	if p.CurrentTrackChecksum == "" {
		// Nothing has been played yet; clamp to a valid position.
		if p.currentIndex >= len(p.Tracks) {
			p.currentIndex = 0
		}
		return
	}

	// Find the currently-playing track and advance one step past it.
	for i, t := range p.Tracks {
		if t.Checksum == p.CurrentTrackChecksum {
			p.currentIndex = (i + 1) % len(p.Tracks)
			return
		}
	}

	// Currently-playing track was removed; keep the cursor in bounds.
	if p.currentIndex >= len(p.Tracks) {
		p.currentIndex = 0
	}
}

// ResolveFromLibrary replaces the Tracks slice with canonical pointers from
// the given library, matched by checksum. Tracks whose checksums are not found
// in the library are silently dropped.
func (p *Playlist) ResolveFromLibrary(lib *TrackLibrary) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.library = lib

	resolved := make([]*Track, 0, len(p.Tracks))
	for _, t := range p.Tracks {
		if canonical := lib.Get(t.Checksum); canonical != nil {
			resolved = append(resolved, canonical)
		}
	}
	p.Tracks = resolved

	p.relocateCursorUnsafe()
}

// NewPlaylist creates a new empty Playlist with the given name and tag.
func NewPlaylist(name string, tag TimeTag) *Playlist {
	return &Playlist{
		ID:     nextPlaylistID(),
		Name:   name,
		Tag:    tag,
		Tracks: make([]*Track, 0),
	}
}

// NewPlaylistWithID creates a Playlist with a pre-assigned ID. This is used
// when loading from persisted data.
func NewPlaylistWithID(id int64, name string, tag TimeTag, tracks []*Track, currentChecksum string) *Playlist {
	if tracks == nil {
		tracks = make([]*Track, 0)
	}
	pl := &Playlist{
		ID:                   id,
		Name:                 name,
		Tag:                  tag,
		Tracks:               tracks,
		CurrentTrackChecksum: currentChecksum,
	}
	// Restore the current index from the checksum if possible.
	if currentChecksum != "" {
		for i, t := range tracks {
			if t.Checksum == currentChecksum {
				pl.currentIndex = i
				return pl
			}
		}
	}
	pl.currentIndex = 0
	return pl
}

// Count returns the number of tracks in the playlist.
func (p *Playlist) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.Tracks)
}

// GetTrack returns the track at the given index, or an error if the index is
// out of range.
func (p *Playlist) GetTrack(index int) (*Track, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if index < 0 || index >= len(p.Tracks) {
		return nil, errors.New("index out of range")
	}
	return p.Tracks[index], nil
}

// FindTrackByID returns the track with the given ID and its index, or an error
// if not found.
func (p *Playlist) FindTrackByID(id int64) (*Track, int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i, t := range p.Tracks {
		if t.ID == id {
			return t, i, nil
		}
	}
	return nil, -1, errors.New("track not found")
}

// FindTrackByFilePath returns the track with the given file path and its index,
// or an error if not found.
func (p *Playlist) FindTrackByFilePath(filePath string) (*Track, int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i, t := range p.Tracks {
		if t.FilePath == filePath {
			return t, i, nil
		}
	}
	return nil, -1, errors.New("track not found")
}

// FindTrackByChecksum returns the track with the given checksum and its index,
// or an error if not found.
func (p *Playlist) FindTrackByChecksum(checksum string) (*Track, int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i, t := range p.Tracks {
		if t.Checksum == checksum {
			return t, i, nil
		}
	}
	return nil, -1, errors.New("track not found")
}

// AddTrack appends a track to the end of the playlist. If the playlist has an
// associated library, the track must exist in the library; the canonical library
// pointer is used instead of the provided pointer to keep references consistent.
func (p *Playlist) AddTrack(track *Track) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.library != nil && track != nil {
		if canonical := p.library.Get(track.Checksum); canonical != nil {
			track = canonical
		}
	}

	p.Tracks = append(p.Tracks, track)
	p.relocateCursorUnsafe()
}

// AddTrackByChecksum looks up the track in the associated library and appends
// it to the playlist. Returns an error if the library is not set or the
// checksum is not found.
func (p *Playlist) AddTrackByChecksum(checksum string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.library == nil {
		return errors.New("playlist has no associated library")
	}

	t := p.library.Get(checksum)
	if t == nil {
		return errors.New("track not found in library")
	}

	p.Tracks = append(p.Tracks, t)
	p.relocateCursorUnsafe()
	return nil
}

// AddTrackAt inserts a track at the specified index. If the index is out of
// range the track is appended to the end. The playback cursor is preserved:
// a track inserted at or before the next-to-play position becomes the new
// next-to-play track; a track inserted strictly after the cursor does not
// disturb it.
func (p *Playlist) AddTrackAt(track *Track, index int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.library != nil && track != nil {
		if canonical := p.library.Get(track.Checksum); canonical != nil {
			track = canonical
		}
	}

	if index < 0 || index >= len(p.Tracks) {
		p.Tracks = append(p.Tracks, track)
		p.relocateCursorUnsafe()
		return
	}

	p.Tracks = append(p.Tracks, nil)
	copy(p.Tracks[index+1:], p.Tracks[index:])
	p.Tracks[index] = track

	p.relocateCursorUnsafe()
}

// AddTracks appends multiple tracks to the end of the playlist.
func (p *Playlist) AddTracks(tracks []*Track) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, track := range tracks {
		t := track
		if p.library != nil && t != nil {
			if canonical := p.library.Get(t.Checksum); canonical != nil {
				t = canonical
			}
		}
		p.Tracks = append(p.Tracks, t)
	}
	p.relocateCursorUnsafe()
}

// RemoveTrack removes the track at the given index and returns it. Returns an
// error if the index is out of range.
func (p *Playlist) RemoveTrack(index int) (*Track, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < 0 || index >= len(p.Tracks) {
		return nil, errors.New("index out of range")
	}

	removed := p.Tracks[index]
	p.Tracks = append(p.Tracks[:index], p.Tracks[index+1:]...)
	p.relocateCursorUnsafe()

	return removed, nil
}

// RemoveTrackByID removes the track with the given ID and returns it.
func (p *Playlist) RemoveTrackByID(id int64) (*Track, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, t := range p.Tracks {
		if t.ID == id {
			removed := p.Tracks[i]
			p.Tracks = append(p.Tracks[:i], p.Tracks[i+1:]...)
			p.relocateCursorUnsafe()
			return removed, nil
		}
	}
	return nil, errors.New("track not found")
}

// RemoveTrackByChecksum removes the track with the given checksum and returns it.
func (p *Playlist) RemoveTrackByChecksum(checksum string) (*Track, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, t := range p.Tracks {
		if t.Checksum == checksum {
			removed := p.Tracks[i]
			p.Tracks = append(p.Tracks[:i], p.Tracks[i+1:]...)
			p.relocateCursorUnsafe()
			return removed, nil
		}
	}
	return nil, errors.New("track not found")
}

// RemoveTracksByChecksum removes ALL occurrences of the given checksum from
// the playlist. This is used when a track is deleted from the library.
// Returns the number of occurrences removed.
func (p *Playlist) RemoveTracksByChecksum(checksum string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	alive := make([]*Track, 0, len(p.Tracks))
	removed := 0
	for _, t := range p.Tracks {
		if t.Checksum == checksum {
			removed++
		} else {
			alive = append(alive, t)
		}
	}
	p.Tracks = alive

	// If the currently-playing track was removed, clear the checksum so
	// relocateCursorUnsafe falls back gracefully.
	if p.CurrentTrackChecksum == checksum {
		p.CurrentTrackChecksum = ""
	}
	p.relocateCursorUnsafe()

	return removed
}

// MoveTrack moves a track from the source index to the destination index.
// Returns an error if either index is out of range.
func (p *Playlist) MoveTrack(from, to int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if from < 0 || from >= len(p.Tracks) {
		return errors.New("source index out of range")
	}
	if to < 0 || to >= len(p.Tracks) {
		return errors.New("destination index out of range")
	}
	if from == to {
		return nil
	}

	track := p.Tracks[from]

	// Remove from old position.
	p.Tracks = append(p.Tracks[:from], p.Tracks[from+1:]...)

	// Insert at new position.
	p.Tracks = append(p.Tracks, nil)
	copy(p.Tracks[to+1:], p.Tracks[to:])
	p.Tracks[to] = track

	p.relocateCursorUnsafe()
	return nil
}

// Shuffle randomises the order of tracks in the playlist. The current track
// (identified by its checksum) stays tracked correctly after the shuffle.
func (p *Playlist) Shuffle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	rand.Shuffle(len(p.Tracks), func(i, j int) {
		p.Tracks[i], p.Tracks[j] = p.Tracks[j], p.Tracks[i]
	})

	p.relocateCursorUnsafe()
}

// Next returns the next track in the playlist and advances the internal
// cursor. Returns nil and false if the playlist is empty.
func (p *Playlist) Next() (*Track, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.Tracks) == 0 {
		return nil, false
	}

	track := p.Tracks[p.currentIndex]
	p.CurrentTrackChecksum = track.Checksum
	p.currentIndex = (p.currentIndex + 1) % len(p.Tracks)

	return track, true
}

// Current returns the track that was most recently returned by Next().
// Returns nil and false if the playlist is empty or Next() hasn't been called.
func (p *Playlist) Current() (*Track, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.Tracks) == 0 || p.CurrentTrackChecksum == "" {
		return nil, false
	}

	for _, t := range p.Tracks {
		if t.Checksum == p.CurrentTrackChecksum {
			return t, true
		}
	}
	return nil, false
}

// Peek returns the track that will be returned by the next call to Next(),
// without advancing the cursor.
func (p *Playlist) Peek() (*Track, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.Tracks) == 0 {
		return nil, false
	}

	return p.Tracks[p.currentIndex], true
}

// ClearTracks removes all tracks from the playlist and resets the cursor.
func (p *Playlist) ClearTracks() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Tracks = make([]*Track, 0)
	p.currentIndex = 0
	p.CurrentTrackChecksum = ""
}

// TrackIDs returns a slice of all track IDs in order.
func (p *Playlist) TrackIDs() []int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ids := make([]int64, len(p.Tracks))
	for i, t := range p.Tracks {
		ids[i] = t.ID
	}
	return ids
}

// ContainsTrack returns true if a track with the given checksum exists in the
// playlist.
func (p *Playlist) ContainsTrack(checksum string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, t := range p.Tracks {
		if t.Checksum == checksum {
			return true
		}
	}
	return false
}

// SeekTo moves the playback cursor to the given index so that the next call to
// Next() will return the track at that position. The index wraps around
// modulo the playlist length. A no-op on an empty playlist.
func (p *Playlist) SeekTo(index int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(p.Tracks)
	if n == 0 {
		return
	}
	p.currentIndex = ((index % n) + n) % n
}

// Clone returns a deep copy of the playlist (with a new ID).
func (p *Playlist) Clone(newName string) *Playlist {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// When cloning, we keep the same track pointers (they point into the
	// library, which is the single source of truth). We do NOT deep-copy
	// the Track structs because we want edits to propagate.
	tracks := make([]*Track, len(p.Tracks))
	copy(tracks, p.Tracks)

	return &Playlist{
		ID:                   nextPlaylistID(),
		Name:                 newName,
		Tag:                  p.Tag,
		Tracks:               tracks,
		CurrentTrackChecksum: p.CurrentTrackChecksum,
		currentIndex:         p.currentIndex,
		library:              p.library,
	}
}

// MaxPlaylistID returns the highest ID found across a slice of playlists.
// Returns 0 if the slice is empty.
func MaxPlaylistID(playlists []*Playlist) int64 {
	var max int64
	for _, pl := range playlists {
		if pl.ID > max {
			max = pl.ID
		}
	}
	return max
}
