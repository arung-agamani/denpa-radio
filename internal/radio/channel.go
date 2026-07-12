package radio

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/channel"
)

// Channel represents a single broadcast channel with its own master playlist,
// broadcaster, and scheduler. All channels share the same TrackLibrary.
type Channel struct {
	ID          string
	Slug        string
	Name        string
	Description string
	SortOrder   int
	Enabled     bool
	Bitrate     string

	Master      *playlist.MasterPlaylist
	Broadcaster *Broadcaster
	Scheduler   *playlist.Scheduler

	cancel context.CancelFunc
}

// ChannelUpdate holds optional fields for updating a channel.
// Nil pointers mean "do not change this field".
type ChannelUpdate struct {
	Name        *string
	Description *string
	SortOrder   *int
	Enabled     *bool
	Bitrate     *string
}

// ChannelManager owns all channels, enforces limits, and coordinates
// lifecycle (start/stop) and persistence.
type ChannelManager struct {
	mu          sync.RWMutex
	channels    map[string]*Channel
	defaultSlug string
	store       *playlist.Store
	encoder     *ffmpeg.Encoder
	library     *playlist.TrackLibrary
	maxChannels int
}

// NewChannelManager creates a manager. All arguments must be non-nil.
func NewChannelManager(store *playlist.Store, encoder *ffmpeg.Encoder, library *playlist.TrackLibrary, maxChannels int) (*ChannelManager, error) {
	if store == nil {
		return nil, apierror.ErrValidation("store is required")
	}
	if encoder == nil {
		return nil, apierror.ErrValidation("encoder is required")
	}
	if library == nil {
		library = playlist.NewTrackLibrary()
	}
	if maxChannels < 1 {
		maxChannels = 1
	}

	return &ChannelManager{
		channels:    make(map[string]*Channel),
		store:       store,
		encoder:     encoder,
		library:     library,
		maxChannels: maxChannels,
	}, nil
}

// Load reads v4 data from the store and reconstructs channels.
// If no v4 file exists or it contains no channels, a default "main" channel
// is created with an empty master playlist sharing the manager's library.
func (cm *ChannelManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	lib, defaultSlug, snapshots, err := cm.store.LoadChannels()
	if err != nil || len(snapshots) == 0 {
		master := playlist.NewMasterPlaylistWithLibrary(cm.library)
		ch := cm.createChannel("main", "Main Channel", master)
		cm.channels["main"] = ch
		cm.defaultSlug = "main"
		if err != nil {
			slog.Warn("failed to load channels, creating default", "error", err)
		}
		return nil
	}

	cm.library = lib

	for _, snap := range snapshots {
		master := playlist.NewMasterPlaylistWithLibrary(lib)
		if snap.Timezone != "" {
			if err := master.SetTimezone(snap.Timezone); err != nil {
				slog.Warn("ignoring invalid persisted timezone", "timezone", snap.Timezone, "error", err)
			}
		}
		if len(snap.TimeSlots) > 0 {
			if err := master.SetTimeSlots(snap.TimeSlots); err != nil {
				slog.Warn("ignoring invalid persisted time slots, using defaults", "error", err)
			}
		}
		for tag, pls := range snap.Playlists {
			for _, pl := range pls {
				_ = master.AssignPlaylist(tag, pl)
			}
		}

		ch := cm.createChannel(snap.Slug, snap.Name, master)
		ch.ID = snap.ID
		ch.Description = snap.Description
		ch.SortOrder = snap.SortOrder
		ch.Enabled = snap.Enabled
		ch.Bitrate = snap.Bitrate
		cm.channels[snap.Slug] = ch
	}

	if defaultSlug != "" {
		cm.defaultSlug = defaultSlug
	} else if len(snapshots) > 0 {
		cm.defaultSlug = snapshots[0].Slug
	} else {
		cm.defaultSlug = "main"
	}

	return nil
}

