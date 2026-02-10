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
type Playlist struct {
	mu                   sync.RWMutex
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Tag                  TimeTag  `json:"tag"`
	Tracks               []*Track `json:"tracks"`
	CurrentTrackChecksum string   `json:"currentTrackChecksum,omitempty"`
	currentIndex         int
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

// AddTrack appends a track to the end of the playlist.
func (p *Playlist) AddTrack(track *Track) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Tracks = append(p.Tracks, track)
}

// AddTrackAt inserts a track at the specified index. If the index is out of
// range the track is appended to the end.
func (p *Playlist) AddTrackAt(track *Track, index int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < 0 || index >= len(p.Tracks) {
		p.Tracks = append(p.Tracks, track)
		return
	}

	p.Tracks = append(p.Tracks, nil)
	copy(p.Tracks[index+1:], p.Tracks[index:])
	p.Tracks[index] = track

	// Adjust current index if needed.
	if index <= p.currentIndex && len(p.Tracks) > 1 {
		p.currentIndex++
	}
}

// AddTracks appends multiple tracks to the end of the playlist.
func (p *Playlist) AddTracks(tracks []*Track) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Tracks = append(p.Tracks, tracks...)
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

	// Adjust current index.
	if len(p.Tracks) == 0 {
		p.currentIndex = 0
		p.CurrentTrackChecksum = ""
	} else if index < p.currentIndex {
		p.currentIndex--
	} else if p.currentIndex >= len(p.Tracks) {
		p.currentIndex = 0
	}

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

			if len(p.Tracks) == 0 {
				p.currentIndex = 0
				p.CurrentTrackChecksum = ""
			} else if i < p.currentIndex {
				p.currentIndex--
			} else if p.currentIndex >= len(p.Tracks) {
				p.currentIndex = 0
			}

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

			if len(p.Tracks) == 0 {
				p.currentIndex = 0
				p.CurrentTrackChecksum = ""
			} else if i < p.currentIndex {
				p.currentIndex--
			} else if p.currentIndex >= len(p.Tracks) {
				p.currentIndex = 0
			}

			return removed, nil
		}
	}
	return nil, errors.New("track not found")
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

	// Update current index to follow the currently-playing track.
	if p.CurrentTrackChecksum != "" {
		for i, t := range p.Tracks {
			if t.Checksum == p.CurrentTrackChecksum {
				p.currentIndex = i
				break
			}
		}
	}

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

	// Re-locate the current track by checksum.
	if p.CurrentTrackChecksum != "" {
		for i, t := range p.Tracks {
			if t.Checksum == p.CurrentTrackChecksum {
				p.currentIndex = i
				return
			}
		}
		// Checksum not found (track was removed); reset.
		p.currentIndex = 0
		p.CurrentTrackChecksum = ""
	}
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

// Clone returns a deep copy of the playlist (with a new ID).
func (p *Playlist) Clone(newName string) *Playlist {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tracks := make([]*Track, len(p.Tracks))
	for i, t := range p.Tracks {
		copied := *t
		tracks[i] = &copied
	}

	return &Playlist{
		ID:                   nextPlaylistID(),
		Name:                 newName,
		Tag:                  p.Tag,
		Tracks:               tracks,
		CurrentTrackChecksum: p.CurrentTrackChecksum,
		currentIndex:         p.currentIndex,
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
