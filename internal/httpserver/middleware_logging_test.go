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
	if !strings.Contains(logOutput, "status=201") {
		t.Errorf("expected status=201 in log, got %q", logOutput)
	}
	if strings.Contains(logOutput, "secret markdown content") {
		t.Errorf("request body leaked into logs: %q", logOutput)
	}
}
