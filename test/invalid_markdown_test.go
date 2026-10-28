package test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/config"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/httpserver"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/problem"
)

// invalidMarkdown is a document Pandoc rejects at parse time (malformed YAML
// metadata block), which must be classified as ErrInvalidMarkdown (422).
const invalidMarkdown = "---\ntitle: [unclosed\n---\n# Hi\n"

// TestConvert_InvalidMarkdown_Real feeds real Pandoc an unparseable document and
// asserts it is classified as ErrInvalidMarkdown (not a generic engine failure).
func TestConvert_InvalidMarkdown_Real(t *testing.T) {
	requireConversionTools(t)

	_, err := converter.Convert(context.Background(), 10*time.Second, invalidMarkdown)
	if err == nil {
		t.Fatal("expected conversion error, got nil")
	}
	if !errors.Is(err, converter.ErrInvalidMarkdown) {
		t.Fatalf("expected ErrInvalidMarkdown, got %v", err)
	}
}

// TestConvertHandler_InvalidMarkdown_Real exercises the full HTTP path with the
// real converter: unparseable Markdown must return 422 with a generic detail.
func TestConvertHandler_InvalidMarkdown_Real(t *testing.T) {
	requireConversionTools(t)

	cfg := config.Config{
		HTTPAddr:         ":8000",
		LogLevel:         "info",
		ConvertTimeout:   10 * time.Second,
		MaxFileSizeMB:    20,
		MaxFileSizeBytes: 20 * 1048576,
	}
	srv := httpserver.NewServer(cfg)

	payload, _ := json.Marshal(map[string]string{"content": invalidMarkdown})
	req := httptest.NewRequest(http.MethodPost, "/convert", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", rec.Code)
	}

	var prob problem.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode problem response: %v", err)
	}
	if prob.Title != "Invalid Markdown" {
		t.Errorf("expected title 'Invalid Markdown', got %q", prob.Title)
	}
	for _, leak := range []string{"yaml", "unterminated", "parse", "stderr"} {
		if strings.Contains(prob.Detail, leak) {
			t.Errorf("expected generic detail, got %q", prob.Detail)
		}
	}
}
