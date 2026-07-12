package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/channel"
	"github.com/arung-agamani/denpa-radio/internal/repository"
)

// PlaylistSummary is the lightweight representation returned in list responses.
type PlaylistSummary struct {
	ID         int64            `json:"id"`
	Name       string           `json:"name"`
	Tag        playlist.TimeTag `json:"tag"`
	TrackCount int              `json:"trackCount"`
}

// AddTrackInput bundles the parameters for PlaylistService.AddTrack.
type AddTrackInput struct {
	PlaylistID int64
	TrackID    *int64
	Checksum   *string
	FilePath   *string
	Index      *int
}

// PlaylistService implements the business logic for playlist CRUD and track
// manipulation operations.
type PlaylistService struct {
	playlists      repository.PlaylistRepository
	master         repository.MasterPlaylistRepository
	cfg            *config.Config
	channelManager channel.ChannelManager
}

func NewPlaylistService(playlists repository.PlaylistRepository, master repository.MasterPlaylistRepository, cfg *config.Config, channelManager channel.ChannelManager) *PlaylistService {
	return &PlaylistService{playlists: playlists, master: master, cfg: cfg, channelManager: channelManager}
}

// resolveMaster returns the *playlist.MasterPlaylist for the given slug.
// An empty slug resolves to the legacy default master.
func (s *PlaylistService) resolveMaster(channelSlug string) (*playlist.MasterPlaylist, error) {
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

// List returns summary information for every playlist in the master.
func (s *PlaylistService) List() []PlaylistSummary {
	allPls := s.playlists.AllPlaylists()
	summaries := make([]PlaylistSummary, 0, len(allPls))
	for _, pl := range allPls {
		summaries = append(summaries, PlaylistSummary{
			ID:         pl.ID,
			Name:       pl.Name,
			Tag:        pl.Tag,
			TrackCount: pl.Count(),
		})
	}
	return summaries
}

// GetByID returns a playlist and its assigned time tag.
func (s *PlaylistService) GetByID(id int64) (*playlist.Playlist, playlist.TimeTag, error) {
	return s.playlists.FindPlaylistByID(id)
}

// Create creates a new playlist and assigns it to the given time tag on the requested channel.
func (s *PlaylistService) Create(name, tag, channelSlug string) (*playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, apierror.ErrValidation("name is required")
	}
	if !master.IsConfiguredTag(playlist.TimeTag(tag)) {
		return nil, apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", tag))
	}
	t := playlist.TimeTag(tag)
	pl := playlist.NewPlaylist(name, t)
	pl.SetLibrary(master.Library)
	if err := master.AssignPlaylist(t, pl); err != nil {
		return nil, err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return pl, nil
}

// Update changes the name and/or tag of an existing playlist on the requested channel.
func (s *PlaylistService) Update(id int64, name, tag *string, channelSlug string) (*playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, err
	}

	pl, currentTag, err := master.FindPlaylistByID(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		pl.Name = *name
	}
	if tag != nil && playlist.TimeTag(*tag) != currentTag {
		newTag := playlist.TimeTag(*tag)
		if !master.IsConfiguredTag(newTag) {
			return nil, apierror.ErrValidation(fmt.Sprintf("invalid tag: %s is not a configured time slot", *tag))
		}
		if err := master.RemovePlaylist(currentTag, id); err != nil {
			return nil, err
		}
		if err := master.AssignPlaylist(newTag, pl); err != nil {
			return nil, err
		}
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return pl, nil
}

// Delete removes a playlist by ID from the requested channel.
func (s *PlaylistService) Delete(id int64, channelSlug string) error {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return err
	}

	_, tag, err := master.FindPlaylistByID(id)
	if err != nil {
		return err
	}
	if err := master.RemovePlaylist(tag, id); err != nil {
		return err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return nil
}

// AddTrack resolves a track via library ID, checksum, or file path, then
// appends it to the specified playlist at the optional index on the requested channel.
func (s *PlaylistService) AddTrack(input AddTrackInput, channelSlug string) (*playlist.Track, *playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, nil, err
	}

	pl, _, err := master.FindPlaylistByID(input.PlaylistID)
	if err != nil {
		return nil, nil, err
	}

	lib := master.Library
	var track *playlist.Track

	// Strategy 1: find by library track ID.
	if input.TrackID != nil {
		if lib != nil {
			track = lib.GetByID(*input.TrackID)
		}
		if track == nil {
			for _, existingPl := range master.AllPlaylists() {
				if t, _, err := existingPl.FindTrackByID(*input.TrackID); err == nil {
					track = t
					break
				}
			}
		}
		if track == nil {
			return nil, nil, apierror.ErrNotFound(fmt.Sprintf("track %d not found", *input.TrackID))
		}
	}

	// Strategy 2: find by checksum.
	if track == nil && input.Checksum != nil {
		if lib != nil {
			track = lib.Get(*input.Checksum)
		}
		if track == nil {
			for _, existingPl := range master.AllPlaylists() {
				if t, _, err := existingPl.FindTrackByChecksum(*input.Checksum); err == nil {
					track = t
					break
				}
			}
		}
		if track == nil {
			return nil, nil, apierror.ErrNotFound(fmt.Sprintf("track with checksum %q not found", *input.Checksum))
		}
	}

	// Strategy 3: create from file path (must be within the music directory).
	if track == nil && input.FilePath != nil {
		if !pathInsideMusicDir(*input.FilePath, s.cfg.MusicDir) {
			return nil, nil, apierror.ErrForbidden("file path must be within the music directory")
		}
		t, err := playlist.NewTrackFromFile(*input.FilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create track from file: %w", err)
		}
		if lib != nil {
			t = lib.AddOrUpdate(t)
		}
		track = t
	}

	if track == nil {
		return nil, nil, fmt.Errorf("must provide one of: trackId, checksum, or filePath")
	}

	if input.Index != nil {
		pl.AddTrackAt(track, *input.Index)
	} else {
		pl.AddTrack(track)
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return track, pl, nil
}

// RemoveTrack removes a track from a playlist by track ID on the requested channel.
func (s *PlaylistService) RemoveTrack(playlistID, trackID int64, channelSlug string) (*playlist.Track, *playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, nil, err
	}

	pl, _, err := master.FindPlaylistByID(playlistID)
	if err != nil {
		return nil, nil, err
	}
	removed, err := pl.RemoveTrackByID(trackID)
	if err != nil {
		return nil, nil, err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return removed, pl, nil
}

// MoveTrack reorders a track within a playlist on the requested channel.
func (s *PlaylistService) MoveTrack(playlistID int64, from, to int, channelSlug string) (*playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, err
	}

	pl, _, err := master.FindPlaylistByID(playlistID)
	if err != nil {
		return nil, err
	}
	if err := pl.MoveTrack(from, to); err != nil {
		return nil, err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return pl, nil
}

// Shuffle randomly reorders the tracks in a playlist on the requested channel.
func (s *PlaylistService) Shuffle(playlistID int64, channelSlug string) (*playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, err
	}

	pl, _, err := master.FindPlaylistByID(playlistID)
	if err != nil {
		return nil, err
	}
	pl.Shuffle()
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return pl, nil
}