func (cm *ChannelManager) createChannel(slug, name string, master *playlist.MasterPlaylist) *Channel {
	b := NewBroadcaster(nil, cm.encoder)
	b.SetMasterPlaylist(master)

	scheduler := playlist.NewScheduler(master, nil, 1*time.Minute)

	return &Channel{
		ID:          slug,
		Slug:        slug,
		Name:        name,
		Description: "",
		SortOrder:   0,
		Enabled:     true,
		Bitrate:     "",
		Master:      master,
		Broadcaster: b,
		Scheduler:   scheduler,
	}
}

// Get returns the channel with the given slug, or nil if not found.
func (cm *ChannelManager) Get(slug string) *Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.channels[slug]
}

// List returns all channels sorted by SortOrder ascending, then by Slug.
func (cm *ChannelManager) List() []*Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make([]*Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		result = append(result, ch)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].SortOrder != result[j].SortOrder {
			return result[i].SortOrder < result[j].SortOrder
		}
		return result[i].Slug < result[j].Slug
	})

	return result
}

// DefaultChannel returns the current default channel.
func (cm *ChannelManager) DefaultChannel() channel.Channel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.channels[cm.defaultSlug]
}

// Add creates a new channel with an empty master playlist sharing the library.
// It enforces the maxChannels limit and returns an error if the slug already exists.
func (cm *ChannelManager) Add(name, slug string) (*Channel, error) {
	if slug == "" {
		return nil, apierror.ErrValidation("slug is required")
	}
	if name == "" {
		return nil, apierror.ErrValidation("name is required")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.channels[slug]; exists {
		return nil, apierror.ErrConflict(fmt.Sprintf("channel with slug %q already exists", slug))
	}

	if len(cm.channels) >= cm.maxChannels {
		return nil, apierror.ErrValidation(fmt.Sprintf("maximum number of channels (%d) reached", cm.maxChannels))
	}

	master := playlist.NewMasterPlaylistWithLibrary(cm.library)
	ch := cm.createChannel(slug, name, master)
	cm.channels[slug] = ch

	return ch, nil
}

// Remove deletes a channel. The default channel cannot be removed.
// The channel's broadcaster and scheduler are stopped before removal.
func (cm *ChannelManager) Remove(slug string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if slug == cm.defaultSlug {
		return apierror.ErrForbidden("cannot remove the default channel")
	}

	ch, exists := cm.channels[slug]
	if !exists {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}

	if ch.cancel != nil {
		ch.cancel()
	}

	delete(cm.channels, slug)
	return nil
}

// Update applies partial updates to a channel. Nil fields in the update are ignored.
func (cm *ChannelManager) Update(slug string, updates ChannelUpdate) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	ch, exists := cm.channels[slug]
	if !exists {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}

	if updates.Name != nil {
		ch.Name = *updates.Name
	}
	if updates.Description != nil {
		ch.Description = *updates.Description
	}
	if updates.SortOrder != nil {
		ch.SortOrder = *updates.SortOrder
	}
	if updates.Enabled != nil {
		ch.Enabled = *updates.Enabled
	}
	if updates.Bitrate != nil {
		ch.Bitrate = *updates.Bitrate
	}

	return nil
}

