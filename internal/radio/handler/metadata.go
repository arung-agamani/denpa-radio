package handler

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// MetadataHandlers holds gin route handlers for album art and enrichment endpoints.
type MetadataHandlers struct {
	svc *service.MetadataService
}

func NewMetadataHandlers(svc *service.MetadataService) *MetadataHandlers {
	return &MetadataHandlers{svc: svc}
}

// Cover handles GET /api/tracks/:id/cover
// Serves the album art image for a track, or 404 if none is available.
func (h *MetadataHandlers) Cover(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid track ID"})
		return
	}

	coverPath := h.svc.GetCoverPath(id)
	if coverPath == "" {
		c.Status(http.StatusNotFound)
		return
	}

	// Resolve relative paths against the working directory.
	absPath := coverPath
	if !filepath.IsAbs(coverPath) {
		wd, err := os.Getwd()
		if err != nil {
			slog.Error("Failed to get working directory", "error", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		absPath = filepath.Join(wd, coverPath)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		slog.Warn("Cover art file not found on disk", "path", absPath, "track_id", id)
		c.Status(http.StatusNotFound)
		return
	}

	// Set content type based on extension.
	ext := strings.ToLower(filepath.Ext(absPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".gif":
		c.Header("Content-Type", "image/gif")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	}

	c.Header("Cache-Control", "public, max-age=86400") // cache for 24 hours
	http.ServeFile(c.Writer, c.Request, absPath)
}

// Enrich handles POST /api/tracks/:id/enrich  (protected)
// Triggers external metadata enrichment for a single track.
func (h *MetadataHandlers) Enrich(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid track ID"})
		return
	}

	result, err := h.svc.EnrichTrack(id)
	if err != nil {
		slog.Error("Enrichment failed", "track_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"result": result,
	})
}

// EnrichAll handles POST /api/library/enrich  (protected)
// Triggers batch enrichment for all tracks in the library.
func (h *MetadataHandlers) EnrichAll(c *gin.Context) {
	attempted, artFound, err := h.svc.EnrichAll()
	if err != nil {
		slog.Error("Batch enrichment failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "ok",
		"tracks_attempted": attempted,
		"art_found":        artFound,
	})
}
