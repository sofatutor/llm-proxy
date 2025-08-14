package database

import (
	"context"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
)

func TestDBTokenStoreAdapter_RevokeToken(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	adapter := NewDBTokenStoreAdapter(db)

	// Create a test token
	testToken := Token{
		Token:     "test-token-123",
		ProjectID: "test-project-456",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	err := db.CreateToken(context.Background(), testToken)
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
			err := adapter.RevokeToken(context.Background(), tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("RevokeToken() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If successful, verify token is deactivated
			if !tt.wantErr && err == nil {
				dbToken, getErr := db.GetTokenByID(context.Background(), tt.tokenID)
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

	// Create test tokens
	tokens := []Token{
		{Token: "token-1", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-2", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-3", ProjectID: "project-2", IsActive: true, CreatedAt: time.Now()},
	}

	for _, token := range tokens {
		err := db.CreateToken(context.Background(), token)
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
			revokedCount, err := adapter.RevokeBatchTokens(context.Background(), tt.tokenIDs)
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

	// Create test tokens
	tokens := []Token{
		{Token: "token-proj1-1", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-proj1-2", ProjectID: "project-1", IsActive: true, CreatedAt: time.Now()},
		{Token: "token-proj2-1", ProjectID: "project-2", IsActive: true, CreatedAt: time.Now()},
	}

	for _, token := range tokens {
		err := db.CreateToken(context.Background(), token)
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
			revokedCount, err := adapter.RevokeProjectTokens(context.Background(), tt.projectID)
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
		err := db.CreateToken(context.Background(), token)
		if err != nil {
			t.Fatalf("Failed to create test token %s: %v", token.Token, err)
		}
	}

	revokedCount, err := adapter.RevokeExpiredTokens(context.Background())
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
		dbToken, getErr := db.GetTokenByID(context.Background(), tokenID)
		if getErr != nil {
			t.Errorf("Failed to get token %s: %v", tokenID, getErr)
		} else if dbToken.IsActive {
			t.Errorf("Expired token %s should be inactive", tokenID)
		}
	}

	// Verify non-expired tokens are still active
	activeTokens := []string{"token-active", "token-no-expiry"}
	for _, tokenID := range activeTokens {
		dbToken, getErr := db.GetTokenByID(context.Background(), tokenID)
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

	// Create a test token
	testToken := Token{
		Token:     "test-token-idem",
		ProjectID: "test-project",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	err := db.CreateToken(context.Background(), testToken)
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// First revocation
	err = adapter.RevokeToken(context.Background(), "test-token-idem")
	if err != nil {
		t.Errorf("First revocation failed: %v", err)
	}

	// Get the deactivated_at time from first revocation
	firstRevoke, err := db.GetTokenByID(context.Background(), "test-token-idem")
	if err != nil {
		t.Fatalf("Failed to get token after first revocation: %v", err)
	}

	// Second revocation (should be idempotent)
	err = adapter.RevokeToken(context.Background(), "test-token-idem")
	if err != nil {
		t.Errorf("Second revocation should be idempotent but failed: %v", err)
	}

	// Verify deactivated_at didn't change
	secondRevoke, err := db.GetTokenByID(context.Background(), "test-token-idem")
	if err != nil {
		t.Fatalf("Failed to get token after second revocation: %v", err)
	}

	if firstRevoke.DeactivatedAt == nil || secondRevoke.DeactivatedAt == nil {
		t.Error("DeactivatedAt should be set after revocation")
	} else if !firstRevoke.DeactivatedAt.Equal(*secondRevoke.DeactivatedAt) {
		t.Error("DeactivatedAt should not change on subsequent revocations")
	}
}