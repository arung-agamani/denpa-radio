package service

import (
	"fmt"
	"log/slog"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/metadata"
	"github.com/arung-agamani/denpa-radio/internal/repository"
)

// MetadataService handles album art serving and external metadata enrichment.
type MetadataService struct {
	tracks   repository.TrackRepository
	master   repository.MasterPlaylistRepository
	enricher *metadata.Enricher
}

// NewMetadataService creates a new MetadataService.
func NewMetadataService(tracks repository.TrackRepository, master repository.MasterPlaylistRepository, enricher *metadata.Enricher) *MetadataService {
	return &MetadataService{
		tracks:   tracks,
		master:   master,
		enricher: enricher,
	}
}

// GetCoverPath returns the file path to a track's cover art, if any.
// Returns empty string if no cover art is available.
func (s *MetadataService) GetCoverPath(trackID int64) string {
	track := s.tracks.GetByID(trackID)
	if track == nil {
		return ""
	}
	return track.CoverPath
}

// EnrichTrack enriches a single track's metadata from external sources.
// Returns the enrichment result.
func (s *MetadataService) EnrichTrack(trackID int64) (*metadata.EnrichResult, error) {
	track := s.tracks.GetByID(trackID)
	if track == nil {
	return nil, apierror.ErrNotFound(fmt.Sprintf("track %d not found", trackID))
}

	artist := track.Artist
	album := track.Album

	if artist == "" && album == "" {
		return nil, fmt.Errorf("track %d has no artist or album metadata to enrich", trackID)
	}

	// Enrich with external sources.
	result, err := s.enricher.Enrich(artist, album)
	if err != nil {
		return nil, fmt.Errorf("enrichment failed: %w", err)
	}

	// If cover art was fetched, update the track's CoverPath.
	if result != nil && result.ArtPath != "" {
		track.CoverPath = result.ArtPath
		if err := s.master.Save(); err != nil {
			slog.Error("failed to save playlist state", "error", err)
		}
	}

	if result != nil {
		updated := false
		if track.Genre == "" && len(result.Genres) > 0 && result.Genres[0] != "" {
			track.Genre = result.Genres[0]
			updated = true
		}
		if track.Year == 0 && result.Year > 0 {
			track.Year = result.Year
			updated = true
		}
		if updated {
			if err := s.master.Save(); err != nil {
				slog.Error("failed to save playlist state", "error", err)
			}
		}
	}

	return result, nil
}

// EnrichAll enriches all tracks in the library that have artist/album metadata.
// Returns the number of tracks attempted and the number that had cover art found.
func (s *MetadataService) EnrichAll() (attempted int, artFound int, err error) {
	tracks := s.tracks.List()
	for _, t := range tracks {
		if t.Artist == "" && t.Album == "" {
			continue
		}
		if t.CoverPath != "" {
			// Already has cover art.
			continue
		}
		attempted++

		result, enrichErr := s.enricher.Enrich(t.Artist, t.Album)
		if enrichErr != nil {
			slog.Warn("Enrichment failed for track",
				"track_id", t.ID,
				"artist", t.Artist,
				"album", t.Album,
				"error", enrichErr,
			)
			continue
		}
		if result != nil && result.ArtPath != "" {
			t.CoverPath = result.ArtPath
			artFound++
		}
	}

	if artFound > 0 {
		if err := s.master.Save(); err != nil {
			slog.Error("failed to save playlist state", "error", err)
		}
	}

	return attempted, artFound, nil
}
