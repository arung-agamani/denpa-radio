package service

import (
	"io"

	"github.com/arung-agamani/denpa-radio/internal/metadata"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// LibraryService delegates library-wide operations to the underlying
// track, radio, and metadata services. It exists so that LibraryHandlers
// can talk to a single service instead of three.
type LibraryService struct {
	trackSvc    *TrackService
	radioSvc    *RadioService
	metadataSvc *MetadataService
}

// NewLibraryService creates a delegator that forwards calls to the
// provided services.
func NewLibraryService(trackSvc *TrackService, radioSvc *RadioService, metadataSvc *MetadataService) *LibraryService {
	return &LibraryService{
		trackSvc:    trackSvc,
		radioSvc:    radioSvc,
		metadataSvc: metadataSvc,
	}
}

// Scan re-scans the music directory and registers newly discovered files.
func (s *LibraryService) Scan() (int, int, error) {
	return s.trackSvc.Scan()
}

// RefreshMetadata probes every library track via ffprobe to discover duration.
func (s *LibraryService) RefreshMetadata() (*RefreshMetadataResult, error) {
	return s.trackSvc.RefreshMetadata()
}

// BatchUpdate applies metadata changes to all tracks matching the filter.
func (s *LibraryService) BatchUpdate(filter playlist.TrackFilter, upd playlist.TrackUpdate) (int, error) {
	return s.trackSvc.BatchUpdate(filter, upd)
}

// BatchUpdateCover saves the uploaded image and applies its path to all tracks
// matching the filter.
func (s *LibraryService) BatchUpdateCover(filter playlist.TrackFilter, r io.Reader, filename string) (int, error) {
	return s.trackSvc.BatchUpdateCover(filter, r, filename)
}

// Reconcile scans the music directory, removes stale tracks, auto-adds
// orphaned tracks to the active playlist, and persists state.
func (s *LibraryService) Reconcile(channelSlug string) (ReconcileResult, error) {
	return s.radioSvc.Reconcile(channelSlug)
}

// EnrichAll triggers batch enrichment for all tracks in the library.
func (s *LibraryService) EnrichAll() (int, int, error) {
	return s.metadataSvc.EnrichAll()
}

// GetCoverPath returns the file path to a track's cover art, if any.
func (s *LibraryService) GetCoverPath(trackID int64) string {
	return s.metadataSvc.GetCoverPath(trackID)
}

// EnrichTrack enriches a single track's metadata from external sources.
func (s *LibraryService) EnrichTrack(trackID int64) (*metadata.EnrichResult, error) {
	return s.metadataSvc.EnrichTrack(trackID)
}
