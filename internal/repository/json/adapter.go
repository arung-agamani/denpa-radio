// Package json provides a repository adapter that persists data to JSON files
// using the internal/playlist Store and MasterPlaylist types.
package json

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/repository"
)

// JSONAdapter implements repository.TrackRepository,
// repository.PlaylistRepository, and repository.MasterPlaylistRepository
// by delegating to an in-memory *playlist.MasterPlaylist and persisting
// via *playlist.Store.
type JSONAdapter struct {
	store  *playlist.Store
	master *playlist.MasterPlaylist
	mu     sync.RWMutex
}

// Compile-time interface checks.
var (
	_ repository.TrackRepository           = (*JSONAdapter)(nil)
	_ repository.PlaylistRepository        = (*JSONAdapter)(nil)
	_ repository.MasterPlaylistRepository  = (*JSONAdapter)(nil)
)

// NewJSONAdapter creates a new JSONAdapter backed by the given Store.
// The adapter is not usable for data access until Load() is called.
func NewJSONAdapter(store *playlist.Store) *JSONAdapter {
	return &JSONAdapter{store: store}
}

// Master returns the underlying *playlist.MasterPlaylist. This is used by
// server.go to wire the concrete master into the Broadcaster and Scheduler.
func (a *JSONAdapter) Master() *playlist.MasterPlaylist {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.master
}

// Load reads the persisted JSON file and caches the resulting MasterPlaylist.
func (a *JSONAdapter) Load() error {
	master, err := a.store.Load()
	if err != nil {
		return fmt.Errorf("failed to load master playlist: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.master = master
	return nil
}

// Save persists the cached MasterPlaylist to disk atomically.
func (a *JSONAdapter) Save() error {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.master == nil {
		return errors.New("adapter not loaded")
	}

	if err := a.store.Save(a.master); err != nil {
		return fmt.Errorf("failed to save master playlist: %w", err)
	}
	return nil
}

// Exists returns true if the store file already exists on disk.
func (a *JSONAdapter) Exists() bool {
	return a.store.Exists()
}

// ---------------------------------------------------------------------------
// TrackRepository delegations
// ---------------------------------------------------------------------------

func (a *JSONAdapter) GetByID(id int64) *playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.GetByID(id)
}

func (a *JSONAdapter) Get(checksum string) *playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.Get(checksum)
}

func (a *JSONAdapter) GetByFilePath(filePath string) *playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.GetByFilePath(filePath)
}

func (a *JSONAdapter) List() []*playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.List()
}

func (a *JSONAdapter) Search(query string) []*playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.Search(query)
}

func (a *JSONAdapter) Add(t *playlist.Track) (*playlist.Track, bool) {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil, false
	}
	return lib.Add(t)
}

func (a *JSONAdapter) AddOrUpdate(t *playlist.Track) *playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.AddOrUpdate(t)
}

func (a *JSONAdapter) RemoveByID(id int64) *playlist.Track {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil
	}
	return lib.RemoveByID(id)
}

func (a *JSONAdapter) Update(id int64, upd playlist.TrackUpdate) (*playlist.Track, error) {
	lib := a.libraryOrNil()
	if lib == nil {
		return nil, errors.New("adapter not loaded")
	}
	return lib.Update(id, upd)
}

func (a *JSONAdapter) BatchUpdate(filter playlist.TrackFilter, upd playlist.TrackUpdate) (int, error) {
	lib := a.libraryOrNil()
	if lib == nil {
		return 0, errors.New("adapter not loaded")
	}
	return lib.BatchUpdate(filter, upd)
}

func (a *JSONAdapter) BatchSetCover(filter playlist.TrackFilter, coverPath string) (int, error) {
	lib := a.libraryOrNil()
	if lib == nil {
		return 0, errors.New("adapter not loaded")
	}
	return lib.BatchSetCover(filter, coverPath)
}

func (a *JSONAdapter) Count() int {
	lib := a.libraryOrNil()
	if lib == nil {
		return 0
	}
	return lib.Count()
}

// ---------------------------------------------------------------------------
// PlaylistRepository delegations
// ---------------------------------------------------------------------------

func (a *JSONAdapter) FindPlaylistByID(id int64) (*playlist.Playlist, playlist.TimeTag, error) {
	m := a.masterOrNil()
	if m == nil {
		return nil, "", errors.New("adapter not loaded")
	}
	return m.FindPlaylistByID(id)
}

func (a *JSONAdapter) AllPlaylists() []*playlist.Playlist {
	m := a.masterOrNil()
	if m == nil {
		return nil
	}
	return m.AllPlaylists()
}

func (a *JSONAdapter) AssignPlaylist(tag playlist.TimeTag, pl *playlist.Playlist) error {
	m := a.masterOrNil()
	if m == nil {
		return errors.New("adapter not loaded")
	}
	return m.AssignPlaylist(tag, pl)
}

