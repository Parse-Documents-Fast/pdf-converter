package dto

// ConvertRequest represents the incoming payload for POST /convert.
type ConvertRequest struct {
	Content string `json:"content"`
}

// ConvertResponse represents the outgoing payload for POST /convert.
type ConvertResponse struct {
	ContentBase64 []byte `json:"content_base64"`
	MimeType      string `json:"mime_type"`
}
