package database

import (
	"context"
	"testing"
	"time"
)

// TestTokenCRUD tests token CRUD operations.
func TestTokenCRUD(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project first
	project := Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		OpenAIAPIKey: "test-api-key",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create two test tokens with different configurations
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)
	maxRequests := 100

	token1 := Token{
		Token:        "test-token-1",
		ProjectID:    project.ID,
		ExpiresAt:    &expiresAt,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  &maxRequests,
		CreatedAt:    now.Truncate(time.Second),
	}

	token2 := Token{
		Token:        "test-token-2",
		ProjectID:    project.ID,
		ExpiresAt:    nil, // no expiration
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  nil, // no request limit
		CreatedAt:    now.Truncate(time.Second),
	}

	// Test CreateToken for both tokens
	err = db.CreateToken(ctx, token1)
	if err != nil {
		t.Fatalf("Failed to create token1: %v", err)
	}

	err = db.CreateToken(ctx, token2)
	if err != nil {
		t.Fatalf("Failed to create token2: %v", err)
	}

	// Test GetTokenByID
	retrievedToken1, err := db.GetTokenByID(ctx, token1.Token)
	if err != nil {
		t.Fatalf("Failed to get token1: %v", err)
	}
	if retrievedToken1.Token != token1.Token {
		t.Fatalf("Expected token ID %s, got %s", token1.Token, retrievedToken1.Token)
	}
	if retrievedToken1.ProjectID != token1.ProjectID {
		t.Fatalf("Expected project ID %s, got %s", token1.ProjectID, retrievedToken1.ProjectID)
	}
	if retrievedToken1.ExpiresAt == nil {
		t.Fatalf("Expected expiration time, got nil")
	}
	if retrievedToken1.MaxRequests == nil {
		t.Fatalf("Expected max requests, got nil")
	}

	retrievedToken2, err := db.GetTokenByID(ctx, token2.Token)
	if err != nil {
		t.Fatalf("Failed to get token2: %v", err)
	}
	if retrievedToken2.ExpiresAt != nil {
		t.Fatalf("Expected nil expiration time, got %v", *retrievedToken2.ExpiresAt)
	}
	if retrievedToken2.MaxRequests != nil {
		t.Fatalf("Expected nil max requests, got %d", *retrievedToken2.MaxRequests)
	}

	// Test GetTokenByID with non-existent ID
	_, err = db.GetTokenByID(ctx, "non-existent")
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound, got %v", err)
	}

	// Test IncrementTokenUsage
	err = db.IncrementTokenUsage(ctx, token1.Token)
	if err != nil {
		t.Fatalf("Failed to increment token usage: %v", err)
	}

	updatedToken1, err := db.GetTokenByID(ctx, token1.Token)
	if err != nil {
		t.Fatalf("Failed to get updated token1: %v", err)
	}
	if updatedToken1.RequestCount != 1 {
		t.Fatalf("Expected request count 1, got %d", updatedToken1.RequestCount)
	}
	if updatedToken1.LastUsedAt == nil {
		t.Fatalf("Expected last_used_at to be set, got nil")
	}

	// Test IncrementTokenUsage with non-existent ID
	err = db.IncrementTokenUsage(ctx, "non-existent")
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound, got %v", err)
	}

	// Test UpdateToken
	updatedToken1.IsActive = false
	updatedToken1.RequestCount = 10
	newExpiry := expiresAt.Add(24 * time.Hour)
	updatedToken1.ExpiresAt = &newExpiry

	err = db.UpdateToken(ctx, updatedToken1)
	if err != nil {
		t.Fatalf("Failed to update token: %v", err)
	}

	retrievedAfterUpdate, err := db.GetTokenByID(ctx, token1.Token)
	if err != nil {
		t.Fatalf("Failed to get token after update: %v", err)
	}
	if retrievedAfterUpdate.IsActive != updatedToken1.IsActive {
		t.Fatalf("Expected IsActive %v, got %v", updatedToken1.IsActive, retrievedAfterUpdate.IsActive)
	}
	if retrievedAfterUpdate.RequestCount != updatedToken1.RequestCount {
		t.Fatalf("Expected RequestCount %d, got %d", updatedToken1.RequestCount, retrievedAfterUpdate.RequestCount)
	}
	if retrievedAfterUpdate.ExpiresAt.Equal(*updatedToken1.ExpiresAt) == false {
		t.Fatalf("Expected ExpiresAt %v, got %v", *updatedToken1.ExpiresAt, *retrievedAfterUpdate.ExpiresAt)
	}

	// Test UpdateToken with non-existent ID
	nonExistentToken := Token{
		Token:        "non-existent",
		ProjectID:    project.ID,
		IsActive:     true,
		RequestCount: 0,
	}
	err = db.UpdateToken(ctx, nonExistentToken)
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound, got %v", err)
	}

	// Test ListTokens
	tokens, err := db.ListTokens(ctx)
	if err != nil {
		t.Fatalf("Failed to list tokens: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("Expected 2 tokens, got %d", len(tokens))
	}

	// Test GetTokensByProjectID
	projectTokens, err := db.GetTokensByProjectID(ctx, project.ID)
	if err != nil {
		t.Fatalf("Failed to get tokens by project ID: %v", err)
	}
	if len(projectTokens) != 2 {
		t.Fatalf("Expected 2 tokens for project, got %d", len(projectTokens))
	}

	// Test GetTokensByProjectID with non-existent project ID
	nonExistentProjectTokens, err := db.GetTokensByProjectID(ctx, "non-existent")
	if err != nil {
		t.Fatalf("Failed to get tokens by non-existent project ID: %v", err)
	}
	if len(nonExistentProjectTokens) != 0 {
		t.Fatalf("Expected 0 tokens for non-existent project, got %d", len(nonExistentProjectTokens))
	}

	// Test DeleteToken
	err = db.DeleteToken(ctx, token1.Token)
	if err != nil {
		t.Fatalf("Failed to delete token: %v", err)
	}

	// Verify token was deleted
	_, err = db.GetTokenByID(ctx, token1.Token)
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound after deletion, got %v", err)
	}

	// Test DeleteToken with non-existent ID
	err = db.DeleteToken(ctx, "non-existent")
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound, got %v", err)
	}
}

