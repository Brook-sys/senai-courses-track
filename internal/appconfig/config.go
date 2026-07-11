package appconfig

import "os"

const (
	defaultAddr   = ":8020"
	defaultDBPath = "courses.db"
)

type Config struct {
	Addr   string
	DBPath string
}

func FromEnv() Config {
	return Config{
		Addr:   envOrDefault("SENAI_TRACK_ADDR", defaultAddr),
		DBPath: envOrDefault("SENAI_TRACK_DB_PATH", defaultDBPath),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
