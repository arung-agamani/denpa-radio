package service

import (
	"fmt"
	"log/slog"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
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
}

// MasterService implements the business logic for master playlist and
// time-tag assignment operations.
type MasterService struct {
	master    *playlist.MasterPlaylist
	store     *playlist.Store
	scheduler *playlist.Scheduler
}

func NewMasterService(master *playlist.MasterPlaylist, store *playlist.Store, scheduler *playlist.Scheduler) *MasterService {
	return &MasterService{master: master, store: store, scheduler: scheduler}
}

func (s *MasterService) save() {
	if err := s.store.Save(s.master); err != nil {
		slog.Error("Failed to save playlist state", "error", err)
	}
}

// Get returns a snapshot of the full master playlist structure.
func (s *MasterService) Get() MasterSnapshot {
	tags := make(map[string]MasterTagInfo)
	for _, tag := range playlist.ValidTimeTags {
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
	}
}

// AssignPlaylistToTag moves or assigns a playlist to a specific time tag.
func (s *MasterService) AssignPlaylistToTag(playlistID int64, tagStr string) error {
	if !playlist.IsValidTimeTag(tagStr) {
		return fmt.Errorf("invalid tag: must be one of morning, afternoon, evening, night")
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
	s.save()
	s.scheduler.ForceCheck()
	return nil
}

// RemovePlaylistFromTag removes a playlist from a specific time tag.
func (s *MasterService) RemovePlaylistFromTag(tagStr string, playlistID int64) error {
	if !playlist.IsValidTimeTag(tagStr) {
		return fmt.Errorf("invalid tag: must be one of morning, afternoon, evening, night")
	}
	tag := playlist.TimeTag(tagStr)
	if err := s.master.RemovePlaylist(tag, playlistID); err != nil {
		return err
	}
	s.save()
	return nil
}
