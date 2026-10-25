package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/problem"
)

func (s *Server) handleConvert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxBytes := s.cfg.MaxFileSizeBytes + 1048576 // JSON envelope leeway
		bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBytes))
		if err != nil {
			problem.WriteProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Request body exceeds maximum allowed size", r.URL.Path)
			return
		}

		var req dto.ConvertRequest
		dec := json.NewDecoder(bytes.NewReader(bodyBytes))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			problem.WriteProblem(w, http.StatusBadRequest, "Invalid Request", "Malformed JSON request body: "+err.Error(), r.URL.Path)
			return
		}

		if req.Content == "" {
			problem.WriteProblem(w, http.StatusBadRequest, "Invalid Request", "Content field cannot be empty", r.URL.Path)
			return
		}

		if int64(len(req.Content)) > s.cfg.MaxFileSizeBytes {
			problem.WriteProblem(w, http.StatusRequestEntityTooLarge, "Payload Too Large", "Markdown content exceeds maximum allowed size", r.URL.Path)
			return
		}

		pdfBytes, err := s.convertFunc(r.Context(), s.cfg.ConvertTimeout, req.Content)
		if err != nil {
			if errors.Is(err, converter.ErrInvalidMarkdown) {
				problem.WriteProblem(w, http.StatusUnprocessableEntity, "Invalid Markdown", err.Error(), r.URL.Path)
				return
			}
			problem.WriteProblem(w, http.StatusInternalServerError, "Conversion failed", err.Error(), r.URL.Path)
			return
		}

		resp := dto.ConvertResponse{
			ContentBase64: pdfBytes,
			MimeType:      "application/pdf",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
