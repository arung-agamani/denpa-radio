package metadata

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// cacheEntry holds one provider's response for a given artist+album key.
type cacheEntry struct {
	Data      []byte `json:"data"`
	FetchedAt int64  `json:"fetched_at"` // unix timestamp
	TTL       int64  `json:"ttl"`        // seconds
}

// metadataCache provides a simple JSON-backed cache for external API responses.
// It is safe for concurrent use. Entries are keyed by a provider-specific
// prefix + normalized artist+album string.
type metadataCache struct {
	mu       sync.RWMutex
	path     string
	entries  map[string]cacheEntry
	modified bool
}

// newMetadataCache creates or loads a cache from the given JSON file path.
func newMetadataCache(path string) *metadataCache {
	c := &metadataCache{
		path:    path,
		entries: make(map[string]cacheEntry),
	}
	c.load()
	return c
}

func (c *metadataCache) load() {
	data, err := os.ReadFile(c.path)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("Failed to read metadata cache", "path", c.path, "error", err)
		}
		return
	}
	if err := json.Unmarshal(data, &c.entries); err != nil {
		slog.Warn("Failed to parse metadata cache, starting fresh", "path", c.path, "error", err)
		c.entries = make(map[string]cacheEntry)
	}
}

// persist writes the cache to disk if it has been modified.
func (c *metadataCache) persist() {
	c.mu.RLock()
	if !c.modified {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	dir := filepath.Dir(c.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("Failed to create cache directory", "dir", dir, "error", err)
		return
	}

	data, err := json.Marshal(c.entries)
	if err != nil {
		slog.Warn("Failed to marshal metadata cache", "error", err)
		return
	}

	if err := os.WriteFile(c.path, data, 0o644); err != nil {
		slog.Warn("Failed to write metadata cache", "path", c.path, "error", err)
		return
	}
	c.modified = false
}

// get returns cached data for the given key, or nil if expired/missing.
func (c *metadataCache) get(key string) []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil
	}

	// Check TTL.
	if entry.TTL > 0 && time.Now().Unix()-entry.FetchedAt > entry.TTL {
		return nil
	}

	return entry.Data
}

// set stores data for the given key with the specified TTL in seconds.
func (c *metadataCache) set(key string, data []byte, ttl int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		Data:      data,
		FetchedAt: time.Now().Unix(),
		TTL:       ttl,
	}
	c.modified = true
}

// Close persists any pending changes.
func (c *metadataCache) Close() {
	c.persist()
}

// defaultTTL is the default time-to-live for cached API responses (7 days).
const defaultTTL int64 = 7 * 24 * 3600

// normalizedKey builds a cache key from provider prefix + artist + album.
func normalizedKey(provider, artist, album string) string {
	return provider + ":" + artist + "||" + album
}
