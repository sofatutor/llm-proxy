package token

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockValidator implements TokenValidator for testing cache
type MockValidator struct {
	tokens          map[string]TokenData
	validationCount int
	trackingCount   int
	failOnValidate  bool
	failOnTracking  bool
}

func NewMockValidator() *MockValidator {
	return &MockValidator{
		tokens:         make(map[string]TokenData),
		failOnValidate: false,
		failOnTracking: false,
	}
}

func (m *MockValidator) ValidateToken(ctx context.Context, tokenID string) (string, error) {
	m.validationCount++
	
	if m.failOnValidate {
		return "", errors.New("mock validation failure")
	}
	
	token, exists := m.tokens[tokenID]
	if !exists {
		return "", ErrTokenNotFound
	}
	
	if !token.IsActive {
		return "", ErrTokenInactive
	}
	
	if IsExpired(token.ExpiresAt) {
		return "", ErrTokenExpired
	}
	
	if token.MaxRequests != nil && token.RequestCount >= *token.MaxRequests {
		return "", ErrTokenRateLimit
	}
	
	return token.ProjectID, nil
}

func (m *MockValidator) ValidateTokenWithTracking(ctx context.Context, tokenID string) (string, error) {
	m.trackingCount++
	
	if m.failOnTracking {
		return "", errors.New("mock tracking failure")
	}
	
	projectID, err := m.ValidateToken(ctx, tokenID)
	if err != nil {
		return "", err
	}
	
	token := m.tokens[tokenID]
	token.RequestCount++
	now := time.Now()
	token.LastUsedAt = &now
	m.tokens[tokenID] = token
	
	return projectID, nil
}

func (m *MockValidator) AddToken(tokenID string, data TokenData) {
	m.tokens[tokenID] = data
}

func TestCachedValidator_ValidateToken(t *testing.T) {
	ctx := context.Background()
	mockValidator := NewMockValidator()
	
	// Create cache with small TTL for testing
	cachedValidator := NewCachedValidator(mockValidator, CacheOptions{
		TTL:             100 * time.Millisecond,
		MaxSize:         10,
		EnableCleanup:   false,
	})
	
	// Add a valid token
	now := time.Now()
	future := now.Add(1 * time.Hour)
	maxReq := 100
	
	validToken := "tkn_validtoken12345678901"
	mockValidator.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})
	
	// Test case 1: First call should miss cache
	projectID, err := cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("First ValidateToken() error = %v", err)
	}
	if projectID != "project1" {
		t.Errorf("First ValidateToken() projectID = %v, want %v", projectID, "project1")
	}
	if mockValidator.validationCount != 1 {
		t.Errorf("First ValidateToken() should call underlying validator once, got %v", mockValidator.validationCount)
	}
	
	// Test case 2: Second call should hit cache
	projectID, err = cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("Second ValidateToken() error = %v", err)
	}
	if projectID != "project1" {
		t.Errorf("Second ValidateToken() projectID = %v, want %v", projectID, "project1")
	}
	if mockValidator.validationCount != 1 {
		t.Errorf("Second ValidateToken() should use cache, validationCount = %v, want 1", mockValidator.validationCount)
	}
	
	// Check cache stats
	hits, misses, _, _ := cachedValidator.GetCacheStats()
	if hits != 1 {
		t.Errorf("Cache hits = %v, want 1", hits)
	}
	if misses != 1 {
		t.Errorf("Cache misses = %v, want 1", misses)
	}
	
	// Test case 3: Wait for cache expiration
	time.Sleep(120 * time.Millisecond)
	
	projectID, err = cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("Third ValidateToken() error = %v", err)
	}
	if mockValidator.validationCount != 2 {
		t.Errorf("After TTL, validator should be called again, validationCount = %v, want 2", mockValidator.validationCount)
	}
	
	// Test case 4: Invalid token should not be cached
	invalidToken := "tkn_invalidtoken12345678901"
	mockValidator.AddToken(invalidToken, TokenData{
		Token:        invalidToken,
		ProjectID:    "project2",
		ExpiresAt:    &future,
		IsActive:     false, // Inactive token
		CreatedAt:    now,
	})
	
	_, err = cachedValidator.ValidateToken(ctx, invalidToken)
	if err == nil {
		t.Errorf("ValidateToken() with invalid token should return error")
	}
	
	mockValidator.validationCount = 0 // Reset for clarity
	
	_, err = cachedValidator.ValidateToken(ctx, invalidToken)
	if err == nil {
		t.Errorf("Second ValidateToken() with invalid token should return error")
	}
	if mockValidator.validationCount != 1 {
		t.Errorf("Invalid token should not be cached, validationCount = %v, want 1", mockValidator.validationCount)
	}
}

