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
	limit1 := 1

	// Token with high limit
	store.AddToken("tkn_highlimittoken123456789", TokenData{
		Token:        "tkn_highlimittoken123456789",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &limit100,
	})

	// Token with low limit that will be exceeded
	store.AddToken("tkn_lowlimittoken1234567890", TokenData{
		Token:        "tkn_lowlimittoken1234567890",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 1,
		MaxRequests:  &limit1,
	})

	// Token with no limit
	store.AddToken("tkn_nolimittoken12345678901", TokenData{
		Token:        "tkn_nolimittoken12345678901",
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
			tokenID:   "tkn_highlimittoken123456789",
			wantErr:   false,
			wantErrIs: nil,
		},
		{
			name:      "Reject token that exceeds limit",
			tokenID:   "tkn_lowlimittoken1234567890",
			wantErr:   true,
			wantErrIs: ErrRateLimitExceeded,
		},
		{
			name:      "Allow token with no limit",
			tokenID:   "tkn_nolimittoken12345678901",
			wantErr:   false,
			wantErrIs: nil,
		},
		{
			name:      "Reject non-existent token",
			tokenID:   "tkn_nonexistenttoken1234567",
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

		err := limiter.AllowRequest(ctx, "tkn_highlimittoken123456789")
		if err == nil {
			t.Errorf("StandardRateLimiter.AllowRequest() expected error on get failure")
		}
	})

	t.Run("Increment usage failure", func(t *testing.T) {
		store.failOnIncr = true
		defer func() { store.failOnIncr = false }()

		err := limiter.AllowRequest(ctx, "tkn_highlimittoken123456789")
		if err == nil {
			t.Errorf("StandardRateLimiter.AllowRequest() expected error on increment failure")
		}
	})
}

func TestStandardRateLimiter_GetRemainingRequests(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Set up test tokens
	limit100 := 100
	limit10 := 10

	// Token with high limit
	store.AddToken("tkn_highlimittoken123456789", TokenData{
		Token:        "tkn_highlimittoken123456789",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &limit100,
	})

	// Token with low remaining count
	store.AddToken("tkn_lowremainingtoken123456", TokenData{
		Token:        "tkn_lowremainingtoken123456",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 9,
		MaxRequests:  &limit10,
	})

	// Token with no limit
	store.AddToken("tkn_nolimittoken12345678901", TokenData{
		Token:        "tkn_nolimittoken12345678901",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 1000,
		MaxRequests:  nil,
	})

	// Token with limit exceeded
	store.AddToken("tkn_exceededtoken1234567890", TokenData{
		Token:        "tkn_exceededtoken1234567890",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 20,
		MaxRequests:  &limit10,
	})

	tests := []struct {
		name            string
		tokenID         string
		wantRemaining   int
		wantErr         bool
		wantErrIs       error
		remainingCheck  func(int) bool
	}{
		{
			name:          "Token with 50 remaining",
			tokenID:       "tkn_highlimittoken123456789",
			wantRemaining: 50,
			wantErr:       false,
			wantErrIs:     nil,
		},
		{
			name:          "Token with 1 remaining",
			tokenID:       "tkn_lowremainingtoken123456",
			wantRemaining: 1,
			wantErr:       false,
			wantErrIs:     nil,
		},
		{
			name:          "Token with no limit",
			tokenID:       "tkn_nolimittoken12345678901",
			wantRemaining: 1000000000, // Large number for unlimited
			wantErr:       false,
			wantErrIs:     nil,
		},
		{
			name:          "Token with limit exceeded",
			tokenID:       "tkn_exceededtoken1234567890",
			wantRemaining: 0,
			wantErr:       false,
			wantErrIs:     nil,
		},
		{
			name:          "Non-existent token",
			tokenID:       "tkn_nonexistenttoken1234567",
			wantRemaining: 0,
			wantErr:       true,
			wantErrIs:     ErrTokenNotFound,
		},
		{
			name:          "Invalid token format",
			tokenID:       "invalid-token-format",
			wantRemaining: 0,
			wantErr:       true,
			wantErrIs:     ErrInvalidTokenFormat,
		},
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

			if err == nil && remaining != tt.wantRemaining {
				t.Errorf("StandardRateLimiter.GetRemainingRequests() remaining = %v, want %v", remaining, tt.wantRemaining)
			}
		})
	}

	// Test store failure
	t.Run("Get token failure", func(t *testing.T) {
		store.failOnGet = true
		defer func() { store.failOnGet = false }()

		_, err := limiter.GetRemainingRequests(ctx, "tkn_highlimittoken123456789")
		if err == nil {
			t.Errorf("StandardRateLimiter.GetRemainingRequests() expected error on get failure")
		}
	})
}

