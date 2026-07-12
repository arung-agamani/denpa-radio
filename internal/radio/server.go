package radio

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/auth"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/metadata"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/arung-agamani/denpa-radio/internal/radio/handler"
	"github.com/arung-agamani/denpa-radio/internal/radio/service"
	jsonadapter "github.com/arung-agamani/denpa-radio/internal/repository/json"
	"github.com/gin-gonic/gin"
)

// Server is the top-level application struct. It owns the gin engine, all
// service instances, all route handler instances, and the underlying
// http.Server.
type Server struct {
	config      *config.Config
	repos       *jsonadapter.JSONAdapter
	scheduler   *playlist.Scheduler
	broadcaster *Broadcaster
	auth        *auth.Auth
	httpServer  *http.Server
	enricher    *metadata.Enricher

	// Services
	trackSvc    *service.TrackService
	playlistSvc *service.PlaylistService
	masterSvc   *service.MasterService
	radioSvc    *service.RadioService
	metadataSvc *service.MetadataService

	// Route handlers
	trackH    *handler.TrackHandlers
	playlistH *handler.PlaylistHandlers
	radioH    *handler.RadioHandlers
	libraryH  *handler.LibraryHandlers
	timezoneH *handler.TimezoneHandlers
	authH     *handler.AuthHandlers
	spaH      *handler.SPAHandler
}

func NewServer(cfg *config.Config) *Server {
	// --- Playlist store / adapter initialisation ---
	store, err := playlist.NewStore(cfg.PlaylistFile)
	if err != nil {
		slog.Error("Failed to create playlist store", "error", err)
		panic(err)
	}

	adapter := jsonadapter.NewJSONAdapter(store)

	var master *playlist.MasterPlaylist

	if store.Exists() {
		if loadErr := adapter.Load(); loadErr != nil {
			slog.Warn("Failed to load saved playlists, will create default", "error", loadErr)
		} else {
			slog.Info("Loaded saved playlists from disk")
			master = adapter.Master()
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

		// Sync adapter with the newly created master so services and
		// broadcaster share the same in-memory object.
		if loadErr := adapter.Load(); loadErr != nil {
			slog.Error("Failed to sync adapter after initial save", "error", loadErr)
		}
		master = adapter.Master()
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
				if saveErr := adapter.Save(); saveErr != nil {
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
	trackSvc := service.NewTrackService(adapter, adapter, cfg, encoder)
	playlistSvc := service.NewPlaylistService(adapter, adapter, cfg)
	masterSvc := service.NewMasterService(adapter, scheduler)
	radioSvc := service.NewRadioService(adapter, scheduler, broadcaster, cfg)

	// --- Metadata enrichment ---
	enricherCfg := metadata.DefaultConfig()
	enricherCfg.MusicBrainzUserAgent = cfg.MusicBrainzUserAgent
	enricherCfg.DiscogsAPIToken = cfg.DiscogsAPIToken
	enricherCfg.EnrichmentEnabled = cfg.EnrichmentEnabled
	enricher := metadata.NewEnricher(enricherCfg)
	metadataSvc := service.NewMetadataService(adapter, adapter, enricher)

	// --- Library service (delegator) ---
	librarySvc := service.NewLibraryService(trackSvc, radioSvc, metadataSvc)

	// --- Route handlers ---
	trackH := handler.NewTrackHandlers(trackSvc, metadataSvc)
	playlistH := handler.NewPlaylistHandlers(playlistSvc, masterSvc)
	radioH := handler.NewRadioHandlers(radioSvc)
	libraryH := handler.NewLibraryHandlers(librarySvc)
	timezoneH := handler.NewTimezoneHandlers(radioSvc)
	authH := handler.NewAuthHandlers(authInstance)
	spaH := handler.NewSPAHandler(cfg.WebDir)

	s := &Server{
		config:      cfg,
		repos:       adapter,
		scheduler:   scheduler,
		broadcaster: broadcaster,
		auth:        authInstance,
		enricher:    enricher,
		trackSvc:    trackSvc,
		playlistSvc: playlistSvc,
		masterSvc:   masterSvc,
		radioSvc:    radioSvc,
		metadataSvc: metadataSvc,
		trackH:      trackH,
		playlistH:   playlistH,
		radioH:      radioH,
		libraryH:    libraryH,
		timezoneH:   timezoneH,
		authH:       authH,
		spaH:        spaH,
	}

	// --- Gin engine ---
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(SecurityHeadersMiddleware())

	// Streaming is registered here because StreamHandler lives in the radio
	// package and cannot be imported by handler (would create a cycle).
	streamHandler := NewStreamHandler(s.broadcaster, s.config.StationName, s.config.MaxClients)
	engine.GET("/stream", gin.WrapH(streamHandler))

	deps := handler.HandlerDeps{
		TrackH:    trackH,
		PlaylistH: playlistH,
		RadioH:    radioH,
		LibraryH:  libraryH,
		TimezoneH: timezoneH,
		AuthH:     authH,
		SpaH:      spaH,
	}
	handler.RegisterRoutes(engine, AuthRequired(authInstance), deps)

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
