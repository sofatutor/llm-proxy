package token

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrRateLimitExceeded is returned when a token exceeds its rate limit
	ErrRateLimitExceeded = errors.New("rate limit exceeded")

	// ErrLimitOperation is returned when an operation on rate limits fails
	ErrLimitOperation = errors.New("limit operation failed")
)

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	// AllowRequest checks if a token is within its rate limits and updates usage
	AllowRequest(ctx context.Context, tokenID string) error
	
	// GetRemainingRequests returns the number of remaining requests for a token
	GetRemainingRequests(ctx context.Context, tokenID string) (int, error)
	
	// ResetUsage resets the usage counter for a token
	ResetUsage(ctx context.Context, tokenID string) error
	
	// UpdateLimit updates the maximum allowed requests for a token
	UpdateLimit(ctx context.Context, tokenID string, maxRequests *int) error
}

// RateLimitStore defines the interface for rate limit persistence
type RateLimitStore interface {
	// GetTokenByID retrieves a token by its ID
	GetTokenByID(ctx context.Context, tokenID string) (TokenData, error)
	
	// IncrementTokenUsage increments the usage count for a token
	IncrementTokenUsage(ctx context.Context, tokenID string) error
	
	// ResetTokenUsage resets the usage count for a token to zero
	ResetTokenUsage(ctx context.Context, tokenID string) error
	
	// UpdateTokenLimit updates the maximum allowed requests for a token
	UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error
}

// StandardRateLimiter implements RateLimiter using a persistent store
type StandardRateLimiter struct {
	store RateLimitStore
}

// NewRateLimiter creates a new StandardRateLimiter with the given store
func NewRateLimiter(store RateLimitStore) *StandardRateLimiter {
	return &StandardRateLimiter{
		store: store,
	}
}

// AllowRequest checks if a token is within its rate limits and updates usage
func (r *StandardRateLimiter) AllowRequest(ctx context.Context, tokenID string) error {
	// Validate token format first
	if err := ValidateTokenFormat(tokenID); err != nil {
		return fmt.Errorf("invalid token format: %w", err)
	}
	
	// Get current token data
	token, err := r.store.GetTokenByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("failed to retrieve token: %w", err)
	}
	
	// Check if token has a rate limit
	if token.MaxRequests != nil {
		// Check if token has exceeded its rate limit
		if token.RequestCount >= *token.MaxRequests {
			return ErrRateLimitExceeded
		}
	}
	
	// Increment usage count
	if err := r.store.IncrementTokenUsage(ctx, tokenID); err != nil {
		return fmt.Errorf("failed to update token usage: %w", err)
	}
	
	return nil
}

// GetRemainingRequests returns the number of remaining requests for a token
func (r *StandardRateLimiter) GetRemainingRequests(ctx context.Context, tokenID string) (int, error) {
	// Validate token format first
	if err := ValidateTokenFormat(tokenID); err != nil {
		return 0, fmt.Errorf("invalid token format: %w", err)
	}
	
	// Get current token data
	token, err := r.store.GetTokenByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return 0, ErrTokenNotFound
		}
		return 0, fmt.Errorf("failed to retrieve token: %w", err)
	}
	
	// If token has no limit, return a high number
	if token.MaxRequests == nil {
		return 1000000000, nil // Unlimited
	}
	
	// Calculate remaining requests
	remaining := *token.MaxRequests - token.RequestCount
	if remaining < 0 {
		remaining = 0
	}
	
	return remaining, nil
}

// ResetUsage resets the usage counter for a token
func (r *StandardRateLimiter) ResetUsage(ctx context.Context, tokenID string) error {
	// Validate token format first
	if err := ValidateTokenFormat(tokenID); err != nil {
		return fmt.Errorf("invalid token format: %w", err)
	}
	
	// Reset token usage
	if err := r.store.ResetTokenUsage(ctx, tokenID); err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("failed to reset token usage: %w", err)
	}
	
	return nil
}

// UpdateLimit updates the maximum allowed requests for a token
func (r *StandardRateLimiter) UpdateLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	// Validate token format first
	if err := ValidateTokenFormat(tokenID); err != nil {
		return fmt.Errorf("invalid token format: %w", err)
	}
	
	// Update token limit
	if err := r.store.UpdateTokenLimit(ctx, tokenID, maxRequests); err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("failed to update token limit: %w", err)
	}
	
	return nil
}

