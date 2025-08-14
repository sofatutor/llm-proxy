package token

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockRateLimitStore implements RateLimitStore for testing
type MockRateLimitStore struct {
	tokens         map[string]TokenData
	failOnGet      bool
	failOnIncr     bool
	failOnReset    bool
	failOnLimit    bool
	trackedIncrs   map[string]int
	trackedResets  map[string]bool
	trackedUpdates map[string]*int
}

func NewMockRateLimitStore() *MockRateLimitStore {
	return &MockRateLimitStore{
		tokens:         make(map[string]TokenData),
		failOnGet:      false,
		failOnIncr:     false,
		failOnReset:    false,
		failOnLimit:    false,
		trackedIncrs:   make(map[string]int),
		trackedResets:  make(map[string]bool),
		trackedUpdates: make(map[string]*int),
	}
}

func (m *MockRateLimitStore) GetTokenByID(ctx context.Context, tokenID string) (TokenData, error) {
	if m.failOnGet {
		return TokenData{}, errors.New("mock store failure")
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return TokenData{}, ErrTokenNotFound
	}

	return token, nil
}

func (m *MockRateLimitStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	if m.failOnIncr {
		return errors.New("mock increment failure")
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.RequestCount++
	now := time.Now()
	token.LastUsedAt = &now
	m.tokens[tokenID] = token

	m.trackedIncrs[tokenID]++
	return nil
}

func (m *MockRateLimitStore) ResetTokenUsage(ctx context.Context, tokenID string) error {
	if m.failOnReset {
		return errors.New("mock reset failure")
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.RequestCount = 0
	m.tokens[tokenID] = token

	m.trackedResets[tokenID] = true
	return nil
}

func (m *MockRateLimitStore) UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	if m.failOnLimit {
		return errors.New("mock limit update failure")
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.MaxRequests = maxRequests
	m.tokens[tokenID] = token

	m.trackedUpdates[tokenID] = maxRequests
	return nil
}

func (m *MockRateLimitStore) AddToken(tokenID string, data TokenData) {
	m.tokens[tokenID] = data
}

func TestStandardRateLimiter_AllowRequest(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Set up test tokens
	limit100 := 100

	// Generate valid tokens
	highLimitToken, _ := GenerateToken()
	lowLimitToken, _ := GenerateToken()
	noLimitToken, _ := GenerateToken()
	nonExistentToken, _ := GenerateToken()

	// Token with high limit
	store.AddToken(highLimitToken, TokenData{
		Token:        highLimitToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &limit100,
	})

	// Token with low limit that will be exceeded
	store.AddToken(lowLimitToken, TokenData{
		Token:        lowLimitToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 100,
		MaxRequests:  &limit100,
	})

	// Token with no limit
	store.AddToken(noLimitToken, TokenData{
		Token:        noLimitToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 1000,
		MaxRequests:  nil,
	})

	tests := []struct {
		name      string
		tokenID   string
		wantErr   bool
		wantErrIs error
	}{
		{
			name:      "Allow token with high limit",
			tokenID:   highLimitToken,
			wantErr:   false,
			wantErrIs: nil,
		},
		{
			name:      "Reject token that exceeds limit",
			tokenID:   lowLimitToken,
			wantErr:   true,
			wantErrIs: ErrRateLimitExceeded,
		},
		{
			name:      "Allow token with no limit",
			tokenID:   noLimitToken,
			wantErr:   false,
			wantErrIs: nil,
		},
		{
			name:      "Reject non-existent token",
			tokenID:   nonExistentToken,
			wantErr:   true,
			wantErrIs: ErrTokenNotFound,
		},
		{
			name:      "Reject invalid token format",
			tokenID:   "invalid-token-format",
			wantErr:   true,
			wantErrIs: ErrInvalidTokenFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := limiter.AllowRequest(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("StandardRateLimiter.AllowRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("StandardRateLimiter.AllowRequest() error = %v, want %v", err, tt.wantErrIs)
			}

			// Verify that usage was incremented for allowed tokens
			if err == nil {
				if store.trackedIncrs[tt.tokenID] != 1 {
					t.Errorf("Usage for token %s should have been incremented", tt.tokenID)
				}
			}
		})
	}

	// Test store failures
	t.Run("Get token failure", func(t *testing.T) {
		store.failOnGet = true
		defer func() { store.failOnGet = false }()

		err := limiter.AllowRequest(ctx, highLimitToken)
		if err == nil {
			t.Errorf("StandardRateLimiter.AllowRequest() expected error on get failure")
		}
	})

	t.Run("Increment usage failure", func(t *testing.T) {
		store.failOnIncr = true
		defer func() { store.failOnIncr = false }()

		err := limiter.AllowRequest(ctx, highLimitToken)
		if err == nil {
			t.Errorf("StandardRateLimiter.AllowRequest() expected error on increment failure")
		}
	})
}

func TestStandardRateLimiter_GetRemainingRequests(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Generate valid tokens
	token50, _ := GenerateToken()
	token1, _ := GenerateToken()
	tokenNoLimit, _ := GenerateToken()
	tokenLimitExceeded, _ := GenerateToken()
	nonExistentToken, _ := GenerateToken()

	limit100 := 100

	store.AddToken(token50, TokenData{
		Token:        token50,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &limit100,
	})
	store.AddToken(token1, TokenData{
		Token:        token1,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 99,
		MaxRequests:  &limit100,
	})
	store.AddToken(tokenNoLimit, TokenData{
		Token:        tokenNoLimit,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 1000,
		MaxRequests:  nil,
	})
	store.AddToken(tokenLimitExceeded, TokenData{
		Token:        tokenLimitExceeded,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 100,
		MaxRequests:  &limit100,
	})

	tests := []struct {
		name      string
		tokenID   string
		want      int
		wantErr   bool
		wantErrIs error
	}{
		{"Token with 50 remaining", token50, 50, false, nil},
		{"Token with 1 remaining", token1, 1, false, nil},
		{"Token with no limit", tokenNoLimit, 1000000000, false, nil},
		{"Token with limit exceeded", tokenLimitExceeded, 0, false, nil},
		{"Non-existent token", nonExistentToken, 0, true, ErrTokenNotFound},
		{"Invalid format token", "invalid-token-format", 0, true, ErrInvalidTokenFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remaining, err := limiter.GetRemainingRequests(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("StandardRateLimiter.GetRemainingRequests() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("StandardRateLimiter.GetRemainingRequests() error = %v, want %v", err, tt.wantErrIs)
			}
			if remaining != tt.want {
				t.Errorf("StandardRateLimiter.GetRemainingRequests() = %v, want %v", remaining, tt.want)
			}
		})
	}
}

func TestStandardRateLimiter_ResetUsage(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Generate valid tokens
	resetToken, _ := GenerateToken()
	nonExistentToken, _ := GenerateToken()

	store.AddToken(resetToken, TokenData{
		Token:        resetToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 10,
	})

	tests := []struct {
		name      string
		tokenID   string
		wantErr   bool
		wantErrIs error
	}{
		{"Reset existing token", resetToken, false, nil},
		{"Reset non-existent token", nonExistentToken, true, ErrTokenNotFound},
		{"Invalid format token", "invalid-token-format", true, ErrInvalidTokenFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := limiter.ResetUsage(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("StandardRateLimiter.ResetUsage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("StandardRateLimiter.ResetUsage() error = %v, want %v", err, tt.wantErrIs)
			}
		})
	}
}

func TestStandardRateLimiter_UpdateLimit(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Generate valid tokens
	updateToken, _ := GenerateToken()
	removeLimitToken, _ := GenerateToken()
	nonExistentToken, _ := GenerateToken()

	store.AddToken(updateToken, TokenData{
		Token:        updateToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 10,
	})
	store.AddToken(removeLimitToken, TokenData{
		Token:        removeLimitToken,
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 10,
		MaxRequests:  nil,
	})

	maxReq := 200
	tests := []struct {
		name      string
		tokenID   string
		maxReq    *int
		wantErr   bool
		wantErrIs error
	}{
		{"Update limit to 200", updateToken, &maxReq, false, nil},
		{"Remove limit", removeLimitToken, nil, false, nil},
		{"Update non-existent token", nonExistentToken, &maxReq, true, ErrTokenNotFound},
		{"Invalid format token", "invalid-token-format", &maxReq, true, ErrInvalidTokenFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := limiter.UpdateLimit(ctx, tt.tokenID, tt.maxReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("StandardRateLimiter.UpdateLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("StandardRateLimiter.UpdateLimit() error = %v, want %v", err, tt.wantErrIs)
			}
		})
	}
}

func TestMemoryRateLimiter(t *testing.T) {
	// Create a memory rate limiter with 0 refill rate and max 2 requests
	limiter := NewMemoryRateLimiter(0, 2)

	// Generate valid token
	validToken, _ := GenerateToken()

	// Reset the bucket to ensure it's empty before the test
	limiter.Reset(validToken)

	// Call Allow once to initialize the bucket
	limiter.Allow(validToken)

	// Manually set the token bucket's tokens to 2
	bucket := limiter.limits[validToken]
	bucket.tokens = 2

	// Allow two requests
	for i := 0; i < 2; i++ {
		if !limiter.Allow(validToken) {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// Third request should be denied
	if limiter.Allow(validToken) {
		t.Errorf("Request should be denied after bucket is empty")
	}
}

func TestMemoryRateLimiter_SetGetRemove(t *testing.T) {
	limiter := NewMemoryRateLimiter(1, 5)
	validToken, _ := GenerateToken()

	// Default limit before SetLimit
	rate, cap, exists := limiter.GetLimit(validToken)
	if rate != 1 || cap != 5 || exists {
		t.Errorf("Default GetLimit = (%v, %v, %v), want (1, 5, false)", rate, cap, exists)
	}

	// Set custom limit
	limiter.SetLimit(validToken, 2, 10)
	rate, cap, exists = limiter.GetLimit(validToken)
	if rate != 2 || cap != 10 || !exists {
		t.Errorf("After SetLimit, GetLimit = (%v, %v, %v), want (2, 10, true)", rate, cap, exists)
	}

	// Remove limit
	limiter.Remove(validToken)
	rate, cap, exists = limiter.GetLimit(validToken)
	if rate != 1 || cap != 5 || exists {
		t.Errorf("After Remove, GetLimit = (%v, %v, %v), want (1, 5, false)", rate, cap, exists)
	}
}

func TestMemoryRateLimiter_Reset_NonExistent(t *testing.T) {
	limiter := NewMemoryRateLimiter(1, 5)
	validToken, _ := GenerateToken()
	// Should not panic or error
	limiter.Reset(validToken)
}

func TestMemoryRateLimiter_Reset_Existing(t *testing.T) {
	limiter := NewMemoryRateLimiter(1, 5)
	validToken, _ := GenerateToken()

	// First, use some quota
	limiter.SetLimit(validToken, 1, 5)
	allowed := limiter.Allow(validToken)
	if !allowed {
		t.Fatal("Expected first request to be allowed")
	}

	// Use more quota to reduce available tokens
	for i := 0; i < 4; i++ {
		limiter.Allow(validToken)
	}

	// Verify we're at capacity
	if limiter.Allow(validToken) {
		t.Fatal("Expected request to be rate limited")
	}

	// Reset should restore full capacity
	limiter.Reset(validToken)

	// Should now allow requests again
	if !limiter.Allow(validToken) {
		t.Fatal("Expected request to be allowed after reset")
	}
}

func TestMemoryRateLimiter_ResetAndSetLimit(t *testing.T) {
	rl := NewMemoryRateLimiter(1.0, 10)

	t.Run("Reset non-existent token", func(t *testing.T) {
		rl.Reset("notfound") // Should not panic or error
	})

	t.Run("SetLimit new token", func(t *testing.T) {
		rl.SetLimit("tok1", 2.0, 5)
		rate, cap, ok := rl.GetLimit("tok1")
		if !ok || rate != 2.0 || cap != 5 {
			t.Errorf("SetLimit failed: got rate=%v, cap=%v, ok=%v", rate, cap, ok)
		}
	})

	t.Run("SetLimit existing token, lower capacity", func(t *testing.T) {
		rl.SetLimit("tok1", 1.0, 2)
		rate, cap, ok := rl.GetLimit("tok1")
		if !ok || rate != 1.0 || cap != 2 {
			t.Errorf("SetLimit update failed: got rate=%v, cap=%v, ok=%v", rate, cap, ok)
		}
	})

	t.Run("Remove token", func(t *testing.T) {
		rl.Remove("tok1")
		_, _, ok := rl.GetLimit("tok1")
		if ok {
			t.Error("Remove did not remove token")
		}
	})
}
