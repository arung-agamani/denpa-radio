package metadata

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// coverArtClient fetches album art from the Cover Art Archive.
// It requires a MusicBrainz release ID (MBID) to look up art.
type coverArtClient struct {
	httpClient *http.Client
	cache      *metadataCache
}

func newCoverArtClient(cache *metadataCache) *coverArtClient {
	return &coverArtClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		cache: cache,
	}
}

// FetchFrontCover fetches the front cover art for a MusicBrainz release ID.
// Returns the image data and the content type (e.g. "image/jpeg").
// Returns nil, nil if no cover art is found.
func (c *coverArtClient) FetchFrontCover(mbid string) (data []byte, contentType string, err error) {
	cacheKey := "caa:" + mbid
	if cached := c.cache.get(cacheKey); cached != nil {
		return nil, "", nil // cached miss marker — we already know there's no art
	}

	url := fmt.Sprintf("https://coverartarchive.org/release/%s/front-250", mbid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create CAA request: %w", err)
	}
	req.Header.Set("User-Agent", "DenpaRadio/1.0")
	req.Header.Set("Accept", "image/*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("CAA request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Mark as not found so we don't retry.
		c.cache.set(cacheKey, []byte("notfound"), defaultTTL)
		return nil, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("CAA returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read CAA response: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	return body, ct, nil
}
