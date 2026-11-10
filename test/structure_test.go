package test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Parse-Documents-Fast/pdf-converter/internal/converter"
)

const structureTimeout = 10 * time.Second

// TestConvert_PreservesStructure verifies that the real Pandoc+Typst pipeline
// turns a Markdown document with headings, lists and a table into a valid PDF,
// and that Pandoc maps every construct to its Typst equivalent (no silent loss
// of structure).
func TestConvert_PreservesStructure(t *testing.T) {
	requireConversionTools(t)

	markdown := readFixture(t, "structure.md")

	pdfBytes, err := converter.Convert(context.Background(), structureTimeout, markdown)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected valid PDF output, got prefix %q", pdfPrefix(pdfBytes))
	}

	typst := pandocTypst(t, markdown)
	for _, want := range []string{
		"= Estructura del documento",
		"== Seccion",
		"- Item uno",
		"- Item dos",
		"+ Primero",
		"+ Segundo",
		"table(",
		"table.header",
		"[Columna A]",
		"[A1]",
		"[B2]",
	} {
		if !strings.Contains(typst, want) {
			t.Errorf("expected Typst output to contain %q", want)
		}
	}
}

// TestConvert_PreservesStructureInPDF extracts the text of the generated PDF
// and asserts the document keeps its headings, list items and table cells. It
// is skipped when pdftotext is not installed, since it is an optional tool.
func TestConvert_PreservesStructureInPDF(t *testing.T) {
	requireConversionTools(t)

	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("skipping test: pdftotext not found in PATH")
	}

	markdown := readFixture(t, "structure.md")
	pdfBytes, err := converter.Convert(context.Background(), structureTimeout, markdown)
	if err != nil {
		t.Fatalf("unexpected conversion error: %v", err)
	}

	pdfPath := filepath.Join(t.TempDir(), "structure.pdf")
	if err := os.WriteFile(pdfPath, pdfBytes, 0o600); err != nil {
		t.Fatalf("write PDF: %v", err)
	}

	cmd := exec.Command(pdftotext, "-layout", pdfPath, "-")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pdftotext: %v", err)
	}

	text := string(out)
	for _, want := range []string{
		"Estructura del documento",
		"Seccion",
		"Item uno",
		"Item dos",
		"Columna A",
		"A1",
		"B2",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected PDF text to contain %q", want)
		}
	}
}

func pandocTypst(t *testing.T, markdown string) string {
	t.Helper()

	cmd := exec.Command("pandoc", "-f", "markdown", "-t", "typst")
	cmd.Stdin = strings.NewReader(markdown)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("pandoc -t typst: %v: %s", err, stderr.String())
	}
	return out.String()
}

func readFixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(data)
}

func pdfPrefix(pdf []byte) string {
	if len(pdf) > 10 {
		return string(pdf[:10])
	}
	return string(pdf)
}
