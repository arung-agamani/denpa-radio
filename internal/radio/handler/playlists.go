package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/apiresponse"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// PlaylistHandlers holds the gin route handlers for playlist and master
// playlist endpoints.
type PlaylistHandlers struct {
	svc      *service.PlaylistService
	masterSvc *service.MasterService
}

func NewPlaylistHandlers(svc *service.PlaylistService, masterSvc *service.MasterService) *PlaylistHandlers {
	return &PlaylistHandlers{svc: svc, masterSvc: masterSvc}
}

// --- Playlist methods (from playlist.go) ---

// List handles GET /api/playlists
func (h *PlaylistHandlers) List(c *gin.Context) {
	apiresponse.OK(c, gin.H{"playlists": h.svc.List()})
}

// GetByID handles GET /api/playlists/:id
func (h *PlaylistHandlers) GetByID(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	pl, tag, err := h.svc.GetByID(id)
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"tag": tag, "playlist": pl})
}

// Create handles POST /api/playlists  (protected)
func (h *PlaylistHandlers) Create(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
		Tag  string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	pl, err := h.svc.Create(body.Name, body.Tag, c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.Created(c, pl)
}

// Update handles PUT /api/playlists/:id  (protected)
func (h *PlaylistHandlers) Update(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	var body struct {
		Name *string `json:"name"`
		Tag  *string `json:"tag"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	pl, err := h.svc.Update(id, body.Name, body.Tag, c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, pl)
}

// Delete handles DELETE /api/playlists/:id  (protected)
func (h *PlaylistHandlers) Delete(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	if err := h.svc.Delete(id, c.Query("channel")); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"message": fmt.Sprintf("playlist %d deleted", id)})
}

// AddTrack handles POST /api/playlists/:id/tracks  (protected)
func (h *PlaylistHandlers) AddTrack(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	var body struct {
		TrackID  *int64  `json:"trackId"`
		Checksum *string `json:"checksum"`
		FilePath *string `json:"filePath"`
		Index    *int    `json:"index"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	track, pl, err := h.svc.AddTrack(service.AddTrackInput{
		PlaylistID: plID,
		TrackID:    body.TrackID,
		Checksum:   body.Checksum,
		FilePath:   body.FilePath,
		Index:      body.Index,
	}, c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"track": sanitiseTrack(track), "playlist": pl})
}

// RemoveTrack handles DELETE /api/playlists/:playlistId/tracks/:trackId  (protected)
func (h *PlaylistHandlers) RemoveTrack(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	trackID, err := parseID(c.Param("trackId"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid track ID"))
		return
	}
	removed, pl, err := h.svc.RemoveTrack(plID, trackID, c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"removed_track": removed, "playlist": pl})
}

// MoveTrack handles POST /api/playlists/:id/tracks/move  (protected)
func (h *PlaylistHandlers) MoveTrack(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	var body struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	pl, err := h.svc.MoveTrack(plID, body.From, body.To, c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, pl)
}

// Shuffle handles POST /api/playlists/:id/shuffle  (protected)
func (h *PlaylistHandlers) Shuffle(c *gin.Context) {
	plID, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	pl, err := h.svc.Shuffle(plID, c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, pl)
}

// Export handles GET /api/playlists/:id/export  (protected)
func (h *PlaylistHandlers) Export(c *gin.Context) {
	id, err := parseID(c.Param("id"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	pl, data, err := h.svc.Export(id)
	if err != nil {
		slog.Error("Failed to export playlist", "id", id, "error", err)
		if isNotFound(err) {
			apiresponse.ErrorFromErr(c, err)
		} else {
			apiresponse.Error(c, apierror.ErrInternal("failed to export playlist"))
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
		apiresponse.Error(c, apierror.ErrValidation("request body too large or unreadable"))
		return
	}
	pl, err := h.svc.Import(data, c.Query("channel"))
	if err != nil {
		slog.Warn("Failed to import playlist", "error", err)
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist data"))
		return
	}
	apiresponse.Created(c, gin.H{
		"message":  "playlist imported successfully",
		"playlist": pl,
	})
}

// --- Master playlist methods (from master.go) ---

// Get handles GET /api/master
func (h *PlaylistHandlers) Get(c *gin.Context) {
	snap, err := h.masterSvc.Get(c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{
		"active_tag":         snap.ActiveTag,
		"active_playlist_id": snap.ActivePlaylistID,
		"total_tracks":       snap.TotalTracks,
		"tags":               snap.Tags,
		"time_slots":         snap.TimeSlots,
	})
}

// AssignPlaylistToTag handles PUT /api/master/:tag  (protected)
func (h *PlaylistHandlers) AssignPlaylistToTag(c *gin.Context) {
	tagStr := c.Param("tag")
	var body struct {
		PlaylistID int64 `json:"playlistId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid request body"))
		return
	}
	if err := h.masterSvc.AssignPlaylistToTag(c.Query("channel"), body.PlaylistID, tagStr); err != nil {
		slog.Error("Failed to assign playlist to tag", "error", err)
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"message": fmt.Sprintf("playlist %d assigned to tag %s", body.PlaylistID, tagStr)})
}

// RemovePlaylistFromTag handles DELETE /api/master/:tag/:playlistId  (protected)
func (h *PlaylistHandlers) RemovePlaylistFromTag(c *gin.Context) {
	tagStr := c.Param("tag")
	plID, err := parseID(c.Param("playlistId"))
	if err != nil {
		apiresponse.Error(c, apierror.ErrValidation("invalid playlist ID"))
		return
	}
	if err := h.masterSvc.RemovePlaylistFromTag(c.Query("channel"), tagStr, plID); err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"message": fmt.Sprintf("playlist %d removed from tag %s", plID, tagStr)})
}

// GetTimeSlots handles GET /api/timeslots
func (h *PlaylistHandlers) GetTimeSlots(c *gin.Context) {
	slots, err := h.masterSvc.GetTimeSlots(c.Query("channel"))
	if err != nil {
		apiresponse.ErrorFromErr(c, err)
		return
	}
	apiresponse.OK(c, gin.H{"timeSlots": slots})
}

// SetTimeSlots handles PUT /api/timeslots  (protected)
func (h *PlaylistHandlers) SetTimeSlots(c *gin.Context) {
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

	if err := h.masterSvc.SetTimeSlots(c.Query("channel"), slots); err != nil {
		slog.Error("Failed to set time slots", "error", err)
		apiresponse.ErrorFromErr(c, err)
		return
	}

	apiresponse.OK(c, gin.H{
		"message":   "time slots updated",
		"timeSlots": slots,
	})
}
