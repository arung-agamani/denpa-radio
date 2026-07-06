package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// musicbrainzRelease represents the relevant fields from a MusicBrainz release search.
type musicbrainzRelease struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Date    string `json:"date"`
	Artists []struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"artist-credit"`
	LabelInfo []struct {
		Label struct {
			Name string `json:"name"`
		} `json:"label"`
	} `json:"label-info"`
	ReleaseGroup struct {
		ID        string `json:"id"`
		PrimaryType string `json:"primary-type"`
	} `json:"release-group"`
}

// musicbrainzSearchResponse is the top-level response from a MusicBrainz search.
type musicbrainzSearchResponse struct {
	Releases []musicbrainzRelease `json:"releases"`
	Count    int                  `json:"count"`
}

// musicbrainzClient searches the MusicBrainz API for release metadata.
type musicbrainzClient struct {
	httpClient *http.Client
	userAgent  string
	baseURL    string
	cache      *metadataCache
	ticker     *time.Ticker // enforces 1 req/s rate limit
}

// newMusicBrainzClient creates a new MusicBrainz client.
// userAgent should identify your application (required by MB).
func newMusicBrainzClient(cache *metadataCache, userAgent string) *musicbrainzClient {
	return &musicbrainzClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		userAgent:  userAgent,
		baseURL:    "https://musicbrainz.org/ws/2",
		cache:      cache,
		ticker:     time.NewTicker(time.Second), // 1 request per second
	}
}

// musicbrainzResult holds metadata fetched from MusicBrainz for an artist+album.
type musicbrainzResult struct {
	MBID       string `json:"mbid,omitempty"`
	ReleaseDate string `json:"releaseDate,omitempty"`
	Label      string `json:"label,omitempty"`
	ArtistMBID string `json:"artistMbid,omitempty"`
	ArtistName string `json:"artistName,omitempty"`
	AlbumTitle string `json:"albumTitle,omitempty"`
	Type       string `json:"type,omitempty"`
}

// SearchRelease searches for a release by artist and album name.
// Returns the best matching release if found.
func (c *musicbrainzClient) SearchRelease(artist, album string) (*musicbrainzResult, error) {
	cacheKey := normalizedKey("mb", artist, album)
	if cached := c.cache.get(cacheKey); cached != nil {
		var result musicbrainzResult
		if err := json.Unmarshal(cached, &result); err == nil {
			return &result, nil
		}
	}

	// Rate limit: wait for ticker.
	<-c.ticker.C

	query := fmt.Sprintf(`artist:"%s" AND release:"%s"`, escapeLucene(artist), escapeLucene(album))
	u := fmt.Sprintf("%s/release?query=%s&fmt=json&limit=1",
		c.baseURL, url.QueryEscape(query))

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create MusicBrainz request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read musicbrainz response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp musicbrainzSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse musicbrainz response: %w", err)
	}

	if searchResp.Count == 0 || len(searchResp.Releases) == 0 {
		return nil, nil // no results
	}

	r := searchResp.Releases[0]
	result := &musicbrainzResult{
		MBID:       r.ID,
		ReleaseDate: r.Date,
		AlbumTitle: r.Title,
	}

	if len(r.Artists) > 0 {
		result.ArtistName = r.Artists[0].Name
		result.ArtistMBID = r.Artists[0].ID
	}
	if len(r.LabelInfo) > 0 {
		result.Label = r.LabelInfo[0].Label.Name
	}
	if r.ReleaseGroup.PrimaryType != "" {
		result.Type = r.ReleaseGroup.PrimaryType
	}

	// Cache the result.
	if data, err := json.Marshal(result); err == nil {
		c.cache.set(cacheKey, data, defaultTTL)
	}

	return result, nil
}

// escapeLucene escapes special characters in Lucene query strings.
func escapeLucene(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '+' || c == '-' || c == '!' || c == '(' || c == ')' ||
			c == '{' || c == '}' || c == '[' || c == ']' || c == '^' ||
			c == '"' || c == '~' || c == '*' || c == '?' || c == ':' ||
			c == '\\' || c == '&' || c == '|' {
			result = append(result, '\\')
		}
		result = append(result, c)
	}
	return string(result)
}
