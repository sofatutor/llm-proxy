package proxy

import (
	"testing"
	"time"
)

func TestInMemoryCache_PurgeAndPurgePrefix(t *testing.T) {
	c := newInMemoryCache()

	// Seed keys
	ttl := time.Now().Add(time.Minute)
	c.Set("a:1", cachedResponse{statusCode: 200, expiresAt: ttl})
	c.Set("a:2", cachedResponse{statusCode: 200, expiresAt: ttl})
	c.Set("b:1", cachedResponse{statusCode: 200, expiresAt: ttl})

	// Purge exact key
	if ok := c.Purge("a:1"); !ok {
		t.Fatalf("expected Purge to return true for existing key")
	}
	if _, ok := c.Get("a:1"); ok {
		t.Fatalf("expected a:1 to be removed")
	}

	// Purge by prefix
	deleted := c.PurgePrefix("a:")
	if deleted != 1 { // a:2 remains under the prefix after a:1 delete
		t.Fatalf("expected to delete 1 remaining key with prefix a:, got %d", deleted)
	}
	if _, ok := c.Get("a:2"); ok {
		t.Fatalf("expected a:2 to be removed by prefix purge")
	}
	// b:1 should remain
	if _, ok := c.Get("b:1"); !ok {
		t.Fatalf("expected b:1 to remain")
	}
}

func TestInMemoryCache_GetRespectsExpiration(t *testing.T) {
	c := newInMemoryCache()

	// Not expired entry should be returned
	c.Set("k1", cachedResponse{statusCode: 200, expiresAt: time.Now().Add(500 * time.Millisecond)})
	if _, ok := c.Get("k1"); !ok {
		t.Fatalf("expected k1 to be present before expiry")
	}

	// Expired entry should be treated as missing
	c.Set("k2", cachedResponse{statusCode: 200, expiresAt: time.Now().Add(-500 * time.Millisecond)})
	if _, ok := c.Get("k2"); ok {
		t.Fatalf("expected k2 to be considered expired and missing")
	}
}
