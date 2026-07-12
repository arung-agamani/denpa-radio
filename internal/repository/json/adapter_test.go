package json

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/arung-agamani/denpa-radio/internal/playlist"
)

// ---------------------------------------------------------------------------
// Fixture data
// ---------------------------------------------------------------------------

var v1Fixture = []byte(`{
  "morning": [
    {
      "id": 1,
      "name": "Morning Mix",
      "tag": "morning",
      "tracks": [
        {
          "id": 1,
          "title": "Track One",
          "artist": "Artist A",
          "album": "Album A",
          "filePath": "/music/track1.mp3",
          "format": "mp3",
          "checksum": "abc123"
        }
      ]
    }
  ],
  "afternoon": [],
  "evening": [],
  "night": []
}`)

var v2Fixture = []byte(`{
  "version": 2,
  "timezone": "Asia/Tokyo",
  "library": [
    {
      "id": 1,
      "title": "Track One",
      "artist": "Artist A",
      "album": "Album A",
      "filePath": "/music/track1.mp3",
      "format": "mp3",
      "checksum": "abc123"
    }
  ],
  "playlists": {
    "morning": [
      {
        "id": 1,
        "name": "Morning Mix",
        "tag": "morning",
        "trackChecksums": ["abc123"],
        "currentTrackChecksum": "abc123"
      }
    ],
    "afternoon": [],
    "evening": [],
    "night": []
  }
}`)

var v3Fixture = []byte(`{
  "version": 3,
  "timezone": "Asia/Tokyo",
  "timeSlots": [
    {"tag": "morning", "label": "Morning", "startHour": 6, "endHour": 12},
    {"tag": "afternoon", "label": "Afternoon", "startHour": 12, "endHour": 18},
    {"tag": "evening", "label": "Evening", "startHour": 18, "endHour": 21},
    {"tag": "night", "label": "Night", "startHour": 21, "endHour": 6}
  ],
  "library": [
    {
      "id": 1,
      "title": "Track One",
      "artist": "Artist A",
      "album": "Album A",
      "filePath": "/music/track1.mp3",
      "format": "mp3",
      "checksum": "abc123"
    }
  ],
  "playlists": {
    "morning": [
      {
        "id": 1,
        "name": "Morning Mix",
        "tag": "morning",
        "trackChecksums": ["abc123"],
        "currentTrackChecksum": "abc123"
      }
    ],
    "afternoon": [],
    "evening": [],
    "night": []
  }
}`)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "playlists.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func newAdapterWithFile(t *testing.T, data []byte) *JSONAdapter {
	t.Helper()
	path := writeTempFile(t, data)
	store, err := playlist.NewStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return NewJSONAdapter(store)
}

// ---------------------------------------------------------------------------
// Load / migration tests
// ---------------------------------------------------------------------------

func TestAdapter_Load_V1_Migrates(t *testing.T) {
	adapter := newAdapterWithFile(t, v1Fixture)

	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	master := adapter.Master()
	if master == nil {
		t.Fatal("Master() is nil after Load")
	}

	// Library should contain the migrated track.
	if got := master.LibraryTrackCount(); got != 1 {
		t.Fatalf("LibraryTrackCount() = %d, want 1", got)
	}

	// Playlists should be migrated to checksum references.
	pls := master.GetPlaylists(playlist.TagMorning)
	if len(pls) != 1 {
		t.Fatalf("len(morning playlists) = %d, want 1", len(pls))
	}
	if len(pls[0].Tracks) != 1 {
		t.Fatalf("len(pl.Tracks) = %d, want 1", len(pls[0].Tracks))
	}
	if pls[0].Tracks[0].Checksum != "abc123" {
		t.Fatalf("track checksum = %q, want abc123", pls[0].Tracks[0].Checksum)
	}
}

