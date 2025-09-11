package proxy

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisCache_PurgeAndPurgePrefix(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := newRedisCache(client, "test:")

	// Seed three keys with different prefixes
	now := time.Now()
	cache.Set("a:one", cachedResponse{statusCode: 200, headers: map[string][]string{"Content-Type": {"text/plain"}}, body: []byte("1"), expiresAt: now.Add(time.Minute)})
	cache.Set("a:two", cachedResponse{statusCode: 200, headers: map[string][]string{"Content-Type": {"text/plain"}}, body: []byte("2"), expiresAt: now.Add(time.Minute)})
	cache.Set("b:one", cachedResponse{statusCode: 200, headers: map[string][]string{"Content-Type": {"text/plain"}}, body: []byte("3"), expiresAt: now.Add(time.Minute)})

	// Ensure keys exist in Redis
	if n := client.DBSize(context.Background()).Val(); n == 0 {
		t.Fatalf("expected seeded keys, dbsize=0")
	}

	// Purge a single key
	if ok := cache.Purge("a:one"); !ok {
		t.Fatalf("expected Purge to delete a:one")
	}
	if _, ok := cache.Get("a:one"); ok {
		t.Fatalf("expected a:one to be gone")
	}

	// Purge by prefix
	deleted := cache.PurgePrefix("a:")
	if deleted < 1 {
		t.Fatalf("expected at least one key deleted by prefix, got %d", deleted)
	}

	// b:one should remain
	if _, ok := cache.Get("b:one"); !ok {
		t.Fatalf("expected b:one to remain after purging a:*")
	}
}
