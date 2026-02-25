package radio

import (
	"context"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// clientSub represents a single subscribed listener.
type clientSub struct {
	ch chan []byte
	id uint64
}

// Broadcaster runs a single, continuous ffmpeg encoding pipeline and fans the
// resulting MP3 chunks out to every connected HTTP client.  It keeps playing
// (advancing the playlist) even when zero clients are connected.
type Broadcaster struct {
	legacyPlaylist *Playlist
	masterPlaylist *playlist.MasterPlaylist
	encoder        *ffmpeg.Encoder

	mu           sync.RWMutex
	clients      map[uint64]*clientSub
	nextID       uint64
	currentTrack atomic.Value // stores string (file path)

	// skipCh is signalled by Skip() to abort the current track and advance.
	skipCh chan struct{}
}

func NewBroadcaster(legacyPlaylist *Playlist, encoder *ffmpeg.Encoder) *Broadcaster {
	b := &Broadcaster{
		legacyPlaylist: legacyPlaylist,
		encoder:        encoder,
		clients:        make(map[uint64]*clientSub),
		skipCh:         make(chan struct{}, 1),
	}
	b.currentTrack.Store("")
	return b
}

// SetMasterPlaylist configures the broadcaster to use the new MasterPlaylist
// system for track resolution. When set, the legacy playlist is ignored.
func (b *Broadcaster) SetMasterPlaylist(master *playlist.MasterPlaylist) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.masterPlaylist = master
}

// nextTrack returns the file path of the next track to play. It prefers the
// MasterPlaylist if available, falling back to the legacy Playlist.
func (b *Broadcaster) nextTrack() (string, bool) {
	b.mu.RLock()
	master := b.masterPlaylist
	b.mu.RUnlock()

	if master != nil {
		track, _, err := master.Next()
		if err != nil {
			slog.Warn("MasterPlaylist.Next() error", "error", err)
			return "", false
		}
		if track == nil {
			return "", false
		}
		return track.FilePath, true
	}

	// Legacy fallback.
	if b.legacyPlaylist != nil {
		return b.legacyPlaylist.Next()
	}

	return "", false
}