func TestAdapter_Load_V2_Migrates(t *testing.T) {
	adapter := newAdapterWithFile(t, v2Fixture)

	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	master := adapter.Master()
	if master == nil {
		t.Fatal("Master() is nil after Load")
	}

	if got := master.LibraryTrackCount(); got != 1 {
		t.Fatalf("LibraryTrackCount() = %d, want 1", got)
	}

	if master.Timezone() != "Asia/Tokyo" {
		t.Fatalf("Timezone() = %q, want Asia/Tokyo", master.Timezone())
	}

	pls := master.GetPlaylists(playlist.TagMorning)
	if len(pls) != 1 {
		t.Fatalf("len(morning playlists) = %d, want 1", len(pls))
	}
	if len(pls[0].Tracks) != 1 {
		t.Fatalf("len(pl.Tracks) = %d, want 1", len(pls[0].Tracks))
	}
}

func TestAdapter_Load_V3_RoundTrip(t *testing.T) {
	adapter := newAdapterWithFile(t, v3Fixture)

	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	master := adapter.Master()
	if master == nil {
		t.Fatal("Master() is nil after Load")
	}

	if master.LibraryTrackCount() != 1 {
		t.Fatalf("LibraryTrackCount() = %d, want 1", master.LibraryTrackCount())
	}

	if master.Timezone() != "Asia/Tokyo" {
		t.Fatalf("Timezone() = %q, want Asia/Tokyo", master.Timezone())
	}

	slots := master.GetTimeSlots()
	if len(slots) != 4 {
		t.Fatalf("len(TimeSlots) = %d, want 4", len(slots))
	}

	pls := master.GetPlaylists(playlist.TagMorning)
	if len(pls) != 1 {
		t.Fatalf("len(morning playlists) = %d, want 1", len(pls))
	}
	if pls[0].Name != "Morning Mix" {
		t.Fatalf("playlist name = %q, want Morning Mix", pls[0].Name)
	}
}

// ---------------------------------------------------------------------------
// Save + Load round-trip
// ---------------------------------------------------------------------------

func TestAdapter_SaveThenLoad(t *testing.T) {
	// Start with a v3 fixture, load it, mutate it, save, then load again.
	adapter := newAdapterWithFile(t, v3Fixture)
	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Mutate: add a new track to the library.
	newTrack := &playlist.Track{
		ID:       2,
		Title:    "Track Two",
		Artist:   "Artist B",
		Album:    "Album B",
		FilePath: "/music/track2.mp3",
		Format:   "mp3",
		Checksum: "def456",
	}
	adapter.AddOrUpdate(newTrack)

	// Mutate: add a new playlist to the afternoon tag.
	newPl := playlist.NewPlaylist("Afternoon Mix", playlist.TagAfternoon)
	newPl.Tracks = append(newPl.Tracks, newTrack)
	if err := adapter.AssignPlaylist(playlist.TagAfternoon, newPl); err != nil {
		t.Fatalf("AssignPlaylist() error = %v", err)
	}

	// Save.
	if err := adapter.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Create a fresh adapter pointing at the same file.
	store, err := playlist.NewStore(adapter.store.Path())
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}
	adapter2 := NewJSONAdapter(store)
	if err := adapter2.Load(); err != nil {
		t.Fatalf("second Load() error = %v", err)
	}

	// Verify library has both tracks.
	if got := adapter2.LibraryTrackCount(); got != 2 {
		t.Fatalf("LibraryTrackCount() after reload = %d, want 2", got)
	}

	// Verify afternoon playlist exists.
	afternoon := adapter2.GetPlaylists(playlist.TagAfternoon)
	if len(afternoon) != 1 {
		t.Fatalf("len(afternoon playlists) = %d, want 1", len(afternoon))
	}
	if afternoon[0].Name != "Afternoon Mix" {
		t.Fatalf("afternoon playlist name = %q, want Afternoon Mix", afternoon[0].Name)
	}
	if len(afternoon[0].Tracks) != 1 {
		t.Fatalf("len(afternoon.Tracks) = %d, want 1", len(afternoon[0].Tracks))
	}
	if afternoon[0].Tracks[0].Checksum != "def456" {
		t.Fatalf("afternoon track checksum = %q, want def456", afternoon[0].Tracks[0].Checksum)
	}
}