// TestTokenExpirationAndRateLimiting tests token expiration and rate limiting.
func TestTokenExpirationAndRateLimiting(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project
	project := Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		OpenAIAPIKey: "test-api-key",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create tokens with different configurations for testing
	now := time.Now().UTC()

	// Token with past expiration
	pastExpiry := now.Add(-1 * time.Hour)
	maxRequests := 10
	expiredToken := Token{
		Token:        "expired-token",
		ProjectID:    project.ID,
		ExpiresAt:    &pastExpiry,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  &maxRequests,
		CreatedAt:    now.Truncate(time.Second),
	}

	// Token with rate limit reached
	rateLimitedToken := Token{
		Token:        "rate-limited-token",
		ProjectID:    project.ID,
		IsActive:     true,
		RequestCount: 10, // Equal to max requests
		MaxRequests:  &maxRequests,
		CreatedAt:    now.Truncate(time.Second),
	}

	// Inactive token
	inactiveToken := Token{
		Token:        "inactive-token",
		ProjectID:    project.ID,
		IsActive:     false,
		RequestCount: 0,
		CreatedAt:    now.Truncate(time.Second),
	}

	// Valid token
	futureExpiry := now.Add(24 * time.Hour)
	validToken := Token{
		Token:        "valid-token",
		ProjectID:    project.ID,
		ExpiresAt:    &futureExpiry,
		IsActive:     true,
		RequestCount: 5, // Less than max requests
		MaxRequests:  &maxRequests,
		CreatedAt:    now.Truncate(time.Second),
	}

	// Create all the tokens
	for _, token := range []Token{expiredToken, rateLimitedToken, inactiveToken, validToken} {
		err = db.CreateToken(ctx, token)
		if err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}
	}

	// Test expiration
	retrievedExpiredToken, err := db.GetTokenByID(ctx, expiredToken.Token)
	if err != nil {
		t.Fatalf("Failed to get expired token: %v", err)
	}
	if !retrievedExpiredToken.IsExpired() {
		t.Fatalf("Expected expired token to be expired")
	}

	// Test rate limiting
	retrievedRateLimitedToken, err := db.GetTokenByID(ctx, rateLimitedToken.Token)
	if err != nil {
		t.Fatalf("Failed to get rate-limited token: %v", err)
	}
	if !retrievedRateLimitedToken.IsRateLimited() {
		t.Fatalf("Expected rate-limited token to be rate limited")
	}

	// Test inactive token
	retrievedInactiveToken, err := db.GetTokenByID(ctx, inactiveToken.Token)
	if err != nil {
		t.Fatalf("Failed to get inactive token: %v", err)
	}
	if retrievedInactiveToken.IsValid() {
		t.Fatalf("Expected inactive token to be invalid")
	}

	// Test valid token
	retrievedValidToken, err := db.GetTokenByID(ctx, validToken.Token)
	if err != nil {
		t.Fatalf("Failed to get valid token: %v", err)
	}
	if !retrievedValidToken.IsValid() {
		t.Fatalf("Expected valid token to be valid")
	}

	// Test CleanExpiredTokens
	cleaned, err := db.CleanExpiredTokens(ctx)
	if err != nil {
		t.Fatalf("Failed to clean expired tokens: %v", err)
	}
	if cleaned != 1 {
		t.Fatalf("Expected 1 expired token to be cleaned, got %d", cleaned)
	}

	// Verify expired token was cleaned
	_, err = db.GetTokenByID(ctx, expiredToken.Token)
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound for cleaned token, got %v", err)
	}
}
