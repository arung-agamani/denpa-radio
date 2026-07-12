package handler

import (
	"fmt"
	"strconv"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// ChannelHandlers holds the gin route handlers for channel CRUD and runtime endpoints.
type ChannelHandlers struct {
	svc *service.ChannelService
}

// NewChannelHandlers creates a new ChannelHandlers instance.
func NewChannelHandlers(svc *service.ChannelService) *ChannelHandlers {
	return &ChannelHandlers{svc: svc}
}

// List handles GET /api/channels (public)
func (h *ChannelHandlers) List(c *gin.Context) {
	channels := h.svc.List()
	apiresponse.OK(c, gin.H{"channels": channels})
}

// Get handles GET /api/channels/:slug (public)
func (h *ChannelHandlers) Get(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	detail, err := h.svc.Get(slug)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, detail)
}

// GetStatus handles GET /api/channels/:slug/status (public)
func (h *ChannelHandlers) GetStatus(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	status, err := h.svc.GetStatus(slug)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	var currentTrackInfo interface{}
	if status.CurrentTrack != nil {
		currentTrackInfo = sanitiseTrack(status.CurrentTrack)
	}
	apiresponse.OK(c, gin.H{
		"active_tag":      status.ActiveTag,
		"active_playlist": status.ActivePlaylist,
		"client_count":    status.ClientCount,
		"max_clients":     status.MaxClients,
		"timezone":        status.Timezone,
		"server_time":     status.ServerTime,
		"current_track":   currentTrackInfo,
	})
}

// GetQueue handles GET /api/channels/:slug/queue (public)
func (h *ChannelHandlers) GetQueue(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	n := 0
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	tracks, err := h.svc.GetQueue(slug, n)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"tracks": sanitiseTracks(tracks)})
}

// Create handles POST /api/channels (protected)
func (h *ChannelHandlers) Create(c *gin.Context) {
	var body service.CreateChannelBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	if body.Name == "" {
		apiresponse.Error(c, apierror.ErrValidation("name is required"))
		return
	}
	if body.Slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	summary, err := h.svc.Create(body)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.Created(c, summary)
}

// Update handles PUT /api/channels/:slug (protected)
func (h *ChannelHandlers) Update(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	var body service.UpdateChannelBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	summary, err := h.svc.Update(slug, body)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, summary)
}

// Delete handles DELETE /api/channels/:slug (protected)
func (h *ChannelHandlers) Delete(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	if err := h.svc.Delete(slug); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"message": fmt.Sprintf("channel %s deleted", slug)})
}

// AssignPlaylistToTag handles PUT /api/channels/:slug/master/:tag (protected)
func (h *ChannelHandlers) AssignPlaylistToTag(c *gin.Context) {
	slug := c.Param("slug")
	tag := c.Param("tag")
	if slug == "" || tag == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug and tag are required"))
		return
	}
	var body struct {
		PlaylistID int64 `json:"playlistId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	if err := h.svc.AssignPlaylistToTag(slug, tag, body.PlaylistID); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"message": fmt.Sprintf("playlist %d assigned to tag %s", body.PlaylistID, tag)})
}

// RemovePlaylistFromTag handles DELETE /api/channels/:slug/master/:tag/:playlistId (protected)
func (h *ChannelHandlers) RemovePlaylistFromTag(c *gin.Context) {
	slug := c.Param("slug")
	tag := c.Param("tag")
	plID, err := parseID(c.Param("playlistId"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	if err := h.svc.RemovePlaylistFromTag(slug, tag, plID); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"message": fmt.Sprintf("playlist %d removed from tag %s", plID, tag)})
}

// SetTimeSlots handles PUT /api/channels/:slug/timeslots (protected)
func (h *ChannelHandlers) SetTimeSlots(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	var body struct {
		TimeSlots []struct {
			Tag       string `json:"tag"`
			Label     string `json:"label"`
			StartHour int    `json:"startHour"`
			EndHour   int    `json:"endHour"`
		} `json:"timeSlots"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}

	slots := make([]playlist.TimeSlot, len(body.TimeSlots))
	for i, s := range body.TimeSlots {
		slots[i] = playlist.TimeSlot{
			Tag:       playlist.TimeTag(s.Tag),
			Label:     s.Label,
			StartHour: s.StartHour,
			EndHour:   s.EndHour,
		}
	}

	result, err := h.svc.SetTimeSlots(slug, slots)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{
		"message":   "time slots updated",
		"timeSlots": result.TimeSlots,
	})
}

// SkipNext handles POST /api/channels/:slug/skip/next (protected)
func (h *ChannelHandlers) SkipNext(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	if err := h.svc.SkipNext(slug); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{})
}

// SkipPrev handles POST /api/channels/:slug/skip/prev (protected)
func (h *ChannelHandlers) SkipPrev(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		apiresponse.Error(c, apierror.ErrValidation("slug is required"))
		return
	}
	if err := h.svc.SkipPrev(slug); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{})
}
