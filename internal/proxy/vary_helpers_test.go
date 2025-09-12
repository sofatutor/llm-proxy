package proxy

import (
	"net/http/httptest"
	"testing"
)

func TestStorageKeyForResponse(t *testing.T) {
	r := httptest.NewRequest("GET", "/v1/models?x=1", nil)
	baseKey := CacheKeyFromRequest(r)

	// Empty Vary -> base key
	if got := storageKeyForResponse(r, "", baseKey); got != baseKey {
		t.Fatalf("empty vary: expected base key, got %q", got)
	}
	// Star Vary -> base key
	if got := storageKeyForResponse(r, "*", baseKey); got != baseKey {
		t.Fatalf("star vary: expected base key, got %q", got)
	}

	// Specific header in Vary: include header value
	r.Header.Set("Accept-Language", "en-US")
	withVary := storageKeyForResponse(r, "Accept-Language", baseKey)
	if withVary == baseKey {
		t.Fatalf("vary should change storage key when header present")
	}

	// Changing header should change key again
	r.Header.Set("Accept-Language", "de-DE")
	changed := storageKeyForResponse(r, "Accept-Language", baseKey)
	if changed == withVary {
		t.Fatalf("different header value should produce different storage key")
	}
}

func TestIsVaryCompatible(t *testing.T) {
	r := httptest.NewRequest("GET", "/v1/models", nil)
	lookup := CacheKeyFromRequest(r)

	// No vary -> compatible
	cr := cachedResponse{vary: ""}
	if !isVaryCompatible(r, cr, lookup) {
		t.Fatalf("expected compatible when vary empty")
	}
	// Star vary -> compatible by definition here
	cr = cachedResponse{vary: "*"}
	if !isVaryCompatible(r, cr, lookup) {
		t.Fatalf("expected compatible when vary is '*'")
	}

	// Concrete vary: Origin (not included by conservative base key)
	r2 := httptest.NewRequest("GET", "/v1/models", nil)
	r2.Header.Set("Origin", "https://example.com")
	// Build a lookup key as if cache was filled without vary (base key)
	baseLookup := CacheKeyFromRequest(r2)
	// For compatibility, we require the vary-derived key to equal the lookup key
	// Here they should not equal, so expect false
	cr = cachedResponse{vary: "Origin"}
	if isVaryCompatible(r2, cr, baseLookup) {
		t.Fatalf("expected not compatible when vary-derived key differs from base lookup key")
	}

	// If lookup key is computed with the same vary, it should be compatible
	varyLookup := CacheKeyFromRequestWithVary(r2, "Origin")
	if !isVaryCompatible(r2, cr, varyLookup) {
		t.Fatalf("expected compatible when vary-derived key matches lookup key")
	}
}
