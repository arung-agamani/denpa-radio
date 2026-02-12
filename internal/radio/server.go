package radio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/auth"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// maxBodySize is the default maximum request body size (1 MB).
const maxBodySize = 1 << 20

// safeFilenameRe matches characters that are safe in Content-Disposition filenames.
var safeFilenameRe = regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)

type Server struct {
	config      *config.Config
	master      *playlist.MasterPlaylist
	store       *playlist.Store
	scheduler   *playlist.Scheduler
	broadcaster *Broadcaster
	auth        *auth.Auth
	httpServer  *http.Server
}

// securityHeaders is middleware that adds standard security headers to every
// response. These mitigate clickjacking, MIME-sniffing, XSS reflection, and
// information leakage.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; media-src 'self'; connect-src 'self'; font-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func NewServer(cfg *config.Config) *Server {
	// Initialize the playlist store.
	store, err := playlist.NewStore(cfg.PlaylistFile)
	if err != nil {
		slog.Error("Failed to create playlist store", "error", err)
		panic(err)
	}

	var master *playlist.MasterPlaylist

	if store.Exists() {
		// Load existing playlists from disk.
		master, err = store.Load()
		if err != nil {
			slog.Warn("Failed to load saved playlists, will create default",
				"error", err)
			master = nil
		} else {
			slog.Info("Loaded saved playlists from disk")
		}
	}

	// Apply timezone from config if the loaded master doesn't already have one
	// persisted (env var acts as a default; persisted value takes precedence).
	if master != nil && master.Timezone() == "" && cfg.Timezone != "" {
		if tzErr := master.SetTimezone(cfg.Timezone); tzErr != nil {
			slog.Warn("Invalid TIMEZONE from config, falling back to UTC",
				"timezone", cfg.Timezone, "error", tzErr)
		}
	}

	if master == nil {
		// First run or load failure: scan music directory and create a default playlist.
		master = playlist.NewMasterPlaylist()

		// Apply timezone from config on first run.
		if cfg.Timezone != "" {
			if tzErr := master.SetTimezone(cfg.Timezone); tzErr != nil {
				slog.Warn("Invalid TIMEZONE from config, falling back to UTC",
					"timezone", cfg.Timezone, "error", tzErr)
			}
		}

		defaultPl, err := playlist.BuildDefaultPlaylistWithLibrary(cfg.MusicDir, master.Library)
		if err != nil {
			slog.Warn("Failed to build default playlist from music directory",
				"error", err)
			// Create an empty master playlist so the server can still start.
			defaultPl = playlist.NewPlaylist("Default Playlist", playlist.CurrentTimeTag())
			defaultPl.SetLibrary(master.Library)
		}

		tag := defaultPl.Tag
		if err := master.AssignPlaylist(tag, defaultPl); err != nil {
			slog.Error("Failed to assign default playlist", "error", err)
		}

		// Set the active tag to whatever the current time dictates.
		master.SetActiveTag(playlist.CurrentTimeTag())

		// Save the initial state.
		if saveErr := store.Save(master); saveErr != nil {
			slog.Error("Failed to save initial playlist", "error", saveErr)
		}
	} else {
		// Existing master loaded — scan the music directory to pick up any
		// new files and register them in the library with stable IDs.
		if master.Library != nil {
			_, added, scanErr := playlist.ScanIntoLibrary(cfg.MusicDir, master.Library)
			if scanErr != nil {
				slog.Warn("Failed to scan music directory into library", "error", scanErr)
			} else if added > 0 {
				slog.Info("Discovered new tracks during startup scan",
					"newly_added", added,
					"library_total", master.Library.Count(),
				)
				// Save so the new library entries are persisted.
				if saveErr := store.Save(master); saveErr != nil {
					slog.Error("Failed to save after startup scan", "error", saveErr)
				}
			}
		}
	}

	// Set the active tag based on current time.
	master.ResolveActiveTag()

	// Create broadcaster using the new playlist system.
	encoder := ffmpeg.NewEncoder(cfg.Bitrate, cfg.SampleRate, cfg.Channels)
	broadcaster := NewBroadcaster(nil, encoder)
	broadcaster.SetMasterPlaylist(master)

	// Initialize auth.
	authInstance := auth.New(auth.Config{
		Username:           cfg.DJUsername,
		Password:           cfg.DJPassword,
		JWTSecret:          cfg.JWTSecret,
		TokenTTL:           24 * time.Hour,
		MaxLoginAttempts:   5,
		LoginWindowSeconds: 900, // 15 minutes
	})

	s := &Server{
		config:      cfg,
		master:      master,
		store:       store,
		broadcaster: broadcaster,
		auth:        authInstance,
	}

	// Create the scheduler.
	s.scheduler = playlist.NewScheduler(master, func(event playlist.SchedulerEvent) {
		slog.Info("Scheduler triggered playlist switch",
			"previous_tag", event.PreviousTag,
			"new_tag", event.NewTag,
		)
		if event.Playlist != nil {
			slog.Info("Switching to playlist",
				"playlist_name", event.Playlist.Name,
				"playlist_id", event.Playlist.ID,
			)
		}
	}, 1*time.Minute)

	streamHandler := NewStreamHandler(broadcaster, cfg.StationName, cfg.MaxClients)

	mux := http.NewServeMux()

	// --- Streaming & public status endpoints (no auth) ---
	mux.Handle("/stream", streamHandler)
	mux.HandleFunc("/health", s.healthHandler)

	// --- Public API endpoints (no auth) ---
	mux.HandleFunc("GET /api/status", s.statusHandler)
	mux.HandleFunc("GET /api/tracks", s.apiListTracks)
	mux.HandleFunc("GET /api/tracks/{id}", s.apiGetTrack)
	mux.HandleFunc("GET /api/tracks/search", s.apiSearchTracks)
	mux.HandleFunc("GET /api/playlists", s.apiListPlaylists)
	mux.HandleFunc("GET /api/playlists/{id}", s.apiGetPlaylist)
	mux.HandleFunc("GET /api/master", s.apiGetMasterPlaylist)
	mux.HandleFunc("GET /api/scheduler/status", s.apiSchedulerStatus)
	mux.HandleFunc("GET /api/timezone", s.apiGetTimezone)

	// --- Auth endpoint (no auth required) ---
	mux.HandleFunc("POST /api/auth/login", s.apiLogin)
	mux.HandleFunc("GET /api/auth/verify", s.auth.MiddlewareFunc(s.apiVerifyToken))

	// --- Legacy endpoints (backwards compat, no auth) ---
	mux.HandleFunc("/status", s.statusHandler)
	mux.HandleFunc("/playlist", s.legacyPlaylistHandler)

	// --- Protected management endpoints (JWT required) ---

	// Legacy reload
	mux.HandleFunc("POST /playlist/reload", s.auth.MiddlewareFunc(s.legacyPlaylistReloadHandler))

	// Track library management
	mux.HandleFunc("GET /api/tracks/orphaned", s.auth.MiddlewareFunc(s.apiListOrphanedTracks))
	mux.HandleFunc("PUT /api/tracks/{id}", s.auth.MiddlewareFunc(s.apiUpdateTrack))
	mux.HandleFunc("DELETE /api/tracks/{id}", s.auth.MiddlewareFunc(s.apiDeleteTrack))
	mux.HandleFunc("POST /api/tracks/scan", s.auth.MiddlewareFunc(s.apiScanTracks))

	// Playlist CRUD
	mux.HandleFunc("POST /api/playlists", s.auth.MiddlewareFunc(s.apiCreatePlaylist))
	mux.HandleFunc("PUT /api/playlists/{id}", s.auth.MiddlewareFunc(s.apiUpdatePlaylist))
	mux.HandleFunc("DELETE /api/playlists/{id}", s.auth.MiddlewareFunc(s.apiDeletePlaylist))

	// Playlist track manipulation
	mux.HandleFunc("POST /api/playlists/{id}/tracks", s.auth.MiddlewareFunc(s.apiAddTrackToPlaylist))
	mux.HandleFunc("DELETE /api/playlists/{playlistId}/tracks/{trackId}", s.auth.MiddlewareFunc(s.apiRemoveTrackFromPlaylist))
	mux.HandleFunc("POST /api/playlists/{id}/tracks/move", s.auth.MiddlewareFunc(s.apiMoveTrackInPlaylist))
	mux.HandleFunc("POST /api/playlists/{id}/shuffle", s.auth.MiddlewareFunc(s.apiShufflePlaylist))

	// Playlist export/import
	mux.HandleFunc("GET /api/playlists/{id}/export", s.auth.MiddlewareFunc(s.apiExportPlaylist))
	mux.HandleFunc("POST /api/playlists/import", s.auth.MiddlewareFunc(s.apiImportPlaylist))

	// Master playlist management
	mux.HandleFunc("PUT /api/master/{tag}", s.auth.MiddlewareFunc(s.apiAssignPlaylistToTag))
	mux.HandleFunc("DELETE /api/master/{tag}/{playlistId}", s.auth.MiddlewareFunc(s.apiRemovePlaylistFromTag))

	// Reconcile / hot-reload
	mux.HandleFunc("POST /api/reconcile", s.auth.MiddlewareFunc(s.apiReconcile))

	// Timezone management
	mux.HandleFunc("PUT /api/timezone", s.auth.MiddlewareFunc(s.apiSetTimezone))

	// --- SPA static file serving (must be last) ---
	mux.HandleFunc("/", s.spaHandler)

	s.httpServer = &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        securityHeaders(mux),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   0, // No timeout for streaming
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB max header size
	}

	return s
}

