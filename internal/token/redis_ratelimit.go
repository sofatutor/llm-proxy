package token

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ErrRedisUnavailable is returned when Redis is unavailable and fallback is disabled
var ErrRedisUnavailable = errors.New("redis unavailable for rate limiting")

// RedisRateLimitClient defines the Redis operations needed for distributed rate limiting.
// This is a subset of the eventbus.RedisClient interface focused on rate limiting operations.
type RedisRateLimitClient interface {
	// Incr atomically increments a key and returns the new value
	Incr(ctx context.Context, key string) (int64, error)
	// Get retrieves the value of a key
	Get(ctx context.Context, key string) (string, error)
	// Set sets the value of a key
	Set(ctx context.Context, key, value string) error
	// Expire sets a TTL on a key
	Expire(ctx context.Context, key string, expiration time.Duration) error
	// SetNX sets a key only if it doesn't exist.
	// Included for interface compatibility with eventbus.RedisClient and potential future enhancements.
	SetNX(ctx context.Context, key string, value string, expiration time.Duration) (bool, error)
	// Del deletes a key
	Del(ctx context.Context, key string) error
}

// RedisRateLimiterConfig contains configuration for the Redis rate limiter
type RedisRateLimiterConfig struct {
	// KeyPrefix is the prefix for all Redis keys used by the rate limiter
	KeyPrefix string
	// KeyHashSecret is the HMAC secret for hashing token IDs in Redis keys.
	// When set, token IDs are hashed using HMAC-SHA256 to prevent cleartext exposure.
	// This is recommended for production deployments to enhance security.
	KeyHashSecret []byte
	// DefaultWindowDuration is the default sliding window duration for rate limiting
	DefaultWindowDuration time.Duration
	// DefaultMaxRequests is the default maximum requests per window
	DefaultMaxRequests int
	// EnableFallback enables fallback to in-memory rate limiting when Redis is unavailable
	EnableFallback bool
	// FallbackRate is the rate for fallback in-memory token bucket (tokens per second)
	FallbackRate float64
	// FallbackCapacity is the capacity for fallback in-memory token bucket
	FallbackCapacity int
}

// DefaultRedisRateLimiterConfig returns default configuration
func DefaultRedisRateLimiterConfig() RedisRateLimiterConfig {
	return RedisRateLimiterConfig{
		KeyPrefix:             "ratelimit:",
		DefaultWindowDuration: time.Minute,
		DefaultMaxRequests:    60,
		EnableFallback:        true,
		FallbackRate:          1.0, // 1 token per second
		FallbackCapacity:      10,
	}
}

// RedisRateLimiter implements distributed rate limiting using Redis.
// It uses a sliding window counter algorithm with Redis INCR for atomic operations.
type RedisRateLimiter struct {
	client   RedisRateLimitClient
	config   RedisRateLimiterConfig
	fallback *MemoryRateLimiter

	// Track Redis availability
	redisAvailable   bool
	redisAvailableMu sync.RWMutex

	// Per-token limits (optional override of defaults)
	tokenLimits   map[string]*TokenRateLimit
	tokenLimitsMu sync.RWMutex
}

// TokenRateLimit holds rate limit configuration for a specific token
type TokenRateLimit struct {
	MaxRequests    int
	WindowDuration time.Duration
}

// NewRedisRateLimiter creates a new distributed rate limiter using Redis
func NewRedisRateLimiter(client RedisRateLimitClient, config RedisRateLimiterConfig) *RedisRateLimiter {
	limiter := &RedisRateLimiter{
		client:         client,
		config:         config,
		redisAvailable: true,
		tokenLimits:    make(map[string]*TokenRateLimit),
	}

	// Create fallback in-memory limiter if enabled
	if config.EnableFallback {
		limiter.fallback = NewMemoryRateLimiter(config.FallbackRate, config.FallbackCapacity)
	}

	return limiter
}

// buildKey constructs the Redis key for a token's rate limit counter.
// If KeyHashSecret is configured, the token ID is hashed using HMAC-SHA256
// to prevent cleartext exposure in Redis keys.
func (r *RedisRateLimiter) buildKey(tokenID string, windowStart int64) string {
	keyID := tokenID
	if len(r.config.KeyHashSecret) > 0 {
		keyID = hashTokenID(tokenID, r.config.KeyHashSecret)
	}
	return fmt.Sprintf("%s%s:%d", r.config.KeyPrefix, keyID, windowStart)
}

// hashTokenID generates a non-reversible identifier from a token ID using HMAC-SHA256.
// Returns the first 16 hex characters of the HMAC for brevity while maintaining uniqueness.
func hashTokenID(tokenID string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(tokenID))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// getWindowStart returns the start timestamp for the current window
func (r *RedisRateLimiter) getWindowStart(windowDuration time.Duration) int64 {
	now := time.Now()
	return now.Truncate(windowDuration).Unix()
}

// getTokenLimit returns the rate limit for a token, using defaults if not set
func (r *RedisRateLimiter) getTokenLimit(tokenID string) (int, time.Duration) {
	r.tokenLimitsMu.RLock()
	limit, exists := r.tokenLimits[tokenID]
	r.tokenLimitsMu.RUnlock()

	if exists && limit != nil {
		return limit.MaxRequests, limit.WindowDuration
	}
	return r.config.DefaultMaxRequests, r.config.DefaultWindowDuration
}

