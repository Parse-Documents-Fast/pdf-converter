package converter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var (
	// ErrInvalidMarkdown is returned when Pandoc fails to parse the markdown input.
	ErrInvalidMarkdown = errors.New("invalid markdown")

	// ErrConversion is returned when the conversion engine (Pandoc/Typst) fails.
	ErrConversion = errors.New("conversion failed")
)

// Convert executes Pandoc reading from stdin and writing to stdout (RAM-only),
// using Typst as the PDF engine. No temporary files are created on disk.
func Convert(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "pandoc",
		"-f", "markdown",
		"-t", "pdf",
		"--pdf-engine", "typst",
		"-o", "-")

	cmd.Stdin = strings.NewReader(markdown)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := stderr.String()
		if errStr == "" {
			errStr = err.Error()
		}

		// Check if context timed out
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%w: conversion timed out: %s", ErrConversion, errStr)
		}

		// Classify error: check for parse errors or invalid markdown patterns in stderr
		lowerErr := strings.ToLower(errStr)
		if strings.Contains(lowerErr, "could not parse") ||
			strings.Contains(lowerErr, "parse error") ||
			strings.Contains(lowerErr, "unexpected") {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMarkdown, errStr)
		}

		return nil, fmt.Errorf("%w: %s", ErrConversion, errStr)
	}

	return out.Bytes(), nil
}