func (s *Server) Start(ctx context.Context) error {
	// Start the scheduler in the background.
	go s.scheduler.Start(ctx)

	// Start the broadcaster in the background.
	go s.broadcaster.Start(ctx)

	errChan := make(chan error, 1)

	go func() {
		slog.Info("HTTP server starting", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(shutdownCtx)
	}
}

// ---------------------------------------------------------------------------
// SPA static file serving
// ---------------------------------------------------------------------------

// spaHandler serves the built Svelte frontend. For any path that doesn't match
// an existing file under WebDir it returns index.html so the client-side router
// can handle the route.
func (s *Server) spaHandler(w http.ResponseWriter, r *http.Request) {
	webDir := s.config.WebDir

	// Resolve webDir to an absolute path for containment checks.
	absWebDir, err := filepath.Abs(webDir)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server configuration error")
		return
	}

	// Determine the file path requested.
	reqPath := r.URL.Path
	if reqPath == "/" {
		reqPath = "/index.html"
	}

	// Clean the path and join with webDir.
	cleanPath := filepath.Clean(reqPath)
	filePath := filepath.Join(absWebDir, cleanPath)

	// Resolve to absolute and verify the result is still within webDir.
	// This prevents path traversal attacks (e.g. ../../etc/passwd).
	absFilePath, err := filepath.Abs(filePath)
	if err != nil || !strings.HasPrefix(absFilePath, absWebDir+string(filepath.Separator)) && absFilePath != absWebDir {
		// Path escaped webDir — treat as SPA fallback or reject.
		absFilePath = filepath.Join(absWebDir, "index.html")
	}

	// Check if the requested file exists and is not a directory.
	info, err := os.Stat(absFilePath)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, absFilePath)
		return
	}

	// SPA fallback: serve index.html for any route the frontend router handles.
	indexPath := filepath.Join(absWebDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		// No frontend build found at all. Return a helpful message without
		// leaking the server-side path.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "Frontend not built. Run 'bun run build' in the web/ directory.",
		})
		return
	}

	http.ServeFile(w, r, indexPath)
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

