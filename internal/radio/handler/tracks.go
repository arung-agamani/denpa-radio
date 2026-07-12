package handler

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// maxUploadSize is the maximum accepted audio file size (100 MiB).
const maxUploadSize = 100 << 20

// TrackHandlers holds the gin route handlers for the track library endpoints.
type TrackHandlers struct {
	svc         *service.TrackService
	metadataSvc *service.MetadataService
}

func NewTrackHandlers(svc *service.TrackService, metadataSvc *service.MetadataService) *TrackHandlers {
	return &TrackHandlers{svc: svc, metadataSvc: metadataSvc}
}

// List handles GET /api/tracks
func (h *TrackHandlers) List(c *gin.Context) {
	tracks := h.svc.List()
	apiresponse.OK(c, gin.H{
		"total_tracks":  len(tracks),
		"tracks":        sanitiseTracks(tracks),
		"library_total": h.svc.LibraryTotal(),
	})
}

// GetByID handles GET /api/tracks/:id
func (h *TrackHandlers) GetByID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid track ID"))
		return
	}
	track, err := h.svc.GetByID(id)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, sanitiseTrack(track))
}

// Search handles GET /api/tracks/search?q=<query>
func (h *TrackHandlers) Search(c *gin.Context) {
	q := c.Query("q")
	results, err := h.svc.Search(q)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{
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
		apiresponse.Error(c, apierror.ErrInternal("failed to scan music directory"))
		return
	}
	apiresponse.OK(c, gin.H{
		"total_tracks": len(orphaned),
		"tracks":       sanitiseTracks(orphaned),
	})
}

// Update handles PUT /api/tracks/:id  (protected)
func (h *TrackHandlers) Update(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid track ID"))
		return
	}
	var upd playlist.TrackUpdate
	if err := c.ShouldBindJSON(&upd); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	track, err := h.svc.Update(id, upd)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, sanitiseTrack(track))
}

// Delete handles DELETE /api/tracks/:id  (protected)
//
// Query parameters:
//   - deleteFromDisk=true  also remove the audio file from the filesystem.
func (h *TrackHandlers) Delete(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid track ID"))
		return
	}
	deleteFromDisk := c.Query("deleteFromDisk") == "true"
	playlistRemovals, err := h.svc.Delete(id, deleteFromDisk)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{
		"playlist_removals": playlistRemovals,
		"file_deleted":      deleteFromDisk,
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
		apiresponse.Error(c, apierror.ErrValidation("audio file must not exceed 100 MB"))
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation(`multipart field "file" is required`))
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		slog.Error("Failed to open uploaded file", "error", err)
		apiresponse.Error(c, apierror.ErrInternal("failed to read uploaded file"))
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
		slog.Warn("Track upload failed", "filename", fileHeader.Filename, "error", err)
		apiresponse.ErrorFromErr(c, err)
		return
	}

	if !result.Added {
		// Duplicate – 200 OK with added=false so the client can distinguish.
		apiresponse.OK(c, gin.H{
			"added": result.Added,
			"track": sanitiseTrack(result.Track),
		})
		return
	}

	apiresponse.Created(c, gin.H{
		"added": result.Added,
		"track": sanitiseTrack(result.Track),
	})
}

// Cover handles GET /api/tracks/:id/cover
// Serves the album art image for a track, or 404 if none is available.
func (h *TrackHandlers) Cover(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid track ID"))
		return
	}

	coverPath := h.metadataSvc.GetCoverPath(id)
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
func (h *TrackHandlers) Enrich(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid track ID"))
		return
	}

	result, err := h.metadataSvc.EnrichTrack(id)
	if err != nil {
		slog.Error("Enrichment failed", "track_id", id, "error", err)
		apiresponse.ErrorFromErr(c, err)
		return
	}

	apiresponse.OK(c, result)
}
