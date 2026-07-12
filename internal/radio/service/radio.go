package service

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/channel"
	"github.com/arung-agamani/denpa-radio/internal/repository"
)

// Broadcaster is an alias for channel.Broadcaster so that RadioService can
// refer to it without importing channel everywhere.
type Broadcaster = channel.Broadcaster

// defaultChannel wraps the legacy master/broadcaster/scheduler so that
// RadioService can treat them as a channel.Channel when no channel manager is
// configured or when channelSlug is empty.
type defaultChannel struct {
	master      *playlist.MasterPlaylist
	broadcaster Broadcaster
	scheduler   *playlist.Scheduler
	saver       func() error
}

func (d *defaultChannel) GetMaster() *playlist.MasterPlaylist { return d.master }
func (d *defaultChannel) GetBroadcaster() Broadcaster         { return d.broadcaster }
func (d *defaultChannel) GetScheduler() *playlist.Scheduler  { return d.scheduler }
func (d *defaultChannel) Save() error {
	if d.saver != nil {
		return d.saver()
	}
	return nil
}
func (d *defaultChannel) GetSlug() string        { return "" }
func (d *defaultChannel) GetName() string        { return "" }
func (d *defaultChannel) GetDescription() string { return "" }
func (d *defaultChannel) GetSortOrder() int      { return 0 }
func (d *defaultChannel) GetEnabled() bool        { return true }
func (d *defaultChannel) GetBitrate() string     { return "" }

// Compile-time check that defaultChannel satisfies channel.Channel.
var _ channel.Channel = (*defaultChannel)(nil)

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
	master         repository.MasterPlaylistRepository
	scheduler      *playlist.Scheduler
	broadcaster    Broadcaster
	cfg            *config.Config
	channelManager channel.ChannelManager
}

func NewRadioService(
	master repository.MasterPlaylistRepository,
	scheduler *playlist.Scheduler,
	broadcaster Broadcaster,
	cfg *config.Config,
	channelManager channel.ChannelManager,
) *RadioService {
	return &RadioService{
		master:         master,
		scheduler:      scheduler,
		broadcaster:    broadcaster,
		cfg:            cfg,
		channelManager: channelManager,
	}
}

// resolveChannel returns the channel.Channel for the given slug. An empty slug
// resolves to the legacy default channel (master/scheduler/broadcaster).
func (s *RadioService) resolveChannel(channelSlug string) (channel.Channel, error) {
	if channelSlug == "" {
		if s.channelManager != nil {
			ch := s.channelManager.DefaultChannel()
			if ch == nil {
				return nil, apierror.ErrInternal("no default channel configured")
			}
			return ch, nil
		}
		return &defaultChannel{
			master:      s.master.MasterPlaylist(),
			broadcaster: s.broadcaster,
			scheduler:   s.scheduler,
			saver:       func() error { return s.master.Save() },
		}, nil
	}
	if s.channelManager == nil {
		return nil, apierror.ErrInternal("channel manager not configured")
	}
	ch := s.channelManager.ChannelBySlug(channelSlug)
	if ch == nil {
		return nil, apierror.ErrNotFound(fmt.Sprintf("channel %q not found", channelSlug))
	}
	return ch, nil
}

// Status builds the full station status snapshot for the requested channel.
func (s *RadioService) Status(channelSlug string) (StatusSnapshot, error) {
	ch, err := s.resolveChannel(channelSlug)
	if err != nil {
		return StatusSnapshot{}, err
	}

	master := ch.GetMaster()
	broadcaster := ch.GetBroadcaster()
	scheduler := ch.GetScheduler()

	currentTrackPath := broadcaster.CurrentTrack()
	trackName := "none"
	if currentTrackPath != "" {
		trackName = filepath.Base(currentTrackPath)
	}

	activeTag := master.ActiveTag()
	activePl, _ := master.ActivePlaylist()
	var activePlaylistName string
	var activePlaylistID *int64
	if activePl != nil {
		activePlaylistName = activePl.Name
		activePlaylistID = &activePl.ID
	}

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

	loc := master.Location()
	tz := master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	return StatusSnapshot{
		StationName:      s.cfg.StationName,
		CurrentTrack:     trackName,
		CurrentTrackRaw:  currentTrackRaw,
		TotalTracks:      master.TotalTracks(),
		LibraryTracks:    master.LibraryTrackCount(),
		ActiveClients:    broadcaster.ActiveClients(),
		MaxClients:       s.cfg.MaxClients,
		ActiveTag:        activeTag,
		ActivePlaylist:   activePlaylistName,
		ActivePlaylistID: activePlaylistID,
		SchedulerRunning: scheduler.Running(),
		PlaylistSummary:  master.Summary(),
		Timezone:         tz,
		ServerTime:       time.Now().In(loc).Format(time.RFC3339),
	}, nil
}

