package handler

import (
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// RadioHandlers holds the gin route handlers for station status, scheduler,
// timezone, legacy compatibility, and reconcile endpoints.
type RadioHandlers struct {
	svc *service.RadioService
}

func NewRadioHandlers(svc *service.RadioService) *RadioHandlers {
	return &RadioHandlers{svc: svc}
}

// Health handles GET /health
func (h *RadioHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Status handles GET /api/status  (and legacy GET /status)
func (h *RadioHandlers) Status(c *gin.Context) {
	snap := h.svc.Status()
	var currentTrackInfo interface{}
	if snap.CurrentTrackRaw != nil {
		currentTrackInfo = sanitiseTrack(snap.CurrentTrackRaw)
	}
	c.JSON(http.StatusOK, gin.H{
		"station_name":       snap.StationName,
		"current_track":      snap.CurrentTrack,
		"current_track_info": currentTrackInfo,
		"total_tracks":       snap.TotalTracks,
		"library_tracks":     snap.LibraryTracks,
		"active_clients":     snap.ActiveClients,
		"max_clients":        snap.MaxClients,
		"active_tag":         snap.ActiveTag,
		"active_playlist":    snap.ActivePlaylist,
		"active_playlist_id": snap.ActivePlaylistID,
		"scheduler_running":  snap.SchedulerRunning,
		"playlist_summary":   snap.PlaylistSummary,
		"timezone":           snap.Timezone,
		"server_time":        snap.ServerTime,
	})
}

// SchedulerStatus handles GET /api/scheduler/status
func (h *RadioHandlers) SchedulerStatus(c *gin.Context) {
	snap := h.svc.SchedulerStatus()
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"running":        snap.Running,
		"last_tag":       snap.LastTag,
		"time_tags":      snap.TimeTags,
		"current_tag":    snap.CurrentTag,
		"summary":        snap.Summary,
		"library_tracks": snap.LibraryTracks,
		"timezone":       snap.Timezone,
		"server_time":    snap.ServerTime,
	})
}

// GetTimezone handles GET /api/timezone
func (h *RadioHandlers) GetTimezone(c *gin.Context) {
	tz, serverTime := h.svc.GetTimezone()
	c.JSON(http.StatusOK, gin.H{"timezone": tz, "server_time": serverTime})
}

// SetTimezone handles PUT /api/timezone  (protected)
func (h *RadioHandlers) SetTimezone(c *gin.Context) {
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	tz, serverTime, activeTag, err := h.svc.SetTimezone(body.Timezone)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"timezone":    tz,
		"server_time": serverTime,
		"active_tag":  activeTag,
	})
}

// Reconcile handles POST /api/reconcile  (protected)
func (h *RadioHandlers) Reconcile(c *gin.Context) {
	slog.Info("Reconcile requested", "remote", c.ClientIP())
	result, err := h.svc.Reconcile()
	if err != nil {
		slog.Error("Reconciliation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "reconciliation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"removed_count":  result.RemovedCount,
		"orphaned_count": result.OrphanedCount,
		"orphaned":       sanitiseTracks(result.Orphaned),
		"total_tracks":   result.TotalTracks,
	})
}

// LegacyPlaylist handles GET /playlist  (backwards compat)
func (h *RadioHandlers) LegacyPlaylist(c *gin.Context) {
	snap := h.svc.Status()
	allTracks := h.svc.LegacyAllTracks()

	type legacyTrackInfo struct {
		Filename string `json:"filename"`
		Path     string `json:"path"`
		Title    string `json:"title,omitempty"`
		Artist   string `json:"artist,omitempty"`
		Album    string `json:"album,omitempty"`
		Genre    string `json:"genre,omitempty"`
		Year     int    `json:"year,omitempty"`
		Format   string `json:"format"`
	}

	tracks := make([]legacyTrackInfo, 0, len(allTracks))
	for _, t := range allTracks {
		tracks = append(tracks, legacyTrackInfo{
			Filename: filepath.Base(t.FilePath),
			Path:     t.FilePath,
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			Genre:    t.Genre,
			Year:     t.Year,
			Format:   t.Format,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"station_name": snap.StationName,
		"total_tracks": len(tracks),
		"tracks":       tracks,
	})
}

// GetQueue handles GET /api/queue  (public)
func (h *RadioHandlers) GetQueue(c *gin.Context) {
	tracks := h.svc.GetQueue(0)
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"tracks": sanitiseTracks(tracks),
	})
}

// SkipNext handles POST /api/skip/next  (protected)
func (h *RadioHandlers) SkipNext(c *gin.Context) {
	h.svc.SkipNext()
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SkipPrev handles POST /api/skip/prev  (protected)
func (h *RadioHandlers) SkipPrev(c *gin.Context) {
	if err := h.svc.SkipPrev(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// LegacyReload handles POST /playlist/reload  (protected, backwards compat)
func (h *RadioHandlers) LegacyReload(c *gin.Context) {
	slog.Info("Playlist reload requested (legacy)")
	result, err := h.svc.Reconcile()
	if err != nil {
		slog.Error("Playlist reconciliation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"removed_count":  result.RemovedCount,
		"orphaned_count": result.OrphanedCount,
		"total_tracks":   result.TotalTracks,
	})
}