// Start begins the continuous broadcast loop.  It blocks until ctx is
// cancelled.
func (b *Broadcaster) Start(ctx context.Context) {
	slog.Info("Broadcaster started")
	for {
		select {
		case <-ctx.Done():
			slog.Info("Broadcaster stopping")
			return
		default:
		}

		track, ok := b.nextTrack()
		if !ok {
			slog.Warn("Playlist empty, waiting before retry")
			select {
			case <-time.After(2 * time.Second):
				continue
			case <-ctx.Done():
				return
			}
		}

		trackName := filepath.Base(track)
		b.currentTrack.Store(track)
		slog.Info("Broadcasting track", "track", trackName)

		// Create a per-track context so we can abort just this track on skip.
		trackCtx, trackCancel := context.WithCancel(ctx)

		// Goroutine that listens for a skip signal and cancels the track context.
		done := make(chan struct{})
		go func() {
			defer close(done)
			select {
			case <-b.skipCh:
				trackCancel()
			case <-trackCtx.Done():
			}
		}()

		writer := &broadcastWriter{broadcaster: b}
		err := b.encoder.Stream(trackCtx, track, writer)
		trackCancel()
		<-done // wait for the skip-watcher goroutine to exit

		if err != nil {
			if ctx.Err() != nil {
				// Main context cancelled – shut down.
				return
			}
			if trackCtx.Err() != nil {
				// Track was skipped – advance to the next one immediately.
				continue
			}
			slog.Error("Broadcast encoding error", "error", err, "track", trackName)
			// Small pause before trying the next track so we don't spin on a
			// persistently broken file.
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// Skip aborts the currently-streaming track and immediately advances to the
// next one. It is safe to call from any goroutine.
func (b *Broadcaster) Skip() {
	select {
	case b.skipCh <- struct{}{}:
	default:
		// A skip is already pending; the current track will stop shortly.
	}
}

// CurrentTrack returns the path of the track currently being broadcast.
func (b *Broadcaster) CurrentTrack() string {
	v, _ := b.currentTrack.Load().(string)
	return v
}

// Subscribe adds a new listener and returns the subscription.  The caller must
// call Unsubscribe when done.
func (b *Broadcaster) Subscribe() *clientSub {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.nextID
	b.nextID++

	sub := &clientSub{
		// Buffered channel so the broadcaster doesn't block on a single slow
		// client.  If the buffer fills up we drop chunks for that client.
		ch: make(chan []byte, 512),
		id: id,
	}
	b.clients[id] = sub
	return sub
}

// Unsubscribe removes a listener.
func (b *Broadcaster) Unsubscribe(sub *clientSub) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.clients, sub.id)
	// Drain channel so any pending write in broadcastWriter doesn't block.
	close(sub.ch)
}

// ActiveClients returns the number of currently connected listeners.
func (b *Broadcaster) ActiveClients() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// ReloadPlaylist triggers a hot-reload of the legacy playlist, preserving
// position relative to the currently playing track.
func (b *Broadcaster) ReloadPlaylist() error {
	if b.legacyPlaylist == nil {
		return nil
	}
	current := b.CurrentTrack()
	return b.legacyPlaylist.Reload(current)
}

// ---------------------------------------------------------------------------
// broadcastWriter implements io.Writer and fans every Write call out to all
// subscribed clients.
// ---------------------------------------------------------------------------

type broadcastWriter struct {
	broadcaster *Broadcaster
}

func (w *broadcastWriter) Write(p []byte) (int, error) {
	// Copy the slice so each client gets its own stable reference.
	chunk := make([]byte, len(p))
	copy(chunk, p)

	w.broadcaster.mu.RLock()
	defer w.broadcaster.mu.RUnlock()

	for _, sub := range w.broadcaster.clients {
		select {
		case sub.ch <- chunk:
		default:
			// Client channel full – drop this chunk for that client to avoid
			// blocking the entire broadcast.
		}
	}

	return len(p), nil
}

// ---------------------------------------------------------------------------
// StreamHandler serves the /stream endpoint.  Each request subscribes to the
// Broadcaster and relays chunks to the HTTP response.
// ---------------------------------------------------------------------------

type StreamHandler struct {
	broadcaster *Broadcaster
	stationName string
	maxClients  int32
}

func NewStreamHandler(broadcaster *Broadcaster, stationName string, maxClients int) *StreamHandler {
	return &StreamHandler{
		broadcaster: broadcaster,
		stationName: stationName,
		maxClients:  int32(maxClients),
	}
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enforce client limit.
	active := int32(h.broadcaster.ActiveClients())
	if active >= h.maxClients {
		http.Error(w, "Too many clients", http.StatusServiceUnavailable)
		slog.Warn("Client rejected", "reason", "max_clients_reached", "max", h.maxClients)
		return
	}

	clientIP := r.RemoteAddr
	sub := h.broadcaster.Subscribe()
	slog.Info("Client connected", "ip", clientIP, "active_clients", h.broadcaster.ActiveClients())

	defer func() {
		h.broadcaster.Unsubscribe(sub)
		slog.Info("Client disconnected", "ip", clientIP, "active_clients", h.broadcaster.ActiveClients())
	}()

	// Set response headers for an infinite MP3 stream.
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("icy-name", h.stationName)
	w.Header().Set("icy-br", "128")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")

	flusher, canFlush := w.(http.Flusher)
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-sub.ch:
			if !ok {
				// Channel was closed (unsubscribed).
				return
			}
			if _, err := w.Write(chunk); err != nil {
				// Client gone (broken pipe, etc.).
				return
			}
			if canFlush {
				flusher.Flush()
			}
		}
	}
}

func (h *StreamHandler) GetActiveClients() int {
	return h.broadcaster.ActiveClients()
}
