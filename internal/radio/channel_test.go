package radio

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/arung-agamani/denpa-radio/internal/apierror"
	"github.com/arung-agamani/denpa-radio/internal/ffmpeg"
	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

func testStore(t *testing.T) *playlist.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "playlists.json")
	store, err := playlist.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

func testEncoder() *ffmpeg.Encoder {
	return ffmpeg.NewEncoder("128k", "44100", "2")
}

func testLibrary() *playlist.TrackLibrary {
	return playlist.NewTrackLibrary()
}

func TestNewChannelManager(t *testing.T) {
	store := testStore(t)
	enc := testEncoder()
	lib := testLibrary()

	_, err := NewChannelManager(nil, enc, lib, 5)
	if err == nil {
		t.Fatal("expected error for nil store")
	}

	_, err = NewChannelManager(store, nil, lib, 5)
	if err == nil {
		t.Fatal("expected error for nil encoder")
	}

	cm, err := NewChannelManager(store, enc, nil, 5)
	if err != nil {
		t.Fatalf("unexpected error for nil library: %v", err)
	}
	if cm.library == nil {
		t.Fatal("expected library to be created for nil input")
	}

	cm, err = NewChannelManager(store, enc, lib, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cm.maxChannels != 1 {
		t.Fatalf("maxChannels = %d, want 1", cm.maxChannels)
	}
}

func TestChannelManager_Load_default(t *testing.T) {
	store := testStore(t)
	cm, err := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	if err != nil {
		t.Fatalf("NewChannelManager() error = %v", err)
	}

	if err := cm.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cm.channels) != 1 {
		t.Fatalf("channels count = %d, want 1", len(cm.channels))
	}

	ch := cm.Get("main")
	if ch == nil {
		t.Fatal("default channel 'main' not found")
	}
	if ch.Name != "Main Channel" {
		t.Fatalf("default channel name = %q, want Main Channel", ch.Name)
	}
	if cm.DefaultChannel() != ch {
		t.Fatal("DefaultChannel() did not return the main channel")
	}
}

