package service

import (
	"fmt"
	"log/slog"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
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
	master    repository.MasterPlaylistRepository
	scheduler *playlist.Scheduler
}

func NewMasterService(master repository.MasterPlaylistRepository, scheduler *playlist.Scheduler) *MasterService {
	return &MasterService{master: master, scheduler: scheduler}
}

// Get returns a snapshot of the full master playlist structure.
func (s *MasterService) Get() MasterSnapshot {
	tags := make(map[string]MasterTagInfo)
	for _, tag := range s.master.ConfiguredTags() {
		pls := s.master.GetPlaylists(tag)
		tags[string(tag)] = MasterTagInfo{Playlists: pls, Count: len(pls)}
	}
	activeTag := s.master.ActiveTag()
	activePl, _ := s.master.ActivePlaylist()
	var activePlaylistID *int64
	if activePl != nil {
		activePlaylistID = &activePl.ID
	}
	return MasterSnapshot{
		ActiveTag:        activeTag,
		ActivePlaylistID: activePlaylistID,
		TotalTracks:      s.master.TotalTracks(),
		Tags:             tags,
		TimeSlots:        s.master.GetTimeSlots(),
	}
}

// AssignPlaylistToTag moves or assigns a playlist to a specific time tag.
func (s *MasterService) AssignPlaylistToTag(playlistID int64, tagStr string) error {
	if !s.master.IsConfiguredTag(playlist.TimeTag(tagStr)) {
		return apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tagStr))
	}
	tag := playlist.TimeTag(tagStr)
	pl, currentTag, err := s.master.FindPlaylistByID(playlistID)
	if err != nil {
		return err
	}
	if currentTag != tag {
		if removeErr := s.master.RemovePlaylist(currentTag, playlistID); removeErr != nil {
			slog.Warn("Failed to remove playlist from old tag during reassignment",
				"error", removeErr)
		}
	}
	if err := s.master.AssignPlaylist(tag, pl); err != nil {
		return err
	}
	if err := s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	s.scheduler.ForceCheck()
	return nil
}

// RemovePlaylistFromTag removes a playlist from a specific time tag.
func (s *MasterService) RemovePlaylistFromTag(tagStr string, playlistID int64) error {
	if !s.master.IsConfiguredTag(playlist.TimeTag(tagStr)) {
		return apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tagStr))
	}
	tag := playlist.TimeTag(tagStr)
	if err := s.master.RemovePlaylist(tag, playlistID); err != nil {
		return err
	}
	if err := s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return nil
}

// GetTimeSlots returns the currently configured time slots.
func (s *MasterService) GetTimeSlots() []playlist.TimeSlot {
	return s.master.GetTimeSlots()
}

// SetTimeSlots replaces all time slots after validation. Saves and forces a
// scheduler re-check.
func (s *MasterService) SetTimeSlots(slots []playlist.TimeSlot) error {
	if err := s.master.SetTimeSlots(slots); err != nil {
		return err
	}
	if err := s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	s.scheduler.ForceCheck()
	return nil
}
