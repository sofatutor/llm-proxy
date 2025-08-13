package proxy

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

// Unit test for extractResponseMetadata branches: nil body, non-JSON, compressed
func TestExtractResponseMetadata_EdgeCases(t *testing.T) {
	p := &TransparentProxy{logger: newNoopLogger(), metrics: &ProxyMetrics{}}

	// nil body
	res := &http.Response{Header: http.Header{"Content-Type": {"application/json"}}}
	if err := p.extractResponseMetadata(res); err == nil {
		t.Fatalf("expected error for nil body")
	}

	// non-JSON content type
	res = &http.Response{Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(bytes.NewReader([]byte("ok")))}
	if err := p.extractResponseMetadata(res); err != nil {
		t.Fatalf("unexpected error for non-JSON: %v", err)
	}

	// compressed should be skipped (Content-Encoding not identity)
	res = &http.Response{Header: http.Header{"Content-Type": {"application/json"}, "Content-Encoding": {"gzip"}}, Body: io.NopCloser(bytes.NewReader([]byte("{}")))}
	if err := p.extractResponseMetadata(res); err != nil {
		t.Fatalf("unexpected error for compressed: %v", err)
	}

	// valid JSON path
	res = &http.Response{Header: http.Header{"Content-Type": {"application/json"}, "Content-Encoding": {"identity"}}, Body: io.NopCloser(bytes.NewReader([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3},"model":"gpt","id":"x","created":1}`)))}
	if err := p.extractResponseMetadata(res); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Header.Get("X-OpenAI-Model") != "gpt" {
		t.Fatalf("missing metadata headers: %v", res.Header)
	}
}
