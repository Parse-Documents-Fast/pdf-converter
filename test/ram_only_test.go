package test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
)

const ramOnlyMarkdown = `# Test Title

- Item 1
- Item 2

| Col 1 | Col 2 |
|---|---|
| A | B |
`

func requireConversionTools(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("skipping test: pandoc not found in PATH")
	}
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("skipping test: typst not found in PATH")
	}
}

// TestConvert_RAMOnly_NoTransientFiles runs the real conversion while watching
// TMPDIR and the working directory for any transient file created (and possibly
// deleted) during the process. Pandoc's typst engine may write an intermediate
// .typ source; this test is the integration-level check for ADR-0002.
func TestConvert_RAMOnly_NoTransientFiles(t *testing.T) {
	requireConversionTools(t)

	tmpDir, err := os.MkdirTemp("", "pdf-converter-ramonly-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	t.Setenv("TMPDIR", tmpDir)

	done := make(chan struct{})
	var mu sync.Mutex
	var seen []string
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}
			for _, dir := range []string{tmpDir, "."} {
				entries, err := os.ReadDir(dir)
				if err != nil {
					continue
				}
				mu.Lock()
				for _, e := range entries {
					seen = append(seen, dir+"/"+e.Name())
				}
				mu.Unlock()
			}
			time.Sleep(time.Millisecond)
		}
	}()

	pdfBytes, err := converter.Convert(context.Background(), 10*time.Second, ramOnlyMarkdown)
	close(done)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Fatalf("expected valid PDF output, got %q", string(pdfBytes)[:10])
	}

	mu.Lock()
	defer mu.Unlock()
	for _, name := range seen {
		if isTransientFile(name) {
			t.Errorf("unexpected transient file during conversion: %s", name)
		}
	}
}

// TestConvert_StraceNoDiskWrites optionally wraps pandoc with strace to verify
// that no temporary file is opened for writing. Skipped when strace is missing.
func TestConvert_StraceNoDiskWrites(t *testing.T) {
	requireConversionTools(t)

	stracePath, err := exec.LookPath("strace")
	if err != nil {
		t.Skip("skipping test: strace not found in PATH")
	}

	logFile, err := os.CreateTemp("", "pdf-converter-strace-*.log")
	if err != nil {
		t.Fatalf("failed to create strace log: %v", err)
	}
	logPath := logFile.Name()
	logFile.Close()
	defer os.RemoveAll(logPath)

	cmd := exec.Command(stracePath,
		"-f", "-e", "trace=%file",
		"-o", logPath,
		"pandoc", "-f", "markdown", "-t", "pdf", "--pdf-engine", "typst", "-o", "-")
	cmd.Stdin = strings.NewReader(ramOnlyMarkdown)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("pandoc under strace failed: %v: %s", err, stderr.String())
	}

	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read strace log: %v", err)
	}

	for _, line := range strings.Split(string(logBytes), "\n") {
		if isDiskWrite(line) {
			t.Errorf("pandoc/typst wrote to disk (RAM-only violated): %s", line)
		}
	}
}

func isTransientFile(path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	return strings.HasSuffix(base, ".typ") ||
		strings.HasPrefix(base, "pandoc") ||
		strings.HasPrefix(base, "typst") ||
		strings.HasPrefix(base, ".typst")
}

func isDiskWrite(straceLine string) bool {
	if !strings.Contains(straceLine, "O_CREAT") &&
		!strings.Contains(straceLine, "O_WRONLY") &&
		!strings.Contains(straceLine, "O_RDWR") {
		return false
	}
	// Ignore writes to the strace log itself.
	if strings.Contains(straceLine, "strace") {
		return false
	}
	return true
}
