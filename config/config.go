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
	SampleRate   string
	Channels     string
	PlaylistFile string
	WebDir       string
	DJUsername   string
	DJPassword   string
	JWTSecret    string
	Timezone     string
}

func Load() *Config {
	return &Config{
		Port:         getEnv("PORT", "8000"),
		MusicDir:     getEnv("MUSIC_DIR", "./music"),
		Bitrate:      getEnv("BITRATE", "128k"),
		StationName:  getEnv("STATION_NAME", "Denpa Radio"),
		MaxClients:   getEnvAsInt("MAX_CLIENTS", 100),
		SampleRate:   getEnv("SAMPLE_RATE", "44100"),
		Channels:     getEnv("CHANNELS", "2"),
		PlaylistFile: getEnv("PLAYLIST_FILE", "./data/playlists.json"),
		WebDir:       getEnv("WEB_DIR", "./web/dist"),
		DJUsername:   getEnv("DJ_USERNAME", "dj"),
		DJPassword:   getEnv("DJ_PASSWORD", "denpa"),
		JWTSecret:    getEnv("JWT_SECRET", "change-me-in-production-please"),
		Timezone:     getEnv("TIMEZONE", ""),
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
