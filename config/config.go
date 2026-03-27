package config

import (
	"os"
	"strconv"
	"time"
)

// Configuration variables (var keyword)
var MaxRequests int = 5
var WindowDuration time.Duration = 10 * time.Second
var JWTSecret string = envOrDefault("JWT_SECRET", "change-this-secret-for-production")
var JWTExpiry time.Duration = durationFromEnv("JWT_EXPIRY_SECONDS", 3600)
var MaxServeBlockedBatch int = intFromEnv("MAX_BLOCKED_SERVE_BATCH", 10)

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func durationFromEnv(key string, fallbackSeconds int) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return time.Duration(fallbackSeconds) * time.Second
	}

	seconds, err := strconv.Atoi(v)
	if err != nil || seconds <= 0 {
		return time.Duration(fallbackSeconds) * time.Second
	}

	return time.Duration(seconds) * time.Second
}

func intFromEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(v)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
