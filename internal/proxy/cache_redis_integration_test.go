package proxy

import (
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// Integration-style test using miniredis to exercise cache_redis Get/Set paths.
func TestRedisCache_GetSet_Miniredis(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	rc := newRedisCache(client, "itest:")

	// store with positive TTL
	cr := cachedResponse{
		statusCode: 201,
		headers:    map[string][]string{"Etag": {"abc"}},
		body:       []byte("payload"),
		expiresAt:  time.Now().Add(200 * time.Millisecond),
	}
	rc.Set("k1", cr)
	got, ok := rc.Get("k1")
	if !ok {
		t.Fatalf("expected hit from redis cache")
	}
	if got.statusCode != 201 || string(got.body) != "payload" || got.headers["Etag"][0] != "abc" {
		t.Fatalf("unexpected data: %#v", got)
	}

	// store with expired TTL should no-op
	cr2 := cachedResponse{statusCode: 200, headers: map[string][]string{}, body: []byte("x"), expiresAt: time.Now().Add(-time.Second)}
	rc.Set("expired", cr2)
	if _, ok := rc.Get("expired"); ok {
		t.Fatalf("expected miss for expired entry")
	}
}
