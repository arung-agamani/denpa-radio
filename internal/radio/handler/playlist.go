package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// PlaylistHandlers holds the gin route handlers for playlist endpoints.
type PlaylistHandlers struct {
	svc *service.PlaylistService
}

func NewPlaylistHandlers(svc *service.PlaylistService) *PlaylistHandlers {
	return &PlaylistHandlers{svc: svc}
}

// List handles GET /api/playlists
func (h *PlaylistHandlers) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "playlists": h.svc.List()})
}

// GetByID handles GET /api/playlists/:id
func (h *PlaylistHandlers) GetByID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	pl, tag, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "tag": tag, "playlist": pl})
}

// Create handles POST /api/playlists  (protected)
func (h *PlaylistHandlers) Create(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
		Tag  string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	pl, err := h.svc.Create(body.Name, body.Tag)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"status": "ok", "playlist": pl})
}

// Update handles PUT /api/playlists/:id  (protected)
func (h *PlaylistHandlers) Update(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	var body struct {
		Name *string `json:"name"`
		Tag  *string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	pl, err := h.svc.Update(id, body.Name, body.Tag)
	if err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		} else if isValidationError(err) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "playlist": pl})
}

// Delete handles DELETE /api/playlists/:id  (protected)
func (h *PlaylistHandlers) Delete(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	if err := h.svc.Delete(id); err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": fmt.Sprintf("playlist %d deleted", id)})
}

// AddTrack handles POST /api/playlists/:id/tracks  (protected)
func (h *PlaylistHandlers) AddTrack(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	var body struct {
		TrackID  *int64  `json:"trackId"`
		Checksum *string `json:"checksum"`
		FilePath *string `json:"filePath"`
		Index    *int    `json:"index"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	track, pl, err := h.svc.AddTrack(service.AddTrackInput{
		PlaylistID: plID,
		TrackID:    body.TrackID,
		Checksum:   body.Checksum,
		FilePath:   body.FilePath,
		Index:      body.Index,
	})
	if err != nil {
		status := http.StatusBadRequest
		if isNotFound(err) {
			status = http.StatusNotFound
		} else if isForbidden(err) {
			status = http.StatusForbidden
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "track": sanitiseTrack(track), "playlist": pl})
}

// RemoveTrack handles DELETE /api/playlists/:playlistId/tracks/:trackId  (protected)
func (h *PlaylistHandlers) RemoveTrack(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	trackID, err := parseID(c.Param("trackId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid track ID"})
		return
	}
	removed, pl, err := h.svc.RemoveTrack(plID, trackID)
	if err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "removed_track": removed, "playlist": pl})
}

// MoveTrack handles POST /api/playlists/:id/tracks/move  (protected)
func (h *PlaylistHandlers) MoveTrack(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	var body struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	pl, err := h.svc.MoveTrack(plID, body.From, body.To)
	if err != nil {
		status := http.StatusBadRequest
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "playlist": pl})
}

// Shuffle handles POST /api/playlists/:id/shuffle  (protected)
func (h *PlaylistHandlers) Shuffle(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	pl, err := h.svc.Shuffle(plID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "playlist": pl})
}

// Export handles GET /api/playlists/:id/export  (protected)
func (h *PlaylistHandlers) Export(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist ID"})
		return
	}
	pl, data, err := h.svc.Export(id)
	if err != nil {
		slog.Error("Failed to export playlist", "id", id, "error", err)
		if isNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to export playlist"})
		}
		return
	}
	safeName := safeFilenameRe.ReplaceAllString(pl.Name, "_")
	if safeName == "" {
		safeName = fmt.Sprintf("playlist_%d", id)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, safeName))
	c.Data(http.StatusOK, "application/json", data)
}

// Import handles POST /api/playlists/import  (protected)
func (h *PlaylistHandlers) Import(c *gin.Context) {
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, 10<<20))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "request body too large or unreadable"})
		return
	}
	pl, err := h.svc.Import(data)
	if err != nil {
		slog.Warn("Failed to import playlist", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid playlist data"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"status":   "ok",
		"message":  "playlist imported successfully",
		"playlist": pl,
	})
}
