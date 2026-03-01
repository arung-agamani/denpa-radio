package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// maxUploadSize is the maximum accepted audio file size (100 MiB).
const maxUploadSize = 100 << 20

// TrackHandlers holds the gin route handlers for the track library endpoints.
type TrackHandlers struct {
	svc *service.TrackService
}

func NewTrackHandlers(svc *service.TrackService) *TrackHandlers {
	return &TrackHandlers{svc: svc}
}

// List handles GET /api/tracks
func (h *TrackHandlers) List(c *gin.Context) {
	tracks := h.svc.List()
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"total_tracks":  len(tracks),
		"tracks":        sanitiseTracks(tracks),
		"library_total": h.svc.LibraryTotal(),
	})
}

// GetByID handles GET /api/tracks/:id
func (h *TrackHandlers) GetByID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid track ID"})
		return
	}
	track, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "track": sanitiseTrack(track)})
}

// Search handles GET /api/tracks/search?q=<query>
func (h *TrackHandlers) Search(c *gin.Context) {
	q := c.Query("q")
	results, err := h.svc.Search(q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"query":        q,
		"total_tracks": len(results),
		"tracks":       sanitiseTracks(results),
	})
}

// ListOrphaned handles GET /api/tracks/orphaned  (protected)
func (h *TrackHandlers) ListOrphaned(c *gin.Context) {
	orphaned, err := h.svc.ListOrphaned()
	if err != nil {
		slog.Error("Failed to find orphaned tracks", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to scan music directory"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":       "ok",
		"total_tracks": len(orphaned),
		"tracks":       sanitiseTracks(orphaned),
	})
}

// Update handles PUT /api/tracks/:id  (protected)
func (h *TrackHandlers) Update(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid track ID"})
		return
	}
	var upd playlist.TrackUpdate
	if err := c.ShouldBindJSON(&upd); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid request body"})
		return
	}
	track, err := h.svc.Update(id, upd)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "track": sanitiseTrack(track)})
}

// Delete handles DELETE /api/tracks/:id  (protected)
//
// Query parameters:
//   - deleteFromDisk=true  also remove the audio file from the filesystem.
func (h *TrackHandlers) Delete(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "error": "invalid track ID"})
		return
	}
	deleteFromDisk := c.Query("deleteFromDisk") == "true"
	playlistRemovals, err := h.svc.Delete(id, deleteFromDisk)
	if err != nil {
		status := http.StatusInternalServerError
		if isNotFound(err) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"status": "error", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":            "ok",
		"message":           gin.H{"id": id, "playlist_removals": playlistRemovals},
		"playlist_removals": playlistRemovals,
		"file_deleted":      deleteFromDisk,
	})
}

// Scan handles POST /api/tracks/scan  (protected)
func (h *TrackHandlers) Scan(c *gin.Context) {
	slog.Info("Track library scan requested", "remote", c.ClientIP())
	added, total, err := h.svc.Scan()
	if err != nil {
		slog.Error("Library scan failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "error": "failed to scan music directory"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"newly_added":   added,
		"library_total": total,
	})
}

// Upload handles POST /api/tracks/upload  (protected)
//
// Accepts a multipart/form-data request with a single field named "file".
// The uploaded audio file is saved to the music directory, its metadata is
// read, and the track is registered in the library. If the file is a duplicate
// (same content hash), the existing track record is returned with added=false.
//
// Max upload size: 100 MiB.
func (h *TrackHandlers) Upload(c *gin.Context) {
	// Cap the request body before the multipart parser reads anything.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	if err := c.Request.ParseMultipartForm(maxUploadSize); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"status": "error",
			"error": gin.H{
				"code":    "FILE_TOO_LARGE",
				"message": "audio file must not exceed 100 MB",
			},
		})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error": gin.H{
				"code":    "MISSING_FILE",
				"message": "multipart field \"file\" is required",
			},
		})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		slog.Error("Failed to open uploaded file", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status": "error",
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to read uploaded file",
			},
		})
		return
	}
	defer f.Close()

	slog.Info("Audio upload received",
		"remote", c.ClientIP(),
		"filename", fileHeader.Filename,
		"size_bytes", fileHeader.Size,
	)

	meta := service.UploadMeta{
		Title:    strings.TrimSpace(c.PostForm("title")),
		Artist:   strings.TrimSpace(c.PostForm("artist")),
		Album:    strings.TrimSpace(c.PostForm("album")),
		Genre:    strings.TrimSpace(c.PostForm("genre")),
		Optimize: c.DefaultPostForm("optimize", "true") == "true",
	}

	result, err := h.svc.Upload(fileHeader.Filename, f, meta)
	if err != nil {
		code := "UPLOAD_FAILED"
		status := http.StatusInternalServerError
		if containsAny(err.Error(), "unsupported audio format") {
			code = "UNSUPPORTED_FORMAT"
			status = http.StatusUnprocessableEntity
		} else if containsAny(err.Error(), "outside the music directory") {
			code = "FORBIDDEN"
			status = http.StatusForbidden
		} else if containsAny(err.Error(), "library not initialised") {
			code = "LIBRARY_NOT_READY"
			status = http.StatusServiceUnavailable
		}
		slog.Warn("Track upload failed", "filename", fileHeader.Filename, "error", err)
		c.JSON(status, gin.H{
			"status": "error",
			"error": gin.H{
				"code":    code,
				"message": err.Error(),
			},
		})
		return
	}

	httpStatus := http.StatusCreated
	if !result.Added {
		// Duplicate – 200 OK with added=false so the client can distinguish.
		httpStatus = http.StatusOK
	}

	c.JSON(httpStatus, gin.H{
		"status": "ok",
		"added":  result.Added,
		"track":  sanitiseTrack(result.Track),
	})
}
