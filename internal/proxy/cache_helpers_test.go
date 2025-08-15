package proxy

import (
	"net/http"
	"testing"
	"time"
)

func TestRequestForcedCacheTTL(t *testing.T) {
	r := &http.Request{Header: http.Header{}}
	if d := requestForcedCacheTTL(r); d != 0 {
		t.Fatalf("expected 0, got %v", d)
	}
	// Not public -> 0
	r.Header.Set("Cache-Control", "max-age=10")
	if d := requestForcedCacheTTL(r); d != 0 {
		t.Fatalf("expected 0 without public, got %v", d)
	}
	// public + s-maxage
	r.Header.Set("Cache-Control", "public, s-maxage=3")
	if d := requestForcedCacheTTL(r); d != 3*time.Second {
		t.Fatalf("expected 3s, got %v", d)
	}
	// public + max-age
	r.Header.Set("Cache-Control", "public, max-age=7")
	if d := requestForcedCacheTTL(r); d != 7*time.Second {
		t.Fatalf("expected 7s, got %v", d)
	}
}

func TestCacheTTLFromHeaders(t *testing.T) {
	res := &http.Response{Header: http.Header{}}
	// no-store wins
	res.Header.Set("Cache-Control", "no-store, max-age=10")
	if ttl := cacheTTLFromHeaders(res, 5*time.Second); ttl != 0 {
		t.Fatalf("expected 0, got %v", ttl)
	}

	// s-maxage preferred
	res.Header.Set("Cache-Control", "public, s-maxage=4, max-age=9")
	if ttl := cacheTTLFromHeaders(res, 0); ttl != 4*time.Second {
		t.Fatalf("expected 4s, got %v", ttl)
	}

	// fallback to max-age
	res.Header.Set("Cache-Control", "public, max-age=6")
	if ttl := cacheTTLFromHeaders(res, 0); ttl != 6*time.Second {
		t.Fatalf("expected 6s, got %v", ttl)
	}

	// default TTL when allowed and no explicit age
	res.Header.Set("Cache-Control", "public")
	if ttl := cacheTTLFromHeaders(res, 2*time.Second); ttl != 2*time.Second {
		t.Fatalf("expected 2s, got %v", ttl)
	}

	// private should not take default TTL
	res.Header.Set("Cache-Control", "private")
	if ttl := cacheTTLFromHeaders(res, 2*time.Second); ttl != 0 {
		t.Fatalf("expected 0 for private, got %v", ttl)
	}
}