func (s *Server) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]interface{}{
		"status": "error",
		"error":  message,
	})
}

func (s *Server) saveState() {
	if err := s.store.Save(s.master); err != nil {
		slog.Error("Failed to save playlist state", "error", err)
	}
}

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ---------------------------------------------------------------------------
// Auth endpoints
// ---------------------------------------------------------------------------

func (s *Server) apiLogin(w http.ResponseWriter, r *http.Request) {
	// Limit login request body to 4 KB to prevent abuse.
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Basic input validation — reject empty or excessively long credentials.
	if len(body.Username) == 0 || len(body.Username) > 256 ||
		len(body.Password) == 0 || len(body.Password) > 256 {
		s.writeError(w, http.StatusBadRequest, "invalid credentials format")
		return
	}

	token, err := s.auth.Authenticate(body.Username, body.Password, r.RemoteAddr)
	if err != nil {
		slog.Warn("Failed login attempt",
			"remote", r.RemoteAddr,
			"error_type", err.Error(),
		)

		if err == auth.ErrRateLimited {
			remaining := s.auth.RemainingLockout(r.RemoteAddr)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(remaining.Seconds())))
			s.writeError(w, http.StatusTooManyRequests, "too many login attempts, please try again later")
			return
		}

		// Intentionally generic error — do not reveal whether username or
		// password was wrong.
		s.writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	slog.Info("DJ logged in", "username", body.Username, "remote", r.RemoteAddr)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"token":    token,
		"username": body.Username,
	})
}

func (s *Server) apiVerifyToken(w http.ResponseWriter, r *http.Request) {
	// If we reach this handler the middleware already validated the token.
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "token is valid",
	})
}

