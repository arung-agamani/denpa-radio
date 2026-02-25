package playlist

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// MasterPlaylist holds collections of playlists organised by time-of-day tags.
// The radio service plays from the active playlist, which is determined by the
// current time and the tag assignments.
type MasterPlaylist struct {
	mu        sync.RWMutex
	Morning   []*Playlist `json:"morning"`
	Afternoon []*Playlist `json:"afternoon"`
	Evening   []*Playlist `json:"evening"`
	Night     []*Playlist `json:"night"`

	// Library is the single source of truth for all track data. Every track
	// referenced by any playlist must exist in this library.
	Library *TrackLibrary `json:"-"`

	// activeTag is the tag currently being used for playback.
	activeTag TimeTag
	// activePlaylistIndex tracks which playlist within the active tag's slice
	// is currently being played.
	activePlaylistIndex int

	// location is the IANA timezone used for time-tag resolution.
	// When nil, time.UTC is used.
	location *time.Location
	// timezoneName stores the IANA name so it can be persisted and returned
	// via the API (e.g. "Asia/Tokyo", "America/New_York").
	timezoneName string
}

// NewMasterPlaylist creates a new MasterPlaylist with empty slices for each
// time tag and a fresh TrackLibrary.
func NewMasterPlaylist() *MasterPlaylist {
	return &MasterPlaylist{
		Morning:   make([]*Playlist, 0),
		Afternoon: make([]*Playlist, 0),
		Evening:   make([]*Playlist, 0),
		Night:     make([]*Playlist, 0),
		Library:   NewTrackLibrary(),
	}
}

// NewMasterPlaylistWithLibrary creates a new MasterPlaylist using an existing
// TrackLibrary instance.
func NewMasterPlaylistWithLibrary(lib *TrackLibrary) *MasterPlaylist {
	if lib == nil {
		lib = NewTrackLibrary()
	}
	return &MasterPlaylist{
		Morning:   make([]*Playlist, 0),
		Afternoon: make([]*Playlist, 0),
		Evening:   make([]*Playlist, 0),
		Night:     make([]*Playlist, 0),
		Library:   lib,
	}
}

// GetPlaylists returns the slice of playlists assigned to the given tag.
func (mp *MasterPlaylist) GetPlaylists(tag TimeTag) []*Playlist {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.getPlaylistsUnsafe(tag)
}

// getPlaylistsUnsafe returns the slice pointer for the given tag without
// locking. The caller must hold at least a read lock.
func (mp *MasterPlaylist) getPlaylistsUnsafe(tag TimeTag) []*Playlist {
	switch tag {
	case TagMorning:
		return mp.Morning
	case TagAfternoon:
		return mp.Afternoon
	case TagEvening:
		return mp.Evening
	case TagNight:
		return mp.Night
	default:
		return nil
	}
}

// setPlaylistsUnsafe replaces the slice for the given tag. The caller must
// hold a write lock.
func (mp *MasterPlaylist) setPlaylistsUnsafe(tag TimeTag, pls []*Playlist) {
	switch tag {
	case TagMorning:
		mp.Morning = pls
	case TagAfternoon:
		mp.Afternoon = pls
	case TagEvening:
		mp.Evening = pls
	case TagNight:
		mp.Night = pls
	}
}

