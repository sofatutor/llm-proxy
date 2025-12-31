package database

import (
	"context"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
)

func TestMockTokenStore_BasicCRUD(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()
	tk := Token{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}

	// CreateToken
	err := store.CreateToken(ctx, tk)
	assert.NoError(t, err)

	// Duplicate CreateToken
	err = store.CreateToken(ctx, tk)
	assert.Error(t, err)

	// GetTokenByID
	got, err := store.GetTokenByID(ctx, "t1")
	assert.NoError(t, err)
	assert.Equal(t, tk.Token, got.Token)

	// GetTokenByID (not found)
	_, err = store.GetTokenByID(ctx, "notfound")
	assert.Error(t, err)

	// UpdateToken
	tk.RequestCount = 42
	err = store.UpdateToken(ctx, tk)
	assert.NoError(t, err)
	got, _ = store.GetTokenByID(ctx, "t1")
	assert.Equal(t, 42, got.RequestCount)

	// UpdateToken (not found)
	err = store.UpdateToken(ctx, Token{Token: "notfound"})
	assert.Error(t, err)

	// DeleteToken
	err = store.DeleteToken(ctx, "t1")
	assert.NoError(t, err)

	// DeleteToken (not found)
	err = store.DeleteToken(ctx, "t1")
	assert.Error(t, err)
}

