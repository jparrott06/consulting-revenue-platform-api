package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultReadTimeoutSec  = 10
	defaultWriteTimeoutSec = 10
	defaultIdleTimeoutSec  = 60
)

// HTTPConfig controls network binding and server timeout behavior.
type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Config is the root app configuration.
type Config struct {
	HTTP HTTPConfig
}

// Load returns configuration from env vars with safe defaults for local dev.
func Load() (Config, error) {
	readTimeoutSec, err := intFromEnv("HTTP_READ_TIMEOUT_SEC", defaultReadTimeoutSec)
	if err != nil {
		return Config{}, err
	}
	writeTimeoutSec, err := intFromEnv("HTTP_WRITE_TIMEOUT_SEC", defaultWriteTimeoutSec)
	if err != nil {
		return Config{}, err
	}
	idleTimeoutSec, err := intFromEnv("HTTP_IDLE_TIMEOUT_SEC", defaultIdleTimeoutSec)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeoutSec, err := intFromEnv("HTTP_SHUTDOWN_TIMEOUT_SEC", 10)
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTP: HTTPConfig{
			Addr:            stringFromEnv("HTTP_ADDR", defaultHTTPAddr),
			ReadTimeout:     time.Duration(readTimeoutSec) * time.Second,
			WriteTimeout:    time.Duration(writeTimeoutSec) * time.Second,
			IdleTimeout:     time.Duration(idleTimeoutSec) * time.Second,
			ShutdownTimeout: time.Duration(shutdownTimeoutSec) * time.Second,
		},
	}, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	raw := stringFromEnv(key, "")
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than 0", key)
	}

	return value, nil
}

func stringFromEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