// MemoryRateLimiter implements in-memory rate limiting with token bucket algorithm
type MemoryRateLimiter struct {
	// Mapping from token ID to rate limit data
	limits      map[string]*TokenBucket
	limitsMutex sync.RWMutex
	
	// Default rate (tokens per second)
	defaultRate float64
	
	// Default capacity
	defaultCapacity int
}

// TokenBucket represents a token bucket rate limiter for a specific token
type TokenBucket struct {
	// Current number of tokens in the bucket
	tokens float64
	
	// Maximum number of tokens the bucket can hold
	capacity int
	
	// Rate at which tokens are added to the bucket (tokens per second)
	rate float64
	
	// Last time the bucket was refilled
	lastRefill time.Time
	
	// Mutex to protect the bucket from concurrent access
	mutex sync.Mutex
}

// NewMemoryRateLimiter creates a new in-memory rate limiter with token bucket algorithm
func NewMemoryRateLimiter(defaultRate float64, defaultCapacity int) *MemoryRateLimiter {
	return &MemoryRateLimiter{
		limits:          make(map[string]*TokenBucket),
		defaultRate:     defaultRate,
		defaultCapacity: defaultCapacity,
	}
}

// Allow checks if a request is allowed for a token
func (m *MemoryRateLimiter) Allow(tokenID string) bool {
	m.limitsMutex.RLock()
	bucket, exists := m.limits[tokenID]
	m.limitsMutex.RUnlock()
	
	if !exists {
		// Create a new bucket for this token
		bucket = &TokenBucket{
			tokens:     float64(m.defaultCapacity),
			capacity:   m.defaultCapacity,
			rate:       m.defaultRate,
			lastRefill: time.Now(),
		}
		
		m.limitsMutex.Lock()
		m.limits[tokenID] = bucket
		m.limitsMutex.Unlock()
		
		return true // First request is always allowed
	}
	
	// Try to consume a token from the bucket
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	
	// Refill the bucket based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.lastRefill = now
	
	bucket.tokens += elapsed * bucket.rate
	if bucket.tokens > float64(bucket.capacity) {
		bucket.tokens = float64(bucket.capacity)
	}
	
	// Check if we have a token to consume
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}
	
	return false
}

// SetLimit sets the rate limit for a specific token
func (m *MemoryRateLimiter) SetLimit(tokenID string, rate float64, capacity int) {
	m.limitsMutex.Lock()
	defer m.limitsMutex.Unlock()
	
	bucket, exists := m.limits[tokenID]
	if !exists {
		bucket = &TokenBucket{
			tokens:     float64(capacity),
			capacity:   capacity,
			rate:       rate,
			lastRefill: time.Now(),
		}
		m.limits[tokenID] = bucket
		return
	}
	
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	
	// Refill first to avoid losing tokens
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.lastRefill = now
	
	bucket.tokens += elapsed * bucket.rate
	
	// Update rate and capacity
	bucket.rate = rate
	
	// Adjust tokens if capacity changed
	if capacity < bucket.capacity && bucket.tokens > float64(capacity) {
		bucket.tokens = float64(capacity)
	}
	bucket.capacity = capacity
}

// GetLimit gets the current rate limit and capacity for a token
func (m *MemoryRateLimiter) GetLimit(tokenID string) (float64, int, bool) {
	m.limitsMutex.RLock()
	defer m.limitsMutex.RUnlock()
	
	bucket, exists := m.limits[tokenID]
	if !exists {
		return m.defaultRate, m.defaultCapacity, false
	}
	
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	
	return bucket.rate, bucket.capacity, true
}

// Reset resets the rate limit for a token
func (m *MemoryRateLimiter) Reset(tokenID string) {
	m.limitsMutex.Lock()
	defer m.limitsMutex.Unlock()
	
	bucket, exists := m.limits[tokenID]
	if !exists {
		return
	}
	
	bucket.mutex.Lock()
	defer bucket.mutex.Unlock()
	
	bucket.tokens = float64(bucket.capacity)
	bucket.lastRefill = time.Now()
}

// Remove removes rate limit data for a token
func (m *MemoryRateLimiter) Remove(tokenID string) {
	m.limitsMutex.Lock()
	defer m.limitsMutex.Unlock()
	
	delete(m.limits, tokenID)
}