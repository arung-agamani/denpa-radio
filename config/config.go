package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	MusicDir     string
	Bitrate      string
	StationName  string
	MaxClients   int
	// MaxChannels bounds the number of broadcast channels. Read from the
	// MAX_CHANNELS env var; defaults to 5 when missing, empty, unparseable,
	// zero, or negative. Clamped to a minimum of 1.
	MaxChannels  int
	SampleRate   string
	Channels     string
	PlaylistFile string
	WebDir       string
	DJUsername   string
	DJPassword   string
	JWTSecret    string
	Timezone              string
	EnrichmentEnabled     bool
	MusicBrainzUserAgent  string
	DiscogsAPIToken       string
}

func Load() *Config {
	maxChannels := getEnvAsInt("MAX_CHANNELS", 5)
	if maxChannels < 1 {
		maxChannels = 5
	}
	return &Config{
		Port:         getEnv("PORT", "8000"),
		MusicDir:     getEnv("MUSIC_DIR", "./music"),
		Bitrate:      getEnv("BITRATE", "128k"),
		StationName:  getEnv("STATION_NAME", "Denpa Radio"),
		MaxClients:   getEnvAsInt("MAX_CLIENTS", 100),
		MaxChannels:  maxChannels,
		SampleRate:   getEnv("SAMPLE_RATE", "44100"),
		Channels:     getEnv("CHANNELS", "2"),
		PlaylistFile: getEnv("PLAYLIST_FILE", "./data/playlists.json"),
		WebDir:       getEnv("WEB_DIR", "./web/build"),
		DJUsername:   getEnv("DJ_USERNAME", "dj"),
		DJPassword:   getEnv("DJ_PASSWORD", "denpa"),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production-please"),
		Timezone:              getEnv("TIMEZONE", ""),
		EnrichmentEnabled:     getEnvAsBool("ENRICHMENT_ENABLED", true),
		MusicBrainzUserAgent:  getEnv("MUSICBRAINZ_USER_AGENT", "DenpaRadio/1.0 (https://github.com/arung-agamani/denpa-radio)"),
		DiscogsAPIToken:       getEnv("DISCOGS_API_TOKEN", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultVal int) int {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultVal
}

func getEnvAsBool(name string, defaultVal bool) bool {
	if valueStr, exists := os.LookupEnv(name); exists {
		switch valueStr {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultVal
}
