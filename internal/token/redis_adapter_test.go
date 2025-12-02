package token

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisGoRateLimitAdapter_AllMethods(t *testing.T) {
	// Start miniredis
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error: %v", err)
	}
	defer s.Close()

	// Create Redis client and adapter
	client := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 0})
	defer func() { _ = client.Close() }()

	adapter := NewRedisGoRateLimitAdapter(client)
	ctx := context.Background()

	// Test Set and Get
	t.Run("Set and Get", func(t *testing.T) {
		err := adapter.Set(ctx, "test-key", "test-value")
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		value, err := adapter.Get(ctx, "test-key")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if value != "test-value" {
			t.Fatalf("expected 'test-value', got '%s'", value)
		}
	})

	// Test Get non-existent key
	t.Run("Get non-existent key", func(t *testing.T) {
		value, err := adapter.Get(ctx, "non-existent-key")
		if err != nil {
			t.Fatalf("Get non-existent key should not error: %v", err)
		}
		if value != "" {
			t.Fatalf("expected empty string, got '%s'", value)
		}
	})

	// Test Incr
	t.Run("Incr", func(t *testing.T) {
		count, err := adapter.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr error: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1, got %d", count)
		}

		count, err = adapter.Incr(ctx, "counter")
		if err != nil {
			t.Fatalf("Incr error: %v", err)
		}
		if count != 2 {
			t.Fatalf("expected 2, got %d", count)
		}
	})

	// Test Expire
	t.Run("Expire", func(t *testing.T) {
		err := adapter.Set(ctx, "expiring-key", "value")
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		err = adapter.Expire(ctx, "expiring-key", time.Second*10)
		if err != nil {
			t.Fatalf("Expire error: %v", err)
		}

		// Verify TTL is set (miniredis stores TTL)
		ttl := s.TTL("expiring-key")
		if ttl < time.Second*9 || ttl > time.Second*11 {
			t.Fatalf("unexpected TTL: %v", ttl)
		}
	})

	// Test SetNX
	t.Run("SetNX", func(t *testing.T) {
		// First SetNX should succeed
		ok, err := adapter.SetNX(ctx, "lock-key", "locked", time.Second*30)
		if err != nil {
			t.Fatalf("SetNX error: %v", err)
		}
		if !ok {
			t.Fatal("first SetNX should succeed")
		}

		// Second SetNX on same key should fail
		ok, err = adapter.SetNX(ctx, "lock-key", "locked-again", time.Second*30)
		if err != nil {
			t.Fatalf("SetNX error: %v", err)
		}
		if ok {
			t.Fatal("second SetNX should fail")
		}
	})

	// Test Del
	t.Run("Del", func(t *testing.T) {
		err := adapter.Set(ctx, "to-delete", "value")
		if err != nil {
			t.Fatalf("Set error: %v", err)
		}

		err = adapter.Del(ctx, "to-delete")
		if err != nil {
			t.Fatalf("Del error: %v", err)
		}

		value, err := adapter.Get(ctx, "to-delete")
		if err != nil {
			t.Fatalf("Get error: %v", err)
		}
		if value != "" {
			t.Fatalf("expected empty string after delete, got '%s'", value)
		}
	})
}

func TestRedisGoRateLimitAdapter_Integration(t *testing.T) {
	// Start miniredis
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run error: %v", err)
	}
	defer s.Close()

	// Create Redis client and adapter
	client := redis.NewClient(&redis.Options{Addr: s.Addr(), DB: 0})
	defer func() { _ = client.Close() }()

	adapter := NewRedisGoRateLimitAdapter(client)

	// Create rate limiter with the adapter
	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    5,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(adapter, config)
	ctx := context.Background()

	tokenID := "integration-test-token"

	// Should allow 5 requests
	for i := 0; i < 5; i++ {
		allowed, err := limiter.Allow(ctx, tokenID)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	allowed, err := limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("6th request should be denied")
	}

	// Reset and verify
	err = limiter.ResetTokenUsage(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected reset error: %v", err)
	}

	// Should allow again after reset
	allowed, err = limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error after reset: %v", err)
	}
	if !allowed {
		t.Fatal("request should be allowed after reset")
	}
}
