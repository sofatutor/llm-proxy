package database

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBTokenStoreAdapter_UpdateToken_Integration(t *testing.T) {
	// Create a temporary SQLite database for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Initialize database
	db, err := New(Config{Path: dbPath})
	require.NoError(t, err)
	defer db.Close()

	// Create a test project first (required for foreign key constraint)
	project := Project{
		ID:           "proj-test",
		Name:         "Test Project",
		OpenAIAPIKey: "sk-test-api-key",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = db.DBCreateProject(context.Background(), project)
	require.NoError(t, err)

	// Create adapter
	adapter := NewDBTokenStoreAdapter(db)

	// Create a test token first
	expiresAt := time.Now().Add(24 * time.Hour)
	testToken := token.TokenData{
		Token:        "sk-test123456789",
		ProjectID:    "proj-test",
		IsActive:     true,
		MaxRequests:  intPtr(1000),
		RequestCount: 0,
		CreatedAt:    time.Now(),
		ExpiresAt:    &expiresAt,
	}

	// Insert the token
	err = adapter.CreateToken(context.Background(), testToken)
	require.NoError(t, err)

	// Update the token
	testToken.IsActive = false
	testToken.MaxRequests = intPtr(2000)
	testToken.RequestCount = 100

	// This should hit the UpdateToken method with 0% coverage
	err = adapter.UpdateToken(context.Background(), testToken)
	require.NoError(t, err)

	// Verify the update worked
	retrievedToken, err := adapter.GetTokenByID(context.Background(), testToken.Token)
	require.NoError(t, err)

	assert.Equal(t, false, retrievedToken.IsActive)
	assert.Equal(t, intPtr(2000), retrievedToken.MaxRequests)
	assert.Equal(t, 100, retrievedToken.RequestCount)
}

func TestDBTokenStoreAdapter_Coverage(t *testing.T) {
	t.Skip("DBTokenStoreAdapter methods require a real database connection; skipping until integration test is implemented.")
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
