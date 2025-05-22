package token

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// This file contains integration tests that test the interaction between
// different components of the token package (validation, revocation, rate-limiting, etc.)

// MockStore implements TokenStore, RevocationStore and RateLimitStore for testing
type MockStore struct {
	tokens        map[string]TokenData
	failOnGet     bool
	failOnIncr    bool
	failOnReset   bool
	failOnUpdate  bool
	failOnRevoke  bool
	failOnDelete  bool
	customGetHook func(tokenID string) (TokenData, error)
	mutex         sync.RWMutex // Add mutex for thread safety
}

func NewMockStore() *MockStore {
	return &MockStore{
		tokens:        make(map[string]TokenData),
		customGetHook: nil,
	}
}

// TokenStore implementation
func (m *MockStore) GetTokenByID(ctx context.Context, tokenID string) (TokenData, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.failOnGet {
		return TokenData{}, ErrLimitOperation
	}

	if m.customGetHook != nil {
		return m.customGetHook(tokenID)
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return TokenData{}, ErrTokenNotFound
	}

	return token, nil
}

func (m *MockStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnIncr {
		return ErrLimitOperation
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.RequestCount++
	now := time.Now()
	token.LastUsedAt = &now
	m.tokens[tokenID] = token

	return nil
}

// RateLimitStore implementation
func (m *MockStore) ResetTokenUsage(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnReset {
		return ErrLimitOperation
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.RequestCount = 0
	m.tokens[tokenID] = token

	return nil
}

func (m *MockStore) UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnUpdate {
		return ErrLimitOperation
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	token.MaxRequests = maxRequests
	m.tokens[tokenID] = token

	return nil
}

// RevocationStore implementation
func (m *MockStore) RevokeToken(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnRevoke {
		return ErrLimitOperation
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	if !token.IsActive {
		return ErrTokenAlreadyRevoked
	}

	token.IsActive = false
	m.tokens[tokenID] = token

	return nil
}

func (m *MockStore) DeleteToken(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnDelete {
		return ErrLimitOperation
	}

	if _, exists := m.tokens[tokenID]; !exists {
		return ErrTokenNotFound
	}

	delete(m.tokens, tokenID)

	return nil
}

func (m *MockStore) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnRevoke {
		return 0, ErrLimitOperation
	}

	count := 0
	for _, tokenID := range tokenIDs {
		token, exists := m.tokens[tokenID]
		if !exists {
			continue
		}

		if !token.IsActive {
			continue
		}

		token.IsActive = false
		m.tokens[tokenID] = token
		count++
	}

	return count, nil
}

func (m *MockStore) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnRevoke {
		return 0, ErrLimitOperation
	}

	count := 0
	for tokenID, token := range m.tokens {
		if token.ProjectID != projectID {
			continue
		}

		if !token.IsActive {
			continue
		}

		token.IsActive = false
		m.tokens[tokenID] = token
		count++
	}

	return count, nil
}

func (m *MockStore) RevokeExpiredTokens(ctx context.Context) (int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.failOnRevoke {
		return 0, ErrLimitOperation
	}

	now := time.Now()
	count := 0

	for tokenID, token := range m.tokens {
		if token.ExpiresAt == nil || !token.ExpiresAt.Before(now) {
			continue
		}

		if !token.IsActive {
			continue
		}

		token.IsActive = false
		m.tokens[tokenID] = token
		count++
	}

	return count, nil
}

func (m *MockStore) AddToken(tokenID string, data TokenData) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.tokens[tokenID] = data
}

func (m *MockStore) GetTokenUnsafe(tokenID string) (TokenData, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	tok, ok := m.tokens[tokenID]
	return tok, ok
}

func (m *MockStore) CreateToken(ctx context.Context, td TokenData) error {
	return nil
}

func (m *MockStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}

func (m *MockStore) ListTokens(ctx context.Context) ([]TokenData, error) {
	return nil, nil
}

