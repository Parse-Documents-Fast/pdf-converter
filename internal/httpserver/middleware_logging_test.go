package httpserver

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())
}

func TestRequestLogger_LogsMetadataOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("secret markdown content"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader("payload"))
	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "http request completed") {
		t.Errorf("expected 'http request completed' log entry, got %q", logOutput)
	}
	for _, field := range []string{"http_method=POST", "http_path=/convert", "http_status=201", "duration_ms=", "bytes=", "request_id="} {
		if !strings.Contains(logOutput, field) {
			t.Errorf("expected %q in log, got %q", field, logOutput)
		}
	}
	if strings.Contains(logOutput, "secret markdown content") {
		t.Errorf("request body leaked into logs: %q", logOutput)
	}
}

func TestRequestLogger_LevelByStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		wantLevel string
	}{
		{name: "2xx is INFO", status: http.StatusOK, wantLevel: "level=INFO"},
		{name: "3xx is INFO", status: http.StatusNotModified, wantLevel: "level=INFO"},
		{name: "4xx is WARN", status: http.StatusBadRequest, wantLevel: "level=WARN"},
		{name: "5xx is ERROR", status: http.StatusInternalServerError, wantLevel: "level=ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			handler := RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			handler.ServeHTTP(rec, req)

			if !strings.Contains(buf.String(), tt.wantLevel) {
				t.Errorf("expected %q in log, got %q", tt.wantLevel, buf.String())
			}
		})
	}
}
