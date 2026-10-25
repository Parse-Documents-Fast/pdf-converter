package converter

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
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

	markdown := `# Test Title

- Item 1
- Item 2

| Col 1 | Col 2 |
|---|---|
| A | B |
`

	ctx := context.Background()
	pdfBytes, err := Convert(ctx, 10*time.Second, markdown)
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

	// Verify that TMPDIR remains empty (RAM-only conversion)
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpdir: %v", err)
	}

	if len(entries) > 0 {
		t.Errorf("expected TMPDIR to be empty (RAM-only), found %d entries", len(entries))
		for _, entry := range entries {
			t.Logf("unexpected file in TMPDIR: %s", entry.Name())
		}
	}
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