// StartAll starts every channel's broadcaster and scheduler in background goroutines.
// It returns immediately; the caller should later call StopAll or cancel ctx.
func (cm *ChannelManager) StartAll(ctx context.Context) {
	cm.mu.RLock()
	channels := make([]*Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	cm.mu.RUnlock()

	for _, ch := range channels {
		chCtx, cancel := context.WithCancel(ctx)
		ch.cancel = cancel

		go ch.Broadcaster.Start(chCtx)
		go ch.Scheduler.Start(chCtx)
	}
}

// StopAll stops every channel's broadcaster and scheduler.
func (cm *ChannelManager) StopAll() {
	cm.mu.RLock()
	channels := make([]*Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	cm.mu.RUnlock()

	for _, ch := range channels {
		if ch.cancel != nil {
			ch.cancel()
		}
	}
}

// SaveAll persists all channels to v4 format via the store.
func (cm *ChannelManager) SaveAll() error {
	cm.mu.RLock()
	channels := make([]*Channel, 0, len(cm.channels))
	for _, ch := range cm.channels {
		channels = append(channels, ch)
	}
	defaultSlug := cm.defaultSlug
	cm.mu.RUnlock()

	snapshots := make([]*playlist.ChannelSnapshot, 0, len(channels))
	for _, ch := range channels {
		snap := &playlist.ChannelSnapshot{
			ID:          ch.ID,
			Slug:        ch.Slug,
			Name:        ch.Name,
			Description: ch.Description,
			SortOrder:   ch.SortOrder,
			Enabled:     ch.Enabled,
			Bitrate:     ch.Bitrate,
			Timezone:    ch.Master.Timezone(),
			TimeSlots:   ch.Master.GetTimeSlots(),
			Playlists:   make(map[playlist.TimeTag][]*playlist.Playlist),
		}
		for _, tag := range ch.Master.ConfiguredTags() {
			snap.Playlists[tag] = ch.Master.GetPlaylists(tag)
		}
		snapshots = append(snapshots, snap)
	}

	return cm.store.SaveChannels(cm.library, defaultSlug, snapshots)
}

// SetDefault changes the default channel slug.
func (cm *ChannelManager) SetDefault(slug string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, exists := cm.channels[slug]; !exists {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}

	cm.defaultSlug = slug
	return nil
}

// ChannelBySlug satisfies channel.ChannelManager.
func (cm *ChannelManager) ChannelBySlug(slug string) channel.Channel {
	return cm.Get(slug)
}

// ListChannels satisfies channel.ChannelManager.
func (cm *ChannelManager) ListChannels() []channel.Channel {
	channels := cm.List()
	result := make([]channel.Channel, 0, len(channels))
	for _, ch := range channels {
		result = append(result, ch)
	}
	return result
}

// AddChannel satisfies channel.ChannelManager.
func (cm *ChannelManager) AddChannel(name, slug string) (channel.Channel, error) {
	return cm.Add(name, slug)
}

// RemoveChannel satisfies channel.ChannelManager.
func (cm *ChannelManager) RemoveChannel(slug string) error {
	return cm.Remove(slug)
}

// UpdateChannel satisfies channel.ChannelManager.
func (cm *ChannelManager) UpdateChannel(slug string, updates channel.ChannelUpdate) error {
	return cm.Update(slug, ChannelUpdate{
		Name:        updates.Name,
		Description: updates.Description,
		SortOrder:   updates.SortOrder,
		Enabled:     updates.Enabled,
		Bitrate:     updates.Bitrate,
	})
}

// GetMaster satisfies channel.Channel.
func (ch *Channel) GetMaster() *playlist.MasterPlaylist {
	return ch.Master
}

// GetBroadcaster satisfies channel.Channel.
func (ch *Channel) GetBroadcaster() channel.Broadcaster {
	return ch.Broadcaster
}

// GetScheduler satisfies channel.Channel.
func (ch *Channel) GetScheduler() *playlist.Scheduler {
	return ch.Scheduler
}

// Save satisfies channel.Channel. It is a no-op because persistence is
// handled by ChannelManager.SaveAll.
func (ch *Channel) Save() error {
	return nil
}

// GetSlug satisfies channel.Channel.
func (ch *Channel) GetSlug() string {
	return ch.Slug
}

// GetName satisfies channel.Channel.
func (ch *Channel) GetName() string {
	return ch.Name
}

// GetDescription satisfies channel.Channel.
func (ch *Channel) GetDescription() string {
	return ch.Description
}

// GetSortOrder satisfies channel.Channel.
func (ch *Channel) GetSortOrder() int {
	return ch.SortOrder
}

// GetEnabled satisfies channel.Channel.
func (ch *Channel) GetEnabled() bool {
	return ch.Enabled
}

// GetBitrate satisfies channel.Channel.
func (ch *Channel) GetBitrate() string {
	return ch.Bitrate
}