// ---------------------------------------------------------------------------
// Graceful pre-Load behaviour
// ---------------------------------------------------------------------------

func TestAdapter_PreLoad_Graceful(t *testing.T) {
	// Create an adapter without calling Load.
	store, err := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	adapter := NewJSONAdapter(store)

	if adapter.Master() != nil {
		t.Error("Master() should be nil before Load")
	}
	if adapter.Exists() {
		t.Error("Exists() should be false for empty store")
	}
	if adapter.GetByID(1) != nil {
		t.Error("GetByID() should be nil before Load")
	}
	if adapter.Get("abc") != nil {
		t.Error("Get() should be nil before Load")
	}
	if adapter.List() != nil {
		t.Error("List() should be nil before Load")
	}
	if adapter.Search("x") != nil {
		t.Error("Search() should be nil before Load")
	}
	if _, added := adapter.Add(nil); added {
		t.Error("Add() should return false before Load")
	}
	if adapter.AddOrUpdate(nil) != nil {
		t.Error("AddOrUpdate() should be nil before Load")
	}
	if adapter.RemoveByID(1) != nil {
		t.Error("RemoveByID() should be nil before Load")
	}
	if adapter.Count() != 0 {
		t.Errorf("Count() = %d, want 0", adapter.Count())
	}
	if adapter.AllPlaylists() != nil {
		t.Error("AllPlaylists() should be nil before Load")
	}
	if adapter.ActiveTag() != "" {
		t.Errorf("ActiveTag() = %q, want empty", adapter.ActiveTag())
	}
	if adapter.ConfiguredTags() != nil {
		t.Error("ConfiguredTags() should be nil before Load")
	}
	if adapter.IsConfiguredTag(playlist.TagMorning) {
		t.Error("IsConfiguredTag() should be false before Load")
	}
	if adapter.GetPlaylists(playlist.TagMorning) != nil {
		t.Error("GetPlaylists() should be nil before Load")
	}
	if adapter.GetTimeSlots() != nil {
		t.Error("GetTimeSlots() should be nil before Load")
	}
	if adapter.Timezone() != "" {
		t.Errorf("Timezone() = %q, want empty", adapter.Timezone())
	}
	if adapter.Location() == nil {
		t.Error("Location() should not be nil before Load")
	}
	if adapter.TotalTracks() != 0 {
		t.Errorf("TotalTracks() = %d, want 0", adapter.TotalTracks())
	}
	if adapter.LibraryTrackCount() != 0 {
		t.Errorf("LibraryTrackCount() = %d, want 0", adapter.LibraryTrackCount())
	}
	if adapter.AllTracksDeduped() != nil {
		t.Error("AllTracksDeduped() should be nil before Load")
	}
	if len(adapter.Summary()) != 0 {
		t.Error("Summary() should be empty before Load")
	}
	if tracks, pl := adapter.PeekQueue(5); tracks != nil || pl != nil {
		t.Error("PeekQueue() should return nils before Load")
	}
	if adapter.Library() != nil {
		t.Error("Library() should be nil before Load")
	}
	if adapter.RemoveTrackFromAll("abc") != 0 {
		t.Errorf("RemoveTrackFromAll() = %d, want 0", adapter.RemoveTrackFromAll("abc"))
	}
	if adapter.TimeTagForHourConfigured(12) != "" {
		t.Errorf("TimeTagForHourConfigured() = %q, want empty", adapter.TimeTagForHourConfigured(12))
	}

	// Methods that return errors should report the adapter is not loaded.
	if _, err := adapter.Update(1, playlist.TrackUpdate{}); err == nil {
		t.Error("Update() should error before Load")
	}
	if _, err := adapter.BatchUpdate(playlist.TrackFilter{Album: "X"}, playlist.TrackUpdate{}); err == nil {
		t.Error("BatchUpdate() should error before Load")
	}
	if _, err := adapter.BatchSetCover(playlist.TrackFilter{Album: "X"}, "cover.jpg"); err == nil {
		t.Error("BatchSetCover() should error before Load")
	}
	if _, _, err := adapter.FindPlaylistByID(1); err == nil {
		t.Error("FindPlaylistByID() should error before Load")
	}
	if err := adapter.AssignPlaylist(playlist.TagMorning, playlist.NewPlaylist("P", playlist.TagMorning)); err == nil {
		t.Error("AssignPlaylist() should error before Load")
	}
	if err := adapter.RemovePlaylist(playlist.TagMorning, 1); err == nil {
		t.Error("RemovePlaylist() should error before Load")
	}
	if _, err := adapter.ActivePlaylist(); err == nil {
		t.Error("ActivePlaylist() should error before Load")
	}
	if err := adapter.SetTimeSlots(playlist.DefaultTimeSlots()); err == nil {
		t.Error("SetTimeSlots() should error before Load")
	}
	if err := adapter.SetTimezone("UTC"); err == nil {
		t.Error("SetTimezone() should error before Load")
	}
	if _, _, err := adapter.Next(); err == nil {
		t.Error("Next() should error before Load")
	}
	if err := adapter.SeekPrev(); err == nil {
		t.Error("SeekPrev() should error before Load")
	}
	if err := adapter.Save(); err == nil {
		t.Error("Save() should error before Load")
	}
}

