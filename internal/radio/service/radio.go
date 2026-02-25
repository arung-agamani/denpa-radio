package service

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// Broadcaster is the minimal interface the RadioService needs from the
// stream broadcaster. Using an interface avoids a circular import with the
// parent radio package.
type Broadcaster interface {
	CurrentTrack() string
	ActiveClients() int
}

// StatusSnapshot holds all fields for the GET /api/status response.
// CurrentTrackRaw carries the raw track so the handler layer can apply
// presentation-level sanitisation (e.g. stripping file-system paths).
type StatusSnapshot struct {
	StationName      string
	CurrentTrack     string
	CurrentTrackRaw  *playlist.Track // nil when nothing is playing
	TotalTracks      int
	LibraryTracks    int
	ActiveClients    int
	MaxClients       int
	ActiveTag        playlist.TimeTag
	ActivePlaylist   string
	ActivePlaylistID *int64
	SchedulerRunning bool
	PlaylistSummary  interface{}
	Timezone         string
	ServerTime       string
}

// SchedulerSnapshot holds all fields for the GET /api/scheduler/status response.
type SchedulerSnapshot struct {
	Running       bool
	LastTag       playlist.TimeTag
	TimeTags      []playlist.TimeTag
	CurrentTag    playlist.TimeTag
	Summary       interface{}
	LibraryTracks int
	Timezone      string
	ServerTime    string
}

// ReconcileResult holds the outcome of a reconciliation operation.
type ReconcileResult struct {
	RemovedCount  int
	OrphanedCount int
	Orphaned      []*playlist.Track
	TotalTracks   int
}

// RadioService implements business logic for station status, scheduler
// monitoring, timezone management, and reconciliation.
type RadioService struct {
	master      *playlist.MasterPlaylist
	store       *playlist.Store
	scheduler   *playlist.Scheduler
	broadcaster Broadcaster
	cfg         *config.Config
}

func NewRadioService(
	master *playlist.MasterPlaylist,
	store *playlist.Store,
	scheduler *playlist.Scheduler,
	broadcaster Broadcaster,
	cfg *config.Config,
) *RadioService {
	return &RadioService{
		master:      master,
		store:       store,
		scheduler:   scheduler,
		broadcaster: broadcaster,
		cfg:         cfg,
	}
}

func (s *RadioService) save() {
	if err := s.store.Save(s.master); err != nil {
		slog.Error("Failed to save playlist state", "error", err)
	}
}

// Status builds the full station status snapshot.
func (s *RadioService) Status() StatusSnapshot {
	currentTrackPath := s.broadcaster.CurrentTrack()
	trackName := "none"
	if currentTrackPath != "" {
		trackName = filepath.Base(currentTrackPath)
	}

	activeTag := s.master.ActiveTag()
	activePl, _ := s.master.ActivePlaylist()
	var activePlaylistName string
	var activePlaylistID *int64
	if activePl != nil {
		activePlaylistName = activePl.Name
		activePlaylistID = &activePl.ID
	}

	var currentTrackRaw *playlist.Track
	if currentTrackPath != "" {
		if s.master.Library != nil {
			currentTrackRaw = s.master.Library.GetByFilePath(currentTrackPath)
		}
		if currentTrackRaw == nil {
			for _, pl := range s.master.AllPlaylists() {
				if t, _, err := pl.FindTrackByFilePath(currentTrackPath); err == nil {
					currentTrackRaw = t
					break
				}
			}
		}
	}

	loc := s.master.Location()
	tz := s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	return StatusSnapshot{
		StationName:      s.cfg.StationName,
		CurrentTrack:     trackName,
		CurrentTrackRaw:  currentTrackRaw,
		TotalTracks:      s.master.TotalTracks(),
		LibraryTracks:    s.master.LibraryTrackCount(),
		ActiveClients:    s.broadcaster.ActiveClients(),
		MaxClients:       s.cfg.MaxClients,
		ActiveTag:        activeTag,
		ActivePlaylist:   activePlaylistName,
		ActivePlaylistID: activePlaylistID,
		SchedulerRunning: s.scheduler.Running(),
		PlaylistSummary:  s.master.Summary(),
		Timezone:         tz,
		ServerTime:       time.Now().In(loc).Format(time.RFC3339),
	}
}

// SchedulerStatus builds the scheduler status snapshot.
func (s *RadioService) SchedulerStatus() SchedulerSnapshot {
	loc := s.master.Location()
	tz := s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}
	return SchedulerSnapshot{
		Running:       s.scheduler.Running(),
		LastTag:       s.scheduler.LastTag(),
		TimeTags:      playlist.ValidTimeTags,
		CurrentTag:    playlist.CurrentTimeTagIn(loc),
		Summary:       s.master.Summary(),
		LibraryTracks: s.master.LibraryTrackCount(),
		Timezone:      tz,
		ServerTime:    time.Now().In(loc).Format(time.RFC3339),
	}
}

// GetTimezone returns the current timezone name and current server time string.
func (s *RadioService) GetTimezone() (tz, serverTime string) {
	loc := s.master.Location()
	tz = s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}
	return tz, time.Now().In(loc).Format(time.RFC3339)
}

// SetTimezone updates the master playlist timezone and forces scheduler
// re-evaluation. Returns the resolved timezone, current server time, and
// the newly active time tag.
func (s *RadioService) SetTimezone(tz string) (resolvedTZ, serverTime string, activeTag playlist.TimeTag, err error) {
	if err = s.master.SetTimezone(tz); err != nil {
		return
	}
	s.scheduler.ForceCheck()
	s.save()

	loc := s.master.Location()
	resolvedTZ = s.master.Timezone()
	if resolvedTZ == "" {
		resolvedTZ = "UTC"
	}
	serverTime = time.Now().In(loc).Format(time.RFC3339)
	activeTag = s.master.ActiveTag()
	return
}

// LegacyAllTracks returns all deduplicated tracks for the legacy /playlist endpoint.
func (s *RadioService) LegacyAllTracks() []*playlist.Track {
	return s.master.AllTracksDeduped()
}

// Reconcile scans the music directory, removes stale tracks, auto-adds
// orphaned tracks to the active playlist, and persists state.
func (s *RadioService) Reconcile() (ReconcileResult, error) {
	orphaned, removedCount, err := playlist.ReconcileTracks(s.cfg.MusicDir, s.master)
	if err != nil {
		return ReconcileResult{}, err
	}
	if len(orphaned) > 0 {
		activePl, plErr := s.master.ActivePlaylist()
		if plErr == nil && activePl != nil {
			activePl.AddTracks(orphaned)
			slog.Info("Added orphaned tracks to active playlist",
				"count", len(orphaned),
				"playlist", activePl.Name,
			)
		}
	}
	s.save()
	return ReconcileResult{
		RemovedCount:  removedCount,
		OrphanedCount: len(orphaned),
		Orphaned:      orphaned,
		TotalTracks:   s.master.TotalTracks(),
	}, nil
}
