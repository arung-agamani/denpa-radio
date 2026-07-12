package service

import (
	"fmt"
	"regexp"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/channel"
)

var slugRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ChannelSummary is the lightweight representation of a channel.
type ChannelSummary struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
	Enabled     bool   `json:"enabled"`
	Bitrate     string `json:"bitrate"`
}

// ChannelDetail extends ChannelSummary with runtime state.
type ChannelDetail struct {
	ChannelSummary
	ActiveTag        playlist.TimeTag `json:"activeTag"`
	ActivePlaylist   string           `json:"activePlaylist"`
	TotalTracks      int              `json:"totalTracks"`
	SchedulerRunning bool             `json:"schedulerRunning"`
}

// ChannelStatus holds the live broadcast state for a channel.
type ChannelStatus struct {
	CurrentTrack   *playlist.Track  `json:"-"`
	ActiveTag      playlist.TimeTag `json:"activeTag"`
	ActivePlaylist string           `json:"activePlaylist"`
	ClientCount    int              `json:"clientCount"`
	MaxClients     int              `json:"maxClients"`
	Timezone       string           `json:"timezone"`
	ServerTime     string           `json:"serverTime"`
}

// CreateChannelBody is the payload for creating a channel.
type CreateChannelBody struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
	Bitrate     *string `json:"bitrate,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`
}

