package radio

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
)

type Server struct {
	config      *config.Config
	playlist    *Playlist
	broadcaster *Broadcaster
	httpServer  *http.Server
}

func NewServer(cfg *config.Config) *Server {
	playlist, err := NewPlaylist(cfg.MusicDir)
	if err != nil {
		slog.Error("Failed to create playlist", "error", err)
		panic(err)
	}

	encoder := ffmpeg.NewEncoder(cfg.Bitrate, cfg.SampleRate, cfg.Channels)
	broadcaster := NewBroadcaster(playlist, encoder)
	streamHandler := NewStreamHandler(broadcaster, cfg.StationName, cfg.MaxClients)

	s := &Server{
		config:      cfg,
		playlist:    playlist,
		broadcaster: broadcaster,
	}

	mux := http.NewServeMux()
	mux.Handle("/stream", streamHandler)
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/status", s.statusHandler)
	mux.HandleFunc("/playlist", s.playlistHandler)
	mux.HandleFunc("/playlist/reload", s.playlistReloadHandler)

	s.httpServer = &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0, // No timeout for streaming
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start(ctx context.Context) error {
	// Start the broadcaster in the background. It runs continuously
	// (even with zero listeners) until ctx is cancelled.
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

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	currentTrack := s.broadcaster.CurrentTrack()
	trackName := "none"
	if currentTrack != "" {
		trackName = filepath.Base(currentTrack)
	}

	var currentTrackInfo *TrackInfo
	if currentTrack != "" {
		if info, ok := s.playlist.GetTrackInfo(currentTrack); ok {
			currentTrackInfo = info
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"station_name":       s.config.StationName,
		"current_track":      trackName,
		"current_track_info": currentTrackInfo,
		"total_tracks":       s.playlist.Count(),
		"active_clients":     s.broadcaster.ActiveClients(),
		"max_clients":        s.config.MaxClients,
	})
}

func (s *Server) playlistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tracks := s.playlist.Tracks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"station_name": s.config.StationName,
		"total_tracks": len(tracks),
		"tracks":       tracks,
	})
}

func (s *Server) playlistReloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed. Use POST.", http.StatusMethodNotAllowed)
		return
	}

	slog.Info("Playlist reload requested")
	if err := s.broadcaster.ReloadPlaylist(); err != nil {
		slog.Error("Playlist reload failed", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "ok",
		"total_tracks": s.playlist.Count(),
		"tracks":       s.playlist.Tracks(),
	})
}
