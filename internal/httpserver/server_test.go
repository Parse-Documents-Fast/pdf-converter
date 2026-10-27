package httpserver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/config"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/problem"
)

func newTestConfig() config.Config {
	return config.Config{
		HTTPAddr:         ":8000",
		LogLevel:         "info",
		ConvertTimeout:   10 * time.Second,
		MaxFileSizeMB:    20,
		MaxFileSizeBytes: 20 * 1048576,
	}
}

func jsonReq(t *testing.T, method, target string, body []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestHealthHandler(t *testing.T) {
	srv := NewServer(newTestConfig())

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
	srv := NewServer(newTestConfig())
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return []byte("%PDF-1.4 mock pdf"), nil
	}

	payload := dto.ConvertRequest{Content: "# Hello"}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := jsonReq(t, http.MethodPost, "/convert", bodyBytes)

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
	srv := NewServer(newTestConfig())

	payload := dto.ConvertRequest{Content: ""}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := jsonReq(t, http.MethodPost, "/convert", bodyBytes)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestConvertHandler_BadRequest_InvalidJSON(t *testing.T) {
	srv := NewServer(newTestConfig())

	rec := httptest.NewRecorder()
	req := jsonReq(t, http.MethodPost, "/convert", []byte("{invalid-json"))
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestConvertHandler_UnsupportedMediaType(t *testing.T) {
	srv := NewServer(newTestConfig())

	tests := []struct {
		name        string
		contentType string
	}{
		{name: "missing content type", contentType: ""},
		{name: "text/plain", contentType: "text/plain"},
		{name: "application/xml", contentType: "application/xml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader([]byte(`{"content":"x"}`)))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnsupportedMediaType {
				t.Errorf("expected status 415, got %d", rec.Code)
			}
		})
	}
}

func TestConvertHandler_ContentTypeWithCharset(t *testing.T) {
	srv := NewServer(newTestConfig())
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return []byte("%PDF-1.4 mock pdf"), nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader([]byte(`{"content":"# Hello"}`)))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
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
	req := jsonReq(t, http.MethodPost, "/convert", bodyBytes)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413, got %d", rec.Code)
	}
}

func TestConvertHandler_HeavilyEscapedContent_NotTooLarge(t *testing.T) {
	cfg := config.Config{
		MaxFileSizeMB:    1,
		MaxFileSizeBytes: 1048576, // 1 MB
	}
	srv := NewServer(cfg)
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return []byte("%PDF-1.4 mock pdf"), nil
	}

	// Content under the limit but heavily escaped by encoding/json: each '<'
	// becomes \u003c (6 bytes). The JSON envelope therefore exceeds the old
	// 1 MB leeway but stays under the deterministic 6x bound.
	content := strings.Repeat("<", 900000)
	bodyBytes, _ := json.Marshal(dto.ConvertRequest{Content: content})

	rec := httptest.NewRecorder()
	req := jsonReq(t, http.MethodPost, "/convert", bodyBytes)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 (no false 413), got %d", rec.Code)
	}
}

func TestConvertHandler_UnprocessableEntity_InvalidMarkdown(t *testing.T) {
	srv := NewServer(newTestConfig())
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return nil, fmt.Errorf("%w: pandoc stderr with markdown fragments", converter.ErrInvalidMarkdown)
	}

	payload := dto.ConvertRequest{Content: "bad markdown"}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := jsonReq(t, http.MethodPost, "/convert", bodyBytes)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d", rec.Code)
	}

	var prob problem.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	if prob.Title != "Invalid Markdown" {
		t.Errorf("expected title 'Invalid Markdown', got %q", prob.Title)
	}
	if strings.Contains(prob.Detail, "stderr") || strings.Contains(prob.Detail, "pandoc") {
		t.Errorf("expected generic detail, got %q", prob.Detail)
	}
}

func TestConvertHandler_InternalServerError_ConversionFailed(t *testing.T) {
	srv := NewServer(newTestConfig())
	srv.convertFunc = func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
		return nil, fmt.Errorf("%w: engine failure", converter.ErrConversion)
	}

	payload := dto.ConvertRequest{Content: "some markdown"}
	bodyBytes, _ := json.Marshal(payload)

	rec := httptest.NewRecorder()
	req := jsonReq(t, http.MethodPost, "/convert", bodyBytes)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	var prob problem.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	if prob.Title != "Conversion failed" {
		t.Errorf("expected title 'Conversion failed', got %q", prob.Title)
	}
	if prob.Detail != "The conversion engine failed to produce a PDF" {
		t.Errorf("expected generic detail, got %q", prob.Detail)
	}
}
