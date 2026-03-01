# Denpa Radio

A self-hosted internet radio station with a web dashboard, multi-playlist scheduling, and DJ authentication. Built as a learning project to explore Go's capabilities for networked audio applications.

Made with frustration <3

## Features

### Streaming
- **Unified Broadcast Stream**: A single ffmpeg pipeline continuously encodes audio and broadcasts to all connected clients simultaneously. Everyone listening hears the exact same audio at the exact same position—no per-client playlist state.
- **Always On**: The radio keeps playing even when zero clients are connected.
- **Auto Reconnect**: The web player detects stream stalls and silently reconnects after 5 seconds.
- **OS Media Session Integration**: Registers with the operating system's media controls (keyboard media keys, lock screen controls on Android/macOS/Windows) and displays current track metadata.
- **ICY Protocol Headers**: Compatible with standard audio players (mpv, VLC, ffplay, etc.).

### Playlist & Scheduling
- **Multiple Playlists**: Create, manage, and switch between named playlists. Each playlist has its own ordered track list.
- **Master Playlist with Time-Based Scheduling**: Assign playlists to time-of-day slots (morning, afternoon, evening, night). The scheduler automatically switches the active playlist when the time window changes.
- **Timezone-Aware Scheduling**: Configure the station timezone; all time-tag calculations respect it.
- **Track Controls**: Skip to next or previous track from the DJ dashboard.
- **Playlist Operations**: Add/remove/move tracks within a playlist, shuffle a playlist, and export/import playlists as JSON.
- **Persistent State**: Playlist configuration is saved to a JSON file and restored on restart.

### Track Library
- **Shared Library**: All tracks belong to a central library referenced by playlists, avoiding file duplication.
- **Rich Metadata Extraction**: Automatically reads ID3 tags (MP3), Vorbis comments (FLAC/OGG), and M4A metadata. Falls back to filenames when tags are unavailable.
- **Track Upload**: Upload audio files directly through the web dashboard.
- **Directory Scan**: Scan the music directory to discover new files and add them to the library.
- **Track Search**: Search the library by title, artist, or album.
- **Orphaned Track Detection**: Find and clean up library entries whose files have been removed from disk.
- **Reconcile**: Sync library state with the filesystem in one operation.

### Authentication & Security
- **DJ Login**: Password-protected DJ dashboard secured with JWT bearer tokens (24-hour TTL).
- **Rate Limiting**: Brute-force protection — accounts are locked out after 5 failed attempts within 15 minutes.
- **Security Headers**: Sensible HTTP security headers applied globally.

### Web Dashboard
- **Now Playing Panel**: Displays currently playing track with title, artist, album, and live status.
- **Tracks Management**: Browse, search, upload, edit metadata, and delete tracks.
- **Playlists Management**: Create playlists, manage their track order, shuffle, import, and export.
- **Master Playlist View**: Visualise and configure time-slot assignments.
- **Scheduler Status**: See which time slot is active and what playlist is assigned to it.
- **Import/Export**: Backup and restore playlists as JSON files.
- **Built with Svelte + Flowbite**: Responsive SPA served directly by the Go binary.


## Architecture

### Broadcast Model

The core of the service is a **unified broadcast architecture**:

```
┌─────────────┐
│  Broadcaster│ (single continuous goroutine)
├─────────────┤
│ Master      │ → Active Playlist → Next Track → FFmpeg → MP3 Chunks
│ Playlist    │
└─────────────┘
      ↓
   MP3 Chunks
      ↓
   ┌─────────────────────────────────────┐
   │  broadcastWriter (fans out chunks)  │
   └─────────────────────────────────────┘
      ↙  ↓  ↘
    Client1 Client2 Client3 (buffered channels per client)
      ↓      ↓       ↓
  Write to  Write to Write to
   HTTP      HTTP    HTTP
  Response  Response Response
```

**Key points:**
- One ffmpeg process runs continuously, advancing through the active playlist.
- Each encoded chunk is sent to all subscribed clients via non-blocking channel sends.
- Slow clients have chunks dropped rather than blocking the broadcaster.
- The broadcaster runs regardless of client count.
- The scheduler checks the clock every minute and switches the active playlist when the time slot changes.

