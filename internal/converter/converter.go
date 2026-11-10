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

// Convert turns Markdown into a PDF entirely through pipes (RAM-only): Pandoc
// emits Typst source to stdout and Typst compiles it from stdin to stdout.
// Pandoc's --pdf-engine=typst path is deliberately avoided because it writes
// toPdfViaTempFile*.html/.pdf files to the working directory.
func Convert(ctx context.Context, timeout time.Duration, markdown string) ([]byte, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	typstSource, err := markdownToTypst(ctx, markdown)
	if err != nil {
		return nil, err
	}

	return typstToPDF(ctx, typstSource)
}

func markdownToTypst(ctx context.Context, markdown string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "pandoc",
		"-f", "markdown",
		"-t", "typst",
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

		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%w: conversion timed out: %s", ErrConversion, errStr)
		}

		// Classify a Pandoc parse error (e.g. malformed YAML metadata): Pandoc
		// reports it with exit code 64 (PandocParseError). Any other non-zero
		// exit is a generic engine failure.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 64 {
			return nil, fmt.Errorf("%w: %s", ErrInvalidMarkdown, errStr)
		}

		return nil, fmt.Errorf("%w: %s", ErrConversion, errStr)
	}

	return out.Bytes(), nil
}

func typstToPDF(ctx context.Context, source []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "typst",
		"compile",
		"--format", "pdf",
		"-", "/dev/stdout")

	cmd.Stdin = bytes.NewReader(source)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errStr := stderr.String()
		if errStr == "" {
			errStr = err.Error()
		}

		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%w: conversion timed out: %s", ErrConversion, errStr)
		}

		return nil, fmt.Errorf("%w: %s", ErrConversion, errStr)
	}

	return out.Bytes(), nil
}
