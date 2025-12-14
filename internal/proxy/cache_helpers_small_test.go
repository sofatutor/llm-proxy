package proxy

import (
	"net/http"
	"testing"
)

func TestWantsRevalidation_Unit(t *testing.T) {
	if wantsRevalidation(nil) {
		t.Fatalf("nil request should not want revalidation")
	}
	r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	if wantsRevalidation(r) {
		t.Fatalf("empty Cache-Control should not want revalidation")
	}
	r.Header.Set("Cache-Control", "no-cache")
	if !wantsRevalidation(r) {
		t.Fatalf("no-cache should want revalidation")
	}
	r.Header.Set("Cache-Control", "max-age=0")
	if !wantsRevalidation(r) {
		t.Fatalf("max-age=0 should want revalidation")
	}
}

func TestCanServeCachedForRequest_WithAuthorizationRules(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	// No Authorization: always allowed
	if !canServeCachedForRequest(r, http.Header{}) {
		t.Fatalf("expected allowed when no Authorization header")
	}

	// With Authorization requires public or s-maxage>0
	r.Header.Set("Authorization", "Bearer x")
	// private => not allowed
	if canServeCachedForRequest(r, http.Header{"Cache-Control": {"private"}}) {
		t.Fatalf("expected not allowed for private cache with Authorization")
	}
	// public => allowed
	if !canServeCachedForRequest(r, http.Header{"Cache-Control": {"public"}}) {
		t.Fatalf("expected allowed for public cache with Authorization")
	}
	// s-maxage>0 => allowed
	if !canServeCachedForRequest(r, http.Header{"Cache-Control": {"s-maxage=60"}}) {
		t.Fatalf("expected allowed for s-maxage with Authorization")
	}
}

func TestRequestForcedCacheTTL_NilRequest(t *testing.T) {
	// Test nil request returns 0
	if got := requestForcedCacheTTL(nil); got != 0 {
		t.Errorf("requestForcedCacheTTL(nil) = %v, want 0", got)
	}
}

func TestAtoiSafe(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"0", 0},
		{"123", 123},
		{"42abc", 42}, // stops at non-digit
		{"abc", 0},    // no leading digits
		{"12 34", 12}, // stops at space
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := atoiSafe(tc.input); got != tc.expected {
				t.Errorf("atoiSafe(%q) = %d, want %d", tc.input, got, tc.expected)
			}
		})
	}
}

func TestCloneHeadersForCache(t *testing.T) {
	h := http.Header{
		"Content-Type":              {"application/json"},
		"X-Custom":                  {"value1", "value2"},
		"Connection":                {"keep-alive"},                    // hop-by-hop, should be dropped
		"Transfer-Encoding":         {"chunked"},                       // hop-by-hop, should be dropped
		"X-Upstream-Request-Start":  {"123456789"},                     // timing, should be dropped
		"X-Upstream-Request-Stop":   {"987654321"},                     // timing, should be dropped
		"X-Proxy-Received-At":       {"2025-12-11T20:00:00Z"},          // timing, should be dropped
		"X-Proxy-Final-Response-At": {"2025-12-11T20:00:01Z"},          // timing, should be dropped
		"X-Proxy-First-Response-At": {"2025-12-11T20:00:00Z"},          // timing, should be dropped
		"Date":                      {"Wed, 11 Dec 2025 20:00:00 GMT"}, // should be dropped
		"Set-Cookie":                {"session=abc123"},                // user-specific, should be dropped
	}
	cloned := cloneHeadersForCache(h)

	// Check kept headers
	if cloned.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type to be preserved")
	}
	if v := cloned["X-Custom"]; len(v) != 2 || v[0] != "value1" || v[1] != "value2" {
		t.Errorf("expected X-Custom values preserved, got %v", v)
	}

	// Check dropped hop-by-hop headers
	if cloned.Get("Connection") != "" {
		t.Errorf("expected Connection to be dropped")
	}
	if cloned.Get("Transfer-Encoding") != "" {
		t.Errorf("expected Transfer-Encoding to be dropped")
	}

	// Check dropped timing headers
	if cloned.Get("X-Upstream-Request-Start") != "" {
		t.Errorf("expected X-Upstream-Request-Start to be dropped")
	}
	if cloned.Get("X-Upstream-Request-Stop") != "" {
		t.Errorf("expected X-Upstream-Request-Stop to be dropped")
	}
	if cloned.Get("X-Proxy-Received-At") != "" {
		t.Errorf("expected X-Proxy-Received-At to be dropped")
	}
	if cloned.Get("X-Proxy-Final-Response-At") != "" {
		t.Errorf("expected X-Proxy-Final-Response-At to be dropped")
	}
	if cloned.Get("X-Proxy-First-Response-At") != "" {
		t.Errorf("expected X-Proxy-First-Response-At to be dropped")
	}
	if cloned.Get("Date") != "" {
		t.Errorf("expected Date to be dropped")
	}
	if cloned.Get("Set-Cookie") != "" {
		t.Errorf("expected Set-Cookie to be dropped")
	}
}