// ---------------------------------------------------------------------------
// Representative delegation tests
// ---------------------------------------------------------------------------

func TestAdapter_GetByID(t *testing.T) {
	adapter := newAdapterWithFile(t, v3Fixture)
	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	track := adapter.GetByID(1)
	if track == nil {
		t.Fatal("GetByID(1) is nil")
	}
	if track.Title != "Track One" {
		t.Fatalf("track.Title = %q, want Track One", track.Title)
	}

	if adapter.GetByID(999) != nil {
		t.Error("GetByID(999) should be nil for missing track")
	}
}

func TestAdapter_AllPlaylists(t *testing.T) {
	adapter := newAdapterWithFile(t, v3Fixture)
	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	all := adapter.AllPlaylists()
	if len(all) != 1 {
		t.Fatalf("len(AllPlaylists()) = %d, want 1", len(all))
	}
	if all[0].Name != "Morning Mix" {
		t.Fatalf("playlist name = %q, want Morning Mix", all[0].Name)
	}
}

func TestAdapter_ActiveTag(t *testing.T) {
	adapter := newAdapterWithFile(t, v3Fixture)
	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Before explicit set, active tag is empty.
	if adapter.ActiveTag() != "" {
		t.Fatalf("ActiveTag() = %q, want empty before set", adapter.ActiveTag())
	}

	adapter.SetActiveTag(playlist.TagEvening)
	if adapter.ActiveTag() != playlist.TagEvening {
		t.Fatalf("ActiveTag() = %q, want evening", adapter.ActiveTag())
	}
}

func TestAdapter_Summary(t *testing.T) {
	adapter := newAdapterWithFile(t, v3Fixture)
	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	summary := adapter.Summary()
	if summary[playlist.TagMorning] != 1 {
		t.Fatalf("Summary()[morning] = %d, want 1", summary[playlist.TagMorning])
	}
	if summary[playlist.TagAfternoon] != 0 {
		t.Fatalf("Summary()[afternoon] = %d, want 0", summary[playlist.TagAfternoon])
	}
}

func TestAdapter_ResolveActiveTag(t *testing.T) {
	adapter := newAdapterWithFile(t, v3Fixture)
	if err := adapter.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tag, changed := adapter.ResolveActiveTag()
	if tag == "" {
		t.Fatal("ResolveActiveTag() returned empty tag")
	}
	if !changed {
		// First call should always report changed because activeTag starts empty.
		t.Error("ResolveActiveTag() should report changed on first call")
	}

	// Second call should not change.
	_, changed2 := adapter.ResolveActiveTag()
	if changed2 {
		t.Error("ResolveActiveTag() should not report changed on second call")
	}
}
