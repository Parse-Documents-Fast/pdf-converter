package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/config"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server wraps the chi router and configuration.
type Server struct {
	router      *chi.Mux
	cfg         config.Config
	convertFunc func(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error)
}

// NewServer creates a new HTTP server with all routes configured.
func NewServer(cfg config.Config) *Server {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	srv := &Server{
		router:      r,
		cfg:         cfg,
		convertFunc: converter.Convert,
	}

	r.Get("/health", srv.handleHealth())
	r.Post("/convert", srv.handleConvert())

	return srv
}

// ServeHTTP makes Server implement http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