// Export serializes a playlist to downloadable JSON bytes.
func (s *PlaylistService) Export(id int64) (*playlist.Playlist, []byte, error) {
	pl, _, err := s.playlists.FindPlaylistByID(id)
	if err != nil {
		return nil, nil, err
	}
	data, err := playlist.ExportPlaylist(pl)
	if err != nil {
		return nil, nil, err
	}
	return pl, data, nil
}

// Import deserializes a playlist from JSON bytes and registers it in the master of the requested channel.
func (s *PlaylistService) Import(data []byte, channelSlug string) (*playlist.Playlist, error) {
	master, err := s.resolveMaster(channelSlug)
	if err != nil {
		return nil, err
	}

	var pl *playlist.Playlist
	lib := master.Library
	if lib != nil {
		pl, err = playlist.ImportPlaylistIntoLibrary(data, lib)
	} else {
		pl, err = playlist.ImportPlaylistFromBytes(data)
	}
	if err != nil {
		return nil, err
	}
	if !playlist.IsValidTimeTag(string(pl.Tag)) {
		pl.Tag = playlist.CurrentTimeTag()
	}
	if err := master.AssignPlaylist(pl.Tag, pl); err != nil {
		return nil, err
	}
	if err := master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return pl, nil
}

// pathInsideMusicDir verifies that filePath resolves to a location within
// musicDir. Prevents local file inclusion attacks via the filePath parameter.
func pathInsideMusicDir(filePath, musicDir string) bool {
	absMusicDir, err := filepath.Abs(musicDir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absMusicDir+string(filepath.Separator)) ||
		absPath == absMusicDir
}
