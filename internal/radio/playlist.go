package radio

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/dhowden/tag"
)

type TrackInfo struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Title    string `json:"title,omitempty"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Genre    string `json:"genre,omitempty"`
	Year     int    `json:"year,omitempty"`
	Track    int    `json:"track,omitempty"`
	Format   string `json:"format"`
}

type Playlist struct {
	mu       sync.RWMutex
	tracks   []string
	metadata map[string]*TrackInfo
	current  int
	musicDir string
}

var supportedFormats = []string{".mp3", ".wav", ".flac", ".aac", ".ogg"}

func NewPlaylist(musicDir string) (*Playlist, error) {
	pl := &Playlist{
		musicDir: musicDir,
		tracks:   make([]string, 0),
		metadata: make(map[string]*TrackInfo),
		current:  0,
	}

	if err := pl.scan(); err != nil {
		return nil, err
	}

	return pl, nil
}

// scan walks the music directory and populates the track list and metadata.
// Must be called with pl.mu held.
func (pl *Playlist) scan() error {
	tracks := make([]string, 0)
	metadata := make(map[string]*TrackInfo)

	err := filepath.Walk(pl.musicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		for _, format := range supportedFormats {
			if ext == format {
				tracks = append(tracks, path)
				metadata[path] = extractMetadata(path, ext)
				break
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	sort.Strings(tracks)
	pl.tracks = tracks
	pl.metadata = metadata

	slog.Info("Playlist scanned", "total_tracks", len(pl.tracks))
	return nil
}

// Scan rescans the music directory and resets the playlist to the beginning.
func (pl *Playlist) Scan() error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	err := pl.scan()
	if err != nil {
		return err
	}
	pl.current = 0
	return nil
}

// Reload rescans the music directory while attempting to preserve the current
// playback position. If currentlyPlaying is found in the new track list, the
// playlist index is set so that the next call to Next() returns the track after
// it. Otherwise the index resets to 0.
func (pl *Playlist) Reload(currentlyPlaying string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if err := pl.scan(); err != nil {
		return err
	}

	if currentlyPlaying != "" && len(pl.tracks) > 0 {
		for i, t := range pl.tracks {
			if t == currentlyPlaying {
				pl.current = (i + 1) % len(pl.tracks)
				slog.Info("Playlist reloaded, preserved position",
					"currently_playing", filepath.Base(currentlyPlaying),
					"next_index", pl.current,
				)
				return nil
			}
		}
	}

	pl.current = 0
	slog.Info("Playlist reloaded, reset to beginning")
	return nil
}

// Next returns the next track path and advances the playlist index.
func (pl *Playlist) Next() (string, bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	if len(pl.tracks) == 0 {
		return "", false
	}

	track := pl.tracks[pl.current]
	pl.current = (pl.current + 1) % len(pl.tracks)

	return track, true
}

// Current returns the track that was most recently returned by Next().
func (pl *Playlist) Current() (string, bool) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	if len(pl.tracks) == 0 {
		return "", false
	}

	idx := pl.current - 1
	if idx < 0 {
		idx = len(pl.tracks) - 1
	}

	return pl.tracks[idx], true
}

// Count returns the number of tracks in the playlist.
func (pl *Playlist) Count() int {
	pl.mu.RLock()
	defer pl.mu.RUnlock()
	return len(pl.tracks)
}

// Tracks returns a copy of the ordered track info list.
func (pl *Playlist) Tracks() []TrackInfo {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	result := make([]TrackInfo, 0, len(pl.tracks))
	for _, path := range pl.tracks {
		if info, ok := pl.metadata[path]; ok {
			result = append(result, *info)
		}
	}
	return result
}

// GetTrackInfo returns metadata for a specific track path.
func (pl *Playlist) GetTrackInfo(path string) (*TrackInfo, bool) {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	info, ok := pl.metadata[path]
	if !ok {
		return nil, false
	}
	copied := *info
	return &copied, true
}

// extractMetadata opens the file and reads ID3/tag metadata.
// Falls back to filename-based info if tags cannot be read.
func extractMetadata(path string, ext string) *TrackInfo {
	filename := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	info := &TrackInfo{
		Filename: filename,
		Path:     path,
		Format:   strings.TrimPrefix(ext, "."),
		Title:    nameWithoutExt,
	}

	f, err := os.Open(path)
	if err != nil {
		slog.Warn("Could not open file for metadata", "path", path, "error", err)
		return info
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		slog.Debug("Could not read tags", "path", path, "error", err)
		return info
	}

	if m.Title() != "" {
		info.Title = m.Title()
	}
	if m.Artist() != "" {
		info.Artist = m.Artist()
	}
	if m.Album() != "" {
		info.Album = m.Album()
	}
	if m.Genre() != "" {
		info.Genre = m.Genre()
	}
	if m.Year() != 0 {
		info.Year = m.Year()
	}
	trackNum, _ := m.Track()
	if trackNum != 0 {
		info.Track = trackNum
	}

	return info
}
