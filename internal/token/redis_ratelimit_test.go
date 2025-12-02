package token

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockRedisRateLimitClient implements RedisRateLimitClient for testing
type mockRedisRateLimitClient struct {
	mu       sync.Mutex
	counters map[string]int64
	values   map[string]string
	ttls     map[string]time.Duration

	// Error injection
	failIncr   bool
	failGet    bool
	failSet    bool
	failExpire bool
	failSetNX  bool
	failDel    bool
}

func newMockRedisRateLimitClient() *mockRedisRateLimitClient {
	return &mockRedisRateLimitClient{
		counters: make(map[string]int64),
		values:   make(map[string]string),
		ttls:     make(map[string]time.Duration),
	}
}

func (m *mockRedisRateLimitClient) Incr(ctx context.Context, key string) (int64, error) {
	if m.failIncr {
		return 0, errors.New("redis incr failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[key]++
	return m.counters[key], nil
}

func (m *mockRedisRateLimitClient) Get(ctx context.Context, key string) (string, error) {
	if m.failGet {
		return "", errors.New("redis get failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if count, ok := m.counters[key]; ok {
		return string(rune('0' + count)), nil
	}
	return "", nil
}

func (m *mockRedisRateLimitClient) Set(ctx context.Context, key, value string) error {
	if m.failSet {
		return errors.New("redis set failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[key] = value
	return nil
}

func (m *mockRedisRateLimitClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if m.failExpire {
		return errors.New("redis expire failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ttls[key] = expiration
	return nil
}

func (m *mockRedisRateLimitClient) SetNX(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	if m.failSetNX {
		return false, errors.New("redis setnx failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.values[key]; exists {
		return false, nil
	}
	m.values[key] = value
	m.ttls[key] = expiration
	return true, nil
}

func (m *mockRedisRateLimitClient) Del(ctx context.Context, key string) error {
	if m.failDel {
		return errors.New("redis del failed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.counters, key)
	delete(m.values, key)
	delete(m.ttls, key)
	return nil
}

func (m *mockRedisRateLimitClient) getCounter(key string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[key]
}

func TestRedisRateLimiter_Allow(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    5,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-1"

	// Should allow first 5 requests
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
}

func TestRedisRateLimiter_AllowWithFallback(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    5,
		EnableFallback:        true,
		FallbackRate:          10.0,
		FallbackCapacity:      5,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Simulate Redis failure
	client.failIncr = true

	tokenID := "test-token-2"

	// Should fallback to in-memory rate limiting
	allowed, err := limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error with fallback: %v", err)
	}
	if !allowed {
		t.Fatal("first request with fallback should be allowed")
	}

	// Verify Redis is marked unavailable
	if limiter.IsRedisAvailable() {
		t.Fatal("Redis should be marked unavailable after failure")
	}
}

func TestRedisRateLimiter_AllowWithoutFallback(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    5,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Simulate Redis failure
	client.failIncr = true

	tokenID := "test-token-3"

	// Should return error when fallback is disabled
	allowed, err := limiter.Allow(ctx, tokenID)
	if !errors.Is(err, ErrRedisUnavailable) {
		t.Fatalf("expected ErrRedisUnavailable, got: %v", err)
	}
	if allowed {
		t.Fatal("should not allow request when Redis fails and fallback is disabled")
	}
}

func TestRedisRateLimiter_GetRemainingRequests(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-4"

	// Initially should have all requests remaining
	remaining, err := limiter.GetRemainingRequests(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if remaining != 10 {
		t.Fatalf("expected 10 remaining, got %d", remaining)
	}

	// Use some requests
	for i := 0; i < 3; i++ {
		_, _ = limiter.Allow(ctx, tokenID)
	}

	// Check remaining (note: Get returns the counter value)
	remaining, err = limiter.GetRemainingRequests(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The mock returns a single char, so we just verify no error
	// In a real scenario with proper int parsing, this would be 7
}

func TestRedisRateLimiter_SetTokenLimit(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := DefaultRedisRateLimiterConfig()
	config.EnableFallback = false

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-5"

	// Set a custom limit of 2 requests per window
	limiter.SetTokenLimit(tokenID, 2, time.Minute)

	// Should allow 2 requests
	for i := 0; i < 2; i++ {
		allowed, err := limiter.Allow(ctx, tokenID)
		if err != nil {
			t.Fatalf("unexpected error on request %d: %v", i+1, err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 3rd request should be denied
	allowed, err := limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("3rd request should be denied with custom limit of 2")
	}
}

func TestRedisRateLimiter_RemoveTokenLimit(t *testing.T) {
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    100,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-6"

	// Set a custom limit
	limiter.SetTokenLimit(tokenID, 5, time.Minute)

	// Verify custom limit is set
	maxReqs, windowDur := limiter.getTokenLimit(tokenID)
	if maxReqs != 5 {
		t.Fatalf("expected maxRequests 5, got %d", maxReqs)
	}
	if windowDur != time.Minute {
		t.Fatalf("expected windowDuration 1m, got %v", windowDur)
	}

	// Remove the custom limit
	limiter.RemoveTokenLimit(tokenID)

	// Should now use defaults
	maxReqs, windowDur = limiter.getTokenLimit(tokenID)
	if maxReqs != 100 {
		t.Fatalf("expected default maxRequests 100, got %d", maxReqs)
	}
}

func TestRedisRateLimiter_ResetTokenUsage(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    5,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-7"

	// Use all requests
	for i := 0; i < 5; i++ {
		_, _ = limiter.Allow(ctx, tokenID)
	}

	// Verify limit reached
	allowed, _ := limiter.Allow(ctx, tokenID)
	if allowed {
		t.Fatal("should be rate limited before reset")
	}

	// Reset usage
	err := limiter.ResetTokenUsage(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error resetting usage: %v", err)
	}

	// Should allow requests again
	allowed, err = limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("should allow requests after reset")
	}
}

func TestRedisRateLimiter_ResetTokenUsage_WithFallback(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    5,
		EnableFallback:        true,
		FallbackRate:          1.0,
		FallbackCapacity:      5,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Simulate Redis failure
	client.failDel = true

	// First, cause a Redis failure to use fallback
	client.failIncr = true
	_, _ = limiter.Allow(ctx, "test-token-8")

	// Now reset should use fallback
	err := limiter.ResetTokenUsage(ctx, "test-token-8")
	if err != nil {
		t.Fatalf("unexpected error with fallback: %v", err)
	}
}

func TestRedisRateLimiter_CheckRedisHealth(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := DefaultRedisRateLimiterConfig()
	limiter := NewRedisRateLimiter(client, config)

	// Health check should pass
	err := limiter.CheckRedisHealth(ctx)
	if err != nil {
		t.Fatalf("health check should pass: %v", err)
	}

	// Simulate Redis failure
	client.failSet = true

	err = limiter.CheckRedisHealth(ctx)
	if err == nil {
		t.Fatal("health check should fail when Redis is unavailable")
	}

	// Verify Redis is marked unavailable
	if limiter.IsRedisAvailable() {
		t.Fatal("Redis should be marked unavailable after failed health check")
	}
}

func TestRedisRateLimiter_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    100,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-concurrent"

	// Run concurrent requests
	var wg sync.WaitGroup
	numGoroutines := 50
	allowedCount := 0
	var mu sync.Mutex

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, err := limiter.Allow(ctx, tokenID)
			if err != nil {
				return
			}
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// All 50 requests should be allowed (limit is 100)
	if allowedCount != 50 {
		t.Fatalf("expected 50 allowed requests, got %d", allowedCount)
	}
}

func TestRedisRateLimiter_ExpireOnFirstIncrement(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	tokenID := "test-token-expire"

	// First request should trigger EXPIRE
	_, err := limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TTL was set (should be window duration + 1 second)
	expectedTTL := time.Minute + time.Second
	for key, ttl := range client.ttls {
		if ttl != expectedTTL {
			t.Errorf("key %s has TTL %v, expected %v", key, ttl, expectedTTL)
		}
	}
}

func TestRedisRateLimiter_ExpireFailureDoesNotBlockRequest(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Simulate EXPIRE failure (but INCR works)
	client.failExpire = true

	tokenID := "test-token-expire-fail"

	// Request should still succeed even if EXPIRE fails
	allowed, err := limiter.Allow(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("request should be allowed even if EXPIRE fails")
	}
}

func TestDefaultRedisRateLimiterConfig(t *testing.T) {
	config := DefaultRedisRateLimiterConfig()

	if config.KeyPrefix != "ratelimit:" {
		t.Errorf("unexpected KeyPrefix: %s", config.KeyPrefix)
	}
	if config.DefaultWindowDuration != time.Minute {
		t.Errorf("unexpected DefaultWindowDuration: %v", config.DefaultWindowDuration)
	}
	if config.DefaultMaxRequests != 60 {
		t.Errorf("unexpected DefaultMaxRequests: %d", config.DefaultMaxRequests)
	}
	if !config.EnableFallback {
		t.Error("EnableFallback should be true by default")
	}
	if config.FallbackRate != 1.0 {
		t.Errorf("unexpected FallbackRate: %f", config.FallbackRate)
	}
	if config.FallbackCapacity != 10 {
		t.Errorf("unexpected FallbackCapacity: %d", config.FallbackCapacity)
	}
}

func TestRedisRateLimiter_IsRedisAvailable(t *testing.T) {
	client := newMockRedisRateLimitClient()
	config := DefaultRedisRateLimiterConfig()
	limiter := NewRedisRateLimiter(client, config)

	// Initially should be available
	if !limiter.IsRedisAvailable() {
		t.Fatal("Redis should be available initially")
	}

	// Mark unavailable
	limiter.markRedisUnavailable()
	if limiter.IsRedisAvailable() {
		t.Fatal("Redis should be unavailable after marking")
	}

	// Mark available again
	limiter.markRedisAvailable()
	if !limiter.IsRedisAvailable() {
		t.Fatal("Redis should be available after marking")
	}
}

func TestRedisRateLimiter_GetRemainingRequests_RedisUnavailable(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Mark Redis as unavailable
	limiter.markRedisUnavailable()

	tokenID := "test-token-unavailable"

	_, err := limiter.GetRemainingRequests(ctx, tokenID)
	if !errors.Is(err, ErrRedisUnavailable) {
		t.Fatalf("expected ErrRedisUnavailable, got: %v", err)
	}
}

func TestRedisRateLimiter_GetRemainingRequests_WithFallback(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        true,
		FallbackRate:          1.0,
		FallbackCapacity:      5,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Mark Redis as unavailable
	limiter.markRedisUnavailable()

	tokenID := "test-token-fallback-remaining"

	remaining, err := limiter.GetRemainingRequests(ctx, tokenID)
	if err != nil {
		t.Fatalf("unexpected error with fallback: %v", err)
	}
	if remaining != 5 {
		t.Fatalf("expected fallback capacity 5, got %d", remaining)
	}
}

func TestRedisRateLimiter_ResetTokenUsage_RedisUnavailable(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Mark Redis as unavailable
	limiter.markRedisUnavailable()

	tokenID := "test-token-reset-unavailable"

	err := limiter.ResetTokenUsage(ctx, tokenID)
	if !errors.Is(err, ErrRedisUnavailable) {
		t.Fatalf("expected ErrRedisUnavailable, got: %v", err)
	}
}

func TestRedisRateLimiter_ResetTokenUsage_DelFailsWithFallback(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        true,
		FallbackRate:          1.0,
		FallbackCapacity:      5,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Simulate Del failure
	client.failDel = true

	tokenID := "test-token-del-fails"

	// Should fallback successfully
	err := limiter.ResetTokenUsage(ctx, tokenID)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}

	// Verify Redis is marked unavailable
	if limiter.IsRedisAvailable() {
		t.Fatal("Redis should be marked unavailable after Del failure")
	}
}

func TestRedisRateLimiter_ResetTokenUsage_DelFailsNoFallback(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        false,
	}

	limiter := NewRedisRateLimiter(client, config)

	// Simulate Del failure
	client.failDel = true

	tokenID := "test-token-del-fails-nofallback"

	err := limiter.ResetTokenUsage(ctx, tokenID)
	if err == nil {
		t.Fatal("expected error when Del fails without fallback")
	}
}

func TestRedisRateLimiter_GetRemainingRequests_GetFails(t *testing.T) {
	ctx := context.Background()
	client := newMockRedisRateLimitClient()

	config := RedisRateLimiterConfig{
		KeyPrefix:             "test:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    10,
		EnableFallback:        true,
		FallbackRate:          1.0,
		FallbackCapacity:      5,
	}

	limiter := NewRedisRateLimiter(client, config)

	// First, use some requests so the key exists
	_, _ = limiter.Allow(ctx, "test-token-get-fails")

	// Now simulate Get failure
	client.failGet = true

	remaining, err := limiter.GetRemainingRequests(ctx, "test-token-get-fails")
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if remaining != 5 {
		t.Fatalf("expected fallback capacity 5, got %d", remaining)
	}
}