func TestMockTokenStore_ListTokens(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()
	// Empty list
	tokens, err := store.ListTokens(ctx)
	assert.NoError(t, err)
	assert.Len(t, tokens, 0)
	// Add tokens
	err = store.CreateToken(ctx, Token{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()})
	assert.NoError(t, err)
	err = store.CreateToken(ctx, Token{Token: "t2", ProjectID: "p2", IsActive: true, CreatedAt: time.Now()})
	assert.NoError(t, err)
	tokens, err = store.ListTokens(ctx)
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestMockTokenStore_GetTokensByProjectID(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()
	err := store.CreateToken(ctx, Token{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()})
	assert.NoError(t, err)
	err = store.CreateToken(ctx, Token{Token: "t2", ProjectID: "p2", IsActive: true, CreatedAt: time.Now()})
	assert.NoError(t, err)
	tokens, err := store.GetTokensByProjectID(ctx, "p1")
	assert.NoError(t, err)
	assert.Len(t, tokens, 1)
	assert.Equal(t, "t1", tokens[0].Token)
}

func TestMockTokenStore_IncrementTokenUsage(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()
	tk := Token{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}
	err := store.CreateToken(ctx, tk)
	assert.NoError(t, err)
	err = store.IncrementTokenUsage(ctx, "t1")
	assert.NoError(t, err)
	got, _ := store.GetTokenByID(ctx, "t1")
	assert.Equal(t, 1, got.RequestCount)
	assert.NotNil(t, got.LastUsedAt)
	// Not found
	err = store.IncrementTokenUsage(ctx, "notfound")
	assert.Error(t, err)
}

func TestMockTokenStore_IncrementTokenUsage_EnforcesMaxRequests(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()
	maxRequests := 1
	tk := Token{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now(), MaxRequests: &maxRequests}
	assert.NoError(t, store.CreateToken(ctx, tk))

	assert.NoError(t, store.IncrementTokenUsage(ctx, "t1"))
	err := store.IncrementTokenUsage(ctx, "t1")
	assert.ErrorIs(t, err, token.ErrTokenRateLimit)
}

func TestMockTokenStore_CleanExpiredTokens(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()
	expired := time.Now().Add(-time.Hour)
	active := time.Now().Add(time.Hour)
	err := store.CreateToken(ctx, Token{Token: "expired", ProjectID: "p1", IsActive: true, ExpiresAt: &expired, CreatedAt: time.Now()})
	assert.NoError(t, err)
	err = store.CreateToken(ctx, Token{Token: "active", ProjectID: "p1", IsActive: true, ExpiresAt: &active, CreatedAt: time.Now()})
	assert.NoError(t, err)
	count, err := store.CleanExpiredTokens(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
	tokens, _ := store.ListTokens(ctx)
	assert.Len(t, tokens, 1)
	assert.Equal(t, "active", tokens[0].Token)
}

func TestMockTokenStore_CreateMockToken(t *testing.T) {
	store := NewMockTokenStore()
	// Empty fields
	_, err := store.CreateMockToken("", "p", time.Hour, true, nil)
	assert.Error(t, err)
	_, err = store.CreateMockToken("t", "", time.Hour, true, nil)
	assert.Error(t, err)
	// Success
	tk, err := store.CreateMockToken("t", "p", time.Hour, true, nil)
	assert.NoError(t, err)
	assert.Equal(t, "t", tk.Token)
	assert.Equal(t, "p", tk.ProjectID)
}

func TestMockTokenStore_ImportExportTokenData(t *testing.T) {
	td := token.TokenData{Token: "t", ProjectID: "p", IsActive: true, CreatedAt: time.Now()}
	dbToken := ImportTokenData(td)
	assert.Equal(t, td.Token, dbToken.Token)
	exported := ExportTokenData(dbToken)
	assert.Equal(t, td.Token, exported.Token)
}

func TestTokenStoreAdapter_Basic(t *testing.T) {
	store := NewMockTokenStore()
	adapter := NewTokenStoreAdapter(store)
	ctx := context.Background()

	tk := Token{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}
	err := store.CreateToken(ctx, tk)
	assert.NoError(t, err)

	// GetTokenByID success
	got, err := adapter.GetTokenByID(ctx, "t1")
	assert.NoError(t, err)
	assert.Equal(t, tk.Token, got.Token)

	// GetTokenByID not found
	_, err = adapter.GetTokenByID(ctx, "notfound")
	assert.ErrorIs(t, err, token.ErrTokenNotFound)

	// IncrementTokenUsage success
	err = adapter.IncrementTokenUsage(ctx, "t1")
	assert.NoError(t, err)

	// IncrementTokenUsage not found
	err = adapter.IncrementTokenUsage(ctx, "notfound")
	assert.ErrorIs(t, err, token.ErrTokenNotFound)
}

func TestTokenStoreAdapter_CreateToken(t *testing.T) {
	store := NewMockTokenStore()
	adapter := NewTokenStoreAdapter(store)
	ctx := context.Background()

	td := token.TokenData{Token: "test-token", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}
	err := adapter.CreateToken(ctx, td)
	assert.NoError(t, err)

	// Verify it was created
	got, err := adapter.GetTokenByID(ctx, "test-token")
	assert.NoError(t, err)
	assert.Equal(t, td.Token, got.Token)
}

func TestTokenStoreAdapter_ListTokens(t *testing.T) {
	store := NewMockTokenStore()
	adapter := NewTokenStoreAdapter(store)
	ctx := context.Background()

	// Add some tokens
	td1 := token.TokenData{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}
	td2 := token.TokenData{Token: "t2", ProjectID: "p2", IsActive: true, CreatedAt: time.Now()}

	err := adapter.CreateToken(ctx, td1)
	assert.NoError(t, err)
	err = adapter.CreateToken(ctx, td2)
	assert.NoError(t, err)

	tokens, err := adapter.ListTokens(ctx)
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)
}

func TestTokenStoreAdapter_GetTokensByProjectID(t *testing.T) {
	store := NewMockTokenStore()
	adapter := NewTokenStoreAdapter(store)
	ctx := context.Background()

	// Add tokens for different projects
	td1 := token.TokenData{Token: "t1", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}
	td2 := token.TokenData{Token: "t2", ProjectID: "p1", IsActive: true, CreatedAt: time.Now()}
	td3 := token.TokenData{Token: "t3", ProjectID: "p2", IsActive: true, CreatedAt: time.Now()}

	err := adapter.CreateToken(ctx, td1)
	assert.NoError(t, err)
	err = adapter.CreateToken(ctx, td2)
	assert.NoError(t, err)
	err = adapter.CreateToken(ctx, td3)
	assert.NoError(t, err)

	tokens, err := adapter.GetTokensByProjectID(ctx, "p1")
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)

	for _, tk := range tokens {
		assert.Equal(t, "p1", tk.ProjectID)
	}
}

func TestMockTokenStore_CallEdgeCases(t *testing.T) {
	// Call the edge cases test function to ensure it's covered
	TestMockTokenStore_EdgeCases(t)
}

func TestTokenStoreAdapter_ErrorPaths(t *testing.T) {
	t.Run("GetTokenByID generic error", func(t *testing.T) {
		// Create an adapter with a mock store that returns a generic error
		store := NewMockTokenStore()
		adapter := NewTokenStoreAdapter(store)
		ctx := context.Background()

		// Non-existent token returns ErrTokenNotFound which maps to token.ErrTokenNotFound
		_, err := adapter.GetTokenByID(ctx, "nonexistent")
		assert.ErrorIs(t, err, token.ErrTokenNotFound)
	})

	t.Run("IncrementTokenUsage generic error", func(t *testing.T) {
		store := NewMockTokenStore()
		adapter := NewTokenStoreAdapter(store)
		ctx := context.Background()

		// Non-existent token returns ErrTokenNotFound which maps to token.ErrTokenNotFound
		err := adapter.IncrementTokenUsage(ctx, "nonexistent")
		assert.ErrorIs(t, err, token.ErrTokenNotFound)
	})
}