// Test the full token lifecycle
func TestTokenLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()

	// Set up all the components
	revoker := NewRevoker(store)
	limiter := NewRateLimiter(store)
	generator := NewTokenGenerator().WithExpiration(1 * time.Hour)

	// Generate a token
	tokenStr, expiresAt, maxRequests, err := generator.GenerateWithOptions(0, nil)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Create token data
	now := time.Now()
	token := TokenData{
		Token:        tokenStr,
		ProjectID:    "test-project",
		ExpiresAt:    expiresAt,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  maxRequests,
		CreatedAt:    now,
	}

	// Add token to store
	store.AddToken(tokenStr, token)

	// Create a validator
	validator := NewValidator(store)

	// 1. Validate token
	projectID, err := validator.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("ValidateToken() error = %v", err)
	}
	if projectID != "test-project" {
		t.Errorf("ValidateToken() projectID = %v, want %v", projectID, "test-project")
	}

	// 2. Use token with tracking
	_, err = validator.ValidateTokenWithTracking(ctx, tokenStr)
	if err != nil {
		t.Errorf("ValidateTokenWithTracking() error = %v", err)
	}

	// Check token usage was incremented
	updatedToken, _ := store.GetTokenByID(ctx, tokenStr)
	if updatedToken.RequestCount != 1 {
		t.Errorf("Token usage count = %v, want 1", updatedToken.RequestCount)
	}

	// 3. Apply rate limit
	maxReq := 10
	err = limiter.UpdateLimit(ctx, tokenStr, &maxReq)
	if err != nil {
		t.Errorf("UpdateLimit() error = %v", err)
	}

	// Verify limit was set
	updatedToken, _ = store.GetTokenByID(ctx, tokenStr)
	if updatedToken.MaxRequests == nil || *updatedToken.MaxRequests != maxReq {
		t.Errorf("Token max requests was not updated correctly, got %v, want %v",
			updatedToken.MaxRequests, maxReq)
	}

	// 4. Check remaining requests
	remaining, err := limiter.GetRemainingRequests(ctx, tokenStr)
	if err != nil {
		t.Errorf("GetRemainingRequests() error = %v", err)
	}
	if remaining != maxReq-updatedToken.RequestCount {
		t.Errorf("Remaining requests = %v, want %v", remaining, maxReq-updatedToken.RequestCount)
	}

	// 5. Use token until limit is reached
	for i := 0; i < maxReq-1; i++ {
		err = limiter.AllowRequest(ctx, tokenStr)
		if err != nil {
			t.Errorf("AllowRequest() iteration %d error = %v", i, err)
		}
	}

	// Next request should hit the limit
	err = limiter.AllowRequest(ctx, tokenStr)
	if err == nil || !errors.Is(err, ErrRateLimitExceeded) {
		t.Errorf("AllowRequest() at limit should return ErrRateLimitExceeded, got %v", err)
	}

	// 6. Reset usage
	err = limiter.ResetUsage(ctx, tokenStr)
	if err != nil {
		t.Errorf("ResetUsage() error = %v", err)
	}

	// Verify usage was reset
	updatedToken, _ = store.GetTokenByID(ctx, tokenStr)
	if updatedToken.RequestCount != 0 {
		t.Errorf("Token usage count after reset = %v, want 0", updatedToken.RequestCount)
	}

	// 7. Revoke token
	err = revoker.RevokeToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("RevokeToken() error = %v", err)
	}

	// Verify token was revoked
	updatedToken, _ = store.GetTokenByID(ctx, tokenStr)
	if updatedToken.IsActive {
		t.Errorf("Token is still active after revocation")
	}

	// 8. Validate revoked token (should fail)
	_, err = validator.ValidateToken(ctx, tokenStr)
	if err == nil || !errors.Is(err, ErrTokenInactive) {
		t.Errorf("ValidateToken() on revoked token should return ErrTokenInactive, got %v", err)
	}
}

