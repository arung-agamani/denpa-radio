// Package repository defines repository interfaces that abstract over the
// internal/playlist domain types. Implementations will be provided in later
// waves of the refactor.
package repository

import (
	"time"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// TrackRepository abstracts access to the track library.
type TrackRepository interface {
	GetByID(id int64) *playlist.Track
	Get(checksum string) *playlist.Track
	GetByFilePath(filePath string) *playlist.Track
	List() []*playlist.Track
	Search(query string) []*playlist.Track
	Add(t *playlist.Track) (*playlist.Track, bool)
	AddOrUpdate(t *playlist.Track) *playlist.Track
	RemoveByID(id int64) *playlist.Track
	Update(id int64, upd playlist.TrackUpdate) (*playlist.Track, error)
	BatchUpdate(filter playlist.TrackFilter, upd playlist.TrackUpdate) (int, error)
	BatchSetCover(filter playlist.TrackFilter, coverPath string) (int, error)
	Count() int
}

// PlaylistRepository abstracts playlist management within the master playlist.
type PlaylistRepository interface {
	FindPlaylistByID(id int64) (*playlist.Playlist, playlist.TimeTag, error)
	AllPlaylists() []*playlist.Playlist
	AssignPlaylist(tag playlist.TimeTag, pl *playlist.Playlist) error
	RemovePlaylist(tag playlist.TimeTag, id int64) error
}

// MasterPlaylistRepository abstracts the master playlist and its persistence.
// It embeds PlaylistRepository so that callers working with the master have
// access to playlist management methods as well.
type MasterPlaylistRepository interface {
	PlaylistRepository
	MasterPlaylist() *playlist.MasterPlaylist
	Load() error
	Save() error
	Exists() bool
	ActiveTag() playlist.TimeTag
	ActivePlaylist() (*playlist.Playlist, error)
	ResolveActiveTag() (playlist.TimeTag, bool)
	SetActiveTag(tag playlist.TimeTag)
	ConfiguredTags() []playlist.TimeTag
	IsConfiguredTag(tag playlist.TimeTag) bool
	GetPlaylists(tag playlist.TimeTag) []*playlist.Playlist
	GetTimeSlots() []playlist.TimeSlot
	SetTimeSlots(slots []playlist.TimeSlot) error
	Timezone() string
	SetTimezone(tz string) error
	Location() *time.Location
	TotalTracks() int
	LibraryTrackCount() int
	AllTracksDeduped() []*playlist.Track
	Summary() map[playlist.TimeTag]int
	PeekQueue(n int) ([]*playlist.Track, *playlist.Playlist)
	Next() (*playlist.Track, *playlist.Playlist, error)
	SeekPrev() error
	Library() *playlist.TrackLibrary
	RemoveTrackFromAll(checksum string) int
	TimeTagForHourConfigured(hour int) playlist.TimeTag
}
