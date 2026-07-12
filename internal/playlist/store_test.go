package playlist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixture data
// ---------------------------------------------------------------------------

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

var v4Fixture = []byte(`{
  "version": 4,
  "defaultChannel": "main",
  "channels": [
    {
      "id": "main",
      "slug": "main",
      "name": "Main Channel",
      "description": "",
      "sortOrder": 0,
      "enabled": true,
      "bitrate": "128k",
      "timezone": "Asia/Tokyo",
      "timeSlots": [
        {"tag": "morning", "label": "Morning", "startHour": 6, "endHour": 12},
        {"tag": "afternoon", "label": "Afternoon", "startHour": 12, "endHour": 18},
        {"tag": "evening", "label": "Evening", "startHour": 18, "endHour": 21},
        {"tag": "night", "label": "Night", "startHour": 21, "endHour": 6}
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
    }
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
  ]
}`)

var v4MultiChannelFixture = []byte(`{
  "version": 4,
  "defaultChannel": "secondary",
  "channels": [
    {
      "id": "main",
      "slug": "main",
      "name": "Main Channel",
      "description": "Primary",
      "sortOrder": 0,
      "enabled": true,
      "bitrate": "128k",
      "timezone": "Asia/Tokyo",
      "timeSlots": [
        {"tag": "morning", "label": "Morning", "startHour": 6, "endHour": 12}
      ],
      "playlists": {
        "morning": [
          {
            "id": 1,
            "name": "Morning Mix",
            "tag": "morning",
            "trackChecksums": ["abc123"],
            "currentTrackChecksum": ""
          }
        ]
      }
    },
    {
      "id": "secondary",
      "slug": "secondary",
      "name": "Secondary Channel",
      "description": "Backup",
      "sortOrder": 1,
      "enabled": true,
      "bitrate": "192k",
      "timezone": "UTC",
      "timeSlots": [
        {"tag": "day", "label": "Day", "startHour": 6, "endHour": 21},
        {"tag": "night", "label": "Night", "startHour": 21, "endHour": 6}
      ],
      "playlists": {
        "night": [
          {
            "id": 2,
            "name": "Night Mix",
            "tag": "night",
            "trackChecksums": ["def456"],
            "currentTrackChecksum": ""
          }
        ]
      }
    }
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
    },
    {
      "id": 2,
      "title": "Track Two",
      "artist": "Artist B",
      "album": "Album B",
      "filePath": "/music/track2.mp3",
      "format": "mp3",
      "checksum": "def456"
    }
  ]
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

func newStoreWithFile(t *testing.T, data []byte) *Store {
	t.Helper()
	path := writeTempFile(t, data)
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	return store
}

// ---------------------------------------------------------------------------
// V4 load tests
// ---------------------------------------------------------------------------

func TestStore_Load_V4_ParsesCorrectly(t *testing.T) {
	store := newStoreWithFile(t, v4Fixture)

	master, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if master == nil {
		t.Fatal("Load() returned nil master")
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

	pls := master.GetPlaylists(TagMorning)
	if len(pls) != 1 {
		t.Fatalf("len(morning playlists) = %d, want 1", len(pls))
	}
	if pls[0].Name != "Morning Mix" {
		t.Fatalf("playlist name = %q, want Morning Mix", pls[0].Name)
	}
	if len(pls[0].Tracks) != 1 {
		t.Fatalf("len(pl.Tracks) = %d, want 1", len(pls[0].Tracks))
	}
	if pls[0].Tracks[0].Checksum != "abc123" {
		t.Fatalf("track checksum = %q, want abc123", pls[0].Tracks[0].Checksum)
	}
}

func TestStore_Load_V4_DefaultChannel(t *testing.T) {
	store := newStoreWithFile(t, v4MultiChannelFixture)

	master, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if master == nil {
		t.Fatal("Load() returned nil master")
	}

	// Default channel is "secondary", so we should get its timezone and playlists.
	if master.Timezone() != "UTC" {
		t.Fatalf("Timezone() = %q, want UTC", master.Timezone())
	}

	slots := master.GetTimeSlots()
	if len(slots) != 2 {
		t.Fatalf("len(TimeSlots) = %d, want 2", len(slots))
	}
	if slots[0].Tag != "day" {
		t.Fatalf("slot[0] tag = %q, want day", slots[0].Tag)
	}
	if slots[1].Tag != TagNight {
		t.Fatalf("slot[1] tag = %q, want night", slots[1].Tag)
	}

	pls := master.GetPlaylists(TagNight)
	if len(pls) != 1 {
		t.Fatalf("len(night playlists) = %d, want 1", len(pls))
	}
	if pls[0].Name != "Night Mix" {
		t.Fatalf("playlist name = %q, want Night Mix", pls[0].Name)
	}
	if len(pls[0].Tracks) != 1 {
		t.Fatalf("len(pl.Tracks) = %d, want 1", len(pls[0].Tracks))
	}
	if pls[0].Tracks[0].Checksum != "def456" {
		t.Fatalf("track checksum = %q, want def456", pls[0].Tracks[0].Checksum)
	}
}

func TestStore_Load_V4_EmptyChannels(t *testing.T) {
	fixture := []byte(`{
		"version": 4,
		"defaultChannel": "main",
		"channels": [],
		"library": []
	}`)
	store := newStoreWithFile(t, fixture)

	master, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if master == nil {
		t.Fatal("Load() returned nil master")
	}
	if master.LibraryTrackCount() != 0 {
		t.Fatalf("LibraryTrackCount() = %d, want 0", master.LibraryTrackCount())
	}
}

// ---------------------------------------------------------------------------
// V4 save / load round-trip
// ---------------------------------------------------------------------------

func TestStore_SaveV4_Load_RoundTrip(t *testing.T) {
	store := newStoreWithFile(t, v4Fixture)

	master, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Mutate: add a track to the library.
	newTrack := &Track{
		ID:       2,
		Title:    "Track Two",
		Artist:   "Artist B",
		Album:    "Album B",
		FilePath: "/music/track2.mp3",
		Format:   "mp3",
		Checksum: "def456",
	}
	master.Library.Import(newTrack)

	// Mutate: add a new playlist to the afternoon tag.
	newPl := NewPlaylist("Afternoon Mix", TagAfternoon)
	newPl.Tracks = append(newPl.Tracks, newTrack)
	if err := master.AssignPlaylist(TagAfternoon, newPl); err != nil {
		t.Fatalf("AssignPlaylist() error = %v", err)
	}

	// Build v4 data and save.
	ch := &StoreChannelV4{
		ID:        "main",
		Slug:      "main",
		Name:      "Main Channel",
		SortOrder: 0,
		Enabled:   true,
		Bitrate:   "128k",
		Timezone:  master.Timezone(),
		TimeSlots: master.GetTimeSlots(),
		Playlists: make(map[string][]*StorePlaylistV2),
	}
	for _, tag := range master.ConfiguredTags() {
		pls := master.GetPlaylists(tag)
		storePls := make([]*StorePlaylistV2, 0, len(pls))
		for _, pl := range pls {
			storePls = append(storePls, playlistToStoreV2(pl))
		}
		ch.Playlists[string(tag)] = storePls
	}
	data := &StoreDataV4{
		Version:        4,
		DefaultChannel: "main",
		Channels:       []*StoreChannelV4{ch},
		Library:        master.Library,
	}

	if err := store.SaveV4(data); err != nil {
		t.Fatalf("SaveV4() error = %v", err)
	}

	// Load again with a fresh store.
	store2, err := NewStore(store.Path())
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}
	master2, err := store2.Load()
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}

	if master2.LibraryTrackCount() != 2 {
		t.Fatalf("LibraryTrackCount() after reload = %d, want 2", master2.LibraryTrackCount())
	}

	afternoon := master2.GetPlaylists(TagAfternoon)
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

	// Verify timezone and time slots are preserved.
	if master2.Timezone() != "Asia/Tokyo" {
		t.Fatalf("Timezone() = %q, want Asia/Tokyo", master2.Timezone())
	}
	slots := master2.GetTimeSlots()
	if len(slots) != 4 {
		t.Fatalf("len(TimeSlots) = %d, want 4", len(slots))
	}
}

// ---------------------------------------------------------------------------
// V3 -> V4 migration tests
// ---------------------------------------------------------------------------

func TestStore_Load_V3_MigratesToV4(t *testing.T) {
	store := newStoreWithFile(t, v3Fixture)

	master, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if master == nil {
		t.Fatal("Load() returned nil master")
	}

	// The loaded master should still work as before.
	if master.LibraryTrackCount() != 1 {
		t.Fatalf("LibraryTrackCount() = %d, want 1", master.LibraryTrackCount())
	}
	if master.Timezone() != "Asia/Tokyo" {
		t.Fatalf("Timezone() = %q, want Asia/Tokyo", master.Timezone())
	}

	// The file on disk should now be v4.
	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	var probe struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("failed to probe version: %v", err)
	}
	if probe.Version != 4 {
		t.Fatalf("migrated file version = %d, want 4", probe.Version)
	}

	// Verify the v4 structure has a single "main" channel.
	var v4Data StoreDataV4
	if err := json.Unmarshal(raw, &v4Data); err != nil {
		t.Fatalf("failed to parse migrated v4 file: %v", err)
	}
	if v4Data.DefaultChannel != "main" {
		t.Fatalf("defaultChannel = %q, want main", v4Data.DefaultChannel)
	}
	if len(v4Data.Channels) != 1 {
		t.Fatalf("len(channels) = %d, want 1", len(v4Data.Channels))
	}
	ch := v4Data.Channels[0]
	if ch.Slug != "main" {
		t.Fatalf("channel slug = %q, want main", ch.Slug)
	}
	if ch.Name != "Main Channel" {
		t.Fatalf("channel name = %q, want Main Channel", ch.Name)
	}
	if ch.ID != "main" {
		t.Fatalf("channel id = %q, want main", ch.ID)
	}
	if !ch.Enabled {
		t.Fatal("channel enabled = false, want true")
	}
	if ch.SortOrder != 0 {
		t.Fatalf("channel sortOrder = %d, want 0", ch.SortOrder)
	}
	if ch.Timezone != "Asia/Tokyo" {
		t.Fatalf("channel timezone = %q, want Asia/Tokyo", ch.Timezone)
	}
	if len(ch.TimeSlots) != 4 {
		t.Fatalf("len(timeSlots) = %d, want 4", len(ch.TimeSlots))
	}
	if len(ch.Playlists) != 4 {
		t.Fatalf("len(playlists) = %d, want 4", len(ch.Playlists))
	}
	if len(ch.Playlists["morning"]) != 1 {
		t.Fatalf("len(morning playlists) = %d, want 1", len(ch.Playlists["morning"]))
	}
	if ch.Playlists["morning"][0].Name != "Morning Mix" {
		t.Fatalf("morning playlist name = %q, want Morning Mix", ch.Playlists["morning"][0].Name)
	}

	// Library should be preserved.
	if v4Data.Library == nil || v4Data.Library.Count() != 1 {
		t.Fatalf("library track count = %d, want 1", v4Data.Library.Count())
	}
}

func TestStore_MigrateV3ToV4(t *testing.T) {
	store := newStoreWithFile(t, v3Fixture)

	master, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Explicit migration should write v4.
	if err := store.MigrateV3ToV4(master); err != nil {
		t.Fatalf("MigrateV3ToV4() error = %v", err)
	}

	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read migrated file: %v", err)
	}

	var v4Data StoreDataV4
	if err := json.Unmarshal(raw, &v4Data); err != nil {
		t.Fatalf("failed to parse migrated v4 file: %v", err)
	}
	if v4Data.Version != 4 {
		t.Fatalf("version = %d, want 4", v4Data.Version)
	}
	if len(v4Data.Channels) != 1 {
		t.Fatalf("len(channels) = %d, want 1", len(v4Data.Channels))
	}
	if v4Data.Channels[0].Slug != "main" {
		t.Fatalf("slug = %q, want main", v4Data.Channels[0].Slug)
	}
}

// ---------------------------------------------------------------------------
// Atomic save failure tests
// ---------------------------------------------------------------------------

func TestStore_SaveV4_AtomicFailureDoesNotCorrupt(t *testing.T) {
	store := newStoreWithFile(t, v4Fixture)

	// Load to verify the file is valid.
	_, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Read the original content.
	original, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("failed to read original file: %v", err)
	}

	// Create a new store pointing at a path inside a read-only directory
	// so that the temp-file creation fails.
	roDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.MkdirAll(roDir, 0o755); err != nil {
		t.Fatalf("failed to create readonly dir: %v", err)
	}
	badPath := filepath.Join(roDir, "playlists.json")
	badStore, err := NewStore(badPath)
	if err != nil {
		t.Fatalf("failed to create bad store: %v", err)
	}

	// Write a valid v4 file at the bad path first.
	if err := os.WriteFile(badPath, original, 0o644); err != nil {
		t.Fatalf("failed to seed bad path: %v", err)
	}

	// Make the directory read-only so CreateTemp fails.
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatalf("failed to chmod readonly: %v", err)
	}
	defer os.Chmod(roDir, 0o755) // restore for cleanup

	// Attempt to save v4 data; this should fail.
	data := &StoreDataV4{
		Version:        4,
		DefaultChannel: "main",
		Channels: []*StoreChannelV4{
			{
				ID:   "main",
				Slug: "main",
				Name: "Main Channel",
			},
		},
		Library: NewTrackLibrary(),
	}
	if err := badStore.SaveV4(data); err == nil {
		t.Fatal("SaveV4() expected error for read-only directory, got nil")
	}

	// Verify the original file is untouched.
	after, err := os.ReadFile(badPath)
	if err != nil {
		t.Fatalf("failed to read file after failed save: %v", err)
	}
	if string(after) != string(original) {
		t.Fatal("file was corrupted after failed atomic save")
	}
}

// ---------------------------------------------------------------------------
// ChannelSnapshot type verification
// ---------------------------------------------------------------------------

func TestChannelSnapshot_TypeExists(t *testing.T) {
	// Ensure ChannelSnapshot can be instantiated and holds expected fields.
	snap := ChannelSnapshot{
		ID:          "ch-1",
		Slug:        "main",
		Name:        "Main",
		Description: "desc",
		SortOrder:   1,
		Enabled:     true,
		Bitrate:     "128k",
		Timezone:    "UTC",
		TimeSlots:   DefaultTimeSlots(),
		Playlists:   make(map[TimeTag][]*Playlist),
	}
	if snap.Slug != "main" {
		t.Fatalf("slug = %q, want main", snap.Slug)
	}
}