func (a *JSONAdapter) RemovePlaylist(tag playlist.TimeTag, id int64) error {
	m := a.masterOrNil()
	if m == nil {
		return errors.New("adapter not loaded")
	}
	return m.RemovePlaylist(tag, id)
}

// ---------------------------------------------------------------------------
// MasterPlaylistRepository delegations
// ---------------------------------------------------------------------------

func (a *JSONAdapter) ActiveTag() playlist.TimeTag {
	m := a.masterOrNil()
	if m == nil {
		return ""
	}
	return m.ActiveTag()
}

func (a *JSONAdapter) ActivePlaylist() (*playlist.Playlist, error) {
	m := a.masterOrNil()
	if m == nil {
		return nil, errors.New("adapter not loaded")
	}
	return m.ActivePlaylist()
}

func (a *JSONAdapter) ResolveActiveTag() (playlist.TimeTag, bool) {
	m := a.masterOrNil()
	if m == nil {
		return "", false
	}
	return m.ResolveActiveTag()
}

func (a *JSONAdapter) SetActiveTag(tag playlist.TimeTag) {
	m := a.masterOrNil()
	if m == nil {
		return
	}
	m.SetActiveTag(tag)
}

func (a *JSONAdapter) ConfiguredTags() []playlist.TimeTag {
	m := a.masterOrNil()
	if m == nil {
		return nil
	}
	return m.ConfiguredTags()
}

func (a *JSONAdapter) IsConfiguredTag(tag playlist.TimeTag) bool {
	m := a.masterOrNil()
	if m == nil {
		return false
	}
	return m.IsConfiguredTag(tag)
}

func (a *JSONAdapter) GetPlaylists(tag playlist.TimeTag) []*playlist.Playlist {
	m := a.masterOrNil()
	if m == nil {
		return nil
	}
	return m.GetPlaylists(tag)
}

func (a *JSONAdapter) GetTimeSlots() []playlist.TimeSlot {
	m := a.masterOrNil()
	if m == nil {
		return nil
	}
	return m.GetTimeSlots()
}

func (a *JSONAdapter) SetTimeSlots(slots []playlist.TimeSlot) error {
	m := a.masterOrNil()
	if m == nil {
		return errors.New("adapter not loaded")
	}
	return m.SetTimeSlots(slots)
}

func (a *JSONAdapter) Timezone() string {
	m := a.masterOrNil()
	if m == nil {
		return ""
	}
	return m.Timezone()
}

func (a *JSONAdapter) SetTimezone(tz string) error {
	m := a.masterOrNil()
	if m == nil {
		return errors.New("adapter not loaded")
	}
	return m.SetTimezone(tz)
}

func (a *JSONAdapter) Location() *time.Location {
	m := a.masterOrNil()
	if m == nil {
		return time.UTC
	}
	return m.Location()
}

func (a *JSONAdapter) TotalTracks() int {
	m := a.masterOrNil()
	if m == nil {
		return 0
	}
	return m.TotalTracks()
}

func (a *JSONAdapter) LibraryTrackCount() int {
	m := a.masterOrNil()
	if m == nil {
		return 0
	}
	return m.LibraryTrackCount()
}

func (a *JSONAdapter) AllTracksDeduped() []*playlist.Track {
	m := a.masterOrNil()
	if m == nil {
		return nil
	}
	return m.AllTracksDeduped()
}

func (a *JSONAdapter) Summary() map[playlist.TimeTag]int {
	m := a.masterOrNil()
	if m == nil {
		return map[playlist.TimeTag]int{}
	}
	return m.Summary()
}

func (a *JSONAdapter) PeekQueue(n int) ([]*playlist.Track, *playlist.Playlist) {
	m := a.masterOrNil()
	if m == nil {
		return nil, nil
	}
	return m.PeekQueue(n)
}

func (a *JSONAdapter) Next() (*playlist.Track, *playlist.Playlist, error) {
	m := a.masterOrNil()
	if m == nil {
		return nil, nil, errors.New("adapter not loaded")
	}
	return m.Next()
}

func (a *JSONAdapter) SeekPrev() error {
	m := a.masterOrNil()
	if m == nil {
		return errors.New("adapter not loaded")
	}
	return m.SeekPrev()
}

func (a *JSONAdapter) Library() *playlist.TrackLibrary {
	m := a.masterOrNil()
	if m == nil {
		return nil
	}
	return m.Library
}

func (a *JSONAdapter) RemoveTrackFromAll(checksum string) int {
	m := a.masterOrNil()
	if m == nil {
		return 0
	}
	return m.RemoveTrackFromAll(checksum)
}

func (a *JSONAdapter) TimeTagForHourConfigured(hour int) playlist.TimeTag {
	m := a.masterOrNil()
	if m == nil {
		return ""
	}
	return m.TimeTagForHourConfigured(hour)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (a *JSONAdapter) masterOrNil() *playlist.MasterPlaylist {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.master
}

func (a *JSONAdapter) libraryOrNil() *playlist.TrackLibrary {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.master == nil {
		return nil
	}
	return a.master.Library
}
