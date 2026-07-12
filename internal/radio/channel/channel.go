// Package channel defines the shared interfaces for radio channels and
// channel managers. It lives between the radio and service packages to break
// import cycles.
package channel

import (
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// Broadcaster is the minimal interface the service layer needs from a
// stream broadcaster.
type Broadcaster interface {
	CurrentTrack() string
	ActiveClients() int
	Skip()
}

// Channel exposes a channel's master playlist, broadcaster, and scheduler.
type Channel interface {
	GetMaster() *playlist.MasterPlaylist
	GetBroadcaster() Broadcaster
	GetScheduler() *playlist.Scheduler
	Save() error
	GetSlug() string
	GetName() string
	GetDescription() string
	GetSortOrder() int
	GetEnabled() bool
	GetBitrate() string
}

// ChannelManager resolves channels by slug and manages their lifecycle.
type ChannelManager interface {
	ChannelBySlug(slug string) Channel
	DefaultChannel() Channel
	ListChannels() []Channel
	AddChannel(name, slug string) (Channel, error)
	RemoveChannel(slug string) error
	UpdateChannel(slug string, updates ChannelUpdate) error
	SaveAll() error
}

// ChannelUpdate holds optional fields for updating a channel.
type ChannelUpdate struct {
	Name        *string
	Description *string
	SortOrder   *int
	Enabled     *bool
	Bitrate     *string
}
