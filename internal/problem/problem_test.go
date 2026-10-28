package problem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteProblem(rec, http.StatusBadRequest, "Invalid Request", "Content cannot be empty", "/convert")

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
	}

	if ct := res.Header.Get("Content-Type"); ct != ContentType {
		t.Errorf("expected Content-Type %q, got %q", ContentType, ct)
	}

	var prob ProblemDetails
	if err := json.NewDecoder(res.Body).Decode(&prob); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if prob.Type != "about:blank" {
		t.Errorf("expected type 'about:blank', got %q", prob.Type)
	}
	if prob.Title != "Invalid Request" {
		t.Errorf("expected title 'Invalid Request', got %q", prob.Title)
	}
	if prob.Status != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, prob.Status)
	}
	if prob.Detail != "Content cannot be empty" {
		t.Errorf("expected detail 'Content cannot be empty', got %q", prob.Detail)
	}
	if prob.Instance != "/convert" {
		t.Errorf("expected instance '/convert', got %q", prob.Instance)
	}
}
