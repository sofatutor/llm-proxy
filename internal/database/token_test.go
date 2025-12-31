package database

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/obfuscate"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/require"
)

// TestTokenCRUD tests token CRUD operations.
func TestTokenCRUD(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project first
	project := proxy.Project{
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

	// Test GetTokenByToken (token string lookup)
	retrievedToken1, err := db.GetTokenByToken(ctx, token1.Token)
	if err != nil {
		t.Fatalf("Failed to get token1: %v", err)
	}
	if retrievedToken1.Token != token1.Token {
		t.Fatalf("Expected token ID %s, got %s", token1.Token, retrievedToken1.Token)
	}
	if retrievedToken1.ID == "" {
		t.Fatalf("Expected token to have ID set, got empty")
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

	retrievedToken2, err := db.GetTokenByToken(ctx, token2.Token)
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

	updatedToken1, err := db.GetTokenByToken(ctx, token1.Token)
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

	retrievedAfterUpdate, err := db.GetTokenByID(ctx, updatedToken1.ID)
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
	_, err = db.GetTokenByToken(ctx, token1.Token)
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound after deletion, got %v", err)
	}

	// Test DeleteToken with non-existent ID
	err = db.DeleteToken(ctx, "non-existent")
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound, got %v", err)
	}
}

func TestIncrementTokenUsageBatch(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	project := proxy.Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		OpenAIAPIKey: "test-api-key",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, db.CreateProject(ctx, project))

	now := time.Now().UTC().Truncate(time.Second)
	lastUsedAt := now.Add(10 * time.Second).UTC()

	token1 := Token{Token: "batch-token-1", ProjectID: project.ID, IsActive: true, CreatedAt: now}
	token2 := Token{Token: "batch-token-2", ProjectID: project.ID, IsActive: true, CreatedAt: now}
	token3 := Token{Token: "batch-token-3", ProjectID: project.ID, IsActive: true, CreatedAt: now}
	require.NoError(t, db.CreateToken(ctx, token1))
	require.NoError(t, db.CreateToken(ctx, token2))
	require.NoError(t, db.CreateToken(ctx, token3))

	// Multiple token updates.
	require.NoError(t, db.IncrementTokenUsageBatch(ctx, map[string]int{token1.Token: 2, token2.Token: 3}, lastUsedAt))

	updated1, err := db.GetTokenByToken(ctx, token1.Token)
	require.NoError(t, err)
	require.Equal(t, 2, updated1.RequestCount)
	require.NotNil(t, updated1.LastUsedAt)
	require.WithinDuration(t, lastUsedAt, *updated1.LastUsedAt, time.Second)

	updated2, err := db.GetTokenByToken(ctx, token2.Token)
	require.NoError(t, err)
	require.Equal(t, 3, updated2.RequestCount)
	require.NotNil(t, updated2.LastUsedAt)
	require.WithinDuration(t, lastUsedAt, *updated2.LastUsedAt, time.Second)

	// Zero/negative deltas are ignored.
	require.NoError(t, db.IncrementTokenUsageBatch(ctx, map[string]int{token3.Token: 0, token1.Token: -5, token2.Token: 1}, lastUsedAt))

	updated3, err := db.GetTokenByToken(ctx, token3.Token)
	require.NoError(t, err)
	require.Equal(t, 0, updated3.RequestCount)
	require.Nil(t, updated3.LastUsedAt)

	updated2, err = db.GetTokenByToken(ctx, token2.Token)
	require.NoError(t, err)
	require.Equal(t, 4, updated2.RequestCount)

	// Missing tokens are skipped (tokens can be deleted while events are buffered).
	require.NoError(t, db.IncrementTokenUsageBatch(ctx, map[string]int{token1.Token: 1, "missing-token": 1}, lastUsedAt))

	updated1, err = db.GetTokenByToken(ctx, token1.Token)
	require.NoError(t, err)
	require.Equal(t, 3, updated1.RequestCount)

	// But if all requested updates target missing tokens, surface ErrTokenNotFound.
	err = db.IncrementTokenUsageBatch(ctx, map[string]int{"missing-token": 1}, lastUsedAt)
	require.ErrorIs(t, err, ErrTokenNotFound)
}

