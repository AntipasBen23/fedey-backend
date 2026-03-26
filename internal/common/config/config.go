package config

import "os"

const (
	defaultHost               = "0.0.0.0"
	defaultPort               = "8080"
	defaultAutomationInterval = "1h"
)

// Config keeps runtime settings loaded from environment variables.
type Config struct {
	host               string
	port               string
	databaseURL        string
	automationInterval string
}

func Load() Config {
	return Config{
		host:               getEnv("FEDEY_API_HOST", defaultHost),
		port:               getEnv("FEDEY_API_PORT", defaultPort),
		databaseURL:        os.Getenv("FEDEY_DATABASE_URL"),
		automationInterval: getEnv("FEDEY_AUTOMATION_INTERVAL", defaultAutomationInterval),
	}
}

func (c Config) APIAddress() string {
	return c.host + ":" + c.port
}

func (c Config) DatabaseURL() string {
	return c.databaseURL
}

func (c Config) AutomationInterval() string {
	return c.automationInterval
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
