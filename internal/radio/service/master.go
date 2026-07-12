package service

import (
	"fmt"
	"log/slog"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/channel"
	"github.com/arung-agamani/denpa-radio/internal/repository"
)

// MasterTagInfo describes the playlists assigned to a single time tag.
type MasterTagInfo struct {
	Playlists []*playlist.Playlist `json:"playlists"`
	Count     int                  `json:"count"`
}

// MasterSnapshot is the full representation of the master playlist.
type MasterSnapshot struct {
	ActiveTag        playlist.TimeTag
	ActivePlaylistID *int64
	TotalTracks      int
	Tags             map[string]MasterTagInfo
	TimeSlots        []playlist.TimeSlot
}

// MasterService implements the business logic for master playlist and
// time-tag assignment operations.
type MasterService struct {
	master         repository.MasterPlaylistRepository
	scheduler      *playlist.Scheduler
	channelManager channel.ChannelManager
}

func NewMasterService(master repository.MasterPlaylistRepository, scheduler *playlist.Scheduler, channelManager channel.ChannelManager) *MasterService {
	return &MasterService{master: master, scheduler: scheduler, channelManager: channelManager}
}

func (s *MasterService) forceCheck(channelSlug string) {
	if s.scheduler != nil {
		s.scheduler.ForceCheck()
		return
	}
	if s.channelManager == nil {
		return
	}
	var ch channel.Channel
	if channelSlug == "" {
		ch = s.channelManager.DefaultChannel()
	} else {
		ch = s.channelManager.ChannelBySlug(channelSlug)
	}
	if ch != nil {
		ch.GetScheduler().ForceCheck()
	}
}

// resolveMaster returns the *playlist.MasterPlaylist for the given slug.
// An empty slug resolves to the legacy default master.
func (s *MasterService) resolveMaster(channelSlug string) (*playlist.MasterPlaylist, error) {
	if channelSlug == "" {
		return s.master.MasterPlaylist(), nil
	}
	if s.channelManager == nil {
		return nil, apierror.ErrInternal("channel manager not configured")
	}
	ch := s.channelManager.ChannelBySlug(channelSlug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", channelSlug))
	}
	return ch.GetMaster(), nil
}

// Get returns a snapshot of the full master playlist structure for the requested channel.
func (s *MasterService) Get(channelSlug string) (MasterSnapshot, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return MasterSnapshot{}, err
	}

	tags := make(map[string]MasterTagInfo)
	for _, tag := range master.ConfiguredTags() {
		pls := master.GetPlaylists(tag)
		tags[string(tag)] = MasterTagInfo{Playlists: pls, Count: len(pls)}
	}
	activeTag := master.ActiveTag()
	activePl, _ := master.ActivePlaylist()
	var activePlaylistID *int64
	if activePl != nil {
		activePlaylistID = &activePl.ID
	}
	return MasterSnapshot{
		ActiveTag:        activeTag,
		ActivePlaylistID: activePlaylistID,
		TotalTracks:      master.TotalTracks(),
		Tags:             tags,
		TimeSlots:        master.GetTimeSlots(),
	}, nil
}

// AssignPlaylistToTag moves or assigns a playlist to a specific time tag on the requested channel.
func (s *MasterService) AssignPlaylistToTag(channelSlug string, playlistID int64, tagStr string) error {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return err
	}

	if !master.IsConfiguredTag(playlist.TimeTag(tagStr)) {
		return apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tagStr))
	}
	tag := playlist.TimeTag(tagStr)
	pl, currentTag, err := master.FindPlaylistByID(playlistID)
	if err != nil {
		return err
	}
	if currentTag != tag {
		if removeErr := master.RemovePlaylist(currentTag, playlistID); removeErr != nil {
			slog.Warn("Failed to remove playlist from old tag during reassignment",
				"error", removeErr)
		}
	}
	if err := master.AssignPlaylist(tag, pl); err != nil {
		return err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	s.forceCheck(channelSlug)
	return nil
}

// RemovePlaylistFromTag removes a playlist from a specific time tag on the requested channel.
func (s *MasterService) RemovePlaylistFromTag(channelSlug string, tagStr string, playlistID int64) error {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return err
	}

	if !master.IsConfiguredTag(playlist.TimeTag(tagStr)) {
		return apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tagStr))
	}
	tag := playlist.TimeTag(tagStr)
	if err := master.RemovePlaylist(tag, playlistID); err != nil {
		return err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return nil
}

// GetTimeSlots returns the currently configured time slots for the requested channel.
func (s *MasterService) GetTimeSlots(channelSlug string) ([]playlist.TimeSlot, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, err
	}
	return master.GetTimeSlots(), nil
}

// SetTimeSlots replaces all time slots after validation for the requested channel.
// Saves and forces a scheduler re-check.
func (s *MasterService) SetTimeSlots(channelSlug string, slots []playlist.TimeSlot) error {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return err
	}

	if err := master.SetTimeSlots(slots); err != nil {
		return err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	s.forceCheck(channelSlug)
	return nil
}