// AssignPlaylist adds a playlist to the specified time tag. If a playlist with
// the same ID already exists under that tag it is replaced. The playlist's
// library reference is set to this master playlist's library.
func (mp *MasterPlaylist) AssignPlaylist(tag TimeTag, pl *Playlist) error {
	if !IsValidTimeTag(string(tag)) {
		return fmt.Errorf("invalid time tag: %s", tag)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Update the playlist's own tag to match.
	pl.Tag = tag

	// Associate the playlist with this master playlist's library.
	if mp.Library != nil {
		pl.SetLibrary(mp.Library)
	}

	existing := mp.getPlaylistsUnsafe(tag)
	for i, p := range existing {
		if p.ID == pl.ID {
			existing[i] = pl
			mp.setPlaylistsUnsafe(tag, existing)
			return nil
		}
	}

	mp.setPlaylistsUnsafe(tag, append(existing, pl))
	return nil
}

// RemovePlaylist removes a playlist with the given ID from the specified tag.
// Returns an error if the playlist is not found under that tag.
func (mp *MasterPlaylist) RemovePlaylist(tag TimeTag, playlistID int64) error {
	if !IsValidTimeTag(string(tag)) {
		return fmt.Errorf("invalid time tag: %s", tag)
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()

	existing := mp.getPlaylistsUnsafe(tag)
	for i, p := range existing {
		if p.ID == playlistID {
			updated := append(existing[:i], existing[i+1:]...)
			mp.setPlaylistsUnsafe(tag, updated)

			// Adjust active playlist index if we removed from the active tag.
			if tag == mp.activeTag {
				if mp.activePlaylistIndex >= len(updated) && len(updated) > 0 {
					mp.activePlaylistIndex = 0
				} else if len(updated) == 0 {
					mp.activePlaylistIndex = 0
				}
			}
			return nil
		}
	}

	return fmt.Errorf("playlist %d not found under tag %s", playlistID, tag)
}

// FindPlaylistByID searches all tags for a playlist with the given ID and
// returns it along with the tag it belongs to.
func (mp *MasterPlaylist) FindPlaylistByID(id int64) (*Playlist, TimeTag, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	for _, tag := range ValidTimeTags {
		for _, p := range mp.getPlaylistsUnsafe(tag) {
			if p.ID == id {
				return p, tag, nil
			}
		}
	}
	return nil, "", fmt.Errorf("playlist %d not found", id)
}

// AllPlaylists returns every playlist across all tags.
func (mp *MasterPlaylist) AllPlaylists() []*Playlist {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var all []*Playlist
	for _, tag := range ValidTimeTags {
		all = append(all, mp.getPlaylistsUnsafe(tag)...)
	}
	return all
}

// AllTracks returns every track across all playlists in the master playlist.
// Tracks that appear in multiple playlists will be returned multiple times.
func (mp *MasterPlaylist) AllTracks() []*Track {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	var tracks []*Track
	for _, tag := range ValidTimeTags {
		for _, pl := range mp.getPlaylistsUnsafe(tag) {
			pl.mu.RLock()
			tracks = append(tracks, pl.Tracks...)
			pl.mu.RUnlock()
		}
	}
	return tracks
}

// AllTracksDeduped returns unique tracks across all playlists, deduped by
// checksum. If a TrackLibrary is available, it returns all library tracks
// instead (which is the authoritative set).
func (mp *MasterPlaylist) AllTracksDeduped() []*Track {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	// If we have a library, it is the canonical source of all known tracks.
	if mp.Library != nil {
		return mp.Library.List()
	}

	// Fallback for when no library is set (shouldn't happen in normal operation).
	seen := make(map[string]bool)
	var tracks []*Track
	for _, tag := range ValidTimeTags {
		for _, pl := range mp.getPlaylistsUnsafe(tag) {
			pl.mu.RLock()
			for _, t := range pl.Tracks {
				if !seen[t.Checksum] {
					seen[t.Checksum] = true
					tracks = append(tracks, t)
				}
			}
			pl.mu.RUnlock()
		}
	}
	return tracks
}

// RemoveTrackFromAll removes a track (by checksum) from ALL playlists in
// the master playlist. This is used when a track is deleted from the library.
// Returns the total number of occurrences removed.
func (mp *MasterPlaylist) RemoveTrackFromAll(checksum string) int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	total := 0
	for _, tag := range ValidTimeTags {
		for _, pl := range mp.getPlaylistsUnsafe(tag) {
			total += pl.RemoveTracksByChecksum(checksum)
		}
	}
	return total
}

// TimeTagForHour returns the appropriate TimeTag for the given hour (0-23).
//
//	Morning:   06:00 – 11:59
//	Afternoon: 12:00 – 17:59
//	Evening:   18:00 – 20:59
//	Night:     21:00 – 05:59
func TimeTagForHour(hour int) TimeTag {
	switch {
	case hour >= 6 && hour < 12:
		return TagMorning
	case hour >= 12 && hour < 18:
		return TagAfternoon
	case hour >= 18 && hour < 21:
		return TagEvening
	default:
		return TagNight
	}
}

// CurrentTimeTag returns the TimeTag for the current time in UTC.
// Prefer CurrentTimeTagIn when a specific timezone is desired.
func CurrentTimeTag() TimeTag {
	return TimeTagForHour(time.Now().UTC().Hour())
}

// CurrentTimeTagIn returns the TimeTag for the current time in the given
// location. If loc is nil, UTC is used.
func CurrentTimeTagIn(loc *time.Location) TimeTag {
	if loc == nil {
		loc = time.UTC
	}
	return TimeTagForHour(time.Now().In(loc).Hour())
}

// ResolveActiveTag determines which time tag should be active based on the
// current time and the master playlist's configured timezone. It returns the
// tag and whether a change from the previous active tag occurred.
func (mp *MasterPlaylist) ResolveActiveTag() (TimeTag, bool) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	tag := CurrentTimeTagIn(mp.location)
	changed := tag != mp.activeTag
	if changed {
		mp.activeTag = tag
		mp.activePlaylistIndex = 0
	}
	return tag, changed
}

// SetActiveTag explicitly sets the active tag (e.g. for testing or manual
// override).
func (mp *MasterPlaylist) SetActiveTag(tag TimeTag) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	mp.activeTag = tag
	mp.activePlaylistIndex = 0
}

