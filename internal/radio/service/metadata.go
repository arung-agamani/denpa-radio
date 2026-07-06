package service

import (
	"fmt"
	"log/slog"

	"github.com/arung-agamani/denpa-radio/internal/metadata"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// MetadataService handles album art serving and external metadata enrichment.
type MetadataService struct {
	master  *playlist.MasterPlaylist
	store   *playlist.Store
	enricher *metadata.Enricher
}

// NewMetadataService creates a new MetadataService.
func NewMetadataService(master *playlist.MasterPlaylist, store *playlist.Store, enricher *metadata.Enricher) *MetadataService {
	return &MetadataService{
		master:   master,
		store:    store,
		enricher: enricher,
	}
}

// GetCoverPath returns the file path to a track's cover art, if any.
// Returns empty string if no cover art is available.
func (s *MetadataService) GetCoverPath(trackID int64) string {
	if s.master.Library == nil {
		return ""
	}
	track := s.master.Library.GetByID(trackID)
	if track == nil {
		return ""
	}
	return track.CoverPath
}

// EnrichTrack enriches a single track's metadata from external sources.
// Returns the enrichment result.
func (s *MetadataService) EnrichTrack(trackID int64) (*metadata.EnrichResult, error) {
	if s.master.Library == nil {
		return nil, fmt.Errorf("track library not initialised")
	}

	track := s.master.Library.GetByID(trackID)
	if track == nil {
		return nil, fmt.Errorf("track %d not found", trackID)
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
		s.save()
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
			s.save()
		}
	}

	return result, nil
}

// EnrichAll enriches all tracks in the library that have artist/album metadata.
// Returns the number of tracks attempted and the number that had cover art found.
func (s *MetadataService) EnrichAll() (attempted int, artFound int, err error) {
	if s.master.Library == nil {
		return 0, 0, fmt.Errorf("track library not initialised")
	}

	tracks := s.master.Library.List()
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
		s.save()
	}

	return attempted, artFound, nil
}

func (s *MetadataService) save() {
	if err := s.store.Save(s.master); err != nil {
		slog.Error("Failed to save playlist state", "error", err)
	}
}