// SchedulerStatus builds the scheduler status snapshot for the requested channel.
func (s *RadioService) SchedulerStatus(channelSlug string) (SchedulerSnapshot, error) {
	ch, err := s.resolveChannel(channelSlug)
	if err != nil {
		return SchedulerSnapshot{}, err
	}

	master := ch.GetMaster()
	scheduler := ch.GetScheduler()

	loc := master.Location()
	tz := master.Timezone()
	if tz == "" {
		tz = "UTC"
	}
	return SchedulerSnapshot{
		Running:       scheduler.Running(),
		LastTag:       scheduler.LastTag(),
		TimeTags:      master.ConfiguredTags(),
		CurrentTag:    master.TimeTagForHourConfigured(time.Now().In(loc).Hour()),
		Summary:       master.Summary(),
		LibraryTracks: master.LibraryTrackCount(),
		Timezone:      tz,
		ServerTime:    time.Now().In(loc).Format(time.RFC3339),
	}, nil
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
	if s.scheduler != nil {
		s.scheduler.ForceCheck()
	} else if s.channelManager != nil {
		ch := s.channelManager.DefaultChannel()
		if ch != nil {
			ch.GetScheduler().ForceCheck()
		}
	}
	if err = s.master.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}

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

// GetQueue returns up to n upcoming tracks from the active playlist of the
// requested channel, starting with the currently-playing track. Pass n <= 0
// to get all tracks.
func (s *RadioService) GetQueue(channelSlug string, n int) ([]*playlist.Track, error) {
	ch, err := s.resolveChannel(channelSlug)
	if err != nil {
		return nil, err
	}
	tracks, _ := ch.GetMaster().PeekQueue(n)
	return tracks, nil
}

// SkipNext immediately skips to the next track by aborting the current one.
func (s *RadioService) SkipNext(channelSlug string) error {
	ch, err := s.resolveChannel(channelSlug)
	if err != nil {
		return err
	}
	ch.GetBroadcaster().Skip()
	return nil
}

// SkipPrev seeks the active playlist cursor back one position, then aborts the
// current track so playback restarts from the previous track.
func (s *RadioService) SkipPrev(channelSlug string) error {
	ch, err := s.resolveChannel(channelSlug)
	if err != nil {
		return err
	}
	if err := ch.GetMaster().SeekPrev(); err != nil {
		return err
	}
	ch.GetBroadcaster().Skip()
	return nil
}

// Reconcile scans the music directory, removes stale tracks, auto-adds
// orphaned tracks to the active playlist of the requested channel, and persists state.
func (s *RadioService) Reconcile(channelSlug string) (ReconcileResult, error) {
	ch, err := s.resolveChannel(channelSlug)
	if err != nil {
		return ReconcileResult{}, err
	}

	master := ch.GetMaster()
	lib := master.Library
	if lib == nil {
		return ReconcileResult{}, apierror.ErrInternal("track library not initialised")
	}

	stale := lib.RemoveStale()
	removedCount := len(stale)
	for _, t := range stale {
		master.RemoveTrackFromAll(t.Checksum)
	}
	if removedCount > 0 {
		slog.Info("Removed stale tracks from library and playlists", "count", removedCount)
	}

	orphaned, err := playlist.FindOrphanedTracksFromLibrary(s.cfg.MusicDir, lib)
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("failed to find orphaned tracks: %w", err)
	}

	// Add orphaned tracks to the library so they get stable IDs.
	if len(orphaned) > 0 {
		for i, t := range orphaned {
			canonical := lib.AddOrUpdate(t)
			orphaned[i] = canonical
		}
		slog.Info("Added orphaned tracks to library", "count", len(orphaned))
	}

	if len(orphaned) > 0 {
		activePl, plErr := master.ActivePlaylist()
		if plErr == nil && activePl != nil {
			activePl.AddTracks(orphaned)
			slog.Info("Added orphaned tracks to active playlist",
				"count", len(orphaned),
				"playlist", activePl.Name,
			)
		}
	}
	if err := ch.Save(); err != nil {
		slog.Error("failed to save playlist state", "error", err)
	}
	return ReconcileResult{
		RemovedCount:  removedCount,
		OrphanedCount: len(orphaned),
		Orphaned:      orphaned,
		TotalTracks:   master.TotalTracks(),
	}, nil
}