### File Structure

```
denpa-radio/
├── main.go                          # Entry point, signal handling
├── config/
│   └── config.go                    # Environment-based configuration
├── data/
│   └── playlists.json               # Persisted playlist/library state
├── internal/
│   ├── auth/
│   │   └── auth.go                  # JWT auth, rate limiting
│   ├── ffmpeg/
│   │   └── encoder.go               # FFmpeg wrapper
│   ├── playlist/
│   │   ├── library.go               # Shared track library
│   │   ├── master.go                # Master playlist + time-tag routing
│   │   ├── playlist.go              # Playlist CRUD model
│   │   ├── scanner.go               # Music directory scanner
│   │   ├── scheduler.go             # Time-based playlist switcher
│   │   ├── store.go                 # JSON persistence
│   │   └── track.go                 # Track model & metadata extraction
│   └── radio/
│       ├── middleware.go            # Auth & security headers middleware
│       ├── stream.go                # Broadcaster & StreamHandler
│       ├── server.go                # HTTP server, route registration
│       ├── handler/                 # Gin route handlers
│       │   ├── auth.go
│       │   ├── master.go
│       │   ├── playlist.go
│       │   ├── radio.go
│       │   ├── track.go
│       │   └── spa.go
│       └── service/                 # Business logic layer
│           ├── master.go
│           ├── playlist.go
│           ├── radio.go
│           └── track.go
├── music/                           # Audio file directory (configurable)
└── web/                             # Svelte SPA (built output served by Go)
    └── src/
        ├── components/              # Navbar, Player, NowPlaying, TrackList, …
        ├── routes/                  # Public, Login, DJ dashboard sub-routes
        └── lib/                     # API client, stores, router, auth helpers
```



## Setup & Installation

### Prerequisites

