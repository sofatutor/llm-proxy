package eventtransformer

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/andybalholm/brotli"
)

func TestDecompressAndDecode_Base64AfterDecompression(t *testing.T) {
	// Prepare a JSON payload
	jsonPayload := `{"foo":"bar"}`

	// Compress with gzip
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(jsonPayload))
	if err != nil {
		t.Fatalf("gzip write failed: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}
	compressed := buf.Bytes()

	// Encode compressed data with standard base64
	b64 := base64.StdEncoding.EncodeToString(compressed)

	headers := map[string]interface{}{"content_encoding": "gzip"}
	decoded, ok := DecompressAndDecode(b64, headers)
	if !ok || decoded != jsonPayload {
		t.Errorf("Expected %q, got %q (ok=%v)", jsonPayload, decoded, ok)
	}
}

func TestDecompressAndDecode_Base64URLEncodingAfterDecompression(t *testing.T) {
	// Prepare a JSON payload
	jsonPayload := `{"baz":"qux"}`

	// Compress with gzip
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, err := gz.Write([]byte(jsonPayload))
	if err != nil {
		t.Fatalf("gzip write failed: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close failed: %v", err)
	}
	compressed := buf.Bytes()

	// Encode compressed data with URL-safe base64
	b64url := base64.URLEncoding.EncodeToString(compressed)

	headers := map[string]interface{}{"content_encoding": "gzip"}
	decoded, ok := DecompressAndDecode(b64url, headers)
	if !ok || decoded != jsonPayload {
		t.Errorf("Expected %q, got %q (ok=%v)", jsonPayload, decoded, ok)
	}
}

func TestDecompressAndDecode_BrotliBase64(t *testing.T) {
	jsonPayload := `{"hello":"brotli"}`

	// Compress with brotli
	var buf bytes.Buffer
	br := brotli.NewWriter(&buf)
	_, err := br.Write([]byte(jsonPayload))
	if err != nil {
		t.Fatalf("brotli write failed: %v", err)
	}
	if err := br.Close(); err != nil {
		t.Fatalf("brotli close failed: %v", err)
	}
	compressed := buf.Bytes()

	// Encode compressed data with standard base64
	b64 := base64.StdEncoding.EncodeToString(compressed)

	headers := map[string]interface{}{"content_encoding": "br"}
	decoded, ok := DecompressAndDecode(b64, headers)
	if !ok || decoded != jsonPayload {
		t.Errorf("Expected %q, got %q (ok=%v)", jsonPayload, decoded, ok)
	}
}

func TestExtractAssistantReplyContent(t *testing.T) {
	tests := []struct {
		name    string
		resp    string
		want    string
		wantErr bool
	}{
		{
			name:    "normal completion",
			resp:    `{"choices":[{"message":{"content":"Hello! How can I help you today?","role":"assistant"}}]}`,
			want:    "Hello! How can I help you today?",
			wantErr: false,
		},
		{
			name:    "error response",
			resp:    `{"code":"canceled","error":"Request canceled"}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "empty choices",
			resp:    `{"choices":[]}`,
			want:    "",
			wantErr: false,
		},
		{
			name:    "malformed json",
			resp:    `{"choices":[{"message":{"content":"Hello!"}`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing content",
			resp:    `{"choices":[{"message":{"role":"assistant"}}]}`,
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAssistantReplyContent(tt.resp)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractAssistantReplyContent() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("extractAssistantReplyContent() = %q, want %q", got, tt.want)
			}
		})
	}
}
