package radio

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/auth"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/handler"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	"github.com/gin-gonic/gin"
)

// Server is the top-level application struct. It owns the gin engine, all
// service instances, all route handler instances, and the underlying
// http.Server.
type Server struct {
	config      *config.Config
	master      *playlist.MasterPlaylist
	store       *playlist.Store
	scheduler   *playlist.Scheduler
	broadcaster *Broadcaster
	auth        *auth.Auth
	httpServer  *http.Server

	// Services
	trackSvc    *service.TrackService
	playlistSvc *service.PlaylistService
	masterSvc   *service.MasterService
	radioSvc    *service.RadioService

	// Route handlers
	trackH    *handler.TrackHandlers
	playlistH *handler.PlaylistHandlers
	masterH   *handler.MasterHandlers
	radioH    *handler.RadioHandlers
	authH     *handler.AuthHandlers
	spaH      *handler.SPAHandler
}

func NewServer(cfg *config.Config) *Server {
	// --- Playlist store / master initialisation ---
	store, err := playlist.NewStore(cfg.PlaylistFile)
	if err != nil {
		slog.Error("Failed to create playlist store", "error", err)
		panic(err)
	}

	var master *playlist.MasterPlaylist

	if store.Exists() {
		master, err = store.Load()
		if err != nil {
			slog.Warn("Failed to load saved playlists, will create default", "error", err)
			master = nil
		} else {
			slog.Info("Loaded saved playlists from disk")
		}
	}

	if master != nil && master.Timezone() == "" && cfg.Timezone != "" {
		if tzErr := master.SetTimezone(cfg.Timezone); tzErr != nil {
			slog.Warn("Invalid TIMEZONE from config, falling back to UTC",
				"timezone", cfg.Timezone, "error", tzErr)
		}
	}

	if master == nil {
		master = playlist.NewMasterPlaylist()
		if cfg.Timezone != "" {
			if tzErr := master.SetTimezone(cfg.Timezone); tzErr != nil {
				slog.Warn("Invalid TIMEZONE from config, falling back to UTC",
					"timezone", cfg.Timezone, "error", tzErr)
			}
		}

		defaultPl, err := playlist.BuildDefaultPlaylistWithLibrary(cfg.MusicDir, master.Library)
		if err != nil {
			slog.Warn("Failed to build default playlist from music directory", "error", err)
			defaultPl = playlist.NewPlaylist("Default Playlist", playlist.CurrentTimeTag())
			defaultPl.SetLibrary(master.Library)
		}

		tag := defaultPl.Tag
		if err := master.AssignPlaylist(tag, defaultPl); err != nil {
			slog.Error("Failed to assign default playlist", "error", err)
		}
		master.SetActiveTag(playlist.CurrentTimeTag())

		if saveErr := store.Save(master); saveErr != nil {
			slog.Error("Failed to save initial playlist", "error", saveErr)
		}
	} else {
		if master.Library != nil {
			_, added, scanErr := playlist.ScanIntoLibrary(cfg.MusicDir, master.Library)
			if scanErr != nil {
				slog.Warn("Failed to scan music directory into library", "error", scanErr)
			} else if added > 0 {
				slog.Info("Discovered new tracks during startup scan",
					"newly_added", added,
					"library_total", master.Library.Count(),
				)
				if saveErr := store.Save(master); saveErr != nil {
					slog.Error("Failed to save after startup scan", "error", saveErr)
				}
			}
		}
	}

	master.ResolveActiveTag()

	// --- Broadcaster & encoder ---
	encoder := ffmpeg.NewEncoder(cfg.Bitrate, cfg.SampleRate, cfg.Channels)
	broadcaster := NewBroadcaster(nil, encoder)
	broadcaster.SetMasterPlaylist(master)

	// --- Auth ---
	authInstance := auth.New(auth.Config{
		Username:           cfg.DJUsername,
		Password:           cfg.DJPassword,
		JWTSecret:          cfg.JWTSecret,
		TokenTTL:           24 * time.Hour,
		MaxLoginAttempts:   5,
		LoginWindowSeconds: 900,
	})

	// --- Scheduler ---
	scheduler := playlist.NewScheduler(master, func(event playlist.SchedulerEvent) {
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

	// --- Services ---
	trackSvc := service.NewTrackService(master, store, cfg)
	playlistSvc := service.NewPlaylistService(master, store, cfg)
	masterSvc := service.NewMasterService(master, store, scheduler)
	radioSvc := service.NewRadioService(master, store, scheduler, broadcaster, cfg)

	// --- Route handlers ---
	trackH := handler.NewTrackHandlers(trackSvc)
	playlistH := handler.NewPlaylistHandlers(playlistSvc)
	masterH := handler.NewMasterHandlers(masterSvc)
	radioH := handler.NewRadioHandlers(radioSvc)
	authH := handler.NewAuthHandlers(authInstance)
	spaH := handler.NewSPAHandler(cfg.WebDir)

	s := &Server{
		config:      cfg,
		master:      master,
		store:       store,
		scheduler:   scheduler,
		broadcaster: broadcaster,
		auth:        authInstance,
		trackSvc:    trackSvc,
		playlistSvc: playlistSvc,
		masterSvc:   masterSvc,
		radioSvc:    radioSvc,
		trackH:      trackH,
		playlistH:   playlistH,
		masterH:     masterH,
		radioH:      radioH,
		authH:       authH,
		spaH:        spaH,
	}

	// --- Gin engine ---
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(SecurityHeadersMiddleware())

	s.registerRoutes(engine, authInstance)

	s.httpServer = &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second, // headers only; body reads (e.g. uploads) are not time-limited here
		WriteTimeout:      0,                // No timeout for streaming
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return s
}

// registerRoutes wires all routes onto the gin engine.
func (s *Server) registerRoutes(engine *gin.Engine, authInstance *auth.Auth) {
	streamHandler := NewStreamHandler(s.broadcaster, s.config.StationName, s.config.MaxClients)

	// --- Streaming (no auth) ---
	engine.GET("/stream", gin.WrapH(streamHandler))

	// --- Public non-API ---
	engine.GET("/health", s.radioH.Health)
	engine.GET("/status", s.radioH.Status)           // legacy
	engine.GET("/playlist", s.radioH.LegacyPlaylist) // legacy

	// --- Auth ---
	authGroup := engine.Group("/api/auth")
	{
		authGroup.POST("/login", s.authH.Login)
		authGroup.GET("/verify", AuthRequired(authInstance), s.authH.VerifyToken)
	}

	// --- Public API ---
	api := engine.Group("/api")
	{
		api.GET("/status", s.radioH.Status)
		api.GET("/scheduler/status", s.radioH.SchedulerStatus)
		api.GET("/timezone", s.radioH.GetTimezone)
		api.GET("/master", s.masterH.Get)

		// Literal sub-paths registered before :id to avoid routing conflicts.
		api.GET("/tracks/search", s.trackH.Search)
		api.GET("/tracks", s.trackH.List)
		api.GET("/tracks/:id", s.trackH.GetByID)

		api.GET("/playlists", s.playlistH.List)
		api.GET("/playlists/:id", s.playlistH.GetByID)
	}

	// --- Protected API (JWT required) ---
	protected := engine.Group("/api")
	protected.Use(AuthRequired(authInstance))
	{
		// Track management
		protected.GET("/tracks/orphaned", s.trackH.ListOrphaned)
		protected.PUT("/tracks/:id", s.trackH.Update)
		protected.DELETE("/tracks/:id", s.trackH.Delete)
		protected.POST("/tracks/scan", s.trackH.Scan)
		protected.POST("/tracks/upload", s.trackH.Upload)

		// Playlist CRUD
		protected.POST("/playlists", s.playlistH.Create)
		protected.PUT("/playlists/:id", s.playlistH.Update)
		protected.DELETE("/playlists/:id", s.playlistH.Delete)

		// Playlist track manipulation
		protected.POST("/playlists/:id/tracks", s.playlistH.AddTrack)
		protected.DELETE("/playlists/:id/tracks/:trackId", s.playlistH.RemoveTrack)
		protected.POST("/playlists/:id/tracks/move", s.playlistH.MoveTrack)
		protected.POST("/playlists/:id/shuffle", s.playlistH.Shuffle)

		// Playlist export / import
		protected.GET("/playlists/:id/export", s.playlistH.Export)
		protected.POST("/playlists/import", s.playlistH.Import)

		// Master playlist tag management
		protected.PUT("/master/:tag", s.masterH.AssignPlaylistToTag)
		protected.DELETE("/master/:tag/:playlistId", s.masterH.RemovePlaylistFromTag)

		// Reconcile & timezone
		protected.POST("/reconcile", s.radioH.Reconcile)
		protected.PUT("/timezone", s.radioH.SetTimezone)

		// Legacy protected reload
		protected.POST("/playlist/reload", s.radioH.LegacyReload)
	}

	// --- SPA fallback (must be last) ---
	engine.NoRoute(s.spaH.Handle)
}

// Start launches the scheduler, broadcaster, and HTTP server. It blocks until
// ctx is cancelled and then performs a graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	go s.scheduler.Start(ctx)
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