func TestStandardRateLimiter_ResetUsage(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Set up test tokens
	limit100 := 100

	// Token to reset
	store.AddToken("tkn_resettoken123456789012", TokenData{
		Token:        "tkn_resettoken123456789012",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &limit100,
	})

	tests := []struct {
		name      string
		tokenID   string
		wantErr   bool
		wantErrIs error
	}{
		{
			name:      "Reset existing token",
			tokenID:   "tkn_resettoken123456789012",
			wantErr:   false,
			wantErrIs: nil,
		},
		{
			name:      "Reset non-existent token",
			tokenID:   "tkn_nonexistenttoken1234567",
			wantErr:   true,
			wantErrIs: ErrTokenNotFound,
		},
		{
			name:      "Reset with invalid token format",
			tokenID:   "invalid-token-format",
			wantErr:   true,
			wantErrIs: ErrInvalidTokenFormat,
		},
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

			// Verify that usage was reset for allowed tokens
			if err == nil {
				if !store.trackedResets[tt.tokenID] {
					t.Errorf("Usage for token %s should have been reset", tt.tokenID)
				}
			}
		})
	}

	// Test store failure
	t.Run("Reset usage failure", func(t *testing.T) {
		store.failOnReset = true
		defer func() { store.failOnReset = false }()

		err := limiter.ResetUsage(ctx, "tkn_resettoken123456789012")
		if err == nil {
			t.Errorf("StandardRateLimiter.ResetUsage() expected error on reset failure")
		}
	})
}

func TestStandardRateLimiter_UpdateLimit(t *testing.T) {
	ctx := context.Background()
	store := NewMockRateLimitStore()
	limiter := NewRateLimiter(store)

	// Set up test tokens
	limit100 := 100

	// Token to update
	store.AddToken("tkn_updatelimittoken12345678", TokenData{
		Token:        "tkn_updatelimittoken12345678",
		ProjectID:    "project1",
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &limit100,
	})

	tests := []struct {
		name        string
		tokenID     string
		maxRequests *int
		wantErr     bool
		wantErrIs   error
	}{
		{
			name:        "Update limit to 200",
			tokenID:     "tkn_updatelimittoken12345678",
			maxRequests: func() *int { limit := 200; return &limit }(),
			wantErr:     false,
			wantErrIs:   nil,
		},
		{
			name:        "Remove limit",
			tokenID:     "tkn_updatelimittoken12345678",
			maxRequests: nil,
			wantErr:     false,
			wantErrIs:   nil,
		},
		{
			name:        "Update non-existent token",
			tokenID:     "tkn_nonexistenttoken1234567",
			maxRequests: func() *int { limit := 50; return &limit }(),
			wantErr:     true,
			wantErrIs:   ErrTokenNotFound,
		},
		{
			name:        "Update with invalid token format",
			tokenID:     "invalid-token-format",
			maxRequests: func() *int { limit := 50; return &limit }(),
			wantErr:     true,
			wantErrIs:   ErrInvalidTokenFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := limiter.UpdateLimit(ctx, tt.tokenID, tt.maxRequests)
			if (err != nil) != tt.wantErr {
				t.Errorf("StandardRateLimiter.UpdateLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("StandardRateLimiter.UpdateLimit() error = %v, want %v", err, tt.wantErrIs)
			}

			// Verify that limit was updated
			if err == nil {
				updatedLimit := store.trackedUpdates[tt.tokenID]
				if tt.maxRequests == nil {
					if updatedLimit != nil {
						t.Errorf("Limit for token %s should be nil", tt.tokenID)
					}
				} else if updatedLimit == nil || *updatedLimit != *tt.maxRequests {
					t.Errorf("Limit for token %s should be %v, got %v", tt.tokenID, *tt.maxRequests, updatedLimit)
				}
			}
		})
	}

	// Test store failure
	t.Run("Update limit failure", func(t *testing.T) {
		store.failOnLimit = true
		defer func() { store.failOnLimit = false }()

		limit := 200
		err := limiter.UpdateLimit(ctx, "tkn_updatelimittoken12345678", &limit)
		if err == nil {
			t.Errorf("StandardRateLimiter.UpdateLimit() expected error on update failure")
		}
	})
}

