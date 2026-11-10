// Command pdf-converter is the internal Markdown-to-PDF service. It wires the
// configuration, the converter core and the HTTP server, and shuts down
// gracefully on SIGINT/SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/config"
	"github.com/Parse-Documents-Fast/pdf-converter/internal/httpserver"
)

const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: httpserver.NewServerWithLogger(cfg, logger),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("service started",
			"http_addr", cfg.HTTPAddr,
			"log_level", strings.ToLower(strings.TrimSpace(cfg.LogLevel)),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		return fmt.Errorf("http server failed: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown failed: %w", err)
	}

	logger.Info("service stopped")
	return nil
}

// newLogger builds the single structured logger of the service (ADR-0007): a
// JSON handler to stdout with the level from LOG_LEVEL, timestamps in UTC and
// the service name attached to every entry.
func newLogger(level string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: config.ParseLogLevel(level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				a.Value = slog.TimeValue(a.Value.Time().UTC())
			}
			return a
		},
	})
	return slog.New(handler).With("service", "pdf-converter")
}
