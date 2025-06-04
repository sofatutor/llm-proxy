package database

import (
	"context"
	"strconv"
	"testing"
	"time"

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

	// Happy path
	res, err := adapter.GetTokenByID(ctx, "tok1")
	require.NoError(t, err)
	require.Equal(t, "tok1", res.Token)

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
		_, err = db.GetTokenByID(ctx, "tok1")
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
		got, err := db.GetTokenByID(ctx, tk.Token)
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
		got, err := db.GetTokenByID(ctx, tk.Token)
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