// Allow checks if a request from the given token should be allowed.
// Returns true if the request is within rate limits, false otherwise.
func (r *RedisRateLimiter) Allow(ctx context.Context, tokenID string) (bool, error) {
	maxRequests, windowDuration := r.getTokenLimit(tokenID)
	windowStart := r.getWindowStart(windowDuration)
	key := r.buildKey(tokenID, windowStart)

	// Check Redis availability
	r.redisAvailableMu.RLock()
	available := r.redisAvailable
	r.redisAvailableMu.RUnlock()

	if !available {
		return r.handleFallback(tokenID)
	}

	// Try to increment counter atomically in Redis
	count, err := r.client.Incr(ctx, key)
	if err != nil {
		// Redis operation failed
		r.markRedisUnavailable()
		return r.handleFallback(tokenID)
	}

	// Mark Redis as available (successful operation)
	r.markRedisAvailable()

	// Set expiration on first increment (count == 1)
	if count == 1 {
		// Set TTL slightly longer than window to handle edge cases
		ttl := windowDuration + time.Second
		// Ignore expire errors for graceful degradation; orphaned keys will be cleaned up by Redis eventually.
		_ = r.client.Expire(ctx, key, ttl)
	}

	// Check if request is within limit
	return count <= int64(maxRequests), nil
}

// handleFallback handles rate limiting when Redis is unavailable
func (r *RedisRateLimiter) handleFallback(tokenID string) (bool, error) {
	if !r.config.EnableFallback || r.fallback == nil {
		return false, ErrRedisUnavailable
	}
	return r.fallback.Allow(tokenID), nil
}

// markRedisUnavailable marks Redis as unavailable
func (r *RedisRateLimiter) markRedisUnavailable() {
	r.redisAvailableMu.Lock()
	r.redisAvailable = false
	r.redisAvailableMu.Unlock()
}

// markRedisAvailable marks Redis as available
func (r *RedisRateLimiter) markRedisAvailable() {
	r.redisAvailableMu.Lock()
	r.redisAvailable = true
	r.redisAvailableMu.Unlock()
}

// GetRemainingRequests returns the number of remaining requests for a token in the current window
func (r *RedisRateLimiter) GetRemainingRequests(ctx context.Context, tokenID string) (int, error) {
	maxRequests, windowDuration := r.getTokenLimit(tokenID)
	windowStart := r.getWindowStart(windowDuration)
	key := r.buildKey(tokenID, windowStart)

	// Check Redis availability
	r.redisAvailableMu.RLock()
	available := r.redisAvailable
	r.redisAvailableMu.RUnlock()

	if !available {
		if r.config.EnableFallback {
			// Return a reasonable default when in fallback mode
			return r.config.FallbackCapacity, nil
		}
		return 0, ErrRedisUnavailable
	}

	// Get current count from Redis
	countStr, err := r.client.Get(ctx, key)
	if err != nil {
		// Redis error - mark unavailable and use fallback
		r.markRedisUnavailable()
		if r.config.EnableFallback {
			return r.config.FallbackCapacity, nil
		}
		return 0, fmt.Errorf("failed to get rate limit counter: %w", err)
	}

	// Key doesn't exist (empty string returned, no error) - all requests remain
	if countStr == "" {
		return maxRequests, nil
	}

	// Parse count using strconv.Atoi for idiomatic integer parsing
	count, parseErr := strconv.Atoi(countStr)
	if parseErr != nil {
		count = 0
	}

	remaining := maxRequests - count
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// SetTokenLimit sets a custom rate limit for a specific token
func (r *RedisRateLimiter) SetTokenLimit(tokenID string, maxRequests int, windowDuration time.Duration) {
	r.tokenLimitsMu.Lock()
	r.tokenLimits[tokenID] = &TokenRateLimit{
		MaxRequests:    maxRequests,
		WindowDuration: windowDuration,
	}
	r.tokenLimitsMu.Unlock()
}

// RemoveTokenLimit removes the custom rate limit for a token (falls back to defaults)
func (r *RedisRateLimiter) RemoveTokenLimit(tokenID string) {
	r.tokenLimitsMu.Lock()
	delete(r.tokenLimits, tokenID)
	r.tokenLimitsMu.Unlock()
}

// ResetTokenUsage resets the rate limit counter for a token
func (r *RedisRateLimiter) ResetTokenUsage(ctx context.Context, tokenID string) error {
	_, windowDuration := r.getTokenLimit(tokenID)
	windowStart := r.getWindowStart(windowDuration)
	key := r.buildKey(tokenID, windowStart)

	// Check Redis availability
	r.redisAvailableMu.RLock()
	available := r.redisAvailable
	r.redisAvailableMu.RUnlock()

	if !available {
		if r.config.EnableFallback && r.fallback != nil {
			r.fallback.Reset(tokenID)
			return nil
		}
		return ErrRedisUnavailable
	}

	if err := r.client.Del(ctx, key); err != nil {
		r.markRedisUnavailable()
		if r.config.EnableFallback && r.fallback != nil {
			r.fallback.Reset(tokenID)
			return nil
		}
		return fmt.Errorf("failed to reset rate limit counter: %w", err)
	}

	r.markRedisAvailable()
	return nil
}

// IsRedisAvailable returns whether Redis is currently available
func (r *RedisRateLimiter) IsRedisAvailable() bool {
	r.redisAvailableMu.RLock()
	defer r.redisAvailableMu.RUnlock()
	return r.redisAvailable
}

// CheckRedisHealth performs a health check on the Redis connection
func (r *RedisRateLimiter) CheckRedisHealth(ctx context.Context) error {
	// Try a simple operation to verify Redis is working
	testKey := r.config.KeyPrefix + "_healthcheck"
	if err := r.client.Set(ctx, testKey, "1"); err != nil {
		r.markRedisUnavailable()
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Cleanup
	_ = r.client.Del(ctx, testKey)

	r.markRedisAvailable()
	return nil
}