// ---------------------------------------------------------------------------
// Public endpoints
// ---------------------------------------------------------------------------

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	currentTrackPath := s.broadcaster.CurrentTrack()
	trackName := "none"
	if currentTrackPath != "" {
		trackName = filepath.Base(currentTrackPath)
	}

	activeTag := s.master.ActiveTag()
	activePl, _ := s.master.ActivePlaylist()
	var activePlaylistName string
	var activePlaylistID *int64
	if activePl != nil {
		activePlaylistName = activePl.Name
		activePlaylistID = &activePl.ID
	}

	// Build a sanitised track info struct that does NOT expose the server
	// file-system path to public consumers.
	var currentTrackInfo interface{}
	if currentTrackPath != "" {
		// Try library first.
		if s.master.Library != nil {
			t := s.master.Library.GetByFilePath(currentTrackPath)
			if t != nil {
				currentTrackInfo = sanitiseTrack(t)
			}
		}
		// Fallback: search playlists.
		if currentTrackInfo == nil {
			for _, pl := range s.master.AllPlaylists() {
				if t, _, err := pl.FindTrackByFilePath(currentTrackPath); err == nil {
					currentTrackInfo = sanitiseTrack(t)
					break
				}
			}
		}
	}

	loc := s.master.Location()
	tz := s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"station_name":       s.config.StationName,
		"current_track":      trackName,
		"current_track_info": currentTrackInfo,
		"total_tracks":       s.master.TotalTracks(),
		"library_tracks":     s.master.LibraryTrackCount(),
		"active_clients":     s.broadcaster.ActiveClients(),
		"max_clients":        s.config.MaxClients,
		"active_tag":         activeTag,
		"active_playlist":    activePlaylistName,
		"active_playlist_id": activePlaylistID,
		"scheduler_running":  s.scheduler.Running(),
		"playlist_summary":   s.master.Summary(),
		"timezone":           tz,
		"server_time":        time.Now().In(loc).Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Legacy endpoints (backwards compatibility)
// ---------------------------------------------------------------------------

func (s *Server) legacyPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	allTracks := s.master.AllTracksDeduped()
	type legacyTrackInfo struct {
		Filename string `json:"filename"`
		Path     string `json:"path"`
		Title    string `json:"title,omitempty"`
		Artist   string `json:"artist,omitempty"`
		Album    string `json:"album,omitempty"`
		Genre    string `json:"genre,omitempty"`
		Year     int    `json:"year,omitempty"`
		Format   string `json:"format"`
	}

	tracks := make([]legacyTrackInfo, 0, len(allTracks))
	for _, t := range allTracks {
		tracks = append(tracks, legacyTrackInfo{
			Filename: filepath.Base(t.FilePath),
			Path:     t.FilePath,
			Title:    t.Title,
			Artist:   t.Artist,
			Album:    t.Album,
			Genre:    t.Genre,
			Year:     t.Year,
			Format:   t.Format,
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"station_name": s.config.StationName,
		"total_tracks": len(tracks),
		"tracks":       tracks,
	})
}

func (s *Server) legacyPlaylistReloadHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info("Playlist reload requested (legacy)")

	orphaned, removedCount, err := playlist.ReconcileTracks(s.config.MusicDir, s.master)
	if err != nil {
		slog.Error("Playlist reconciliation failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Auto-add orphaned tracks to the active playlist if there is one.
	if len(orphaned) > 0 {
		activePl, err := s.master.ActivePlaylist()
		if err == nil && activePl != nil {
			activePl.AddTracks(orphaned)
			slog.Info("Added orphaned tracks to active playlist",
				"count", len(orphaned),
				"playlist", activePl.Name,
			)
		}
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"removed_count":  removedCount,
		"orphaned_count": len(orphaned),
		"total_tracks":   s.master.TotalTracks(),
	})
}

// ---------------------------------------------------------------------------
// Track API
// ---------------------------------------------------------------------------

func (s *Server) apiListTracks(w http.ResponseWriter, r *http.Request) {
	var tracks []*playlist.Track
	if s.master.Library != nil {
		tracks = s.master.Library.List()
	} else {
		tracks = s.master.AllTracksDeduped()
	}
	sanitised := sanitiseTracks(tracks)
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"total_tracks":  len(sanitised),
		"tracks":        sanitised,
		"library_total": s.master.LibraryTrackCount(),
	})
}

func (s *Server) apiSearchTracks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if s.master.Library == nil {
		s.writeError(w, http.StatusInternalServerError, "track library not initialised")
		return
	}
	results := s.master.Library.Search(query)
	sanitised := sanitiseTracks(results)
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"query":        query,
		"total_tracks": len(sanitised),
		"tracks":       sanitised,
	})
}

