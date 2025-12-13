package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	NodeID   string
	HTTPPort int
	Debug    bool
	LogLevel string

	VolunteerHeartbeatInterval time.Duration
	VolunteerTimeout           time.Duration
}

func Load() *Config {
	return &Config{
		NodeID:                     getEnv("NODE_ID", "node-default"),
		HTTPPort:                   getEnvInt("HTTP_PORT", 8000),
		Debug:                      getEnvBool("DEBUG", false),
		LogLevel:                   getEnv("LOG_LEVEL", "info"),
		VolunteerHeartbeatInterval: time.Duration(getEnvInt("VOLUNTEER_HEARTBEAT_INTERVAL", 30)) * time.Second,
		VolunteerTimeout:           time.Duration(getEnvInt("VOLUNTEER_TIMEOUT", 60)) * time.Second,
	}
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%d", c.HTTPPort)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true" || v == "1"
	}
	return fallback
}

