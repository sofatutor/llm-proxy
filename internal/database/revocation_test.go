package database

import (
	"context"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

func TestDBTokenStoreAdapter_RevokeToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create a test project first
	project := proxy.Project{
		ID:        "test-project-456",
		Name:      "Test Project",
		APIKey:    "test-api-key",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Create a test token
	testToken := Token{
		Token:     "test-token-123",
		ProjectID: "test-project-456",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	err = db.CreateToken(ctx, testToken)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	tests := []struct {
		name    string
		tokenID string
		wantErr bool
	}{
		{"revoke existing token", "test-token-123", false},
		{"revoke non-existent token", "non-existent", true},
		{"revoke empty token", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.RevokeToken(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("RevokeToken() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If successful, verify token is deactivated
			if !tt.wantErr && err == nil {
				dbToken, getErr := db.GetTokenByToken(ctx, tt.tokenID)
				if getErr != nil {
					t.Errorf("Failed to get revoked token: %v", getErr)
				} else if dbToken.IsActive {
					t.Errorf("Token should be inactive after revocation")
				} else if dbToken.DeactivatedAt == nil {
					t.Errorf("Token should have deactivated_at set after revocation")
				}
			}
		})
	}
}

func TestDBTokenStoreAdapter_RevokeBatchTokens(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create test projects first
	projects := []proxy.Project{
		{
			ID:        "project-1",
			Name:      "Test Project 1",
			APIKey:    "test-api-key-1",
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		},
		{
			ID:        "project-2",
			Name:      "Test Project 2",
			APIKey:    "test-api-key-2",
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		},
	}

	for _, project := range projects {
		err := db.CreateProject(ctx, project)
		if err != nil {
			t.Fatalf("Failed to create test project %s: %v", project.ID, err)
		}
	}

	// Create test tokens
	tokens := []Token{
		{Token: "token-1", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-2", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-3", ProjectID: "project-2", IsActive: true, CreatedAt: time.Now()},
	}

	for _, token := range tokens {
		err := db.CreateToken(ctx, token)
		if err != nil {
			t.Fatalf("Failed to create test token %s: %v", token.Token, err)
		}
	}

	tests := []struct {
		name        string
		tokenIDs    []string
		wantRevoked int
		wantErr     bool
	}{
		{"revoke multiple tokens", []string{"token-1", "token-2"}, 2, false},
		{"revoke with non-existent", []string{"token-3", "non-existent"}, 1, false},
		{"revoke empty list", []string{}, 0, false},
		{"revoke already revoked", []string{"token-1"}, 0, false}, // Already revoked in first test
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revokedCount, err := adapter.RevokeBatchTokens(ctx, tt.tokenIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("RevokeBatchTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
			if revokedCount != tt.wantRevoked {
				t.Errorf("RevokeBatchTokens() revoked = %v, want %v", revokedCount, tt.wantRevoked)
			}
		})
	}
}

func TestDBTokenStoreAdapter_RevokeProjectTokens(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create test projects first
	projects := []proxy.Project{
		{
			ID:        "project-1",
			Name:      "Test Project 1",
			APIKey:    "test-api-key-1",
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		},
		{
			ID:        "project-2",
			Name:      "Test Project 2",
			APIKey:    "test-api-key-2",
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		},
	}

	for _, project := range projects {
		err := db.CreateProject(ctx, project)
		if err != nil {
			t.Fatalf("Failed to create test project %s: %v", project.ID, err)
		}
	}

	// Create test tokens
	tokens := []Token{
		{Token: "token-proj1-1", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-proj1-2", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-proj2-1", ProjectID: "project-2", IsActive: true, CreatedAt: time.Now()},
	}

	for _, token := range tokens {
		err := db.CreateToken(ctx, token)
		if err != nil {
			t.Fatalf("Failed to create test token %s: %v", token.Token, err)
		}
	}

	tests := []struct {
		name        string
		projectID   string
		wantRevoked int
		wantErr     bool
	}{
		{"revoke project tokens", "project-1", 2, false},
		{"revoke non-existent project", "non-existent", 0, false},
		{"revoke empty project", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			revokedCount, err := adapter.RevokeProjectTokens(ctx, tt.projectID)
			if (err != nil) != tt.wantErr {
				t.Errorf("RevokeProjectTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
			if revokedCount != tt.wantRevoked {
				t.Errorf("RevokeProjectTokens() revoked = %v, want %v", revokedCount, tt.wantRevoked)
			}
		})
	}
}

func TestDBTokenStoreAdapter_RevokeExpiredTokens(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create a test project first
	project := proxy.Project{
		ID:        "project-1",
		Name:      "Test Project",
		APIKey:    "test-api-key",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	// Create test tokens
	tokens := []Token{
		{Token: "token-expired-1", ProjectID: "project-1", IsActive: true, ExpiresAt: &past, CreatedAt: now},
		{Token: "token-expired-2", ProjectID: "project-1", IsActive: true, ExpiresAt: &past, CreatedAt: now},
		{Token: "token-active", ProjectID: "project-1", IsActive: true, ExpiresAt: &future, CreatedAt: now},
		{Token: "token-no-expiry", ProjectID: "project-1", IsActive: true, CreatedAt: now},
	}

	for _, token := range tokens {
		err := db.CreateToken(ctx, token)
		if err != nil {
			t.Fatalf("Failed to create test token %s: %v", token.Token, err)
		}
	}

	revokedCount, err := adapter.RevokeExpiredTokens(ctx)
	if err != nil {
		t.Errorf("RevokeExpiredTokens() error = %v", err)
	}

	expectedRevoked := 2 // Only the expired tokens should be revoked
	if revokedCount != expectedRevoked {
		t.Errorf("RevokeExpiredTokens() revoked = %v, want %v", revokedCount, expectedRevoked)
	}

	// Verify expired tokens are revoked
	expiredTokens := []string{"token-expired-1", "token-expired-2"}
	for _, tokenID := range expiredTokens {
		dbToken, getErr := db.GetTokenByToken(ctx, tokenID)
		if getErr != nil {
			t.Errorf("Failed to get token %s: %v", tokenID, getErr)
		} else if dbToken.IsActive {
			t.Errorf("Expired token %s should be inactive", tokenID)
		}
	}

	// Verify non-expired tokens are still active
	activeTokens := []string{"token-active", "token-no-expiry"}
	for _, tokenID := range activeTokens {
		dbToken, getErr := db.GetTokenByToken(ctx, tokenID)
		if getErr != nil {
			t.Errorf("Failed to get token %s: %v", tokenID, getErr)
		} else if !dbToken.IsActive {
			t.Errorf("Non-expired token %s should still be active", tokenID)
		}
	}
}

func TestDBTokenStoreAdapter_RevokeToken_Idempotency(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create a test project first
	project := proxy.Project{
		ID:        "test-project",
		Name:      "Test Project",
		APIKey:    "test-api-key",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Create a test token
	testToken := Token{
		Token:     "test-token-idem",
		ProjectID: "test-project",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	err = db.CreateToken(ctx, testToken)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// First revocation
	err = adapter.RevokeToken(ctx, "test-token-idem")
	if err != nil {
		t.Errorf("First revocation failed: %v", err)
	}

	// Get the deactivated_at time from first revocation
	firstRevoke, err := db.GetTokenByToken(ctx, "test-token-idem")
	if err != nil {
		t.Fatalf("Failed to get token after first revocation: %v", err)
	}

	// Second revocation (should be idempotent)
	err = adapter.RevokeToken(ctx, "test-token-idem")
	if err != nil {
		t.Errorf("Second revocation should be idempotent but failed: %v", err)
	}

	// Verify deactivated_at didn't change
	secondRevoke, err := db.GetTokenByToken(ctx, "test-token-idem")
	if err != nil {
		t.Fatalf("Failed to get token after second revocation: %v", err)
	}

	if firstRevoke.DeactivatedAt == nil || secondRevoke.DeactivatedAt == nil {
		t.Error("DeactivatedAt should be set after revocation")
	} else if !firstRevoke.DeactivatedAt.Equal(*secondRevoke.DeactivatedAt) {
		t.Error("DeactivatedAt should not change on subsequent revocations")
	}
}

func TestDBTokenStoreAdapter_DeleteToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create a test project first
	project := proxy.Project{
		ID:        "test-project-delete",
		Name:      "Test Project",
		APIKey:    "test-api-key",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	// Create a test token
	testToken := Token{
		Token:     "test-token-delete",
		ProjectID: "test-project-delete",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	err = db.CreateToken(ctx, testToken)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	tests := []struct {
		name    string
		tokenID string
		wantErr bool
		errType error
	}{
		{"delete existing token", "test-token-delete", false, nil},
		{"delete non-existent token", "non-existent-token", true, nil},
		{"delete empty token ID", "", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.DeleteToken(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteToken() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If successful deletion of existing token, verify it's completely gone
			if !tt.wantErr && err == nil && tt.tokenID == "test-token-delete" {
				_, getErr := db.GetTokenByToken(ctx, tt.tokenID)
				if getErr == nil {
					t.Error("Token should be completely deleted from database")
				}
			}
		})
	}
}

func TestDBTokenStoreAdapter_DeleteToken_Comprehensive(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create test projects
	projects := []proxy.Project{
		{
			ID:        "project-del-1",
			Name:      "Test Project 1",
			APIKey:    "test-api-key-1",
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		},
		{
			ID:        "project-del-2",
			Name:      "Test Project 2",
			APIKey:    "test-api-key-2",
			CreatedAt: time.Now().UTC().Truncate(time.Second),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		},
	}

	for _, project := range projects {
		err := db.CreateProject(ctx, project)
		if err != nil {
			t.Fatalf("Failed to create test project %s: %v", project.ID, err)
		}
	}

	// Create multiple test tokens
	tokens := []Token{
		{Token: "token-del-1", ProjectID: "project-del-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-del-2", ProjectID: "project-del-1", IsActive: false, CreatedAt: time.Now()}, // inactive token
		{Token: "token-del-3", ProjectID: "project-del-2", IsActive: true, CreatedAt: time.Now()},
	}

	for _, token := range tokens {
		err := db.CreateToken(ctx, token)
		if err != nil {
			t.Fatalf("Failed to create test token %s: %v", token.Token, err)
		}
	}

	// Test deleting active token
	err := adapter.DeleteToken(ctx, "token-del-1")
	if err != nil {
		t.Errorf("Failed to delete active token: %v", err)
	}

	// Test deleting inactive token
	err = adapter.DeleteToken(ctx, "token-del-2")
	if err != nil {
		t.Errorf("Failed to delete inactive token: %v", err)
	}

	// Verify tokens are actually deleted
	for _, tokenID := range []string{"token-del-1", "token-del-2"} {
		_, getErr := db.GetTokenByToken(ctx, tokenID)
		if getErr == nil {
			t.Errorf("Token %s should be completely deleted from database", tokenID)
		}
	}

	// Verify remaining token is still there
	remainingToken, err := db.GetTokenByToken(ctx, "token-del-3")
	if err != nil {
		t.Errorf("Remaining token should still exist: %v", err)
	} else if remainingToken.Token != "token-del-3" {
		t.Errorf("Unexpected token retrieved: %s", remainingToken.Token)
	}
}

func TestDBTokenStoreAdapter_ErrorHandling(t *testing.T) {
	// Test error handling with closed database
	db, cleanup := testDB(t)
	cleanup() // Close DB immediately

	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Test RevokeToken with closed DB
	err := adapter.RevokeToken(ctx, "some-token")
	if err == nil {
		t.Error("Expected error when revoking token with closed database")
	}

	// Test DeleteToken with closed DB
	err = adapter.DeleteToken(ctx, "some-token")
	if err == nil {
		t.Error("Expected error when deleting token with closed database")
	}

	// Test RevokeBatchTokens with closed DB
	_, err = adapter.RevokeBatchTokens(ctx, []string{"token1", "token2"})
	if err == nil {
		t.Error("Expected error when batch revoking tokens with closed database")
	}

	// Test RevokeProjectTokens with closed DB
	_, err = adapter.RevokeProjectTokens(ctx, "some-project")
	if err == nil {
		t.Error("Expected error when revoking project tokens with closed database")
	}

	// Test RevokeExpiredTokens with closed DB
	_, err = adapter.RevokeExpiredTokens(ctx)
	if err == nil {
		t.Error("Expected error when revoking expired tokens with closed database")
	}
}

func TestDBTokenStoreAdapter_RevokeExpiredTokens_EdgeCases(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	ctx := context.Background()

	// Create a test project first
	project := proxy.Project{
		ID:        "project-edge",
		Name:      "Test Project",
		APIKey:    "test-api-key",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	now := time.Now().UTC()
	veryOld := now.Add(-24 * time.Hour)
	barelyExpired := now.Add(-1 * time.Millisecond)

	// Create test tokens with various expiration scenarios
	tokens := []Token{
		{Token: "token-very-old", ProjectID: "project-edge", IsActive: true, ExpiresAt: &veryOld, CreatedAt: now},
		{Token: "token-barely-expired", ProjectID: "project-edge", IsActive: true, ExpiresAt: &barelyExpired, CreatedAt: now},
		{Token: "token-already-inactive", ProjectID: "project-edge", IsActive: false, ExpiresAt: &veryOld, CreatedAt: now},
	}

	for _, token := range tokens {
		err := db.CreateToken(ctx, token)
		if err != nil {
			t.Fatalf("Failed to create test token %s: %v", token.Token, err)
		}
	}

	// First call to revoke expired tokens
	revokedCount, err := adapter.RevokeExpiredTokens(ctx)
	if err != nil {
		t.Errorf("RevokeExpiredTokens() error = %v", err)
	}

	expectedRevoked := 2 // token-very-old and token-barely-expired
	if revokedCount != expectedRevoked {
		t.Errorf("RevokeExpiredTokens() revoked = %v, want %v", revokedCount, expectedRevoked)
	}

	// Second call should revoke 0 tokens (idempotent)
	revokedCount, err = adapter.RevokeExpiredTokens(ctx)
	if err != nil {
		t.Errorf("Second RevokeExpiredTokens() error = %v", err)
	}

	if revokedCount != 0 {
		t.Errorf("Second RevokeExpiredTokens() should revoke 0 tokens, got %v", revokedCount)
	}
}

func TestDBTokenStoreAdapter_RevokeBatchTokens_EmptyAndDuplicates(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)
	ctx := context.Background()

	// Test with empty list
	revokedCount, err := adapter.RevokeBatchTokens(ctx, []string{})
	if err != nil {
		t.Errorf("RevokeBatchTokens with empty list error = %v", err)
	}
	if revokedCount != 0 {
		t.Errorf("RevokeBatchTokens with empty list should return 0, got %v", revokedCount)
	}

	// Test with nil list
	revokedCount, err = adapter.RevokeBatchTokens(ctx, nil)
	if err != nil {
		t.Errorf("RevokeBatchTokens with nil list error = %v", err)
	}
	if revokedCount != 0 {
		t.Errorf("RevokeBatchTokens with nil list should return 0, got %v", revokedCount)
	}

	// Create a test project and token
	project := proxy.Project{
		ID:        "project-dup",
		Name:      "Test Project",
		APIKey:    "test-api-key",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		UpdatedAt: time.Now().UTC().Truncate(time.Second),
	}
	err = db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create test project: %v", err)
	}

	token := Token{
		Token:     "token-dup",
		ProjectID: "project-dup",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	err = db.CreateToken(ctx, token)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Test with duplicate token IDs in list
	revokedCount, err = adapter.RevokeBatchTokens(ctx, []string{"token-dup", "token-dup", "token-dup"})
	if err != nil {
		t.Errorf("RevokeBatchTokens with duplicates error = %v", err)
	}
	if revokedCount != 1 {
		t.Errorf("RevokeBatchTokens with duplicates should return 1, got %v", revokedCount)
	}
}