func (s *Server) apiListOrphanedTracks(w http.ResponseWriter, r *http.Request) {
	orphaned, err := playlist.FindOrphanedTracks(s.config.MusicDir, s.master)
	if err != nil {
		slog.Error("Failed to find orphaned tracks", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to scan music directory")
		return
	}
	sanitised := sanitiseTracks(orphaned)
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "ok",
		"total_tracks": len(sanitised),
		"tracks":       sanitised,
	})
}

func (s *Server) apiGetTrack(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid track ID")
		return
	}

	// Look up in the library first (preferred).
	if s.master.Library != nil {
		track := s.master.Library.GetByID(id)
		if track != nil {
			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
				"track":  sanitiseTrack(track),
			})
			return
		}
	}

	// Fallback: search all playlists.
	for _, pl := range s.master.AllPlaylists() {
		if track, _, err := pl.FindTrackByID(id); err == nil {
			s.writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "ok",
				"track":  sanitiseTrack(track),
			})
			return
		}
	}

	s.writeError(w, http.StatusNotFound, fmt.Sprintf("track %d not found", id))
}

// ---------------------------------------------------------------------------
// Track Library management (protected)
// ---------------------------------------------------------------------------

func (s *Server) apiUpdateTrack(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid track ID")
		return
	}

	if s.master.Library == nil {
		s.writeError(w, http.StatusInternalServerError, "track library not initialised")
		return
	}

	var upd playlist.TrackUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	track, err := s.master.Library.Update(id, upd)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.saveState()

	slog.Info("Track metadata updated via library",
		"track_id", id,
		"title", track.Title,
	)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"track":  sanitiseTrack(track),
	})
}

func (s *Server) apiDeleteTrack(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid track ID")
		return
	}

	if s.master.Library == nil {
		s.writeError(w, http.StatusInternalServerError, "track library not initialised")
		return
	}

	track := s.master.Library.GetByID(id)
	if track == nil {
		s.writeError(w, http.StatusNotFound, fmt.Sprintf("track %d not found in library", id))
		return
	}

	// Remove from all playlists first.
	playlistRemovals := s.master.RemoveTrackFromAll(track.Checksum)

	// Remove from library.
	s.master.Library.RemoveByID(id)

	s.saveState()

	slog.Info("Track deleted from library",
		"track_id", id,
		"title", track.Title,
		"playlist_removals", playlistRemovals,
	)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":            "ok",
		"message":           fmt.Sprintf("track %d removed from library and %d playlist(s)", id, playlistRemovals),
		"playlist_removals": playlistRemovals,
	})
}

func (s *Server) apiScanTracks(w http.ResponseWriter, r *http.Request) {
	if s.master.Library == nil {
		s.writeError(w, http.StatusInternalServerError, "track library not initialised")
		return
	}

	slog.Info("Track library scan requested", "remote", r.RemoteAddr)

	_, added, err := playlist.ScanIntoLibrary(s.config.MusicDir, s.master.Library)
	if err != nil {
		slog.Error("Library scan failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to scan music directory")
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"newly_added":   added,
		"library_total": s.master.Library.Count(),
	})
}

// ---------------------------------------------------------------------------
// Playlist API
// ---------------------------------------------------------------------------

func (s *Server) apiListPlaylists(w http.ResponseWriter, r *http.Request) {
	allPls := s.master.AllPlaylists()

	type playlistSummary struct {
		ID         int64            `json:"id"`
		Name       string           `json:"name"`
		Tag        playlist.TimeTag `json:"tag"`
		TrackCount int              `json:"trackCount"`
	}

	summaries := make([]playlistSummary, 0, len(allPls))
	for _, pl := range allPls {
		summaries = append(summaries, playlistSummary{
			ID:         pl.ID,
			Name:       pl.Name,
			Tag:        pl.Tag,
			TrackCount: pl.Count(),
		})
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"playlists": summaries,
	})
}

func (s *Server) apiCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var body struct {
		Name string `json:"name"`
		Tag  string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if !playlist.IsValidTimeTag(body.Tag) {
		s.writeError(w, http.StatusBadRequest,
			"invalid tag: must be one of morning, afternoon, evening, night")
		return
	}

	tag := playlist.TimeTag(body.Tag)
	pl := playlist.NewPlaylist(body.Name, tag)

	if err := s.master.AssignPlaylist(tag, pl); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "ok",
		"playlist": pl,
	})
}

