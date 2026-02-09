package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arung-agamani/denpa-radio/config"
	"github.com/arung-agamani/denpa-radio/internal/radio"
)

func main() {
	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg := config.Load()

	slog.Info("Starting radio service",
		"port", cfg.Port,
		"music_dir", cfg.MusicDir,
		"station_name", cfg.StationName,
	)

	// Create radio server
	server := radio.NewServer(cfg)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		slog.Info("Shutdown signal received")
		cancel()
	}()

	// Start server
	if err := server.Start(ctx); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(1)
	}

	// Graceful shutdown
	slog.Info("Shutting down gracefully...")
	time.Sleep(2 * time.Second) // Allow cleanup
	slog.Info("Server stopped")
}
