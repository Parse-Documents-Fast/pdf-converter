package httpserver

import (
	"context"
	"log/slog"
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

// NewServer creates a new HTTP server with all routes configured, using the
// default slog logger (ADR-0007). Use NewServerWithLogger to inject a logger.
func NewServer(cfg config.Config) *Server {
	return NewServerWithLogger(cfg, slog.Default())
}

// NewServerWithLogger creates a new HTTP server with the given logger.
func NewServerWithLogger(cfg config.Config, logger *slog.Logger) *Server {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(RequestLogger(logger))

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
