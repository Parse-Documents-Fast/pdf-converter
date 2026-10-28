package problem

import (
	"encoding/json"
	"net/http"
)

const ContentType = "application/problem+json"

// ProblemDetails represents an RFC 9457 error object.
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// WriteProblem writes an RFC 9457 Problem Details response.
func WriteProblem(w http.ResponseWriter, status int, title, detail, instance string) {
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)

	prob := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}

	_ = json.NewEncoder(w).Encode(prob)
}
