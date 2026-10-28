package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/dto"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/problem"
)

const (
	// jsonEscapeWorstCase is the maximum expansion factor of a single input byte
	// when encoding/json escapes it (e.g. '<' -> \u003c, control bytes -> \u00XX).
	// The envelope is bounded to this factor to avoid a false 413 on heavily
	// escaped content while still capping memory usage.
	jsonEscapeWorstCase = 6

	// jsonEnvelopeOverhead accounts for the fixed JSON structure around the
	// content field ({"content":"..."}).
	jsonEnvelopeOverhead = 4096
)

func (s *Server) handleConvert() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			problem.WriteProblem(w, http.StatusUnsupportedMediaType, "Unsupported Media Type", "Content-Type must be application/json", r.URL.Path)
			return
		}

		maxBytes := s.cfg.MaxFileSizeBytes*jsonEscapeWorstCase + jsonEnvelopeOverhead
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
				problem.WriteProblem(w, http.StatusUnprocessableEntity, "Invalid Markdown", "The provided Markdown could not be converted", r.URL.Path)
				return
			}
			problem.WriteProblem(w, http.StatusInternalServerError, "Conversion failed", "The conversion engine failed to produce a PDF", r.URL.Path)
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
