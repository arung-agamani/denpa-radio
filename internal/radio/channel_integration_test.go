package radio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
	"github.com/gin-gonic/gin"
)

func testStoreAt(t *testing.T, path string) *playlist.Store {
	t.Helper()
	store, err := playlist.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

func testEncoderOrSkip(t *testing.T) *ffmpeg.Encoder {
	t.Helper()
	if _, err := os.Stat("/usr/bin/ffmpeg"); err != nil {
		if _, err := os.Stat("/usr/local/bin/ffmpeg"); err != nil {
			if _, err := os.Stat("/home/haruka/.local/bin/ffmpeg"); err != nil {
				t.Skip("ffmpeg not found in PATH, skipping stream-related test")
			}
		}
	}
	return ffmpeg.NewEncoder("128k", "44100", "2")
}

func Test_ChannelManager_Lifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 2)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}

	// Load empty store → default "main" channel should be created.
	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	channels := cm.List()
	if len(channels) != 1 {
		t.Fatalf("channels count = %d, want 1", len(channels))
	}
	mainCh := cm.Get("main")
	if mainCh == nil {
		t.Fatal("default channel 'main' not found after Load")
	}
	if mainCh.Name != "Main Channel" {
		t.Fatalf("default channel name = %q, want Main Channel", mainCh.Name)
	}

	// Add a "jazz" channel.
	jazz, err := cm.Add("Jazz", "jazz")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if jazz.Slug != "jazz" {
		t.Fatalf("jazz.Slug = %q, want jazz", jazz.Slug)
	}

	list := cm.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}

	// StartAll with a cancellable context.
	ctx, cancel := context.WithCancel(context.Background())
	cm.StartAll(ctx)

	// Give goroutines time to start.
	time.Sleep(200 * time.Millisecond)

	for _, ch := range cm.List() {
		if !ch.Scheduler.Running() {
			t.Fatalf("scheduler for %q not running after StartAll", ch.Slug)
		}
	}

	// StopAll.
	cancel()
	cm.StopAll()

	// Give goroutines time to stop.
	time.Sleep(200 * time.Millisecond)

	for _, ch := range cm.List() {
		if ch.Scheduler.Running() {
			t.Fatalf("scheduler for %q still running after StopAll", ch.Slug)
		}
	}
}

func Test_ChannelManager_V3Migration(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "playlists.json")

	// Write a V3-format JSON file manually.
	v3Data := map[string]interface{}{
		"version":   3,
		"timezone":  "Asia/Tokyo",
		"timeSlots": []playlist.TimeSlot{},
		"library": map[string]interface{}{
			"tracks": []interface{}{},
			"nextId": 1,
		},
		"playlists": map[string]interface{}{},
	}
	raw, err := json.MarshalIndent(v3Data, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal v3 data: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("failed to write v3 file: %v", err)
	}

	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}

	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	channels := cm.List()
	if len(channels) != 1 {
		t.Fatalf("channels count = %d, want 1 after v3 migration", len(channels))
	}

	mainCh := cm.Get("main")
	if mainCh == nil {
		t.Fatal("main channel not found after v3 migration")
	}
	if mainCh.Name != "Main Channel" {
		t.Fatalf("main.Name = %q, want Main Channel", mainCh.Name)
	}
}

func Test_ChannelManager_MaxChannels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 1)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}
	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Adding a second channel should fail because maxChannels=1.
	_, err = cm.Add("Jazz", "jazz")
	if err == nil {
		t.Fatal("expected error when adding channel beyond maxChannels")
	}
	apiErr, ok := err.(*apierror.Error)
	if !ok {
		t.Fatalf("expected *apierror.Error, got %T", err)
	}
	if apiErr.Code != apierror.CodeValidation {
		t.Fatalf("error code = %q, want VALIDATION", apiErr.Code)
	}
}

func Test_ChannelManager_RemoveDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}
	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	err = cm.Remove("main")
	if err == nil {
		t.Fatal("expected error when removing default channel")
	}
	apiErr, ok := err.(*apierror.Error)
	if !ok {
		t.Fatalf("expected *apierror.Error, got %T", err)
	}
	if apiErr.Code != apierror.CodeForbidden {
		t.Fatalf("error code = %q, want FORBIDDEN", apiErr.Code)
	}
}

func Test_ChannelManager_SaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}
	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	_, _ = cm.Add("Jazz", "jazz")
	_ = cm.Update("jazz", ChannelUpdate{
		Description: strPtr("Smooth jazz"),
		SortOrder:   intPtr(3),
		Enabled:     boolPtr(false),
		Bitrate:     strPtr("192k"),
	})
	_ = cm.SetDefault("jazz")

	if err := cm.SaveAll(); err != nil {
		t.Fatalf("SaveAll() error = %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("store file not found after SaveAll: %v", err)
	}

	// Load into a fresh manager using the same store.
	cm2, err := NewChannelManager(store, enc, testLibrary(), 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}
	if err := cm2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cm2.List()) != 2 {
		t.Fatalf("channels count = %d, want 2", len(cm2.List()))
	}

	mainCh := cm2.Get("main")
	if mainCh == nil {
		t.Fatal("main channel not found after reload")
	}
	if mainCh.Name != "Main Channel" {
		t.Fatalf("main.Name = %q, want Main Channel", mainCh.Name)
	}

	jazzCh := cm2.Get("jazz")
	if jazzCh == nil {
		t.Fatal("jazz channel not found after reload")
	}
	if jazzCh.Name != "Jazz" {
		t.Fatalf("jazz.Name = %q, want Jazz", jazzCh.Name)
	}
	if jazzCh.Description != "Smooth jazz" {
		t.Fatalf("jazz.Description = %q, want Smooth jazz", jazzCh.Description)
	}
	if jazzCh.SortOrder != 3 {
		t.Fatalf("jazz.SortOrder = %d, want 3", jazzCh.SortOrder)
	}
	if jazzCh.Enabled != false {
		t.Fatalf("jazz.Enabled = %v, want false", jazzCh.Enabled)
	}
	if jazzCh.Bitrate != "192k" {
		t.Fatalf("jazz.Bitrate = %q, want 192k", jazzCh.Bitrate)
	}
	if cm2.DefaultChannel().GetSlug() != "jazz" {
		t.Fatalf("default = %q, want jazz", cm2.DefaultChannel().GetSlug())
	}
}

func Test_ChannelStreamHandler_ExistingChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}
	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	handler := NewChannelStreamHandler(cm, "Test Station", 100)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodHead, "/stream/main", nil)
	c.Params = gin.Params{{Key: "slug", Value: "main"}}

	handler.Handle(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HEAD /stream/main status = %d, want %d", w.Code, http.StatusOK)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "audio/mpeg" {
		t.Fatalf("Content-Type = %q, want audio/mpeg", ct)
	}
}

func Test_ChannelStreamHandler_NonExistentChannel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := testStoreAt(t, path)
	enc := testEncoderOrSkip(t)
	lib := testLibrary()

	cm, err := NewChannelManager(store, enc, lib, 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}
	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	handler := NewChannelStreamHandler(cm, "Test Station", 100)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/stream/missing", nil)
	c.Params = gin.Params{{Key: "slug", Value: "missing"}}

	handler.Handle(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("GET /stream/missing status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
