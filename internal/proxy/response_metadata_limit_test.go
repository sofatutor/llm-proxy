package proxy

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"go.uber.org/zap"
)

func TestExtractResponseMetadata_Limited_SkipsLargeBodiesAndPreservesBody(t *testing.T) {
	p := &TransparentProxy{
		config: ProxyConfig{ResponseMetadataMaxBytes: 64},
		logger: zap.NewNop(),
	}

	// Large JSON body (unknown content length), will be truncated by max bytes.
	origBody := `{"model":"gpt-4","usage":{"prompt_tokens":10,"completion_tokens":20,"total_tokens":30},"pad":"` +
		string(bytes.Repeat([]byte("x"), 1024)) + `"}` // definitely > 64 bytes

	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewBufferString(origBody)),
		ContentLength: -1, // unknown
		Request:       req,
	}

	err := p.extractResponseMetadata(res)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := res.Header.Get("X-OpenAI-Model"); got != "" {
		t.Fatalf("expected no metadata headers for truncated body, got %q", got)
	}

	after, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatalf("read restored body: %v", readErr)
	}
	if string(after) != origBody {
		t.Fatalf("expected preserved body; mismatch")
	}
}

func TestExtractResponseMetadata_Limited_ParsesSmallBodies(t *testing.T) {
	p := &TransparentProxy{
		config: ProxyConfig{ResponseMetadataMaxBytes: 1024},
		logger: zap.NewNop(),
	}

	origBody := `{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewBufferString(origBody)),
		ContentLength: -1,
		Request:       req,
	}

	err := p.extractResponseMetadata(res)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := res.Header.Get("X-OpenAI-Model"); got != "gpt-4" {
		t.Fatalf("expected model header, got %q", got)
	}

	after, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatalf("read restored body: %v", readErr)
	}
	if string(after) != origBody {
		t.Fatalf("expected preserved body; mismatch")
	}
}

func TestExtractResponseMetadata_Limited_KnownContentLength_ExceedsLimit_SkipsWithoutConsuming(t *testing.T) {
	p := &TransparentProxy{
		config: ProxyConfig{ResponseMetadataMaxBytes: 64},
		logger: zap.NewNop(),
	}

	origBody := `{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewBufferString(origBody)),
		ContentLength: 999, // known and exceeds limit => early exit
		Request:       req,
	}

	err := p.extractResponseMetadata(res)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := res.Header.Get("X-OpenAI-Model"); got != "" {
		t.Fatalf("expected no metadata headers on early exit, got %q", got)
	}

	after, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	if string(after) != origBody {
		t.Fatalf("expected body not to be consumed on early exit; mismatch")
	}
}

func TestExtractResponseMetadata_Unlimited_MaxBytesZero_ExtractsMetadata(t *testing.T) {
	p := &TransparentProxy{
		config: ProxyConfig{ResponseMetadataMaxBytes: 0},
		logger: zap.NewNop(),
	}

	origBody := `{"model":"gpt-4","usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3},"pad":"` +
		string(bytes.Repeat([]byte("x"), 2048)) + `"}` // > typical small cap
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"application/json"}},
		Body:          io.NopCloser(bytes.NewBufferString(origBody)),
		ContentLength: 9999, // known; should NOT early exit when maxBytes==0
		Request:       req,
	}

	err := p.extractResponseMetadata(res)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := res.Header.Get("X-OpenAI-Model"); got != "gpt-4" {
		t.Fatalf("expected model header, got %q", got)
	}

	after, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		t.Fatalf("read body: %v", readErr)
	}
	if string(after) != origBody {
		t.Fatalf("expected body to be preserved; mismatch")
	}
}
