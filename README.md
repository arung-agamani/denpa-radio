# Denpa Radio

Ultra-lightweight radio app built for the sake of connecting it to external players and Lavalink.

Entirely built using Golang.

Made with frustration <3

## Features

- **Unified Broadcast Stream**: A single ffmpeg pipeline continuously encodes audio and broadcasts to all connected clients simultaneously. Everyone listening to the same stream hears the exact same audio at the exact same position—no independent playlist states per client.
- **Always On**: The radio keeps playing even when zero clients are connected. Perfect for a persistent "radio station" experience.
- **Hot-Reload Playlist**: Add, remove, or update audio files in the music directory, then trigger a reload via the `/playlist/reload` endpoint without restarting the service.
- **Rich Metadata Extraction**: Automatically reads ID3 tags (MP3), Vorbis comments (FLAC/OGG), and M4A metadata. Falls back to filenames when tags are unavailable.
- **Lightweight & Fast**: Built entirely in Go with minimal dependencies. Streams directly via HTTP with ICY protocol headers compatible with standard audio clients.
- **RESTful API**: Simple JSON endpoints for health checks, status queries, playlist inspection, and management.

## Architecture

### Broadcast Model

The core innovation is the **unified broadcast architecture**:

```
┌─────────────┐
│  Broadcaster│ (single continuous goroutine)
├─────────────┤
│ Playlist    │ → Next Track → FFmpeg → Encode
│ Index: 2    │   to MP3       Stream
└─────────────┘
      ↓
   MP3 Chunks
      ↓
   ┌─────────────────────────────────────┐
   │  broadcastWriter (fans out chunks)  │
   └─────────────────────────────────────┘
      ↙  ↓  ↘
    Client1 Client2 Client3 (via buffered channels)
      ↓      ↓       ↓
  Write to  Write to Write to
   HTTP      HTTP    HTTP
  Response  Response Response
```

**Key points:**
- One ffmpeg process runs continuously, advancing through the playlist.
- Each chunk encoded is sent to all subscribed clients via non-blocking channel sends.
- Slow clients have chunks dropped rather than blocking the broadcaster.
- The broadcaster runs regardless of client count.
- New clients subscribe at any time and start receiving the current broadcast position.

### File Structure

```
denpa-radio/
├── main.go                    # Entry point, logging setup
├── config/
│   └── config.go              # Environment-based configuration
├── internal/
│   ├── radio/
│   │   ├── playlist.go        # Playlist management, metadata extraction
│   │   ├── stream.go          # Broadcaster & StreamHandler
│   │   └── server.go          # HTTP server & endpoints
│   └── ffmpeg/
│       └── encoder.go         # FFmpeg wrapper for encoding
└── music/                     # Audio file directory (configurable)
```

## Setup & Installation

### Prerequisites