// TestTokenExpirationAndRateLimiting tests token expiration and rate limiting.
func TestTokenExpirationAndRateLimiting(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project
	project := proxy.Project{
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
	retrievedExpiredToken, err := db.GetTokenByToken(ctx, expiredToken.Token)
	if err != nil {
		t.Fatalf("Failed to get expired token: %v", err)
	}
	if !retrievedExpiredToken.IsExpired() {
		t.Fatalf("Expected expired token to be expired")
	}

	// Test rate limiting
	retrievedRateLimitedToken, err := db.GetTokenByToken(ctx, rateLimitedToken.Token)
	if err != nil {
		t.Fatalf("Failed to get rate-limited token: %v", err)
	}
	if !retrievedRateLimitedToken.IsRateLimited() {
		t.Fatalf("Expected rate-limited token to be rate limited")
	}

	// Test inactive token
	retrievedInactiveToken, err := db.GetTokenByToken(ctx, inactiveToken.Token)
	if err != nil {
		t.Fatalf("Failed to get inactive token: %v", err)
	}
	if retrievedInactiveToken.IsValid() {
		t.Fatalf("Expected inactive token to be invalid")
	}

	// Test valid token
	retrievedValidToken, err := db.GetTokenByToken(ctx, validToken.Token)
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
	_, err = db.GetTokenByToken(ctx, expiredToken.Token)
	if err != ErrTokenNotFound {
		t.Fatalf("Expected ErrTokenNotFound for cleaned token, got %v", err)
	}
}

func TestIncrementTokenUsage_EnforcesMaxRequests(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	project := proxy.Project{
		ID:           "proj-quota-1",
		Name:         "Quota Test",
		OpenAIAPIKey: "test-api-key",
		IsActive:     true,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, db.CreateProject(ctx, project))

	maxRequests := 3
	tk := Token{
		Token:        "token-quota-1",
		ProjectID:    project.ID,
		IsActive:     true,
		RequestCount: 2,
		MaxRequests:  &maxRequests,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, db.CreateToken(ctx, tk))

	// One increment should succeed and reach the quota.
	require.NoError(t, db.IncrementTokenUsage(ctx, tk.Token))
	got, err := db.GetTokenByToken(ctx, tk.Token)
	require.NoError(t, err)
	require.Equal(t, 3, got.RequestCount)

	// Next increment should fail and must not overshoot.
	require.ErrorIs(t, db.IncrementTokenUsage(ctx, tk.Token), token.ErrTokenRateLimit)
	got, err = db.GetTokenByToken(ctx, tk.Token)
	require.NoError(t, err)
	require.Equal(t, 3, got.RequestCount)
}

func TestGetTokenByID_NotFound(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	_, err := db.GetTokenByID(ctx, "does-not-exist")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestListTokens_Multiple(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	project := proxy.Project{ID: "p", Name: "P", OpenAIAPIKey: "k", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = db.CreateProject(ctx, project)
	for i := 0; i < 5; i++ {
		tk := Token{
			Token:     "tk-" + strconv.Itoa(i),
			ProjectID: project.ID,
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		if err := db.CreateToken(ctx, tk); err != nil {
			t.Fatalf("Failed to create token: %v", err)
		}
	}
	tokens, err := db.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(tokens) != 5 {
		t.Errorf("expected 5 tokens, got %d", len(tokens))
	}
}

func TestUpdateToken_InvalidInput(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	tk := Token{Token: "", ProjectID: "", IsActive: true, CreatedAt: time.Now()}
	if err := db.UpdateToken(ctx, tk); err == nil {
		t.Error("expected error for empty token in UpdateToken")
	}
}

func TestDeleteToken_InvalidInput(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	if err := db.DeleteToken(ctx, ""); err == nil {
		t.Error("expected error for empty token in DeleteToken")
	}
}

func TestListTokens_Empty(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	tokens, err := db.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestTokenCRUD_ClosedDB(t *testing.T) {
	db, cleanup := testDB(t)
	cleanup()
	ctx := context.Background()
	tk := Token{Token: "x", ProjectID: "x", IsActive: true, CreatedAt: time.Now()}
	if err := db.CreateToken(ctx, tk); err == nil {
		t.Error("expected error for CreateToken on closed DB")
	}
	_, err := db.GetTokenByID(ctx, "x")
	if err == nil {
		t.Error("expected error for GetTokenByID on closed DB")
	}
	if err := db.UpdateToken(ctx, tk); err == nil {
		t.Error("expected error for UpdateToken on closed DB")
	}
	if err := db.DeleteToken(ctx, "x"); err == nil {
		t.Error("expected error for DeleteToken on closed DB")
	}
	_, err = db.ListTokens(ctx)
	if err == nil {
		t.Error("expected error for ListTokens on closed DB")
	}
	_, err = db.GetTokensByProjectID(ctx, "x")
	if err == nil {
		t.Error("expected error for GetTokensByProjectID on closed DB")
	}
	if err := db.IncrementTokenUsage(ctx, "x"); err == nil {
		t.Error("expected error for IncrementTokenUsage on closed DB")
	}
	_, err = db.CleanExpiredTokens(ctx)
	if err == nil {
		t.Error("expected error for CleanExpiredTokens on closed DB")
	}
}

func TestUpdateToken_EmptyToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	tk := Token{Token: "", ProjectID: "p", IsActive: true, CreatedAt: time.Now()}
	if err := db.UpdateToken(ctx, tk); err == nil {
		t.Error("expected error for empty token in UpdateToken")
	}
}

func TestDeleteToken_EmptyToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	if err := db.DeleteToken(ctx, ""); err == nil {
		t.Error("expected error for empty token in DeleteToken")
	}
}

func TestQueryTokens_LongToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	longToken := make([]byte, 300)
	for i := range longToken {
		longToken[i] = 'x'
	}
	tk := Token{Token: string(longToken), ProjectID: "p", IsActive: true, CreatedAt: time.Now()}
	_ = db.CreateProject(ctx, proxy.Project{ID: "p", Name: "P", OpenAIAPIKey: "k", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = db.CreateToken(ctx, tk)
	tokens, err := db.ListTokens(ctx)
	if err != nil {
		t.Fatalf("ListTokens failed: %v", err)
	}
	found := false
	for _, t := range tokens {
		if t.Token == string(longToken) {
			found = true
		}
	}
	if !found {
		t.Error("expected to find token with long value")
	}
}

func TestDBTokenStoreAdapter_GetTokenByID(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Insert required project
	proj := Project{ID: "pid", Name: "test", OpenAIAPIKey: "sk-test", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.DBCreateProject(ctx, proj))

	// Insert a token
	tok := Token{
		Token:        "tok1",
		ProjectID:    "pid",
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    time.Now(),
	}
	require.NoError(t, db.CreateToken(ctx, tok))
	created, err := db.GetTokenByToken(ctx, tok.Token)
	require.NoError(t, err)

	// Happy path
	res, err := adapter.GetTokenByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "tok1", res.Token)
	require.Equal(t, created.ID, res.ID)

	// Error path
	_, err = adapter.GetTokenByID(ctx, "notfound")
	require.ErrorIs(t, err, token.ErrTokenNotFound)
}

func TestDBTokenStoreAdapter_IncrementTokenUsage(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Insert required project
	proj := Project{ID: "pid", Name: "test", OpenAIAPIKey: "sk-test", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.DBCreateProject(ctx, proj))

	tok := Token{
		Token:        "tok2",
		ProjectID:    "pid",
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    time.Now(),
	}
	require.NoError(t, db.CreateToken(ctx, tok))

	require.NoError(t, adapter.IncrementTokenUsage(ctx, "tok2"))

	// Error path
	require.Error(t, adapter.IncrementTokenUsage(ctx, "notfound"))
}

func TestDBTokenStoreAdapter_CreateToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Insert required project
	proj := Project{ID: "pid", Name: "test", OpenAIAPIKey: "sk-test", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.DBCreateProject(ctx, proj))

	td := token.TokenData{
		Token:        "tok3",
		ProjectID:    "pid",
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    time.Now(),
	}
	require.NoError(t, adapter.CreateToken(ctx, td))

	// Duplicate
	require.Error(t, adapter.CreateToken(ctx, td))
}

func TestDBTokenStoreAdapter_ListTokens(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Insert required project
	proj := Project{ID: "pid", Name: "test", OpenAIAPIKey: "sk-test", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.DBCreateProject(ctx, proj))

	// Insert tokens
	t1 := Token{Token: "tok4", ProjectID: "pid", IsActive: true, CreatedAt: time.Now()}
	t2 := Token{Token: "tok5", ProjectID: "pid", IsActive: true, CreatedAt: time.Now()}
	require.NoError(t, db.CreateToken(ctx, t1))
	require.NoError(t, db.CreateToken(ctx, t2))

	tokens, err := adapter.ListTokens(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(tokens), 2)
}

func TestDBTokenStoreAdapter_GetTokensByProjectID(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Insert required projects
	projA := Project{ID: "pidA", Name: "A", OpenAIAPIKey: "sk-a", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	projB := Project{ID: "pidB", Name: "B", OpenAIAPIKey: "sk-b", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.DBCreateProject(ctx, projA))
	require.NoError(t, db.DBCreateProject(ctx, projB))

	t1 := Token{Token: "tok6", ProjectID: "pidA", IsActive: true, CreatedAt: time.Now()}
	t2 := Token{Token: "tok7", ProjectID: "pidB", IsActive: true, CreatedAt: time.Now()}
	require.NoError(t, db.CreateToken(ctx, t1))
	require.NoError(t, db.CreateToken(ctx, t2))

	toksA, err := adapter.GetTokensByProjectID(ctx, "pidA")
	require.NoError(t, err)
	require.Len(t, toksA, 1)
	require.Equal(t, "tok6", toksA[0].Token)

	toksB, err := adapter.GetTokensByProjectID(ctx, "pidB")
	require.NoError(t, err)
	require.Len(t, toksB, 1)
	require.Equal(t, "tok7", toksB[0].Token)
}

func TestDeleteToken_UpdateToken_IncrementTokenUsage_EdgeCases(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()

	// Insert a project and token for happy path
	p := proxy.Project{ID: "p1", Name: "P1", OpenAIAPIKey: "k", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.CreateProject(ctx, p))
	tk := Token{Token: "tok1", ProjectID: p.ID, IsActive: true, CreatedAt: time.Now()}
	require.NoError(t, db.CreateToken(ctx, tk))

	t.Run("delete happy path", func(t *testing.T) {
		err := db.DeleteToken(ctx, "tok1")
		require.NoError(t, err)
		_, err = db.GetTokenByToken(ctx, "tok1")
		require.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("delete non-existent", func(t *testing.T) {
		err := db.DeleteToken(ctx, "notfound")
		require.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("delete empty token", func(t *testing.T) {
		err := db.DeleteToken(ctx, "")
		require.Error(t, err)
	})

	// Re-insert for update/increment
	require.NoError(t, db.CreateToken(ctx, tk))

	t.Run("update happy path", func(t *testing.T) {
		tk.RequestCount = 42
		err := db.UpdateToken(ctx, tk)
		require.NoError(t, err)
		got, err := db.GetTokenByToken(ctx, tk.Token)
		require.NoError(t, err)
		require.Equal(t, 42, got.RequestCount)
	})

	t.Run("update non-existent", func(t *testing.T) {
		tk2 := Token{Token: "notfound", ProjectID: p.ID, IsActive: true, CreatedAt: time.Now()}
		err := db.UpdateToken(ctx, tk2)
		require.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("update empty token", func(t *testing.T) {
		tk3 := Token{Token: "", ProjectID: p.ID, IsActive: true, CreatedAt: time.Now()}
		err := db.UpdateToken(ctx, tk3)
		require.Error(t, err)
	})

	t.Run("increment happy path", func(t *testing.T) {
		err := db.IncrementTokenUsage(ctx, tk.Token)
		require.NoError(t, err)
		got, err := db.GetTokenByToken(ctx, tk.Token)
		require.NoError(t, err)
		require.Equal(t, 43, got.RequestCount)
	})

	t.Run("increment non-existent", func(t *testing.T) {
		err := db.IncrementTokenUsage(ctx, "notfound")
		require.ErrorIs(t, err, ErrTokenNotFound)
	})

	t.Run("increment empty token", func(t *testing.T) {
		err := db.IncrementTokenUsage(ctx, "")
		require.Error(t, err)
	})
}

func TestQueryTokens_ErrorBranches(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()

	// 1. Query error: invalid SQL
	_, err := db.queryTokens(ctx, "SELECT * FROM not_a_table")
	if err == nil {
		t.Error("expected error for invalid table")
	}

	// 2. Scan error: create a table with missing columns and query it
	_, err = db.db.ExecContext(ctx, `CREATE TABLE bad_tokens (foo TEXT)`)
	if err != nil {
		t.Fatalf("failed to create bad_tokens: %v", err)
	}
	_, err = db.db.ExecContext(ctx, `INSERT INTO bad_tokens (foo) VALUES ('bar')`)
	if err != nil {
		t.Fatalf("failed to insert into bad_tokens: %v", err)
	}
	_, err = db.queryTokens(ctx, "SELECT * FROM bad_tokens")
	if err == nil {
		t.Error("expected scan error for bad_tokens table")
	}
}

func TestIncrementCacheHitCount(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create a project and token
	p := proxy.Project{ID: "proj-cache-1", Name: "Cache Test", OpenAIAPIKey: "key", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.CreateProject(ctx, p))

	tk := Token{Token: "token-cache-1", ProjectID: p.ID, IsActive: true, RequestCount: 0, CacheHitCount: 0, CreatedAt: time.Now()}
	require.NoError(t, db.CreateToken(ctx, tk))

	t.Run("increment single token", func(t *testing.T) {
		err := db.IncrementCacheHitCount(ctx, tk.Token, 5)
		require.NoError(t, err)

		got, err := db.GetTokenByToken(ctx, tk.Token)
		require.NoError(t, err)
		require.Equal(t, 5, got.CacheHitCount)
	})

	t.Run("increment again", func(t *testing.T) {
		err := db.IncrementCacheHitCount(ctx, tk.Token, 3)
		require.NoError(t, err)

		got, err := db.GetTokenByToken(ctx, tk.Token)
		require.NoError(t, err)
		require.Equal(t, 8, got.CacheHitCount)
	})

	t.Run("increment zero delta (no-op)", func(t *testing.T) {
		err := db.IncrementCacheHitCount(ctx, tk.Token, 0)
		require.NoError(t, err)

		got, err := db.GetTokenByToken(ctx, tk.Token)
		require.NoError(t, err)
		require.Equal(t, 8, got.CacheHitCount) // Unchanged
	})

	t.Run("increment negative delta (no-op)", func(t *testing.T) {
		err := db.IncrementCacheHitCount(ctx, tk.Token, -1)
		require.NoError(t, err)

		got, err := db.GetTokenByToken(ctx, tk.Token)
		require.NoError(t, err)
		require.Equal(t, 8, got.CacheHitCount) // Unchanged
	})

	t.Run("increment non-existent token (no error, just no rows affected)", func(t *testing.T) {
		err := db.IncrementCacheHitCount(ctx, "nonexistent-token", 5)
		require.NoError(t, err) // No error, just no rows updated
	})
}

func TestIncrementCacheHitCountBatch(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create a project and multiple tokens
	p := proxy.Project{ID: "proj-cache-batch", Name: "Cache Batch Test", OpenAIAPIKey: "key", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.CreateProject(ctx, p))

	tk1 := Token{Token: "token-batch-1", ProjectID: p.ID, IsActive: true, CacheHitCount: 0, CreatedAt: time.Now()}
	tk2 := Token{Token: "token-batch-2", ProjectID: p.ID, IsActive: true, CacheHitCount: 0, CreatedAt: time.Now()}
	tk3 := Token{Token: "token-batch-3", ProjectID: p.ID, IsActive: true, CacheHitCount: 0, CreatedAt: time.Now()}
	require.NoError(t, db.CreateToken(ctx, tk1))
	require.NoError(t, db.CreateToken(ctx, tk2))
	require.NoError(t, db.CreateToken(ctx, tk3))

	t.Run("batch increment multiple tokens", func(t *testing.T) {
		deltas := map[string]int{
			tk1.Token: 5,
			tk2.Token: 10,
			tk3.Token: 3,
		}
		err := db.IncrementCacheHitCountBatch(ctx, deltas)
		require.NoError(t, err)

		got1, _ := db.GetTokenByToken(ctx, tk1.Token)
		got2, _ := db.GetTokenByToken(ctx, tk2.Token)
		got3, _ := db.GetTokenByToken(ctx, tk3.Token)

		require.Equal(t, 5, got1.CacheHitCount)
		require.Equal(t, 10, got2.CacheHitCount)
		require.Equal(t, 3, got3.CacheHitCount)
	})

	t.Run("batch with empty map (no-op)", func(t *testing.T) {
		err := db.IncrementCacheHitCountBatch(ctx, map[string]int{})
		require.NoError(t, err)
	})

	t.Run("batch with zero/negative deltas (skipped)", func(t *testing.T) {
		deltas := map[string]int{
			tk1.Token: 0,  // Skip
			tk2.Token: -1, // Skip
			tk3.Token: 2,  // Apply
		}
		err := db.IncrementCacheHitCountBatch(ctx, deltas)
		require.NoError(t, err)

		got1, _ := db.GetTokenByToken(ctx, tk1.Token)
		got2, _ := db.GetTokenByToken(ctx, tk2.Token)
		got3, _ := db.GetTokenByToken(ctx, tk3.Token)

		require.Equal(t, 5, got1.CacheHitCount)  // Unchanged from previous test
		require.Equal(t, 10, got2.CacheHitCount) // Unchanged from previous test
		require.Equal(t, 5, got3.CacheHitCount)  // 3 + 2 from previous test
	})

	t.Run("batch with nonexistent tokens (no error)", func(t *testing.T) {
		deltas := map[string]int{
			"nonexistent-1": 5,
			"nonexistent-2": 10,
		}
		err := db.IncrementCacheHitCountBatch(ctx, deltas)
		require.NoError(t, err) // No error, just no rows updated
	})
}

func TestIncrementCacheHitCountBatch_ErrorDoesNotLeakToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()

	// Create a project and a token (token value is a secret in production).
	p := proxy.Project{ID: "proj-cache-batch-redact", Name: "Cache Batch Redact Test", OpenAIAPIKey: "key", IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, db.CreateProject(ctx, p))

	secretToken := "sk-THIS_IS_A_SECRET_TOKEN"
	require.NoError(t, db.CreateToken(ctx, Token{Token: secretToken, ProjectID: p.ID, IsActive: true, CacheHitCount: 0, CreatedAt: time.Now()}))

	// Get the backing SQLite file path so we can open a second connection.
	rows, err := db.db.Query("PRAGMA database_list")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, rows.Close())
	})

	var dbPath string
	for rows.Next() {
		var seq int
		var name string
		var file string
		require.NoError(t, rows.Scan(&seq, &name, &file))
		if name == "main" {
			dbPath = file
			break
		}
	}
	require.NoError(t, rows.Err())
	require.NotEmpty(t, dbPath)

	lockDB, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_foreign_keys=on&_loc=UTC&_busy_timeout=1")
	require.NoError(t, err)
	defer func() { _ = lockDB.Close() }()

	// Hold a write lock to force the batch update to fail.
	_, err = lockDB.Exec("BEGIN IMMEDIATE")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = lockDB.Exec("ROLLBACK")
	})

	callCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()

	err = db.IncrementCacheHitCountBatch(callCtx, map[string]int{secretToken: 1})
	require.Error(t, err)

	// Must never leak the raw token in errors (errors are often logged).
	require.NotContains(t, err.Error(), secretToken)
	require.Contains(t, err.Error(), obfuscate.ObfuscateTokenGeneric(secretToken))
}
