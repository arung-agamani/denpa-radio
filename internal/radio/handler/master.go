package handler

import (
	"fmt"
	"log/slog"
	"net/http"

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