- **Go 1.25** or later
- **FFmpeg** installed and available in PATH
  - Ubuntu/Debian: `sudo apt-get install ffmpeg`
  - macOS: `brew install ffmpeg`
  - Windows: Download from [ffmpeg.org](https://ffmpeg.org/download.html)
- **Bun** (for building the web dashboard)
  - Install from [bun.sh](https://bun.sh)

### Build

```bash
git clone https://github.com/arung-agamani/denpa-radio.git
cd denpa-radio

# 1. Build the web dashboard
cd web
bun install
bun run build   # outputs to web/dist/
cd ..

# 2. Build the Go binary
go build -o denpa-radio .
```

### Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8000` | HTTP server listen port |
| `MUSIC_DIR` | `./music` | Directory containing audio files |
| `STATION_NAME` | `Denpa Radio` | Station name sent to clients |
| `BITRATE` | `128k` | Audio bitrate (FFmpeg format) |
| `SAMPLE_RATE` | `44100` | Sample rate in Hz |
| `CHANNELS` | `2` | Audio channels (1=mono, 2=stereo) |
| `MAX_CLIENTS` | `100` | Maximum concurrent listeners |
| `PLAYLIST_FILE` | `./data/playlists.json` | Path to the playlist persistence file |
| `WEB_DIR` | `./web/dist` | Path to the built web dashboard |
| `DJ_USERNAME` | `dj` | DJ dashboard login username |
| `DJ_PASSWORD` | `denpa` | DJ dashboard login password |
| `JWT_SECRET` | `change-me-in-production-please` | Secret key for signing JWT tokens |
| `TIMEZONE` | *(system UTC)* | IANA timezone for time-based scheduling (e.g. `Asia/Tokyo`) |

> **Important:** Always set `DJ_PASSWORD` and `JWT_SECRET` to strong values in production.

**Example:**

```bash
export PORT=9000
export MUSIC_DIR=/path/to/music
export STATION_NAME="My Anime Radio"
export BITRATE=192k
export DJ_USERNAME=mydj
export DJ_PASSWORD=supersecret
export JWT_SECRET=a-long-random-string
export TIMEZONE=Asia/Tokyo
./denpa-radio
```


## Usage

### Starting the Service

```bash
./denpa-radio
```

You should see structured JSON log output as the server starts, the library is loaded/scanned, and the broadcaster begins.

### Web Dashboard

Open `http://localhost:8000` in your browser. The public page shows the Now Playing panel and the audio player. Log in at `/login` with your DJ credentials to access the full dashboard.

### Listening to the Stream

**Using `ffplay`:**
```bash
ffplay http://localhost:8000/stream
```

**Using `mpv`:**
```bash
mpv http://localhost:8000/stream
```

**Using `curl`:**
```bash
curl http://localhost:8000/stream --output recording.mp3
```

The stream responds with `Content-Type: audio/mpeg` and `Transfer-Encoding: chunked`.

### API Endpoints

#### Public

| Method | Path | Description |
|---|---|---|
| `GET` | `/stream` | Live audio stream |
| `GET` | `/health` | Health check |
| `GET` | `/api/status` | Station status and current track |
| `GET` | `/api/master` | Master playlist time-slot assignments |
| `GET` | `/api/scheduler/status` | Active time slot and assigned playlist |
| `GET` | `/api/timezone` | Configured station timezone |
| `GET` | `/api/queue` | Current playback queue |
| `GET` | `/api/tracks` | List all tracks in the library |
| `GET` | `/api/tracks/search` | Search tracks by title/artist/album |
| `GET` | `/api/tracks/:id` | Get a single track |
| `GET` | `/api/playlists` | List all playlists |
| `GET` | `/api/playlists/:id` | Get a single playlist |
| `POST` | `/api/auth/login` | Log in and receive a JWT |
| `GET` | `/api/auth/verify` | Verify a JWT token |

#### Protected (JWT required)

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/tracks/upload` | Upload a new audio file |
| `POST` | `/api/tracks/scan` | Scan music directory for new files |
| `GET` | `/api/tracks/orphaned` | List tracks with missing files |
| `PUT` | `/api/tracks/:id` | Update track metadata |
| `DELETE` | `/api/tracks/:id` | Remove a track from the library |
| `POST` | `/api/playlists` | Create a new playlist |
| `PUT` | `/api/playlists/:id` | Update playlist name/settings |
| `DELETE` | `/api/playlists/:id` | Delete a playlist |
| `POST` | `/api/playlists/:id/tracks` | Add a track to a playlist |
| `DELETE` | `/api/playlists/:id/tracks/:trackId` | Remove a track from a playlist |
| `POST` | `/api/playlists/:id/tracks/move` | Reorder a track within a playlist |
| `POST` | `/api/playlists/:id/shuffle` | Shuffle a playlist |
| `GET` | `/api/playlists/:id/export` | Export a playlist as JSON |
| `POST` | `/api/playlists/import` | Import a playlist from JSON |
| `PUT` | `/api/master/:tag` | Assign a playlist to a time-slot tag |
| `DELETE` | `/api/master/:tag/:playlistId` | Unassign a playlist from a time slot |
| `POST` | `/api/reconcile` | Sync library with filesystem |
| `PUT` | `/api/timezone` | Set the station timezone |
| `POST` | `/api/skip/next` | Skip to the next track |
| `POST` | `/api/skip/prev` | Jump to the previous track |

### Supported Audio Formats

- MP3 (`.mp3`)
- FLAC (`.flac`)
- Ogg Vorbis (`.ogg`)
- WAV (`.wav`)
- AAC/M4A (`.aac`, `.m4a`)

Metadata (ID3 tags, Vorbis comments, M4A atoms) is automatically extracted for all supported formats.

## Notes

- **Slow Clients**: If a client falls behind, chunks are dropped rather than stalling the broadcast. All listeners remain in sync.
- **Zero Listeners**: The broadcaster keeps running when no clients are connected. The radio never stops.
- **Persistent Playlists**: Playlist state is saved to `PLAYLIST_FILE` on every write operation and restored at startup. New files discovered in `MUSIC_DIR` are automatically added to the library on restart.
- **Scheduler Resolution**: The time-based scheduler checks the clock every minute. The granularity of time-slot transitions is therefore ~1 minute.
- **Error Handling**: If an audio file is unreadable, the encoder logs the error and advances to the next track.

## License?
No license, just use this thing freely. Maybe that fits to GPL


Made with frustration, multiple cans of Redb*ll, while marathoning Silent Witch anime

