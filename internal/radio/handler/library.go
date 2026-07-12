package handler

import (
	"log/slog"
	"strings"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// LibraryHandlers holds the gin route handlers for library-wide operations.
type LibraryHandlers struct {
	svc *service.LibraryService
}

func NewLibraryHandlers(svc *service.LibraryService) *LibraryHandlers {
	return &LibraryHandlers{svc: svc}
}

// Scan handles POST /api/tracks/scan  (protected)
func (h *LibraryHandlers) Scan(c *gin.Context) {
	slog.Info("Track library scan requested", "remote", c.ClientIP())
	added, total, err := h.svc.Scan()
	if err != nil {
		slog.Error("Library scan failed", "error", err)
		apiresponse.Error(c, apierror.ErrInternal("failed to scan music directory"))
		return
	}
	apiresponse.OK(c, gin.H{
		"newly_added":   added,
		"library_total": total,
	})
}

// RefreshMetadata handles POST /api/tracks/refresh-metadata  (protected).
func (h *LibraryHandlers) RefreshMetadata(c *gin.Context) {
	slog.Info("Metadata refresh requested", "remote", c.ClientIP())
	result, err := h.svc.RefreshMetadata()
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, result)
}

// BatchUpdate handles POST /api/library/batch-update  (protected).
func (h *LibraryHandlers) BatchUpdate(c *gin.Context) {
	var req struct {
		Filter  playlist.TrackFilter `json:"filter"`
		Updates playlist.TrackUpdate `json:"updates"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}

	updated, err := h.svc.BatchUpdate(req.Filter, req.Updates)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}

	apiresponse.OK(c, gin.H{"updated": updated})
}

// BatchUpdateCover handles POST /api/library/batch-cover  (protected).
func (h *LibraryHandlers) BatchUpdateCover(c *gin.Context) {
	filter := playlist.TrackFilter{
		Album:  strings.TrimSpace(c.PostForm("album")),
		Artist: strings.TrimSpace(c.PostForm("artist")),
		Genre:  strings.TrimSpace(c.PostForm("genre")),
	}

	fileHeader, err := c.FormFile("cover")
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("cover file is required"))
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		slog.Error("Failed to open uploaded cover", "error", err)
		apiresponse.Error(c, apierror.ErrInternal("failed to read cover file"))
		return
	}
	defer f.Close()

	updated, err := h.svc.BatchUpdateCover(filter, f, fileHeader.Filename)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}

	apiresponse.OK(c, gin.H{"updated": updated})
}

// EnrichAll handles POST /api/library/enrich  (protected)
// Triggers batch enrichment for all tracks in the library.
func (h *LibraryHandlers) EnrichAll(c *gin.Context) {
	attempted, artFound, err := h.svc.EnrichAll()
	if err != nil {
		slog.Error("Batch enrichment failed", "error", err)
		apiresponse.ErrorFromErr(c, err)
		return
	}

	apiresponse.OK(c, gin.H{
		"tracks_attempted": attempted,
		"art_found":        artFound,
	})
}

// Reconcile handles POST /api/reconcile  (protected)
func (h *LibraryHandlers) Reconcile(c *gin.Context) {
	slog.Info("Reconcile requested", "remote", c.ClientIP())
	result, err := h.svc.Reconcile(c.Query("channel"))
	if err != nil {
		slog.Error("Reconciliation failed", "error", err)
		apiresponse.Error(c, apierror.ErrInternal("reconciliation failed"))
		return
	}
	apiresponse.OK(c, gin.H{
		"removed_count":  result.RemovedCount,
		"orphaned_count": result.OrphanedCount,
		"orphaned":       sanitiseTracks(result.Orphaned),
		"total_tracks":   result.TotalTracks,
	})
}