// UpdateChannelBody is the payload for updating a channel.
type UpdateChannelBody struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	SortOrder   *int    `json:"sortOrder,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Bitrate     *string `json:"bitrate,omitempty"`
}

// TimeSlotResult is returned after updating time slots.
type TimeSlotResult struct {
	TimeSlots []playlist.TimeSlot `json:"timeSlots"`
}

// ChannelService implements business logic for channel CRUD and runtime queries.
type ChannelService struct {
	cm  channel.ChannelManager
	cfg *config.Config
}

// NewChannelService creates a new ChannelService.
func NewChannelService(cm channel.ChannelManager, cfg *config.Config) *ChannelService {
	return &ChannelService{cm: cm, cfg: cfg}
}

func (s *ChannelService) validateSlug(slug string) error {
	if slug == "" {
		return apierror.ErrValidation("slug is required")
	}
	if !slugRe.MatchString(slug) {
		return apierror.ErrValidation("slug must contain only alphanumeric characters, hyphens, and underscores")
	}
	return nil
}

func toChannelSummary(ch channel.Channel) ChannelSummary {
	return ChannelSummary{
		ID:          ch.GetSlug(),
		Slug:        ch.GetSlug(),
		Name:        "", // filled by caller if available
		Description: "",
		SortOrder:   0,
		Enabled:     true,
		Bitrate:     "",
	}
}

// List returns all channels sorted by SortOrder.
func (s *ChannelService) List() []*ChannelSummary {
	channels := s.cm.ListChannels()
	result := make([]*ChannelSummary, 0, len(channels))
	for _, ch := range channels {
		summary := ChannelSummary{
			ID:          ch.GetSlug(),
			Slug:        ch.GetSlug(),
			Name:        ch.GetName(),
			Description: ch.GetDescription(),
			SortOrder:   ch.GetSortOrder(),
			Enabled:     ch.GetEnabled(),
			Bitrate:     ch.GetBitrate(),
		}
		result = append(result, &summary)
	}
	return result
}

// Get returns detail for a single channel.
func (s *ChannelService) Get(slug string) (*ChannelDetail, error) {
	if err := s.validateSlug(slug); err != nil {
		return nil, err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}

	master := ch.GetMaster()
	activeTag := master.ActiveTag()
	activePl, _ := master.ActivePlaylist()
	var activePlaylistName string
	if activePl != nil {
		activePlaylistName = activePl.Name
	}

	return &ChannelDetail{
		ChannelSummary:   toChannelSummary(ch),
		ActiveTag:        activeTag,
		ActivePlaylist:   activePlaylistName,
		TotalTracks:      master.TotalTracks(),
		SchedulerRunning: ch.GetScheduler().Running(),
	}, nil
}

// GetStatus returns the live broadcast status for a channel.
func (s *ChannelService) GetStatus(slug string) (*ChannelStatus, error) {
	if err := s.validateSlug(slug); err != nil {
		return nil, err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}

	master := ch.GetMaster()
	broadcaster := ch.GetBroadcaster()

	currentTrackPath := broadcaster.CurrentTrack()
	var currentTrackRaw *playlist.Track
	if currentTrackPath != "" {
		lib := master.Library
		if lib != nil {
			currentTrackRaw = lib.GetByFilePath(currentTrackPath)
		}
		if currentTrackRaw == nil {
			for _, pl := range master.AllPlaylists() {
				if t, _, err := pl.FindTrackByFilePath(currentTrackPath); err == nil {
					currentTrackRaw = t
					break
				}
			}
		}
	}

	activeTag := master.ActiveTag()
	activePl, _ := master.ActivePlaylist()
	var activePlaylistName string
	if activePl != nil {
		activePlaylistName = activePl.Name
	}

	tz := master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	return &ChannelStatus{
		CurrentTrack:   currentTrackRaw,
		ActiveTag:      activeTag,
		ActivePlaylist: activePlaylistName,
		ClientCount:    broadcaster.ActiveClients(),
		MaxClients:     s.cfg.MaxClients,
		Timezone:       tz,
		ServerTime:     time.Now().In(master.Location()).Format(time.RFC3339),
	}, nil
}

// GetQueue returns upcoming tracks for a channel.
func (s *ChannelService) GetQueue(slug string, n int) ([]*playlist.Track, error) {
	if err := s.validateSlug(slug); err != nil {
		return nil, err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	tracks, _ := ch.GetMaster().PeekQueue(n)
	return tracks, nil
}

// Create creates a new channel.
func (s *ChannelService) Create(body CreateChannelBody) (*ChannelSummary, error) {
	if err := s.validateSlug(body.Slug); err != nil {
		return nil, err
	}
	if body.Name == "" {
		return nil, apierror.ErrValidation("name is required")
	}

	ch, err := s.cm.AddChannel(body.Name, body.Slug)
	if err != nil {
		return nil, err
	}

	upd := channel.ChannelUpdate{}
	if body.Description != nil {
		upd.Description = body.Description
	}
	if body.Bitrate != nil {
		upd.Bitrate = body.Bitrate
	}
	if body.SortOrder != nil {
		upd.SortOrder = body.SortOrder
	}
	if upd.Description != nil || upd.Bitrate != nil || upd.SortOrder != nil {
		if err := s.cm.UpdateChannel(body.Slug, upd); err != nil {
			return nil, err
		}
	}

	if err := s.cm.SaveAll(); err != nil {
		return nil, fmt.Errorf("failed to save channels: %w", err)
	}

	summary := ChannelSummary{
		ID:          ch.GetSlug(),
		Slug:        ch.GetSlug(),
		Name:        ch.GetName(),
		Description: ch.GetDescription(),
		SortOrder:   ch.GetSortOrder(),
		Enabled:     ch.GetEnabled(),
		Bitrate:     ch.GetBitrate(),
	}
	return &summary, nil
}

// Update updates a channel's metadata.
func (s *ChannelService) Update(slug string, body UpdateChannelBody) (*ChannelSummary, error) {
	if err := s.validateSlug(slug); err != nil {
		return nil, err
	}

	upd := channel.ChannelUpdate{
		Name:        body.Name,
		Description: body.Description,
		SortOrder:   body.SortOrder,
		Enabled:     body.Enabled,
		Bitrate:     body.Bitrate,
	}
	if err := s.cm.UpdateChannel(slug, upd); err != nil {
		return nil, err
	}

	if err := s.cm.SaveAll(); err != nil {
		return nil, fmt.Errorf("failed to save channels: %w", err)
	}

	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	summary := ChannelSummary{
		ID:          ch.GetSlug(),
		Slug:        ch.GetSlug(),
		Name:        ch.GetName(),
		Description: ch.GetDescription(),
		SortOrder:   ch.GetSortOrder(),
		Enabled:     ch.GetEnabled(),
		Bitrate:     ch.GetBitrate(),
	}
	return &summary, nil
}

// Delete removes a channel. The default channel cannot be deleted.
func (s *ChannelService) Delete(slug string) error {
	if err := s.validateSlug(slug); err != nil {
		return err
	}
	if err := s.cm.RemoveChannel(slug); err != nil {
		return err
	}
	if err := s.cm.SaveAll(); err != nil {
		return fmt.Errorf("failed to save channels: %w", err)
	}
	return nil
}

// AssignPlaylistToTag assigns a playlist to a time tag on a channel.
func (s *ChannelService) AssignPlaylistToTag(slug, tag string, playlistID int64) error {
	if err := s.validateSlug(slug); err != nil {
		return err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	master := ch.GetMaster()
	if !master.IsConfiguredTag(playlist.TimeTag(tag)) {
		return apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tag))
	}

	pl, currentTag, err := master.FindPlaylistByID(playlistID)
	if err != nil {
		return err
	}
	if currentTag != playlist.TimeTag(tag) {
		if removeErr := master.RemovePlaylist(currentTag, playlistID); removeErr != nil {
			// Log but don't fail; the playlist might not be assigned yet.
		}
	}
	if err := master.AssignPlaylist(playlist.TimeTag(tag), pl); err != nil {
		return err
	}
	if err := s.cm.SaveAll(); err != nil {
		return fmt.Errorf("failed to save channels: %w", err)
	}
	return nil
}

// RemovePlaylistFromTag removes a playlist from a time tag on a channel.
func (s *ChannelService) RemovePlaylistFromTag(slug, tag string, playlistID int64) error {
	if err := s.validateSlug(slug); err != nil {
		return err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	master := ch.GetMaster()
	if !master.IsConfiguredTag(playlist.TimeTag(tag)) {
		return apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tag))
	}
	if err := master.RemovePlaylist(playlist.TimeTag(tag), playlistID); err != nil {
		return err
	}
	if err := s.cm.SaveAll(); err != nil {
		return fmt.Errorf("failed to save channels: %w", err)
	}
	return nil
}

// SetTimeSlots replaces all time slots for a channel.
func (s *ChannelService) SetTimeSlots(slug string, slots []playlist.TimeSlot) (*TimeSlotResult, error) {
	if err := s.validateSlug(slug); err != nil {
		return nil, err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	if err := ch.GetMaster().SetTimeSlots(slots); err != nil {
		return nil, err
	}
	if err := s.cm.SaveAll(); err != nil {
		return nil, fmt.Errorf("failed to save channels: %w", err)
	}
	ch.GetScheduler().ForceCheck()
	return &TimeSlotResult{TimeSlots: ch.GetMaster().GetTimeSlots()}, nil
}

// SkipNext skips to the next track on a channel.
func (s *ChannelService) SkipNext(slug string) error {
	if err := s.validateSlug(slug); err != nil {
		return err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	ch.GetBroadcaster().Skip()
	return nil
}

// SkipPrev seeks back one track and restarts playback on a channel.
func (s *ChannelService) SkipPrev(slug string) error {
	if err := s.validateSlug(slug); err != nil {
		return err
	}
	ch := s.cm.ChannelBySlug(slug)
	if ch == nil {
		return apierror.ErrNotFound(fmt.Sprintf("channel %q not found", slug))
	}
	if err := ch.GetMaster().SeekPrev(); err != nil {
		return err
	}
	ch.GetBroadcaster().Skip()
	return nil
}
