package config

import "os"

const (
	defaultHost               = "0.0.0.0"
	defaultPort               = "8080"
	defaultAutomationInterval = "1h"
	defaultXAPIBaseURL        = "https://api.x.com"
	defaultWebAppURL          = "http://localhost:3000"
)

// Config keeps runtime settings loaded from environment variables.
type Config struct {
	host               string
	port               string
	databaseURL        string
	automationInterval string
	xAPIBaseURL        string
	xAccessToken       string
	xUserID            string
	xClientID          string
	xRedirectURI       string
	webAppURL          string
}

func Load() Config {
	return Config{
		host:               getEnv("FEDEY_API_HOST", defaultHost),
		port:               getEnv("FEDEY_API_PORT", defaultPort),
		databaseURL:        os.Getenv("FEDEY_DATABASE_URL"),
		automationInterval: getEnv("FEDEY_AUTOMATION_INTERVAL", defaultAutomationInterval),
		xAPIBaseURL:        getEnv("FEDEY_X_API_BASE_URL", defaultXAPIBaseURL),
		xAccessToken:       os.Getenv("FEDEY_X_ACCESS_TOKEN"),
		xUserID:            os.Getenv("FEDEY_X_USER_ID"),
		xClientID:          os.Getenv("FEDEY_X_CLIENT_ID"),
		xRedirectURI:       os.Getenv("FEDEY_X_REDIRECT_URI"),
		webAppURL:          getEnv("FEDEY_WEB_APP_URL", defaultWebAppURL),
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

func (c Config) XAPIBaseURL() string {
	return c.xAPIBaseURL
}

func (c Config) XAccessToken() string {
	return c.xAccessToken
}

func (c Config) XUserID() string {
	return c.xUserID
}

func (c Config) XClientID() string {
	return c.xClientID
}

func (c Config) XRedirectURI() string {
	return c.xRedirectURI
}

func (c Config) WebAppURL() string {
	return c.webAppURL
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