func (s *Server) apiGetPlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	pl, tag, err := s.master.FindPlaylistByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"tag":      tag,
		"playlist": pl,
	})
}

func (s *Server) apiUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	var body struct {
		Name *string `json:"name"`
		Tag  *string `json:"tag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	pl, currentTag, err := s.master.FindPlaylistByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if body.Name != nil {
		pl.Name = *body.Name
	}

	// If the tag is changing, move the playlist from the old tag to the new tag.
	if body.Tag != nil && playlist.TimeTag(*body.Tag) != currentTag {
		newTag := playlist.TimeTag(*body.Tag)
		if !playlist.IsValidTimeTag(*body.Tag) {
			s.writeError(w, http.StatusBadRequest,
				"invalid tag: must be one of morning, afternoon, evening, night")
			return
		}

		// Remove from old tag.
		if err := s.master.RemovePlaylist(currentTag, id); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Assign to new tag.
		if err := s.master.AssignPlaylist(newTag, pl); err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"playlist": pl,
	})
}

func (s *Server) apiDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	_, tag, err := s.master.FindPlaylistByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := s.master.RemovePlaylist(tag, id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("playlist %d deleted", id),
	})
}

// ---------------------------------------------------------------------------
// Playlist track manipulation
// ---------------------------------------------------------------------------

func (s *Server) apiAddTrackToPlaylist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	idStr := r.PathValue("id")
	plID, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	pl, _, err := s.master.FindPlaylistByID(plID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "playlist not found")
		return
	}

	var body struct {
		TrackID  *int64  `json:"trackId"`
		Checksum *string `json:"checksum"`
		FilePath *string `json:"filePath"`
		Index    *int    `json:"index"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	lib := s.master.Library
	var trackToAdd *playlist.Track

	// Strategy 1: Find by track ID in the library.
	if body.TrackID != nil {
		if lib != nil {
			trackToAdd = lib.GetByID(*body.TrackID)
		}
		// Fallback: search playlists (for backward compat during migration).
		if trackToAdd == nil {
			for _, existingPl := range s.master.AllPlaylists() {
				if t, _, err := existingPl.FindTrackByID(*body.TrackID); err == nil {
					trackToAdd = t
					break
				}
			}
		}
		if trackToAdd == nil {
			s.writeError(w, http.StatusNotFound, fmt.Sprintf("track %d not found", *body.TrackID))
			return
		}
	}

	// Strategy 2: Find by checksum in the library.
	if trackToAdd == nil && body.Checksum != nil {
		if lib != nil {
			trackToAdd = lib.Get(*body.Checksum)
		}
		// Fallback: search playlists.
		if trackToAdd == nil {
			for _, existingPl := range s.master.AllPlaylists() {
				if t, _, err := existingPl.FindTrackByChecksum(*body.Checksum); err == nil {
					trackToAdd = t
					break
				}
			}
		}
		if trackToAdd == nil {
			s.writeError(w, http.StatusNotFound, fmt.Sprintf("track with checksum %q not found", *body.Checksum))
			return
		}
	}

	// Strategy 3: Create from file path — restricted to the configured music
	// directory to prevent local file inclusion attacks.
	if trackToAdd == nil && body.FilePath != nil {
		if !s.isPathInsideMusicDir(*body.FilePath) {
			s.writeError(w, http.StatusForbidden, "file path must be within the music directory")
			return
		}
		t, err := playlist.NewTrackFromFile(*body.FilePath)
		if err != nil {
			slog.Warn("Failed to create track from file",
				"path", *body.FilePath,
				"error", err,
			)
			s.writeError(w, http.StatusBadRequest, "failed to create track from file")
			return
		}
		// Register in the library so it gets a stable ID.
		if lib != nil {
			t = lib.AddOrUpdate(t)
		}
		trackToAdd = t
	}

	if trackToAdd == nil {
		s.writeError(w, http.StatusBadRequest,
			"must provide one of: trackId, checksum, or filePath")
		return
	}

	if body.Index != nil {
		pl.AddTrackAt(trackToAdd, *body.Index)
	} else {
		pl.AddTrack(trackToAdd)
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"track":    sanitiseTrack(trackToAdd),
		"playlist": pl,
	})
}

