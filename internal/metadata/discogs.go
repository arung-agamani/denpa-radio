package metadata

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// discogsResult holds metadata fetched from Discogs for an artist+album.
type discogsResult struct {
	Title       string   `json:"title,omitempty"`
	Artist      string   `json:"artist,omitempty"`
	Year        int      `json:"year,omitempty"`
	Genres      []string `json:"genres,omitempty"`
	Styles      []string `json:"styles,omitempty"`
	Label       string   `json:"label,omitempty"`
	ReleaseDate string   `json:"releaseDate,omitempty"`
	CoverImage  string   `json:"coverImage,omitempty"`
	CoverThumb  string   `json:"coverThumb,omitempty"`
}

// discogsSearchResult represents one entry from the Discogs database search.
type discogsSearchResult struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Type        string   `json:"type"`
	Year        string   `json:"year"`
	Genres      []string `json:"genre"`
	Styles      []string `json:"style"`
	Label       []string `json:"label"`
	CoverImage  string   `json:"cover_image"`
	Thumb       string   `json:"thumb"`
	MasterURL   string   `json:"master_url"`
	ReleaseURL  string   `json:"resource_url"`
}

// discogsSearchResponse is the top-level response from Discogs search.
type discogsSearchResponse struct {
	Results    []discogsSearchResult `json:"results"`
	Pagination struct {
		Items int `json:"items"`
	} `json:"pagination"`
}

// discogsClient searches the Discogs API for release metadata and album art.
type discogsClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
	userAgent  string
	cache      *metadataCache
	mu         sync.Mutex
	lastReq    time.Time
}

// newDiscogsClient creates a new Discogs client.
// token is optional but strongly recommended; unauthenticated requests are severely rate-limited.
func newDiscogsClient(cache *metadataCache, token, userAgent string) *discogsClient {
	return &discogsClient{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    "https://api.discogs.com",
		token:      token,
		userAgent:  userAgent,
		cache:      cache,
	}
}

// rateLimit waits at least 2.5 seconds between requests to respect the
// authenticated limit of 25 requests per minute.
func (c *discogsClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	minInterval := 2500 * time.Millisecond
	if since := time.Since(c.lastReq); since < minInterval {
		time.Sleep(minInterval - since)
	}
	c.lastReq = time.Now()
}

// SearchRelease searches Discogs for a release by artist and album.
// It returns the best matching result if found.
func (c *discogsClient) SearchRelease(artist, album string) (*discogsResult, error) {
	cacheKey := normalizedKey("discogs", artist, album)
	if cached := c.cache.get(cacheKey); cached != nil {
		var result discogsResult
		if err := json.Unmarshal(cached, &result); err == nil {
			return &result, nil
		}
	}

	c.rateLimit()

	queryParts := make([]string, 0, 2)
	if artist != "" {
		queryParts = append(queryParts, artist)
	}
	if album != "" {
		queryParts = append(queryParts, album)
	}
	query := url.QueryEscape(joinQuery(queryParts))

	u := fmt.Sprintf("%s/database/search?q=%s&type=master&per_page=1", c.baseURL, query)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Discogs request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Discogs token="+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discogs request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read discogs response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discogs returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResp discogsSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse discogs response: %w", err)
	}

	if searchResp.Pagination.Items == 0 || len(searchResp.Results) == 0 {
		return nil, nil
	}

	r := searchResp.Results[0]
	result := &discogsResult{
		Title:      r.Title,
		CoverImage: r.CoverImage,
		CoverThumb: r.Thumb,
	}

	if len(r.Genres) > 0 {
		result.Genres = r.Genres
	}
	if len(r.Styles) > 0 {
		result.Styles = r.Styles
	}
	if len(r.Label) > 0 {
		result.Label = r.Label[0]
	}

	if r.Year != "" {
		var year int
		if _, err := fmt.Sscanf(r.Year, "%d", &year); err == nil && year > 0 {
			result.Year = year
			result.ReleaseDate = r.Year
		}
	}

	// The search result title is usually "Artist - Album".
	if artist != "" {
		result.Artist = artist
	}

	cached, err := json.Marshal(result)
	if err == nil {
		c.cache.set(cacheKey, cached, defaultTTL)
	}

	return result, nil
}

// joinQuery joins search terms with a space.
func joinQuery(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += " "
		}
		s += p
	}
	return s
}
