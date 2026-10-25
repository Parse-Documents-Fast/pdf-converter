package httpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/config"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/dto"
)

func TestHealthHandler(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:       ":8000",
		LogLevel:       "info",
		ConvertTimeout: 10 * time.Second,
		MaxFileSizeMB:  20,
	}
	srv := NewServer(cfg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	srv.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestConvertHandler_Success(t *testing.T) {
	cfg := config.Config{
		HTTPAddr:         ":8000",
		LogLevel:         "info",
		ConvertTimeout:   10 * time.Second,
		MaxFileSizeMB:    20,
		MaxFileSizeBytes: 20 * 1048576,
	}
	srv := NewServer(cfg)
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return []byte("%PDF-1.4 mock pdf"), nil
	}

	payload := dto.ConvertRequest{Content: "# Hello"}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	srv.ServeHTTP(rec, req)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	var resp dto.ConvertResponse
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode convert response: %v", err)
	}

	expectedB64 := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 mock pdf"))
	gotB64 := base64.StdEncoding.EncodeToString(resp.ContentBase64)
	if gotB64 != expectedB64 {
		t.Errorf("expected content base64 %q, got %q", expectedB64, gotB64)
	}
	if resp.MimeType != "application/pdf" {
		t.Errorf("expected mime_type 'application/pdf', got %q", resp.MimeType)
	}
}

func TestConvertHandler_BadRequest_EmptyContent(t *testing.T) {
	cfg := config.Config{MaxFileSizeMB: 20, MaxFileSizeBytes: 20 * 1048576}
	srv := NewServer(cfg)

	payload := dto.ConvertRequest{Content: ""}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(bodyBytes))
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestConvertHandler_BadRequest_InvalidJSON(t *testing.T) {
	cfg := config.Config{MaxFileSizeMB: 20, MaxFileSizeBytes: 20 * 1048576}
	srv := NewServer(cfg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader([]byte("{invalid-json")))
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestConvertHandler_PayloadTooLarge(t *testing.T) {
	cfg := config.Config{
		MaxFileSizeMB:    1,
		MaxFileSizeBytes: 1048576, // 1 MB
	}
	srv := NewServer(cfg)

	// Create content larger than 1 MB
	largeContent := string(make([]byte, 1048577))
	payload := dto.ConvertRequest{Content: largeContent}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(bodyBytes))
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", rec.Code)
	}
}

func TestConvertHandler_UnprocessableEntity_InvalidMarkdown(t *testing.T) {
	cfg := config.Config{MaxFileSizeMB: 20, MaxFileSizeBytes: 20 * 1048576}
	srv := NewServer(cfg)
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return nil, converter.ErrInvalidMarkdown
	}

	payload := dto.ConvertRequest{Content: "bad markdown"}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(bodyBytes))
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", rec.Code)
	}
}

func TestConvertHandler_InternalServerError_ConversionFailed(t *testing.T) {
	cfg := config.Config{MaxFileSizeMB: 20, MaxFileSizeBytes: 20 * 1048576}
	srv := NewServer(cfg)
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return nil, converter.ErrConversion
	}

	payload := dto.ConvertRequest{Content: "some markdown"}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(bodyBytes))
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}