func (s *Server) apiRemoveTrackFromPlaylist(w http.ResponseWriter, r *http.Request) {
	plIDStr := r.PathValue("playlistId")
	plID, err := parseID(plIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	trackIDStr := r.PathValue("trackId")
	trackID, err := parseID(trackIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid track ID")
		return
	}

	pl, _, err := s.master.FindPlaylistByID(plID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	removed, err := pl.RemoveTrackByID(trackID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":        "ok",
		"removed_track": removed,
		"playlist":      pl,
	})
}

func (s *Server) apiMoveTrackInPlaylist(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	idStr := r.PathValue("id")
	plID, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	pl, _, err := s.master.FindPlaylistByID(plID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "playlist not found")
		return
	}

	var body struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := pl.MoveTrack(body.From, body.To); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"playlist": pl,
	})
}

func (s *Server) apiShufflePlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	plID, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	pl, _, err := s.master.FindPlaylistByID(plID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	pl.Shuffle()
	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"playlist": pl,
	})
}

// ---------------------------------------------------------------------------
// Playlist export/import
// ---------------------------------------------------------------------------

func (s *Server) apiExportPlaylist(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := parseID(idStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	pl, _, err := s.master.FindPlaylistByID(id)
	if err != nil {
		s.writeError(w, http.StatusNotFound, "playlist not found")
		return
	}

	data, err := playlist.ExportPlaylist(pl)
	if err != nil {
		slog.Error("Failed to export playlist", "id", id, "error", err)
		s.writeError(w, http.StatusInternalServerError, "failed to export playlist")
		return
	}

	// Sanitize name for Content-Disposition to prevent header injection.
	// Strip everything except alphanumeric, underscore, hyphen, and dot.
	safeName := safeFilenameRe.ReplaceAllString(pl.Name, "_")
	if safeName == "" {
		safeName = fmt.Sprintf("playlist_%d", id)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s.json"`, safeName))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (s *Server) apiImportPlaylist(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 10 MB.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "request body too large or unreadable")
		return
	}

	var pl *playlist.Playlist
	if s.master.Library != nil {
		pl, err = playlist.ImportPlaylistIntoLibrary(data, s.master.Library)
	} else {
		pl, err = playlist.ImportPlaylistFromBytes(data)
	}
	if err != nil {
		slog.Warn("Failed to import playlist", "error", err)
		s.writeError(w, http.StatusBadRequest, "invalid playlist data")
		return
	}

	// Validate the tag.
	if !playlist.IsValidTimeTag(string(pl.Tag)) {
		// Default to current time tag if the imported playlist has an invalid tag.
		pl.Tag = playlist.CurrentTimeTag()
	}

	if err := s.master.AssignPlaylist(pl.Tag, pl); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status":   "ok",
		"message":  "playlist imported successfully",
		"playlist": pl,
	})
}

// ---------------------------------------------------------------------------
// Master Playlist API
// ---------------------------------------------------------------------------

func (s *Server) apiGetMasterPlaylist(w http.ResponseWriter, r *http.Request) {
	type tagInfo struct {
		Playlists []*playlist.Playlist `json:"playlists"`
		Count     int                  `json:"count"`
	}

	result := make(map[string]tagInfo)
	for _, tag := range playlist.ValidTimeTags {
		pls := s.master.GetPlaylists(tag)
		result[string(tag)] = tagInfo{
			Playlists: pls,
			Count:     len(pls),
		}
	}

	activeTag := s.master.ActiveTag()
	activePl, _ := s.master.ActivePlaylist()
	var activePlaylistID *int64
	if activePl != nil {
		activePlaylistID = &activePl.ID
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "ok",
		"active_tag":         activeTag,
		"active_playlist_id": activePlaylistID,
		"total_tracks":       s.master.TotalTracks(),
		"tags":               result,
	})
}

