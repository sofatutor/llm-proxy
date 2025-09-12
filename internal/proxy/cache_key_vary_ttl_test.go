package proxy

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCacheKeyFromRequestWithVary_IncludesBodyAndTTLForPOST(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/chat/completions?model=gpt-4o&n=1", nil)
	r.Header.Set("X-Body-Hash", "abc123")
	// Client opts in to shared cache with s-maxage
	r.Header.Set("Cache-Control", "public, s-maxage=120")
	// Upstream responded with Vary: Accept-Language
	key := CacheKeyFromRequestWithVary(r, "Accept-Language")

	if !strings.Contains(key, "|body=abc123") {
		t.Fatalf("expected key to contain body hash, got %q", key)
	}
	if !strings.Contains(key, "|ttl=smax=120") {
		t.Fatalf("expected key to contain ttl smax=120, got %q", key)
	}
}