- **Go 1.25.7** or later
- **FFmpeg** installed and available in PATH (for audio encoding)
  - On Ubuntu/Debian: `sudo apt-get install ffmpeg`
  - On macOS: `brew install ffmpeg`
  - On Windows: Download from [ffmpeg.org](https://ffmpeg.org/download.html)

### Build

```bash
git clone https://github.com/arung-agamani/denpa-radio.git
cd denpa-radio
go build -o denpa-radio .
```

### Configuration

Configuration is environment-variable based:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8000` | HTTP server listen port |
| `MUSIC_DIR` | `./music` | Directory containing audio files |
| `STATION_NAME` | `Denpa Radio` | Station name sent to clients |
| `BITRATE` | `128k` | Audio bitrate (FFmpeg format) |
| `SAMPLE_RATE` | `44100` | Sample rate in Hz |
| `CHANNELS` | `2` | Audio channels (1=mono, 2=stereo) |
| `MAX_CLIENTS` | `100` | Maximum concurrent listeners |

**Example:**

```bash
export PORT=9000
export MUSIC_DIR=/path/to/music
export STATION_NAME="My Anime Radio"
export BITRATE=192k
./denpa-radio
```

Or pass inline:

```bash
PORT=9000 MUSIC_DIR=./music ./denpa-radio
```

## Usage

### Starting the Service

```bash
./denpa-radio
```

You should see output like:

```
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"Starting radio service","port":"8000","music_dir":"./music","station_name":"Denpa Radio"}
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"HTTP server starting","addr":":8000"}
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"Broadcaster started"}
{"time":"2024-01-15T10:30:46Z","level":"INFO","msg":"Playlist scanned","total_tracks":3}
{"time":"2024-01-15T10:30:46Z","level":"INFO","msg":"Broadcasting track","track":"track_name.mp3"}
```

### Listening to the Stream

**Using `ffplay` (from FFmpeg):**

```bash
ffplay http://localhost:8000/stream
```

**Using `mpv`:**

```bash
mpv http://localhost:8000/stream
```

**Using `curl` (download raw audio):**

```bash
curl http://localhost:8000/stream --output radio_recording.mp3
```

**Programmatically (e.g., HTTP client in your app):**

```bash
curl -i http://localhost:8000/stream
```

The response will have `Content-Type: audio/mpeg` with `Transfer-Encoding: chunked`.

### API Endpoints

#### GET `/health`

Quick health check.

```bash
curl http://localhost:8000/health
```

**Response:**

```json
{
  "status": "ok"
}
```

#### GET `/status`

Current station status and broadcast info.

```bash
curl http://localhost:8000/status
```

**Response:**

```json
{
  "station_name": "Denpa Radio",
  "current_track": "song_name.mp3",
  "current_track_info": {
    "filename": "song_name.mp3",
    "path": "/path/to/music/song_name.mp3",
    "title": "Song Title",
    "artist": "Artist Name",
    "album": "Album Name",
    "genre": "J-pop",
    "year": 2023,
    "track": 1,
    "format": "mp3"
  },
  "total_tracks": 42,
  "active_clients": 3,
  "max_clients": 100
}
```

#### GET `/playlist`

Retrieve the full playlist with metadata for all tracks.

```bash
curl http://localhost:8000/playlist
```

**Response:**

```json
{
  "station_name": "Denpa Radio",
  "total_tracks": 3,
  "tracks": [
    {
      "filename": "song1.mp3",
      "path": "/path/to/music/song1.mp3",
      "title": "First Song",
      "artist": "Artist A",
      "album": "Album X",
      "genre": "J-pop",
      "year": 2023,
      "track": 1,
      "format": "mp3"
    },
    {
      "filename": "song2.flac",
      "path": "/path/to/music/song2.flac",
      "title": "Second Song",
      "artist": "Artist B",
      "album": "Album Y",
      "genre": "Vocaloid",
      "year": 2022,
      "track": 2,
      "format": "flac"
    }
  ]
}
```

#### POST `/playlist/reload`

Hot-reload the playlist from disk. Call this after adding/removing audio files.

```bash
curl -X POST http://localhost:8000/playlist/reload
```

**Response:**

```json
{
  "status": "ok",
  "total_tracks": 45,
  "tracks": [ ... ]
}
```

The broadcaster will continue playing the currently-playing track and seamlessly transition to the next track using the reloaded playlist.

### Supported Audio Formats

- MP3 (`.mp3`)
- FLAC (`.flac`)
- Ogg Vorbis (`.ogg`)
- WAV (`.wav`)
- AAC (`.aac`)

Metadata (ID3 tags, Vorbis comments, etc.) is automatically extracted for all supported formats.

## Use Cases

### Home Anime Radio

Point the service at a directory of your favorite anime songs, expose it on your local network, and stream to any audio player.

### Integration with Lavalink

Use this as a custom audio source for Discord bots or other Lavalink-compatible applications. Treat the `/stream` endpoint as a remote audio URL.

### Simple Music Broadcast

Perfect for game servers, office spaces, or community events where multiple devices need to hear the same music simultaneously.

### Testing & Development

Lightweight enough to run in containers or minimal VPS setups without heavy dependencies.

## Notes

- **ID3 Tag Reading**: Metadata is extracted at startup and on every reload. If a file lacks tags, the filename (without extension) is used as the title.
- **Slow Clients**: If a client falls behind (full channel buffer), chunks are dropped rather than stalling the broadcast. This keeps the stream responsive for all listeners.
- **Zero Listeners**: The broadcaster keeps running even if all clients disconnect. The radio never "stops."
- **Error Handling**: If an audio file is corrupted or unreadable, the encoder logs an error and moves to the next track after a brief pause.

## Future Improvements

- Per-client bitrate adaptation
- Playlist shuffling & repeat modes
- Seek/skip control (would require re-architecting the stream model)
- WebSocket support for richer control protocols
- Metrics & analytics (play counts, listener duration, etc.)

## License

Made with frustration and a lot of Go <3