func (s *Server) apiAssignPlaylistToTag(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	tagStr := r.PathValue("tag")
	if !playlist.IsValidTimeTag(tagStr) {
		s.writeError(w, http.StatusBadRequest,
			"invalid tag: must be one of morning, afternoon, evening, night")
		return
	}
	tag := playlist.TimeTag(tagStr)

	var body struct {
		PlaylistID int64 `json:"playlistId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Find the playlist.
	pl, currentTag, err := s.master.FindPlaylistByID(body.PlaylistID)
	if err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// If it's already under a different tag, remove it first.
	if currentTag != tag {
		if removeErr := s.master.RemovePlaylist(currentTag, body.PlaylistID); removeErr != nil {
			slog.Warn("Failed to remove playlist from old tag during reassignment",
				"error", removeErr)
		}
	}

	// Assign to the new tag.
	if err := s.master.AssignPlaylist(tag, pl); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.saveState()
	s.scheduler.ForceCheck()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("playlist %d assigned to tag %s", body.PlaylistID, tag),
	})
}

func (s *Server) apiRemovePlaylistFromTag(w http.ResponseWriter, r *http.Request) {
	tagStr := r.PathValue("tag")
	if !playlist.IsValidTimeTag(tagStr) {
		s.writeError(w, http.StatusBadRequest,
			"invalid tag: must be one of morning, afternoon, evening, night")
		return
	}
	tag := playlist.TimeTag(tagStr)

	plIDStr := r.PathValue("playlistId")
	plID, err := parseID(plIDStr)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid playlist ID")
		return
	}

	if err := s.master.RemovePlaylist(tag, plID); err != nil {
		s.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	s.saveState()

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("playlist %d removed from tag %s", plID, tag),
	})
}

// ---------------------------------------------------------------------------
// Reconcile / hot-reload
// ---------------------------------------------------------------------------

func (s *Server) apiReconcile(w http.ResponseWriter, r *http.Request) {
	slog.Info("Reconcile requested", "remote", r.RemoteAddr)

	orphaned, removedCount, err := playlist.ReconcileTracks(s.config.MusicDir, s.master)
	if err != nil {
		slog.Error("Reconciliation failed", "error", err)
		s.writeError(w, http.StatusInternalServerError, "reconciliation failed")
		return
	}

	s.saveState()

	sanitisedOrphaned := sanitiseTracks(orphaned)
	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"removed_count":  removedCount,
		"orphaned_count": len(sanitisedOrphaned),
		"orphaned":       sanitisedOrphaned,
		"total_tracks":   s.master.TotalTracks(),
	})
}

// ---------------------------------------------------------------------------
// Scheduler API
// ---------------------------------------------------------------------------

func (s *Server) apiSchedulerStatus(w http.ResponseWriter, r *http.Request) {
	loc := s.master.Location()
	tz := s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "ok",
		"running":        s.scheduler.Running(),
		"last_tag":       s.scheduler.LastTag(),
		"time_tags":      playlist.ValidTimeTags,
		"current_tag":    playlist.CurrentTimeTagIn(loc),
		"summary":        s.master.Summary(),
		"library_tracks": s.master.LibraryTrackCount(),
		"timezone":       tz,
		"server_time":    time.Now().In(loc).Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// Timezone
// ---------------------------------------------------------------------------

func (s *Server) apiGetTimezone(w http.ResponseWriter, r *http.Request) {
	loc := s.master.Location()
	tz := s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"timezone":    tz,
		"server_time": time.Now().In(loc).Format(time.RFC3339),
	})
}

func (s *Server) apiSetTimezone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Timezone string `json:"timezone"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBodySize)).Decode(&body); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.master.SetTimezone(body.Timezone); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Force the scheduler to re-evaluate with the new timezone.
	s.scheduler.ForceCheck()

	// Persist the change.
	s.saveState()

	loc := s.master.Location()
	tz := s.master.Timezone()
	if tz == "" {
		tz = "UTC"
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"timezone":    tz,
		"server_time": time.Now().In(loc).Format(time.RFC3339),
		"active_tag":  s.master.ActiveTag(),
	})
}

// ---------------------------------------------------------------------------
// Security helpers
// ---------------------------------------------------------------------------

// isPathInsideMusicDir verifies that the given file path resolves to a
// location within the configured music directory. This prevents local file
// inclusion (LFI) attacks through the filePath parameter in the API.
func (s *Server) isPathInsideMusicDir(filePath string) bool {
	absMusicDir, err := filepath.Abs(s.config.MusicDir)
	if err != nil {
		return false
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	// Ensure the resolved path starts with the music directory.
	return strings.HasPrefix(absPath, absMusicDir+string(filepath.Separator)) || absPath == absMusicDir
}

// sanitiseTrack returns a map representation of a track with the absolute
// file-system path replaced by just the filename. This prevents leaking
// server directory structure to API consumers.
func sanitiseTrack(t *playlist.Track) map[string]interface{} {
	return map[string]interface{}{
		"id":       t.ID,
		"title":    t.Title,
		"artist":   t.Artist,
		"album":    t.Album,
		"genre":    t.Genre,
		"year":     t.Year,
		"trackNum": t.TrackNum,
		"duration": t.Duration,
		"filePath": filepath.Base(t.FilePath),
		"format":   t.Format,
		"checksum": t.Checksum,
	}
}

// sanitiseTracks applies sanitiseTrack to a slice of tracks.
func sanitiseTracks(tracks []*playlist.Track) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(tracks))
	for _, t := range tracks {
		result = append(result, sanitiseTrack(t))
	}
	return result
}