// SetTimezone sets the IANA timezone used for time-tag resolution.
// An empty string resets to UTC. Returns an error if the name is invalid.
func (mp *MasterPlaylist) SetTimezone(name string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	if name == "" {
		mp.location = time.UTC
		mp.timezoneName = ""
		slog.Info("Timezone set to UTC")
		return nil
	}

	loc, err := time.LoadLocation(name)
	if err != nil {
		return fmt.Errorf("invalid timezone %q: %w", name, err)
	}

	mp.location = loc
	mp.timezoneName = name
	slog.Info("Timezone updated", "timezone", name)
	return nil
}

// Timezone returns the IANA timezone name currently configured.
// An empty string means UTC.
func (mp *MasterPlaylist) Timezone() string {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.timezoneName
}

// Location returns the *time.Location currently configured.
// Returns time.UTC if no timezone has been set.
func (mp *MasterPlaylist) Location() *time.Location {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	if mp.location == nil {
		return time.UTC
	}
	return mp.location
}

// ActiveTag returns the currently active time tag.
func (mp *MasterPlaylist) ActiveTag() TimeTag {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return mp.activeTag
}

// ActivePlaylist returns the playlist that should currently be playing based on
// the active tag. If no playlists are assigned to the active tag, it falls back
// through the tags in order: the current tag, then whichever tag has playlists.
// Returns nil and an error if no playlists exist at all.
func (mp *MasterPlaylist) ActivePlaylist() (*Playlist, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// First try the active tag.
	if pls := mp.getPlaylistsUnsafe(mp.activeTag); len(pls) > 0 {
		if mp.activePlaylistIndex >= len(pls) {
			mp.activePlaylistIndex = 0
		}
		return pls[mp.activePlaylistIndex], nil
	}

	// Fallback: find any tag that has playlists.
	for _, tag := range ValidTimeTags {
		if pls := mp.getPlaylistsUnsafe(tag); len(pls) > 0 {
			return pls[0], nil
		}
	}

	return nil, errors.New("no playlists available in master playlist")
}

// Next returns the next track to play from the active playlist. It handles
// advancing through tracks within a playlist and cycling through playlists
// within the active tag.
//
// The caller should periodically call ResolveActiveTag to allow time-based
// playlist switching.
func (mp *MasterPlaylist) Next() (*Track, *Playlist, error) {
	mp.mu.Lock()

	// Determine the effective playlists for the active tag (with fallback).
	var effectiveTag TimeTag
	var playlists []*Playlist

	if pls := mp.getPlaylistsUnsafe(mp.activeTag); len(pls) > 0 {
		effectiveTag = mp.activeTag
		playlists = pls
	} else {
		// Fallback to any tag with playlists.
		for _, tag := range ValidTimeTags {
			if pls := mp.getPlaylistsUnsafe(tag); len(pls) > 0 {
				effectiveTag = tag
				playlists = pls
				break
			}
		}
	}

	if len(playlists) == 0 {
		mp.mu.Unlock()
		return nil, nil, errors.New("no playlists available")
	}

	_ = effectiveTag

	if mp.activePlaylistIndex >= len(playlists) {
		mp.activePlaylistIndex = 0
	}

	pl := playlists[mp.activePlaylistIndex]
	mp.mu.Unlock()

	// Get next track from the playlist. If the playlist is exhausted (wrapped
	// around), move to the next playlist in the same tag.
	track, ok := pl.Next()
	if !ok {
		// Empty playlist – try the next one.
		mp.mu.Lock()
		mp.activePlaylistIndex = (mp.activePlaylistIndex + 1) % len(playlists)
		nextPl := playlists[mp.activePlaylistIndex]
		mp.mu.Unlock()

		track, ok = nextPl.Next()
		if !ok {
			return nil, nextPl, errors.New("all playlists are empty")
		}
		return track, nextPl, nil
	}

	return track, pl, nil
}

// AdvanceToNextPlaylist moves the active playlist index to the next playlist
// within the current tag. This is useful when the user wants to skip to a
// different playlist.
func (mp *MasterPlaylist) AdvanceToNextPlaylist() (*Playlist, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	pls := mp.getPlaylistsUnsafe(mp.activeTag)
	if len(pls) == 0 {
		return nil, fmt.Errorf("no playlists for tag %s", mp.activeTag)
	}

	mp.activePlaylistIndex = (mp.activePlaylistIndex + 1) % len(pls)
	return pls[mp.activePlaylistIndex], nil
}

