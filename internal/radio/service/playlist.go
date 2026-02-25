package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
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
	master *playlist.MasterPlaylist
	store  *playlist.Store
	cfg    *config.Config
}

func NewPlaylistService(master *playlist.MasterPlaylist, store *playlist.Store, cfg *config.Config) *PlaylistService {
	return &PlaylistService{master: master, store: store, cfg: cfg}
}

func (s *PlaylistService) save() {
	if err := s.store.Save(s.master); err != nil {
		slog.Error("Failed to save playlist state", "error", err)
	}
}

// List returns summary information for every playlist in the master.
func (s *PlaylistService) List() []PlaylistSummary {
	allPls := s.master.AllPlaylists()
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
	return s.master.FindPlaylistByID(id)
}

// Create creates a new playlist and assigns it to the given time tag.
func (s *PlaylistService) Create(name, tag string) (*playlist.Playlist, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if !playlist.IsValidTimeTag(tag) {
		return nil, fmt.Errorf("invalid tag: must be one of morning, afternoon, evening, night")
	}
	t := playlist.TimeTag(tag)
	pl := playlist.NewPlaylist(name, t)
	pl.SetLibrary(s.master.Library)
	if err := s.master.AssignPlaylist(t, pl); err != nil {
		return nil, err
	}
	s.save()
	return pl, nil
}

// Update changes the name and/or tag of an existing playlist.
func (s *PlaylistService) Update(id int64, name, tag *string) (*playlist.Playlist, error) {
	pl, currentTag, err := s.master.FindPlaylistByID(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		pl.Name = *name
	}
	if tag != nil && playlist.TimeTag(*tag) != currentTag {
		newTag := playlist.TimeTag(*tag)
		if !playlist.IsValidTimeTag(*tag) {
			return nil, fmt.Errorf("invalid tag: must be one of morning, afternoon, evening, night")
		}
		if err := s.master.RemovePlaylist(currentTag, id); err != nil {
			return nil, err
		}
		if err := s.master.AssignPlaylist(newTag, pl); err != nil {
			return nil, err
		}
	}
	s.save()
	return pl, nil
}

// Delete removes a playlist by ID.
func (s *PlaylistService) Delete(id int64) error {
	_, tag, err := s.master.FindPlaylistByID(id)
	if err != nil {
		return err
	}
	if err := s.master.RemovePlaylist(tag, id); err != nil {
		return err
	}
	s.save()
	return nil
}

// AddTrack resolves a track via library ID, checksum, or file path, then
// appends it to the specified playlist at the optional index.
func (s *PlaylistService) AddTrack(input AddTrackInput) (*playlist.Track, *playlist.Playlist, error) {
	pl, _, err := s.master.FindPlaylistByID(input.PlaylistID)
	if err != nil {
		return nil, nil, err
	}

	lib := s.master.Library
	var track *playlist.Track

	// Strategy 1: find by library track ID.
	if input.TrackID != nil {
		if lib != nil {
			track = lib.GetByID(*input.TrackID)
		}
		if track == nil {
			for _, existingPl := range s.master.AllPlaylists() {
				if t, _, err := existingPl.FindTrackByID(*input.TrackID); err == nil {
					track = t
					break
				}
			}
		}
		if track == nil {
			return nil, nil, fmt.Errorf("track %d not found", *input.TrackID)
		}
	}

	// Strategy 2: find by checksum.
	if track == nil && input.Checksum != nil {
		if lib != nil {
			track = lib.Get(*input.Checksum)
		}
		if track == nil {
			for _, existingPl := range s.master.AllPlaylists() {
				if t, _, err := existingPl.FindTrackByChecksum(*input.Checksum); err == nil {
					track = t
					break
				}
			}
		}
		if track == nil {
			return nil, nil, fmt.Errorf("track with checksum %q not found", *input.Checksum)
		}
	}

	// Strategy 3: create from file path (must be within the music directory).
	if track == nil && input.FilePath != nil {
		if !pathInsideMusicDir(*input.FilePath, s.cfg.MusicDir) {
			return nil, nil, fmt.Errorf("file path must be within the music directory")
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
	s.save()
	return track, pl, nil
}

// RemoveTrack removes a track from a playlist by track ID.
func (s *PlaylistService) RemoveTrack(playlistID, trackID int64) (*playlist.Track, *playlist.Playlist, error) {
	pl, _, err := s.master.FindPlaylistByID(playlistID)
	if err != nil {
		return nil, nil, err
	}
	removed, err := pl.RemoveTrackByID(trackID)
	if err != nil {
		return nil, nil, err
	}
	s.save()
	return removed, pl, nil
}

// MoveTrack reorders a track within a playlist.
func (s *PlaylistService) MoveTrack(playlistID int64, from, to int) (*playlist.Playlist, error) {
	pl, _, err := s.master.FindPlaylistByID(playlistID)
	if err != nil {
		return nil, err
	}
	if err := pl.MoveTrack(from, to); err != nil {
		return nil, err
	}
	s.save()
	return pl, nil
}

// Shuffle randomly reorders the tracks in a playlist.
func (s *PlaylistService) Shuffle(playlistID int64) (*playlist.Playlist, error) {
	pl, _, err := s.master.FindPlaylistByID(playlistID)
	if err != nil {
		return nil, err
	}
	pl.Shuffle()
	s.save()
	return pl, nil
}

// Export serializes a playlist to downloadable JSON bytes.
func (s *PlaylistService) Export(id int64) (*playlist.Playlist, []byte, error) {
	pl, _, err := s.master.FindPlaylistByID(id)
	if err != nil {
		return nil, nil, err
	}
	data, err := playlist.ExportPlaylist(pl)
	if err != nil {
		return nil, nil, err
	}
	return pl, data, nil
}

// Import deserializes a playlist from JSON bytes and registers it in the master.
func (s *PlaylistService) Import(data []byte) (*playlist.Playlist, error) {
	var (
		pl  *playlist.Playlist
		err error
	)
	if s.master.Library != nil {
		pl, err = playlist.ImportPlaylistIntoLibrary(data, s.master.Library)
	} else {
		pl, err = playlist.ImportPlaylistFromBytes(data)
	}
	if err != nil {
		return nil, err
	}
	if !playlist.IsValidTimeTag(string(pl.Tag)) {
		pl.Tag = playlist.CurrentTimeTag()
	}
	if err := s.master.AssignPlaylist(pl.Tag, pl); err != nil {
		return nil, err
	}
	s.save()
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