func TestChannelManager_Add(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 3)
	_ = cm.Load()

	ch, err := cm.Add("Anime", "anime")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if ch.Slug != "anime" {
		t.Fatalf("ch.Slug = %q, want anime", ch.Slug)
	}
	if ch.Master == nil {
		t.Fatal("new channel has nil Master")
	}
	if ch.Broadcaster == nil {
		t.Fatal("new channel has nil Broadcaster")
	}
	if ch.Scheduler == nil {
		t.Fatal("new channel has nil Scheduler")
	}

	_, err = cm.Add("Anime", "anime")
	if err == nil {
		t.Fatal("expected error for duplicate slug")
	}
	apiErr, ok := err.(*apierror.Error)
	if !ok {
		t.Fatalf("expected *apierror.Error, got %T", err)
	}
	if apiErr.Code != apierror.CodeConflict {
		t.Fatalf("error code = %q, want CONFLICT", apiErr.Code)
	}

	_, err = cm.Add("Game", "game")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	_, err = cm.Add("Vocaloid", "vocaloid")
	if err == nil {
		t.Fatal("expected error when maxChannels reached")
	}

	_, err = cm.Add("", "")
	if err == nil {
		t.Fatal("expected error for empty slug")
	}
	_, err = cm.Add("Name", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestChannelManager_Get(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()
	_, _ = cm.Add("Anime", "anime")

	if cm.Get("anime") == nil {
		t.Fatal("Get(anime) returned nil")
	}
	if cm.Get("missing") != nil {
		t.Fatal("Get(missing) returned non-nil")
	}
}

func TestChannelManager_List(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()
	_, _ = cm.Add("Beta", "beta")
	_, _ = cm.Add("Alpha", "alpha")
	_, _ = cm.Add("Gamma", "gamma")

	_ = cm.Update("alpha", ChannelUpdate{SortOrder: intPtr(1)})
	_ = cm.Update("beta", ChannelUpdate{SortOrder: intPtr(2)})
	_ = cm.Update("gamma", ChannelUpdate{SortOrder: intPtr(1)})

	list := cm.List()
	if len(list) != 4 {
		t.Fatalf("List() len = %d, want 4", len(list))
	}

	// main has SortOrder 0, alpha and gamma have 1, beta has 2.
	want := []string{"main", "alpha", "gamma", "beta"}
	for i, ch := range list {
		if ch.Slug != want[i] {
			t.Fatalf("List()[%d].Slug = %q, want %q", i, ch.Slug, want[i])
		}
	}
}

func TestChannelManager_Remove(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()
	_, _ = cm.Add("Anime", "anime")

	err := cm.Remove("main")
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

	err = cm.Remove("anime")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if cm.Get("anime") != nil {
		t.Fatal("channel anime still exists after removal")
	}

	err = cm.Remove("missing")
	if err == nil {
		t.Fatal("expected error when removing non-existent channel")
	}
}

func TestChannelManager_Update(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()

	err := cm.Update("main", ChannelUpdate{
		Name:        strPtr("Updated"),
		Description: strPtr("Desc"),
		SortOrder:   intPtr(5),
		Enabled:     boolPtr(false),
		Bitrate:     strPtr("192k"),
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	ch := cm.Get("main")
	if ch.Name != "Updated" {
		t.Fatalf("Name = %q, want Updated", ch.Name)
	}
	if ch.Description != "Desc" {
		t.Fatalf("Description = %q, want Desc", ch.Description)
	}
	if ch.SortOrder != 5 {
		t.Fatalf("SortOrder = %d, want 5", ch.SortOrder)
	}
	if ch.Enabled != false {
		t.Fatalf("Enabled = %v, want false", ch.Enabled)
	}
	if ch.Bitrate != "192k" {
		t.Fatalf("Bitrate = %q, want 192k", ch.Bitrate)
	}

	err = cm.Update("missing", ChannelUpdate{Name: strPtr("X")})
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestChannelManager_StartAll_StopAll(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()
	_, _ = cm.Add("Anime", "anime")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cm.StartAll(ctx)

	// Give goroutines time to start.
	time.Sleep(100 * time.Millisecond)

	for _, ch := range cm.channels {
		if !ch.Scheduler.Running() {
			t.Fatalf("scheduler for %q not running after StartAll", ch.Slug)
		}
	}

	cm.StopAll()

	// Give goroutines time to stop.
	time.Sleep(100 * time.Millisecond)

	for _, ch := range cm.channels {
		if ch.Scheduler.Running() {
			t.Fatalf("scheduler for %q still running after StopAll", ch.Slug)
		}
	}
}

func TestChannelManager_SetDefault(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()
	_, _ = cm.Add("Anime", "anime")

	err := cm.SetDefault("anime")
	if err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}
	if cm.DefaultChannel().GetSlug() != "anime" {
		t.Fatalf("DefaultChannel().GetSlug() = %q, want anime", cm.DefaultChannel().GetSlug())
	}

	err = cm.SetDefault("missing")
	if err == nil {
		t.Fatal("expected error for non-existent slug")
	}
}

func TestChannelManager_SaveAll_Load(t *testing.T) {
	store := testStore(t)
	cm, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	_ = cm.Load()
	_, _ = cm.Add("Anime", "anime")
	_ = cm.Update("anime", ChannelUpdate{
		Description: strPtr("Anime music"),
		SortOrder:   intPtr(2),
		Enabled:     boolPtr(false),
		Bitrate:     strPtr("192k"),
	})
	_ = cm.SetDefault("anime")

	if err := cm.SaveAll(); err != nil {
		t.Fatalf("SaveAll() error = %v", err)
	}

	// Verify file exists.
	if _, err := os.Stat(store.Path()); err != nil {
		t.Fatalf("store file not found after SaveAll: %v", err)
	}

	// Load into a fresh manager.
	cm2, _ := NewChannelManager(store, testEncoder(), testLibrary(), 5)
	if err := cm2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cm2.channels) != 2 {
		t.Fatalf("channels count = %d, want 2", len(cm2.channels))
	}

	mainCh := cm2.Get("main")
	if mainCh == nil {
		t.Fatal("main channel not found after reload")
	}
	if mainCh.Name != "Main Channel" {
		t.Fatalf("main.Name = %q, want Main Channel", mainCh.Name)
	}

	animeCh := cm2.Get("anime")
	if animeCh == nil {
		t.Fatal("anime channel not found after reload")
	}
	if animeCh.Name != "Anime" {
		t.Fatalf("anime.Name = %q, want Anime", animeCh.Name)
	}
	if animeCh.Description != "Anime music" {
		t.Fatalf("anime.Description = %q, want Anime music", animeCh.Description)
	}
	if animeCh.SortOrder != 2 {
		t.Fatalf("anime.SortOrder = %d, want 2", animeCh.SortOrder)
	}
	if animeCh.Enabled != false {
		t.Fatalf("anime.Enabled = %v, want false", animeCh.Enabled)
	}
	if animeCh.Bitrate != "192k" {
		t.Fatalf("anime.Bitrate = %q, want 192k", animeCh.Bitrate)
	}
	if cm2.DefaultChannel().GetSlug() != "anime" {
		t.Fatalf("default = %q, want anime", cm2.DefaultChannel().GetSlug())
	}
}

func strPtr(s string) *string   { return &s }
func intPtr(i int) *int         { return &i }
func boolPtr(b bool) *bool      { return &b }
