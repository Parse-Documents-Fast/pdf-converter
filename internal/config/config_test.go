package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("CONVERT_TIMEOUT", "")
	t.Setenv("MAX_FILE_SIZE_MB", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.HTTPAddr != ":8000" {
		t.Errorf("expected HTTPAddr ':8000', got %q", cfg.HTTPAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel 'info', got %q", cfg.LogLevel)
	}
	if cfg.ConvertTimeout != 10*time.Second {
		t.Errorf("expected ConvertTimeout 10s, got %v", cfg.ConvertTimeout)
	}
	if cfg.MaxFileSizeMB != 20 {
		t.Errorf("expected MaxFileSizeMB 20, got %d", cfg.MaxFileSizeMB)
	}
	expectedBytes := int64(20 * 1048576) // 20971520
	if cfg.MaxFileSizeBytes != expectedBytes {
		t.Errorf("expected MaxFileSizeBytes %d, got %d", expectedBytes, cfg.MaxFileSizeBytes)
	}
}

func TestLoad_CustomEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9000")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("CONVERT_TIMEOUT", "5s")
	t.Setenv("MAX_FILE_SIZE_MB", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.HTTPAddr != ":9000" {
		t.Errorf("expected HTTPAddr ':9000', got %q", cfg.HTTPAddr)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %q", cfg.LogLevel)
	}
	if cfg.ConvertTimeout != 5*time.Second {
		t.Errorf("expected ConvertTimeout 5s, got %v", cfg.ConvertTimeout)
	}
	if cfg.MaxFileSizeMB != 10 {
		t.Errorf("expected MaxFileSizeMB 10, got %d", cfg.MaxFileSizeMB)
	}
	expectedBytes := int64(10 * 1048576) // 10485760
	if cfg.MaxFileSizeBytes != expectedBytes {
		t.Errorf("expected MaxFileSizeBytes %d, got %d", expectedBytes, cfg.MaxFileSizeBytes)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name          string
		envKey        string
		envVal        string
		expectedError string
	}{
		{
			name:          "invalid convert timeout",
			envKey:        "CONVERT_TIMEOUT",
			envVal:        "invalid",
			expectedError: "invalid CONVERT_TIMEOUT",
		},
		{
			name:          "invalid max file size MB non-integer",
			envKey:        "MAX_FILE_SIZE_MB",
			envVal:        "abc",
			expectedError: "invalid MAX_FILE_SIZE_MB",
		},
		{
			name:          "invalid max file size MB negative",
			envKey:        "MAX_FILE_SIZE_MB",
			envVal:        "-5",
			expectedError: "invalid MAX_FILE_SIZE_MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			_, err := Load()
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.expectedError)
			}
		})
	}
}
