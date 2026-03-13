package config

import "os"

const (
	defaultHost = "0.0.0.0"
	defaultPort = "8080"
)

// Config keeps runtime settings loaded from environment variables.
type Config struct {
	host        string
	port        string
	databaseURL string
}

func Load() Config {
	return Config{
		host:        getEnv("FEDEY_API_HOST", defaultHost),
		port:        getEnv("FEDEY_API_PORT", defaultPort),
		databaseURL: os.Getenv("FEDEY_DATABASE_URL"),
	}
}

func (c Config) APIAddress() string {
	return c.host + ":" + c.port
}

func (c Config) DatabaseURL() string {
	return c.databaseURL
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