// RemoveDeletedTracks walks every playlist in the master playlist and removes
// tracks whose files no longer exist on disk. Returns the number of tracks
// removed.
func (mp *MasterPlaylist) RemoveDeletedTracks() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	removed := 0
	for _, tag := range ValidTimeTags {
		for _, pl := range mp.getPlaylistsUnsafe(tag) {
			pl.mu.Lock()
			alive := make([]*Track, 0, len(pl.Tracks))
			for _, t := range pl.Tracks {
				if t.FileExists() {
					alive = append(alive, t)
				} else {
					removed++
				}
			}
			pl.Tracks = alive
			// Reset index if it's now out of bounds.
			if pl.currentIndex >= len(pl.Tracks) {
				pl.currentIndex = 0
			}
			pl.mu.Unlock()
		}
	}

	// Also remove stale tracks from the library itself.
	if mp.Library != nil {
		stale := mp.Library.RemoveStale()
		_ = stale // already counted above via playlist removal
	}

	return removed
}

// Summary returns a map of tag -> number of playlists for quick inspection.
func (mp *MasterPlaylist) Summary() map[TimeTag]int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	return map[TimeTag]int{
		TagMorning:   len(mp.Morning),
		TagAfternoon: len(mp.Afternoon),
		TagEvening:   len(mp.Evening),
		TagNight:     len(mp.Night),
	}
}

// activePlaylistUnsafe returns the active playlist without locking.
// The caller must hold mp.mu in at least read mode.
func (mp *MasterPlaylist) activePlaylistUnsafe() *Playlist {
	if pls := mp.getPlaylistsUnsafe(mp.activeTag); len(pls) > 0 {
		idx := mp.activePlaylistIndex
		if idx >= len(pls) {
			idx = 0
		}
		return pls[idx]
	}
	for _, tag := range ValidTimeTags {
		if pls := mp.getPlaylistsUnsafe(tag); len(pls) > 0 {
			return pls[0]
		}
	}
	return nil
}

// PeekQueue returns up to n upcoming tracks from the active playlist, starting
// from the track that is currently playing (identified by CurrentTrackChecksum).
// If nothing has played yet, it starts from the next-to-play position.
// Pass n <= 0 to return all tracks in the playlist.
func (mp *MasterPlaylist) PeekQueue(n int) ([]*Track, *Playlist) {
	mp.mu.RLock()
	pl := mp.activePlaylistUnsafe()
	mp.mu.RUnlock()

	if pl == nil {
		return nil, nil
	}

	pl.mu.RLock()
	defer pl.mu.RUnlock()

	total := len(pl.Tracks)
	if total == 0 {
		return nil, pl
	}

	if n <= 0 || n > total {
		n = total
	}

	// Default start is the next-to-play position.
	startIdx := pl.currentIndex

	// If a track is currently playing, include it as the first in the queue.
	if pl.CurrentTrackChecksum != "" {
		for i, t := range pl.Tracks {
			if t.Checksum == pl.CurrentTrackChecksum {
				startIdx = i
				break
			}
		}
	}

	result := make([]*Track, 0, n)
	for i := 0; i < n; i++ {
		result = append(result, pl.Tracks[(startIdx+i)%total])
	}
	return result, pl
}

// SeekPrev moves the active playlist's cursor back one position so that after
// the current track is cancelled and Next() is called again, it will return the
// track that preceded the one currently playing.
func (mp *MasterPlaylist) SeekPrev() error {
	mp.mu.RLock()
	pl := mp.activePlaylistUnsafe()
	mp.mu.RUnlock()

	if pl == nil {
		return errors.New("no active playlist")
	}

	pl.mu.Lock()
	defer pl.mu.Unlock()

	n := len(pl.Tracks)
	if n == 0 {
		return errors.New("active playlist is empty")
	}

	if pl.CurrentTrackChecksum == "" {
		// Nothing played yet; jump to the last track.
		pl.currentIndex = n - 1
		return nil
	}

	for i, t := range pl.Tracks {
		if t.Checksum == pl.CurrentTrackChecksum {
			// Set cursor to (i-1) so Next() returns the track before current.
			pl.currentIndex = (i - 1 + n) % n
			return nil
		}
	}

	// Currently-playing track no longer in playlist; step back one from cursor.
	pl.currentIndex = (pl.currentIndex - 2 + n*2) % n
	return nil
}

// TotalTracks returns the total number of tracks across all playlists (may
// include duplicates).
func (mp *MasterPlaylist) TotalTracks() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	total := 0
	for _, tag := range ValidTimeTags {
		for _, pl := range mp.getPlaylistsUnsafe(tag) {
			total += pl.Count()
		}
	}
	return total
}

// LibraryTrackCount returns the number of unique tracks in the library.
// Returns 0 if no library is set.
func (mp *MasterPlaylist) LibraryTrackCount() int {
	if mp.Library == nil {
		return 0
	}
	return mp.Library.Count()
}

// IsEmpty returns true if there are no playlists assigned to any tag.
func (mp *MasterPlaylist) IsEmpty() bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	for _, tag := range ValidTimeTags {
		if len(mp.getPlaylistsUnsafe(tag)) > 0 {
			return false
		}
	}
	return true
}
