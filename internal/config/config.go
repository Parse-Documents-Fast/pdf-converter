package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	BytesPerMB = 1048576 // 1 MB = 1 048 576 bytes (ADR-0008)
)

// Config represents the application configuration.
type Config struct {
	HTTPAddr         string
	LogLevel         string
	ConvertTimeout   time.Duration
	MaxFileSizeMB    int
	MaxFileSizeBytes int64
}

// Load loads configuration from environment variables with sensible defaults.
func Load() (Config, error) {
	httpAddr := getEnv("HTTP_ADDR", ":8000")
	logLevel := getEnv("LOG_LEVEL", "info")

	timeoutStr := getEnv("CONVERT_TIMEOUT", "10s")
	convertTimeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid CONVERT_TIMEOUT %q: %w", timeoutStr, err)
	}

	maxSizeStr := getEnv("MAX_FILE_SIZE_MB", "20")
	maxFileSizeMB, err := strconv.Atoi(maxSizeStr)
	if err != nil || maxFileSizeMB <= 0 {
		return Config{}, fmt.Errorf("invalid MAX_FILE_SIZE_MB %q: %w", maxSizeStr, err)
	}

	maxFileSizeBytes := int64(maxFileSizeMB) * BytesPerMB

	return Config{
		HTTPAddr:         httpAddr,
		LogLevel:         logLevel,
		ConvertTimeout:   convertTimeout,
		MaxFileSizeMB:    maxFileSizeMB,
		MaxFileSizeBytes: maxFileSizeBytes,
	}, nil
}

func getEnv(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// ParseLogLevel maps the LOG_LEVEL string (debug, info, warn, error) to a
// slog.Level. Unknown values fall back to slog.LevelInfo.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
