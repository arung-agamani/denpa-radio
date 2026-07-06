// Package metadata provides external metadata enrichment and album art lookup
// for tracks in the Denpa Radio library. It implements a multi-provider strategy:
//
//  1. MusicBrainz (primary metadata) — release dates, labels, track listings, MBID
//  2. Cover Art Archive (primary art) — album art via MusicBrainz ID (no rate limits)
//  3. Discogs (fallback + enrichment) — genres, styles, year, label, album art
//
// All API responses are cached locally to avoid redundant network calls.
package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Config holds configuration for the metadata enrichment system.
type Config struct {
	// CacheFile is the path to the metadata cache JSON file.
	// Default: "data/metadata-cache.json"
	CacheFile string

	// CoversDir is the directory where album art images are stored.
	// Default: "data/covers"
	CoversDir string

	// MusicBrainzUserAgent is the User-Agent header sent to MusicBrainz.
	// Required by MusicBrainz policy. Should identify your application.
	MusicBrainzUserAgent string

	// DiscogsAPIToken is an optional Discogs personal access token.
// Without a token, Discogs requests are severely rate-limited.
	DiscogsAPIToken string

	// EnrichmentEnabled globally enables or disables external enrichment.
	EnrichmentEnabled bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		CacheFile:            "data/metadata-cache.json",
		CoversDir:            "data/covers",
		MusicBrainzUserAgent: "DenpaRadio/1.0 (https://github.com/arung-agamani/denpa-radio)",
		EnrichmentEnabled:    true,
	}
}

// Enricher is the top-level orchestrator for metadata enrichment and album art.
type Enricher struct {
	config      Config
	cache       *metadataCache
	musicbrainz *musicbrainzClient
	coverArt    *coverArtClient
	discogs     *discogsClient
}

// NewEnricher creates a new Enricher with the given config.
func NewEnricher(cfg Config) *Enricher {
	if cfg.CacheFile == "" {
		cfg.CacheFile = DefaultConfig().CacheFile
	}
	if cfg.CoversDir == "" {
		cfg.CoversDir = DefaultConfig().CoversDir
	}
	if cfg.MusicBrainzUserAgent == "" {
		cfg.MusicBrainzUserAgent = DefaultConfig().MusicBrainzUserAgent
	}

	cache := newMetadataCache(cfg.CacheFile)

	return &Enricher{
		config:      cfg,
		cache:       cache,
		musicbrainz: newMusicBrainzClient(cache, cfg.MusicBrainzUserAgent),
		coverArt:    newCoverArtClient(cache),
		discogs:     newDiscogsClient(cache, cfg.DiscogsAPIToken, cfg.MusicBrainzUserAgent),
	}
}