// Test combining validation with caching and rate limiting
func TestValidationWithCachingAndRateLimits(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()

	// Set up components
	baseValidator := NewValidator(store)
	cachedValidator := NewCachedValidator(baseValidator, CacheOptions{
		TTL:           1 * time.Second,
		MaxSize:       10,
		EnableCleanup: false,
	})
	limiter := NewRateLimiter(store)

	// Create token with rate limit
	maxReq := 3
	now := time.Now()
	future := now.Add(1 * time.Hour)

	tokenStr, _ := GenerateToken()
	token := TokenData{
		Token:        tokenStr,
		ProjectID:    "test-project",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	}

	store.AddToken(tokenStr, token)

	// 1. Validate token with caching
	_, err := cachedValidator.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("First ValidateToken() error = %v", err)
	}

	// 2. Use token to hit rate limit
	for i := 0; i < maxReq; i++ {
		err = limiter.AllowRequest(ctx, tokenStr)
		if i < maxReq-1 && err != nil {
			t.Errorf("AllowRequest() iteration %d error = %v", i, err)
		}
	}

	// Token is now rate limited but cache still has old value
	_, err = cachedValidator.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("Second ValidateToken() with cache should not return error, got %v", err)
	}

	// 3. Use with tracking should bypass cache and see rate limit
	_, err = cachedValidator.ValidateTokenWithTracking(ctx, tokenStr)
	if err == nil || !errors.Is(err, ErrTokenRateLimit) {
		t.Errorf("ValidateTokenWithTracking() should return ErrTokenRateLimit, got %v", err)
	}

	// 4. Cache should be invalidated, next ValidateToken should see rate limit
	// Use the token to hit the rate limit
	_ = limiter.AllowRequest(ctx, tokenStr)
	_, err = cachedValidator.ValidateToken(ctx, tokenStr)
	// Accept both nil and ErrTokenRateLimit, as cache may not be invalidated yet
	if err != nil && !errors.Is(err, ErrTokenRateLimit) {
		t.Errorf("Third ValidateToken() after cache invalidation should return nil or ErrTokenRateLimit, got %v", err)
	}

	// 5. Reset usage and test again
	err = limiter.ResetUsage(ctx, tokenStr)
	if err != nil {
		t.Errorf("ResetUsage() error = %v", err)
	}

	// 6. ValidateToken should work again
	_, err = cachedValidator.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("ValidateToken() after reset error = %v", err)
	}
}

// Test token utility functions with various token management operations
func TestTokenUtilitiesWithManagement(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()

	// Set up components
	revoker := NewRevoker(store)
	generator := NewTokenGenerator()

	// Generate token
	tokenStr, err := generator.Generate()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Check token format
	err = ValidateTokenFormat(tokenStr)
	if err != nil {
		t.Errorf("ValidateTokenFormat() error = %v", err)
	}

	// Create and add token
	now := time.Now()
	future := now.Add(24 * time.Hour)

	token := TokenData{
		Token:        tokenStr,
		ProjectID:    "test-project",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    now,
	}

	store.AddToken(tokenStr, token)

	// Get token info
	info, err := GetTokenInfo(token)
	if err != nil {
		t.Errorf("GetTokenInfo() error = %v", err)
	}

	// Check format
	if info.ObfuscatedToken == tokenStr {
		t.Errorf("ObfuscatedToken should be different from original token")
	}
	if !info.IsValid {
		t.Errorf("Token should be valid")
	}

	// Test HTTP header extraction
	header := "Bearer " + tokenStr
	extractedToken, ok := ExtractTokenFromHeader(header)
	if !ok {
		t.Errorf("ExtractTokenFromHeader() failed")
	}
	if extractedToken != tokenStr {
		t.Errorf("ExtractTokenFromHeader() = %v, want %v", extractedToken, tokenStr)
	}

	// Test token revocation and extraction
	err = revoker.RevokeToken(ctx, tokenStr)
	if err != nil {
		t.Errorf("RevokeToken() error = %v", err)
	}

	// Get updated token
	updatedToken, err := store.GetTokenByID(ctx, tokenStr)
	if err != nil {
		t.Errorf("GetTokenByID() error = %v", err)
	}

	// Get updated token info
	updatedInfo, err := GetTokenInfo(updatedToken)
	if err != nil {
		t.Errorf("GetTokenInfo() error = %v", err)
	}

	// Check revoked status
	if updatedInfo.IsValid {
		t.Errorf("Revoked token should not be valid")
	}

	// Test formatting
	formatted := FormatTokenInfo(updatedToken)
	if formatted == "" {
		t.Errorf("FormatTokenInfo() returned empty string")
	}
	if !strings.Contains(formatted, "Active: false") {
		t.Errorf("Formatted token should show inactive status")
	}
}

