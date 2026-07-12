package radio

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
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

// channelAwareAdapter wraps JSONAdapter so that Save() writes v4 via the
// channel manager instead of v3 via the legacy store.Save(master).
type channelAwareAdapter struct {
	*jsonadapter.JSONAdapter
	cm *ChannelManager
}

func (a *channelAwareAdapter) Save() error {
	if a.cm != nil {
		return a.cm.SaveAll()
	}
	return a.JSONAdapter.Save()
}

// Server is the top-level application struct. It owns the gin engine, all
// service instances, all route handler instances, and the underlying
// http.Server.
type Server struct {
	config         *config.Config
	repos          *channelAwareAdapter
	channelManager *ChannelManager
	auth           *auth.Auth
	httpServer     *http.Server
	enricher       *metadata.Enricher

	// Services
	trackSvc    *service.TrackService
	playlistSvc *service.PlaylistService
	masterSvc   *service.MasterService
	radioSvc    *service.RadioService
	metadataSvc *service.MetadataService
	channelSvc  *service.ChannelService

	// Route handlers
	trackH    *handler.TrackHandlers
	playlistH *handler.PlaylistHandlers
	radioH    *handler.RadioHandlers
	libraryH  *handler.LibraryHandlers
	timezoneH *handler.TimezoneHandlers
	authH     *handler.AuthHandlers
	spaH      *handler.SPAHandler
	channelH  *handler.ChannelHandlers
}

func NewServer(cfg *config.Config) *Server {
	// --- Playlist store ---
	store, err := playlist.NewStore(cfg.PlaylistFile)
	if err != nil {
		slog.Error("Failed to create playlist store", "error", err)
		panic(err)
	}

	// --- V3 migration: if file exists and is < v4, trigger migration ---
	if store.Exists() {
		raw, _ := os.ReadFile(store.Path())
		var versionProbe struct {
			Version int `json:"version"`
		}
		_ = json.Unmarshal(raw, &versionProbe)
		if versionProbe.Version < 4 {
			if _, migrateErr := store.Load(); migrateErr != nil {
				slog.Warn("Failed to migrate v3 playlist to v4", "error", migrateErr)
			} else {
				slog.Info("Migrated v3 playlist to v4 format")
			}
		}
	}

	// --- Encoder ---
	encoder := ffmpeg.NewEncoder(cfg.Bitrate, cfg.SampleRate, cfg.Channels)

	// --- Channel Manager ---
	channelManager, err := NewChannelManager(store, encoder, nil, cfg.MaxChannels)
	if err != nil {
		slog.Error("Failed to create channel manager", "error", err)
		panic(err)
	}

	if loadErr := channelManager.Load(); loadErr != nil {
		slog.Warn("Failed to load channels, will use default", "error", loadErr)
	}

	// Ensure at least a default "main" channel exists
	if len(channelManager.List()) == 0 {
		if _, addErr := channelManager.Add("Main Channel", "main"); addErr != nil {
			slog.Error("Failed to create default main channel", "error", addErr)
			panic(addErr)
		}
		if saveErr := channelManager.SaveAll(); saveErr != nil {
			slog.Error("Failed to save default channel", "error", saveErr)
		}
	}

	// Set timezone on default channel if needed
	defaultCh := channelManager.DefaultChannel()
	if defaultCh != nil {
		master := defaultCh.GetMaster()
		if master != nil && master.Timezone() == "" && cfg.Timezone != "" {
			if tzErr := master.SetTimezone(cfg.Timezone); tzErr != nil {
				slog.Warn("Invalid TIMEZONE from config, falling back to UTC",
					"timezone", cfg.Timezone, "error", tzErr)
			}
		}
	}

	// --- Scan music directory into shared library ---
	if defaultCh != nil {
		master := defaultCh.GetMaster()
		if master != nil && master.Library != nil {
			_, added, scanErr := playlist.ScanIntoLibrary(cfg.MusicDir, master.Library)
			if scanErr != nil {
				slog.Warn("Failed to scan music directory into library", "error", scanErr)
			} else if added > 0 {
				slog.Info("Discovered new tracks during startup scan",
					"newly_added", added,
					"library_total", master.Library.Count(),
				)
				if saveErr := channelManager.SaveAll(); saveErr != nil {
					slog.Error("Failed to save after startup scan", "error", saveErr)
				}
			}
		}

		// Build a default playlist if the master has none
		if master != nil && len(master.AllPlaylists()) == 0 {
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
			if saveErr := channelManager.SaveAll(); saveErr != nil {
				slog.Error("Failed to save initial playlist", "error", saveErr)
			}
		} else {
			master.ResolveActiveTag()
		}
	}

	// --- Adapter (points to default channel's master for legacy compatibility) ---
	adapter := &channelAwareAdapter{
		JSONAdapter: jsonadapter.NewJSONAdapter(store),
		cm:          channelManager,
	}
	if defaultCh != nil {
		adapter.SetMaster(defaultCh.GetMaster())
	}

	// --- Auth ---
	authInstance := auth.New(auth.Config{
		Username:           cfg.DJUsername,
		Password:           cfg.DJPassword,
		JWTSecret:          cfg.JWTSecret,
		TokenTTL:           24 * time.Hour,
		MaxLoginAttempts:   5,
		LoginWindowSeconds: 900,
	})

	// --- Services ---
	trackSvc := service.NewTrackService(adapter, adapter, cfg, encoder)
	playlistSvc := service.NewPlaylistService(adapter, adapter, cfg, channelManager)
	masterSvc := service.NewMasterService(adapter, nil, channelManager)
	radioSvc := service.NewRadioService(adapter, nil, nil, cfg, channelManager)
	channelSvc := service.NewChannelService(channelManager, cfg)

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
	channelH := handler.NewChannelHandlers(channelSvc)

	s := &Server{
		config:         cfg,
		repos:          adapter,
		channelManager: channelManager,
		auth:           authInstance,
		enricher:       enricher,
		trackSvc:       trackSvc,
		playlistSvc:    playlistSvc,
		masterSvc:      masterSvc,
		radioSvc:       radioSvc,
		metadataSvc:    metadataSvc,
		channelSvc:     channelSvc,
		trackH:         trackH,
		playlistH:      playlistH,
		radioH:         radioH,
		libraryH:       libraryH,
		timezoneH:      timezoneH,
		authH:          authH,
		spaH:           spaH,
		channelH:       channelH,
	}

	// --- Gin engine ---
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(SecurityHeadersMiddleware())

	// Streaming routes: /stream redirects to default channel, /stream/:slug serves per-channel streams.
	streamHandler := NewChannelStreamHandler(channelManager, cfg.StationName, cfg.MaxClients)
	engine.GET("/stream", NewRedirectToDefaultStreamHandler(channelManager))
	engine.HEAD("/stream", NewRedirectToDefaultStreamHandler(channelManager))
	engine.GET("/stream/:slug", streamHandler.Handle)
	engine.HEAD("/stream/:slug", streamHandler.Handle)

	deps := handler.HandlerDeps{
		TrackH:    trackH,
		PlaylistH: playlistH,
		RadioH:    radioH,
		LibraryH:  libraryH,
		TimezoneH: timezoneH,
		AuthH:     authH,
		SpaH:      spaH,
		ChannelH:  channelH,
	}
	handler.RegisterRoutes(engine, AuthRequired(authInstance), deps)

	s.httpServer = &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	return s
}

// Start launches all channel broadcasters/schedulers and the HTTP server. It
// blocks until ctx is cancelled and then performs a graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	go s.channelManager.StartAll(ctx)

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