// EnrichResult contains all metadata fetched for a track's artist+album.
type EnrichResult struct {
	MBID        string   `json:"mbid,omitempty"`
	ReleaseDate string   `json:"releaseDate,omitempty"`
	Label       string   `json:"label,omitempty"`
	ArtistName  string   `json:"artistName,omitempty"`
	AlbumTitle  string   `json:"albumTitle,omitempty"`
	Type        string   `json:"type,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Styles      []string `json:"styles,omitempty"`
	Year        int      `json:"year,omitempty"`
	ArtFetched  bool     `json:"artFetched,omitempty"`
	ArtPath     string   `json:"artPath,omitempty"`
}

// Enrich fetches metadata and album art for the given artist and album.
// It returns the enriched data and any album art that was downloaded.
func (e *Enricher) Enrich(artist, album string) (*EnrichResult, error) {
	if !e.config.EnrichmentEnabled {
		return nil, nil
	}
	if artist == "" && album == "" {
		return nil, fmt.Errorf("at least artist or album is required")
	}

	result := &EnrichResult{}

	// Step 1: Search MusicBrainz for release metadata.
	mbResult, err := e.musicbrainz.SearchRelease(artist, album)
	if err != nil {
		slog.Warn("MusicBrainz search failed", "artist", artist, "album", album, "error", err)
	} else if mbResult != nil {
		result.MBID = mbResult.MBID
		result.ReleaseDate = mbResult.ReleaseDate
		result.Label = mbResult.Label
		result.ArtistName = mbResult.ArtistName
		result.AlbumTitle = mbResult.AlbumTitle
		result.Type = mbResult.Type
	}

	// Step 2: Fetch album art via CAA (if we have an MBID).
	if result.MBID != "" {
		imageData, ct, err := e.coverArt.FetchFrontCover(result.MBID)
		if err != nil {
			slog.Warn("CAA cover art fetch failed", "mbid", result.MBID, "error", err)
		} else if imageData != nil {
			path, err := e.saveCover(artist, album, imageData, ct)
			if err == nil {
				result.ArtFetched = true
				result.ArtPath = path
				return result, nil
			}
			slog.Warn("Failed to save CAA cover art", "error", err)
		}
	}

	// Step 3: Search Discogs for metadata + fallback album art.
	discogsResult, err := e.discogs.SearchRelease(artist, album)
	if err != nil {
		slog.Warn("Discogs search failed", "artist", artist, "album", album, "error", err)
	} else if discogsResult != nil {
		// Fill in metadata gaps from Discogs.
		if result.AlbumTitle == "" && discogsResult.Title != "" {
			result.AlbumTitle = discogsResult.Title
		}
		if result.ArtistName == "" && discogsResult.Artist != "" {
			result.ArtistName = discogsResult.Artist
		}
		if result.Label == "" && discogsResult.Label != "" {
			result.Label = discogsResult.Label
		}
		if result.ReleaseDate == "" && discogsResult.ReleaseDate != "" {
			result.ReleaseDate = discogsResult.ReleaseDate
		}
		if result.Year == 0 && discogsResult.Year > 0 {
			result.Year = discogsResult.Year
		}
		if len(result.Genres) == 0 && len(discogsResult.Genres) > 0 {
			result.Genres = discogsResult.Genres
		}
		if len(result.Styles) == 0 && len(discogsResult.Styles) > 0 {
			result.Styles = discogsResult.Styles
		}

		// Download cover art from Discogs if available.
		if discogsResult.CoverImage != "" {
			path, err := e.downloadAndSave(discogsResult.CoverImage, artist, album)
			if err == nil {
				result.ArtFetched = true
				result.ArtPath = path
				return result, nil
			}
			slog.Warn("Failed to download Discogs cover art", "error", err)
		}
	}

	return result, nil
}

// saveCover saves raw image data to the covers directory.
func (e *Enricher) saveCover(artist, album string, data []byte, contentType string) (string, error) {
	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	} else if strings.Contains(contentType, "gif") {
		ext = ".gif"
	} else if strings.Contains(contentType, "webp") {
		ext = ".webp"
	}

	filename := fmt.Sprintf("%s_%s%s", sanitizeName(artist), sanitizeName(album), ext)
	return e.writeCover(filename, data)
}

// downloadAndSave downloads an image from a URL and saves it to the covers directory.
func (e *Enricher) downloadAndSave(url, artist, album string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download cover art: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover art download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read cover art response: %w", err)
	}

	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "png") {
		ext = ".png"
	} else if strings.Contains(ct, "gif") {
		ext = ".gif"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	} else if strings.Contains(url, ".png") {
		ext = ".png"
	} else if strings.Contains(url, ".gif") {
		ext = ".gif"
	} else if strings.Contains(url, ".webp") {
		ext = ".webp"
	}

	filename := fmt.Sprintf("%s_%s%s", sanitizeName(artist), sanitizeName(album), ext)
	return e.writeCover(filename, data)
}

// writeCover saves image data to the covers directory.
func (e *Enricher) writeCover(filename string, data []byte) (string, error) {
	dir := e.config.CoversDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create covers directory: %w", err)
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("failed to write cover art: %w", err)
	}

	return path, nil
}

// sanitizeName makes a string safe for use in filenames.
func sanitizeName(s string) string {
	if s == "" {
		return "unknown"
	}
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			result = append(result, c)
		} else if c == ' ' || c == '.' {
			result = append(result, '_')
		}
		// Skip other characters.
	}
	if len(result) == 0 {
		return "unknown"
	}
	return string(result)
}

// EnrichedTrackMetadata holds the complete enriched metadata for a track.
type EnrichedTrackMetadata struct {
	Artist      string   `json:"artist,omitempty"`
	Album       string   `json:"album,omitempty"`
	Genre       string   `json:"genre,omitempty"`
	Styles      []string `json:"styles,omitempty"`
	Year        int      `json:"year,omitempty"`
	MBID        string   `json:"mbid,omitempty"`
	Label       string   `json:"label,omitempty"`
	ReleaseDate string   `json:"releaseDate,omitempty"`
	AlbumType   string   `json:"albumType,omitempty"`
}

// EnrichTrack enriches all available metadata fields for a track.
func (e *Enricher) EnrichTrack(artist, album string) (*EnrichedTrackMetadata, error) {
	if !e.config.EnrichmentEnabled {
		return nil, nil
	}
	if artist == "" && album == "" {
		return nil, fmt.Errorf("at least artist or album required")
	}

	result := &EnrichedTrackMetadata{
		Artist: artist,
		Album:  album,
	}

	// Search MusicBrainz for release metadata.
	mbResult, err := e.musicbrainz.SearchRelease(artist, album)
	if err != nil {
		slog.Warn("MusicBrainz metadata search failed", "artist", artist, "album", album, "error", err)
	}
	if mbResult != nil {
		result.MBID = mbResult.MBID
		result.ReleaseDate = mbResult.ReleaseDate
		result.Label = mbResult.Label
		result.AlbumType = mbResult.Type

		if len(mbResult.ReleaseDate) >= 4 {
			year := 0
			if _, err := fmt.Sscanf(mbResult.ReleaseDate[:4], "%d", &year); err == nil && year > 0 {
				result.Year = year
			}
		}
	}

	// Search Discogs for additional metadata (genres, styles, year).
	discogsResult, err := e.discogs.SearchRelease(artist, album)
	if err != nil {
		slog.Warn("Discogs metadata search failed", "artist", artist, "album", album, "error", err)
	} else if discogsResult != nil {
		if result.Label == "" && discogsResult.Label != "" {
			result.Label = discogsResult.Label
		}
		if result.ReleaseDate == "" && discogsResult.ReleaseDate != "" {
			result.ReleaseDate = discogsResult.ReleaseDate
		}
		if result.Year == 0 && discogsResult.Year > 0 {
			result.Year = discogsResult.Year
		}
		if len(discogsResult.Genres) > 0 {
			result.Genre = discogsResult.Genres[0]
		}
		if len(discogsResult.Styles) > 0 {
			result.Styles = discogsResult.Styles
		}
	}

	return result, nil
}

// Close persists any pending cache data.
func (e *Enricher) Close() {
	e.cache.Close()
}

// MarshalJSON serialises EnrichResult for JSON output.
func (r *EnrichResult) MarshalJSON() ([]byte, error) {
	type alias EnrichResult
	return json.Marshal((*alias)(r))
}
