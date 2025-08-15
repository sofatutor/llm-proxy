package proxy

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"go.uber.org/zap"
)

// Test modifyResponse branches: streaming -> early return, non-streaming with ttl zero -> debug header
func TestModifyResponse_NoCache_TTLZero_SetsDebug(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{HTTPCacheEnabled: true, HTTPCacheDefaultTTL: 0}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	// ensure cache branch executes
	p.cache = newMemoryCache()
	// non-streaming JSON OK but no cache directives and no forced TTL => ttl zero path
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString("{}")), Request: req}
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	if res.Header.Get("X-CACHE-DEBUG") == "" {
		t.Fatalf("expected X-CACHE-DEBUG to explain zero ttl")
	}
}

func TestModifyResponse_StoreForcedTTL_SetsHeaders(t *testing.T) {
	// force TTL via request header, make body small, expect stored
	p := &TransparentProxy{config: ProxyConfig{HTTPCacheEnabled: true, HTTPCacheDefaultTTL: 0, HTTPCacheMaxObjectBytes: 1024}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	// attach a simple in-memory cache
	p.cache = newMemoryCache()
	req, _ := http.NewRequest(http.MethodPost, "http://x", nil)
	req.Header.Set("Cache-Control", "public, max-age=1")
	res := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString("{\"ok\":true}")), Request: req}
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	if res.Header.Get("X-PROXY-CACHE") != "stored" {
		t.Fatalf("expected stored, got %q", res.Header.Get("X-PROXY-CACHE"))
	}
	if res.Header.Get("Cache-Status") == "" {
		t.Fatalf("expected Cache-Status to be set")
	}
}

// minimal in-memory cache implementing httpCache for tests
type memoryCache struct{ m map[string]cachedResponse }

func newMemoryCache() *memoryCache                           { return &memoryCache{m: map[string]cachedResponse{}} }
func (m *memoryCache) Get(key string) (cachedResponse, bool) { v, ok := m.m[key]; return v, ok }
func (m *memoryCache) Set(key string, value cachedResponse)  { m.m[key] = value }

// guard: test that streaming response returns early
func TestModifyResponse_StreamingEarlyReturn(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	// simulate SSE
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(bytes.NewBuffer(nil)), Request: req}
	// ensure no panic and no mutations aside from allowed
	before := res.Header.Clone()
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	// For streaming, modifyResponse returns early; ensure no X-PROXY-CACHE header set
	if res.Header.Get("X-PROXY-CACHE") != "" {
		t.Fatalf("expected no cache store for streaming responses")
	}
	// headers generally unchanged (X-Proxy may not be set for streaming)
	_ = before
}

// additional branches
type errReadCloser struct{}

func (e errReadCloser) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (e errReadCloser) Close() error               { return nil }

func TestModifyResponse_StatusNotCacheable_Debug(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{HTTPCacheEnabled: true}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	p.cache = newMemoryCache()
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{StatusCode: 500, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString("{}")), Request: req}
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	if res.Header.Get("X-CACHE-DEBUG") != "status-not-cacheable" {
		t.Fatalf("expected status-not-cacheable debug, got %q", res.Header.Get("X-CACHE-DEBUG"))
	}
}

func TestModifyResponse_ReadBodyError_Debug(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{HTTPCacheEnabled: true, HTTPCacheDefaultTTL: 60}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	p.cache = newMemoryCache()
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	res := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}, "Cache-Control": {"public, max-age=60"}}, Body: errReadCloser{}, Request: req}
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	if res.Header.Get("X-CACHE-DEBUG") == "" {
		t.Fatalf("expected X-CACHE-DEBUG for read-body-error")
	}
}

func TestModifyResponse_CopyUpstreamRequestStart(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	req, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	req.Header.Set("X-UPSTREAM-REQUEST-START", "t=123")
	res := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/plain"}}, Body: io.NopCloser(bytes.NewBuffer(nil)), Request: req}
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	if res.Header.Get("X-UPSTREAM-REQUEST-START") != "t=123" {
		t.Fatalf("expected header copied to response")
	}
}

func TestModifyResponse_TooLarge_NotStored(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{HTTPCacheEnabled: true, HTTPCacheMaxObjectBytes: 1}, logger: zap.NewNop(), metrics: &ProxyMetrics{}}
	p.cache = newMemoryCache()
	req, _ := http.NewRequest(http.MethodPost, "http://x", nil)
	req.Header.Set("Cache-Control", "public, max-age=5")
	res := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(bytes.NewBufferString("0123456789")), Request: req}
	if err := p.modifyResponse(res); err != nil {
		t.Fatalf("modifyResponse err: %v", err)
	}
	if res.Header.Get("X-PROXY-CACHE") == "stored" {
		t.Fatalf("did not expect store when object exceeds max size")
	}
}