func TestCachedValidator_ValidateTokenWithTracking(t *testing.T) {
	ctx := context.Background()
	mockValidator := NewMockValidator()
	
	cachedValidator := NewCachedValidator(mockValidator, CacheOptions{
		TTL:             1 * time.Minute,
		MaxSize:         10,
		EnableCleanup:   false,
	})
	
	// Add a valid token
	now := time.Now()
	future := now.Add(1 * time.Hour)
	maxReq := 100
	
	validToken := "tkn_validtoken12345678901"
	mockValidator.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})
	
	// Test case 1: Validate with tracking should bypass cache
	projectID, err := cachedValidator.ValidateTokenWithTracking(ctx, validToken)
	if err != nil {
		t.Errorf("ValidateTokenWithTracking() error = %v", err)
	}
	if projectID != "project1" {
		t.Errorf("ValidateTokenWithTracking() projectID = %v, want %v", projectID, "project1")
	}
	if mockValidator.trackingCount != 1 {
		t.Errorf("ValidateTokenWithTracking() should call underlying validator, trackingCount = %v, want 1", mockValidator.trackingCount)
	}
	
	// Test case 2: Cache should be invalidated
	mockValidator.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1-updated", // Changed project ID
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 51, // Incremented
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})
	
	// Regular validate should bypass cache after tracking
	mockValidator.validationCount = 0 // Reset for clarity
	projectID, err = cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("ValidateToken() after tracking error = %v", err)
	}
	if projectID != "project1-updated" {
		t.Errorf("ValidateToken() after tracking projectID = %v, want %v", projectID, "project1-updated")
	}
	if mockValidator.validationCount != 1 {
		t.Errorf("ValidateToken() after tracking should not use cache, validationCount = %v, want 1", mockValidator.validationCount)
	}
}

func TestCachedValidator_CacheEviction(t *testing.T) {
	ctx := context.Background()
	mockValidator := NewMockValidator()
	
	// Create cache with small size for testing eviction
	cachedValidator := NewCachedValidator(mockValidator, CacheOptions{
		TTL:             1 * time.Minute,
		MaxSize:         2, // Only 2 entries
		EnableCleanup:   false,
	})
	
	// Add tokens
	now := time.Now()
	future := now.Add(1 * time.Hour)
	
	// Create and validate 3 tokens
	for i := 1; i <= 3; i++ {
		tokenID := "tkn_token" + string(rune('0'+i)) + "1234567890123456"
		projectID := "project" + string(rune('0'+i))
		
		mockValidator.AddToken(tokenID, TokenData{
			Token:        tokenID,
			ProjectID:    projectID,
			ExpiresAt:    &future,
			IsActive:     true,
			CreatedAt:    now,
		})
		
		_, err := cachedValidator.ValidateToken(ctx, tokenID)
		if err != nil {
			t.Errorf("ValidateToken() for token %d error = %v", i, err)
		}
	}
	
	// Check cache size
	_, _, _, size := cachedValidator.GetCacheStats()
	if size > 2 {
		t.Errorf("Cache size = %v, should not exceed max size 2", size)
	}
	
	// Clear cache
	cachedValidator.ClearCache()
	_, _, _, size = cachedValidator.GetCacheStats()
	if size != 0 {
		t.Errorf("After ClearCache(), cache size = %v, want 0", size)
	}
}

func TestCachedValidator_Cleanup(t *testing.T) {
	mockValidator := NewMockValidator()
	
	// Create cache with short TTL and cleanup
	cachedValidator := NewCachedValidator(mockValidator, CacheOptions{
		TTL:             50 * time.Millisecond,
		MaxSize:         10,
		EnableCleanup:   true,
		CleanupInterval: 100 * time.Millisecond,
	})
	
	// Add a valid token
	now := time.Now()
	future := now.Add(1 * time.Hour)
	
	validToken := "tkn_validtoken12345678901"
	mockValidator.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		ExpiresAt:    &future,
		IsActive:     true,
		CreatedAt:    now,
	})
	
	// Validate to put in cache
	ctx := context.Background()
	_, err := cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("ValidateToken() error = %v", err)
	}
	
	// Check initial cache size
	_, _, _, size := cachedValidator.GetCacheStats()
	if size != 1 {
		t.Errorf("Initial cache size = %v, want 1", size)
	}
	
	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)
	
	// Check cache size after cleanup
	_, _, _, size = cachedValidator.GetCacheStats()
	if size != 0 {
		t.Errorf("After cleanup, cache size = %v, want 0", size)
	}
}