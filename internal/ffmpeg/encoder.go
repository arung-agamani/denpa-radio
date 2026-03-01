package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

type Encoder struct {
	bitrate    string
	sampleRate string
	channels   string
}

func NewEncoder(bitrate, sampleRate, channels string) *Encoder {
	return &Encoder{
		bitrate:    bitrate,
		sampleRate: sampleRate,
		channels:   channels,
	}
}

func (e *Encoder) Stream(ctx context.Context, inputFile string, output io.Writer) error {
	args := []string{
		"-re",           // Real-time processing
		"-i", inputFile, // Input file
		"-f", "mp3", // Output format
		"-b:a", e.bitrate, // Audio bitrate
		"-ac", e.channels, // Audio channels (stereo)
		"-ar", e.sampleRate, // Sample rate
		"-vn",    // No video
		"pipe:1", // Output to stdout
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start FFmpeg
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Log FFmpeg errors in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			if n > 0 {
				slog.Debug("ffmpeg", "output", string(buf[:n]))
			}
		}
	}()

	// Copy output to writer
	_, copyErr := io.Copy(output, stdout)

	// Wait for command to finish
	waitErr := cmd.Wait()

	if copyErr != nil && ctx.Err() == nil {
		return fmt.Errorf("stream copy error: %w", copyErr)
	}

	if waitErr != nil && ctx.Err() == nil {
		return fmt.Errorf("ffmpeg process error: %w", waitErr)
	}

	return nil
}

// ConvertToOGG converts an audio file to OGG Vorbis format. The output file
// is written to outputFile. The conversion uses the encoder's configured
// bitrate, sample rate, and channel count. Metadata from the source file is
// preserved automatically by ffmpeg.
func (e *Encoder) ConvertToOGG(ctx context.Context, inputFile, outputFile string) error {
	args := []string{
		"-y",            // Overwrite output without asking
		"-i", inputFile, // Input file
		"-vn",               // No video
		"-c:a", "libvorbis", // OGG Vorbis codec
		"-b:a", e.bitrate, // Audio bitrate
		"-ac", e.channels, // Audio channels
		"-ar", e.sampleRate, // Sample rate
		"-map_metadata", "0", // Preserve metadata from input
		outputFile,
	}

	slog.Info("Converting audio to OGG",
		"input", inputFile,
		"output", outputFile,
		"bitrate", e.bitrate,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		slog.Error("ffmpeg OGG conversion failed",
			"input", inputFile,
			"output", outputFile,
			"stderr", stderrBuf.String(),
			"error", err,
		)
		return fmt.Errorf("ffmpeg OGG conversion failed: %w", err)
	}

	slog.Info("OGG conversion complete", "output", outputFile)
	return nil
}
