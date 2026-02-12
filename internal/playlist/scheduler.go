package playlist

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// SchedulerEvent describes a playlist switch triggered by the scheduler.
type SchedulerEvent struct {
	PreviousTag TimeTag
	NewTag      TimeTag
	Playlist    *Playlist
	Timestamp   time.Time
}

// SchedulerCallback is called whenever the scheduler detects a time-tag
// transition. Implementations must be safe for concurrent use.
type SchedulerCallback func(event SchedulerEvent)

// Scheduler periodically checks the current time and compares it against the
// active tag of a MasterPlaylist. When the time-of-day category changes (e.g.
// morning → afternoon) the scheduler triggers a callback so the radio service
// can switch to the appropriate playlist.
type Scheduler struct {
	mu       sync.RWMutex
	master   *MasterPlaylist
	callback SchedulerCallback
	interval time.Duration

	// lastTag records the tag that was active on the most recent tick so that
	// the callback only fires on transitions.
	lastTag TimeTag
	running bool
}

// NewScheduler creates a Scheduler that watches the given MasterPlaylist for
// time-tag transitions. The check interval controls how often the clock is
// polled; a value of 1 minute is a reasonable default.
func NewScheduler(master *MasterPlaylist, callback SchedulerCallback, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	return &Scheduler{
		master:   master,
		callback: callback,
		interval: interval,
		lastTag:  CurrentTimeTagIn(master.Location()),
	}
}

// Start begins the scheduler loop. It blocks until ctx is cancelled. The
// scheduler fires an initial check immediately, then re-checks every interval.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	slog.Info("Scheduler started",
		"interval", s.interval,
		"initial_tag", s.lastTag,
	)

	// Perform an initial resolution so the master playlist's activeTag is set
	// correctly from the start.
	s.check()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Scheduler stopping")
			return
		case <-ticker.C:
			s.check()
		}
	}
}

// check performs a single time-tag evaluation and fires the callback if a
// transition occurred.
func (s *Scheduler) check() {
	newTag, changed := s.master.ResolveActiveTag()

	if !changed {
		return
	}

	s.mu.Lock()
	previousTag := s.lastTag
	s.lastTag = newTag
	s.mu.Unlock()

	slog.Info("Time-tag transition detected",
		"previous", previousTag,
		"new", newTag,
	)

	// Attempt to resolve the playlist that will now be active.
	activePl, err := s.master.ActivePlaylist()
	if err != nil {
		slog.Warn("No playlist available for new time tag",
			"tag", newTag,
			"error", err,
		)
		// Still fire the callback with a nil playlist so the radio service
		// knows a transition happened, even if there's nothing to play.
	}

	if s.callback != nil {
		s.callback(SchedulerEvent{
			PreviousTag: previousTag,
			NewTag:      newTag,
			Playlist:    activePl,
			Timestamp:   time.Now(),
		})
	}
}

// Running returns true if the scheduler loop is currently active.
func (s *Scheduler) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// LastTag returns the most recently observed time tag.
func (s *Scheduler) LastTag() TimeTag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastTag
}

// SetMasterPlaylist replaces the master playlist the scheduler is watching.
// This is safe to call while the scheduler is running.
func (s *Scheduler) SetMasterPlaylist(master *MasterPlaylist) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.master = master
}

// ForceCheck triggers an immediate time-tag check outside the normal ticker
// interval. This is useful after the user modifies playlists or tags and wants
// the scheduler to re-evaluate immediately.
func (s *Scheduler) ForceCheck() {
	s.check()
}
