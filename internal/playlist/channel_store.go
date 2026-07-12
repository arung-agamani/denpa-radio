package playlist

// StoreDataV4 is the v4 on-disk format supporting multiple channels with a
// shared track library.
type StoreDataV4 struct {
	Version        int               `json:"version"`
	DefaultChannel string            `json:"defaultChannel"`
	Channels       []*StoreChannelV4 `json:"channels"`
	Library        *TrackLibrary     `json:"library"`
}

// StoreChannelV4 is the per-channel representation in the v4 format.
type StoreChannelV4 struct {
	ID          string                        `json:"id"`
	Slug        string                        `json:"slug"`
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	SortOrder   int                           `json:"sortOrder"`
	Enabled     bool                          `json:"enabled"`
	Bitrate     string                        `json:"bitrate"`
	Timezone    string                        `json:"timezone"`
	TimeSlots   []TimeSlot                    `json:"timeSlots"`
	Playlists   map[string][]*StorePlaylistV2 `json:"playlists"`
}

// ChannelSnapshot holds the runtime representation of a loaded channel.
// It contains enough data for ChannelManager to reconstruct each channel's
// MasterPlaylist while sharing the same TrackLibrary.
type ChannelSnapshot struct {
	ID          string
	Slug        string
	Name        string
	Description string
	SortOrder   int
	Enabled     bool
	Bitrate     string
	Timezone    string
	TimeSlots   []TimeSlot
	Playlists   map[TimeTag][]*Playlist
}
