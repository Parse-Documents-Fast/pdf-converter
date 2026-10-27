package converter

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func checkTools(t *testing.T) {
	if _, err := exec.LookPath("pandoc"); err != nil {
		t.Skip("skipping test: pandoc not found in PATH")
	}
	if _, err := exec.LookPath("typst"); err != nil {
		t.Skip("skipping test: typst not found in PATH")
	}
}

func TestConvert_SuccessAndRAMOnly(t *testing.T) {
	checkTools(t)

	// Create a controlled TMPDIR to verify no temporary files are created on disk
	tmpDir, err := os.MkdirTemp("", "pdf-converter-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Setenv("TMPDIR", tmpDir)

	// Watch TMPDIR and the current working directory during the whole
	// conversion to catch transient files (created and then deleted), not just
	// leftovers after Convert returns.
	done := make(chan struct{})
	stop := func() { close(done) }
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

	markdown := `# Test Title

- Item 1
- Item 2

| Col 1 | Col 2 |
|---|---|
| A | B |
`

	ctx := context.Background()
	pdfBytes, err := Convert(ctx, 10*time.Second, markdown)
	stop()
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}

	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF output")
	}

	// Verify magic bytes %PDF
	if !strings.HasPrefix(string(pdfBytes), "%PDF") {
		t.Errorf("expected output to start with %%PDF, got %q", string(pdfBytes)[:10])
	}

	// Verify that no temporary file was observed at any point (RAM-only). The
	// "." directory legitimately contains the test source files, so only flag
	// files that are clearly transient (pandoc/typst intermediates).
	mu.Lock()
	defer mu.Unlock()
	for _, name := range seen {
		if isTransientFile(name) {
			t.Errorf("unexpected transient file during conversion: %s", name)
		}
	}
}

// isTransientFile reports whether a path looks like an intermediate produced by
// pandoc/typst (rather than a legitimate source or test file).
func isTransientFile(path string) bool {
	base := path[strings.LastIndex(path, "/")+1:]
	return strings.HasSuffix(base, ".typ") ||
		strings.HasPrefix(base, "pandoc") ||
		strings.HasPrefix(base, "typst") ||
		strings.HasPrefix(base, ".typst")
}

func TestConvert_Timeout(t *testing.T) {
	checkTools(t)

	// A very short timeout that should trigger context cancellation
	ctx := context.Background()
	markdown := "# Long document\n" + strings.Repeat("Some text\n", 1000)

	_, err := Convert(ctx, 1*time.Nanosecond, markdown)
	if err == nil {
		t.Fatal("expected error due to timeout, got nil")
	}

	if !errors.Is(err, ErrConversion) {
		t.Errorf("expected ErrConversion, got %v", err)
	}
}
