package handler

import (
	"net/http"

	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// RadioHandlers holds the gin route handlers for station status, scheduler,
// and skip endpoints.
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

// Status handles GET /api/status
func (h *RadioHandlers) Status(c *gin.Context) {
	snap := h.svc.Status()
	var currentTrackInfo interface{}
	if snap.CurrentTrackRaw != nil {
		currentTrackInfo = sanitiseTrack(snap.CurrentTrackRaw)
	}
	apiresponse.OK(c, gin.H{
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
	apiresponse.OK(c, gin.H{
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

// GetQueue handles GET /api/queue  (public)
func (h *RadioHandlers) GetQueue(c *gin.Context) {
	tracks := h.svc.GetQueue(0)
	apiresponse.OK(c, gin.H{
		"tracks": sanitiseTracks(tracks),
	})
}

// SkipNext handles POST /api/skip/next  (protected)
func (h *RadioHandlers) SkipNext(c *gin.Context) {
	h.svc.SkipNext()
	apiresponse.OK(c, gin.H{})
}

// SkipPrev handles POST /api/skip/prev  (protected)
func (h *RadioHandlers) SkipPrev(c *gin.Context) {
	if err := h.svc.SkipPrev(); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{})
}