func TestMemoryRateLimiter(t *testing.T) {
	// Create a memory rate limiter with 10 requests per second and max 20 requests
	limiter := NewMemoryRateLimiter(10, 20)

	// Test that a token with default settings can make requests
	tokenID := "tkn_testmemorytoken12345678"
	
	// All requests should be allowed initially (up to capacity)
	for i := 0; i < 20; i++ {
		if !limiter.Allow(tokenID) {
			t.Errorf("Request %d should be allowed", i)
		}
	}
	
	// The next request should be denied (bucket empty)
	if limiter.Allow(tokenID) {
		t.Errorf("Request should be denied after bucket is empty")
	}
	
	// After waiting, some tokens should be refilled
	time.Sleep(200 * time.Millisecond) // Should refill 2 tokens (at 10/sec)
	
	if !limiter.Allow(tokenID) {
		t.Errorf("Request should be allowed after refill")
	}
	
	if !limiter.Allow(tokenID) {
		t.Errorf("Second request should be allowed after refill")
	}
	
	// Third request should be denied (bucket empty again)
	if limiter.Allow(tokenID) {
		t.Errorf("Third request should be denied after bucket is empty again")
	}
	
	// Test setting custom limits
	customTokenID := "tkn_customlimittoken12345678"
	limiter.SetLimit(customTokenID, 5, 10) // 5 per second, max 10
	
	// All requests should be allowed initially (up to capacity)
	for i := 0; i < 10; i++ {
		if !limiter.Allow(customTokenID) {
			t.Errorf("Custom token request %d should be allowed", i)
		}
	}
	
	// The next request should be denied (bucket empty)
	if limiter.Allow(customTokenID) {
		t.Errorf("Custom token request should be denied after bucket is empty")
	}
	
	// Test resetting the limiter
	limiter.Reset(customTokenID)
	
	// Should be able to make requests again
	if !limiter.Allow(customTokenID) {
		t.Errorf("Request should be allowed after reset")
	}
	
	// Test getting limits
	rate, capacity, exists := limiter.GetLimit(customTokenID)
	if !exists {
		t.Errorf("Custom token limits should exist")
	}
	if rate != 5 {
		t.Errorf("Custom token rate should be 5, got %v", rate)
	}
	if capacity != 10 {
		t.Errorf("Custom token capacity should be 10, got %v", capacity)
	}
	
	// Test getting limits for non-existent token
	rate, capacity, exists = limiter.GetLimit("tkn_nonexistenttoken1234567")
	if exists {
		t.Errorf("Non-existent token limits should not exist")
	}
	if rate != 10 {
		t.Errorf("Default rate should be 10, got %v", rate)
	}
	if capacity != 20 {
		t.Errorf("Default capacity should be 20, got %v", capacity)
	}
	
	// Test removing a token
	limiter.Remove(customTokenID)
	
	// The token should use default limits now
	rate, capacity, exists = limiter.GetLimit(customTokenID)
	if exists {
		t.Errorf("Removed token limits should not exist")
	}
}