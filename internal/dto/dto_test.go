package dto

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestConvertRequest_JSON(t *testing.T) {
	req := ConvertRequest{Content: "# Hello World"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal ConvertRequest: %v", err)
	}

	expected := `{"content":"# Hello World"}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}

	var decoded ConvertRequest
	if err := json.Unmarshal([]byte(expected), &decoded); err != nil {
		t.Fatalf("failed to unmarshal ConvertRequest: %v", err)
	}
	if decoded.Content != req.Content {
		t.Errorf("expected Content %q, got %q", req.Content, decoded.Content)
	}
}

func TestConvertResponse_JSON(t *testing.T) {
	pdfBytes := []byte("%PDF-1.4 test content")
	resp := ConvertResponse{
		ContentBase64: pdfBytes,
		MimeType:      "application/pdf",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal ConvertResponse: %v", err)
	}

	encodedB64 := base64.StdEncoding.EncodeToString(pdfBytes)
	expected := `{"content_base64":"` + encodedB64 + `","mime_type":"application/pdf"}`
	if string(data) != expected {
		t.Errorf("expected %s, got %s", expected, string(data))
	}

	var decoded ConvertResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ConvertResponse: %v", err)
	}

	if string(decoded.ContentBase64) != string(pdfBytes) {
		t.Errorf("expected ContentBase64 %q, got %q", string(pdfBytes), string(decoded.ContentBase64))
	}
	if decoded.MimeType != "application/pdf" {
		t.Errorf("expected MimeType 'application/pdf', got %q", decoded.MimeType)
	}
}
