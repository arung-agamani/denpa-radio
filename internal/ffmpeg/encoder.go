package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os/exec"
	"strconv"
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

type ffprobeFormat struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ProbeDuration returns the duration of an audio file in seconds via ffprobe.
// Returns 0 if the file cannot be probed.
func ProbeDuration(filePath string) int {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		filePath,
	)

	out, err := cmd.Output()
	if err != nil {
		slog.Debug("ffprobe failed", "path", filePath, "error", err)
		return 0
	}

	var data ffprobeFormat
	if err := json.Unmarshal(out, &data); err != nil {
		slog.Debug("ffprobe output parse failed", "path", filePath, "error", err)
		return 0
	}

	if data.Format.Duration == "" {
		return 0
	}

	f, err := strconv.ParseFloat(data.Format.Duration, 64)
	if err != nil {
		return 0
	}

	return int(math.Round(f))
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