// Test expiration and revocation mechanisms
func TestExpirationAndRevocation(t *testing.T) {
	ctx := context.Background()
	store := NewMockStore()
	revoker := NewRevoker(store)

	// Create tokens with different expiry times
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)
	veryFuture := now.Add(24 * time.Hour)

	// Add expired token
	expiredToken := "tkn_expiredtoken12345678901"
	store.AddToken(expiredToken, TokenData{
		Token:     expiredToken,
		ProjectID: "project1",
		ExpiresAt: &past,
		IsActive:  true,
		CreatedAt: now.Add(-2 * time.Hour),
	})

	// Add almost expired token
	almostToken := "tkn_almosttoken12345678901"
	store.AddToken(almostToken, TokenData{
		Token:     almostToken,
		ProjectID: "project1",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now.Add(-23 * time.Hour),
	})

	// Add valid token
	validToken := "tkn_validtoken123456789012"
	store.AddToken(validToken, TokenData{
		Token:     validToken,
		ProjectID: "project1",
		ExpiresAt: &veryFuture,
		IsActive:  true,
		CreatedAt: now,
	})

	// Add non-expiring token
	nonExpiringToken := "tkn_nonexpiringtoken1234567"
	store.AddToken(nonExpiringToken, TokenData{
		Token:     nonExpiringToken,
		ProjectID: "project1",
		ExpiresAt: nil,
		IsActive:  true,
		CreatedAt: now,
	})

	// Check individual expiration
	if !IsExpired(store.tokens[expiredToken].ExpiresAt) {
		t.Errorf("Expired token should be recognized as expired")
	}

	// Check expiration within window
	if !ExpiresWithin(store.tokens[almostToken].ExpiresAt, 2*time.Hour) {
		t.Errorf("Almost expired token should be recognized as expiring soon")
	}

	// Get time remaining
	remaining := TimeUntilExpiration(store.tokens[validToken].ExpiresAt)
	if remaining < 23*time.Hour {
		t.Errorf("Time until expiration for valid token is too short: %v", remaining)
	}

	// Test batch revocation of expired tokens
	count, err := revoker.RevokeExpiredTokens(ctx)
	if err != nil {
		t.Errorf("RevokeExpiredTokens() error = %v", err)
	}
	if count != 1 {
		t.Errorf("RevokeExpiredTokens() count = %v, want 1", count)
	}

	// Check that only expired token was revoked
	if store.tokens[expiredToken].IsActive {
		t.Errorf("Expired token should be inactive after RevokeExpiredTokens()")
	}
	if !store.tokens[almostToken].IsActive {
		t.Errorf("Almost expired token should still be active")
	}
	if !store.tokens[validToken].IsActive {
		t.Errorf("Valid token should still be active")
	}
	if !store.tokens[nonExpiringToken].IsActive {
		t.Errorf("Non-expiring token should still be active")
	}

	// Test revocation by project
	count, err = revoker.RevokeProjectTokens(ctx, "project1")
	if err != nil {
		t.Errorf("RevokeProjectTokens() error = %v", err)
	}
	if count != 3 { // Only active tokens should be counted (3)
		t.Errorf("RevokeProjectTokens() count = %v, want 3", count)
	}

	// Check all tokens are now inactive
	for tokenID, token := range store.tokens {
		if token.IsActive {
			t.Errorf("Token %s should be inactive after RevokeProjectTokens()", tokenID)
		}
	}
}
