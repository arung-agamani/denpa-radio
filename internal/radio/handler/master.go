package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// MasterHandlers holds the gin route handlers for master playlist endpoints.
type MasterHandlers struct {
	svc *service.MasterService
}

func NewMasterHandlers(svc *service.MasterService) *MasterHandlers {
	return &MasterHandlers{svc: svc}
}

// Get handles GET /api/master
func (h *MasterHandlers) Get(c *gin.Context) {
	snap := h.svc.Get()
	c.JSON(http.StatusOK, gin.H{
		"status":             "ok",
		"active_tag":         snap.ActiveTag,
		"active_playlist_id": snap.ActivePlaylistID,
		"total_tracks":       snap.TotalTracks,
		"tags":               snap.Tags,
		"time_slots":         snap.TimeSlots,
	})
}

// AssignPlaylistToTag handles PUT /api/master/:tag  (protected)
func (h *MasterHandlers) AssignPlaylistToTag(c *gin.Context) {
	tagStr := c.Param("tag")
	var body struct {
		PlaylistID int64 `json:"playlistId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	if err := h.svc.AssignPlaylistToTag(body.PlaylistID, tagStr); err != nil {
		slog.Error("Failed to assign playlist to tag", "error", err)
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		} else if isValidationError(err) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("playlist %d assigned to tag %s", body.PlaylistID, tagStr),
	})
}

// RemovePlaylistFromTag handles DELETE /api/master/:tag/:playlistId  (protected)
func (h *MasterHandlers) RemovePlaylistFromTag(c *gin.Context) {
	tagStr := c.Param("tag")
	plID, err := parseID(c.Param("playlistId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	if err := h.svc.RemovePlaylistFromTag(tagStr, plID); err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		} else if isValidationError(err) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": fmt.Sprintf("playlist %d removed from tag %s", plID, tagStr),
	})
}

// GetTimeSlots handles GET /api/timeslots
func (h *MasterHandlers) GetTimeSlots(c *gin.Context) {
	slots := h.svc.GetTimeSlots()
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timeSlots": slots,
	})
}

// SetTimeSlots handles PUT /api/timeslots  (protected)
func (h *MasterHandlers) SetTimeSlots(c *gin.Context) {
	var body struct {
		TimeSlots []struct {
			Tag       string `json:"tag"`
			Label     string `json:"label"`
			StartHour int    `json:"startHour"`
			EndHour   int    `json:"endHour"`
		} `json:"timeSlots"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
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

	if err := h.svc.SetTimeSlots(slots); err != nil {
		slog.Error("Failed to set time slots", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"message":   "time slots updated",
		"timeSlots": slots,
	})
}
