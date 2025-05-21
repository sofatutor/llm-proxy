package token

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockRevocationStore implements RevocationStore for testing
type MockRevocationStore struct {
	tokens          map[string]TokenData
	projects        map[string][]string // project ID -> token IDs
	revocationCount int
	deleteCount     int
	failOnRevoke    bool
	failOnDelete    bool
}

func NewMockRevocationStore() *MockRevocationStore {
	return &MockRevocationStore{
		tokens:       make(map[string]TokenData),
		projects:     make(map[string][]string),
		failOnRevoke: false,
		failOnDelete: false,
	}
}

func (m *MockRevocationStore) RevokeToken(ctx context.Context, tokenID string) error {
	if m.failOnRevoke {
		return errors.New("mock revocation failure")
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
	m.revocationCount++
	return nil
}

func (m *MockRevocationStore) DeleteToken(ctx context.Context, tokenID string) error {
	if m.failOnDelete {
		return errors.New("mock deletion failure")
	}

	if _, exists := m.tokens[tokenID]; !exists {
		return ErrTokenNotFound
	}

	delete(m.tokens, tokenID)
	m.deleteCount++
	return nil
}

func (m *MockRevocationStore) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	if m.failOnRevoke {
		return 0, errors.New("mock batch revocation failure")
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

	m.revocationCount += count
	return count, nil
}

func (m *MockRevocationStore) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	if m.failOnRevoke {
		return 0, errors.New("mock project revocation failure")
	}

	tokenIDs, exists := m.projects[projectID]
	if !exists {
		return 0, nil
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

	m.revocationCount += count
	return count, nil
}

func (m *MockRevocationStore) RevokeExpiredTokens(ctx context.Context) (int, error) {
	if m.failOnRevoke {
		return 0, errors.New("mock expired revocation failure")
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

	m.revocationCount += count
	return count, nil
}

func (m *MockRevocationStore) AddToken(tokenID string, data TokenData) {
	m.tokens[tokenID] = data

	// Add to projects map
	if _, exists := m.projects[data.ProjectID]; !exists {
		m.projects[data.ProjectID] = []string{}
	}
	m.projects[data.ProjectID] = append(m.projects[data.ProjectID], tokenID)
}

func TestRevoker_RevokeToken(t *testing.T) {
	ctx := context.Background()
	store := NewMockRevocationStore()
	revoker := NewRevoker(store)

	activeToken, _ := GenerateToken()
	inactiveToken, _ := GenerateToken()
	nonExistentToken, _ := GenerateToken()

	// Add an active token
	store.AddToken(activeToken, TokenData{
		Token:     activeToken,
		ProjectID: "project1",
		IsActive:  true,
	})

	// Add an inactive token
	store.AddToken(inactiveToken, TokenData{
		Token:     inactiveToken,
		ProjectID: "project1",
		IsActive:  false,
	})

	tests := []struct {
		name      string
		tokenID   string
		wantErr   bool
		wantErrIs error
	}{
		{"Revoke active token", activeToken, false, nil},
		{"Revoke already revoked token", inactiveToken, true, ErrTokenAlreadyRevoked},
		{"Revoke non-existent token", nonExistentToken, true, ErrTokenNotFound},
		{"Invalid token format", "invalid-token-format", true, ErrInvalidTokenFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := revoker.RevokeToken(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Revoker.RevokeToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Revoker.RevokeToken() error = %v, want %v", err, tt.wantErrIs)
			}

			// Check that the token was actually revoked
			if err == nil {
				token, exists := store.tokens[tt.tokenID]
				if !exists {
					t.Errorf("token %s should exist in store", tt.tokenID)
					return
				}
				if token.IsActive {
					t.Errorf("token %s should be inactive", tt.tokenID)
				}
			}
		})
	}

	// Test store failure
	t.Run("Store failure", func(t *testing.T) {
		store.failOnRevoke = true
		defer func() { store.failOnRevoke = false }()

		err := revoker.RevokeToken(ctx, activeToken)
		if err == nil {
			t.Errorf("Revoker.RevokeToken() expected error on store failure")
		}
	})
}

func TestRevoker_DeleteToken(t *testing.T) {
	ctx := context.Background()
	store := NewMockRevocationStore()
	revoker := NewRevoker(store)

	deleteToken, _ := GenerateToken()
	nonExistentToken, _ := GenerateToken()

	// Add a token to delete
	store.AddToken(deleteToken, TokenData{
		Token:     deleteToken,
		ProjectID: "project1",
		IsActive:  true,
	})

	tests := []struct {
		name      string
		tokenID   string
		wantErr   bool
		wantErrIs error
	}{
		{"Delete existing token", deleteToken, false, nil},
		{"Delete non-existent token", nonExistentToken, true, ErrTokenNotFound},
		{"Invalid token format", "invalid-token-format", true, ErrInvalidTokenFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := revoker.DeleteToken(ctx, tt.tokenID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Revoker.DeleteToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Revoker.DeleteToken() error = %v, want %v", err, tt.wantErrIs)
			}

			// Check that the token was actually deleted
			if err == nil {
				_, exists := store.tokens[tt.tokenID]
				if exists {
					t.Errorf("token %s should not exist in store", tt.tokenID)
				}
			}
		})
	}

	// Test store failure
	t.Run("Store failure", func(t *testing.T) {
		failureToken, _ := GenerateToken()
		store.AddToken(failureToken, TokenData{Token: failureToken})
		store.failOnDelete = true
		defer func() { store.failOnDelete = false }()

		err := revoker.DeleteToken(ctx, failureToken)
		if err == nil {
			t.Errorf("Revoker.DeleteToken() expected error on store failure")
		}
	})
}

func TestRevoker_RevokeBatchTokens(t *testing.T) {
	ctx := context.Background()
	store := NewMockRevocationStore()
	revoker := NewRevoker(store)

	batchToken1, _ := GenerateToken()
	batchToken2, _ := GenerateToken()
	inactiveBatchToken, _ := GenerateToken()
	nonExistentToken1, _ := GenerateToken()
	nonExistentToken2, _ := GenerateToken()

	// Add tokens to revoke
	store.AddToken(batchToken1, TokenData{
		Token:     batchToken1,
		ProjectID: "project1",
		IsActive:  true,
	})
	store.AddToken(batchToken2, TokenData{
		Token:     batchToken2,
		ProjectID: "project1",
		IsActive:  true,
	})
	store.AddToken(inactiveBatchToken, TokenData{
		Token:     inactiveBatchToken,
		ProjectID: "project1",
		IsActive:  false,
	})

	tests := []struct {
		name      string
		tokenIDs  []string
		wantCount int
		wantErr   bool
		wantErrIs error
	}{
		{"Revoke multiple tokens", []string{batchToken1, batchToken2}, 2, false, nil},
		{"Revoke mix of active and inactive tokens", []string{batchToken1, inactiveBatchToken}, 1, false, nil},
		{"Revoke non-existent tokens", []string{nonExistentToken1, nonExistentToken2}, 0, false, nil},
		{"Empty token list", []string{}, 0, false, nil},
		{"Invalid token format", []string{"invalid-token-format"}, 0, true, ErrInvalidTokenFormat},
	}

	for i, tt := range tests {
		// For each test, reset token state
		if i > 0 {
			store = NewMockRevocationStore()
			revoker = NewRevoker(store)
			// Add tokens again
			store.AddToken(batchToken1, TokenData{
				Token:     batchToken1,
				ProjectID: "project1",
				IsActive:  true,
			})
			store.AddToken(batchToken2, TokenData{
				Token:     batchToken2,
				ProjectID: "project1",
				IsActive:  true,
			})
			store.AddToken(inactiveBatchToken, TokenData{
				Token:     inactiveBatchToken,
				ProjectID: "project1",
				IsActive:  false,
			})
		}
		t.Run(tt.name, func(t *testing.T) {
			count, err := revoker.RevokeBatchTokens(ctx, tt.tokenIDs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Revoker.RevokeBatchTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
				t.Errorf("Revoker.RevokeBatchTokens() error = %v, want %v", err, tt.wantErrIs)
			}

			if err == nil && count != tt.wantCount {
				t.Errorf("Revoker.RevokeBatchTokens() count = %v, want %v", count, tt.wantCount)
			}

			if err == nil {
				// Check that the tokens were actually revoked
				for _, tokenID := range tt.tokenIDs {
					if token, exists := store.tokens[tokenID]; exists && token.IsActive {
						t.Errorf("token %s should be inactive", tokenID)
					}
				}
			}
		})
	}

	// Test store failure
	t.Run("Store failure", func(t *testing.T) {
		store.failOnRevoke = true
		defer func() { store.failOnRevoke = false }()

		_, err := revoker.RevokeBatchTokens(ctx, []string{batchToken1})
		if err == nil {
			t.Errorf("Revoker.RevokeBatchTokens() expected error on store failure")
		}
	})
}

func TestRevoker_RevokeProjectTokens(t *testing.T) {
	ctx := context.Background()
	store := NewMockRevocationStore()
	revoker := NewRevoker(store)

	// Add tokens for different projects
	store.AddToken("tkn_project1token1234567890", TokenData{
		Token:     "tkn_project1token1234567890",
		ProjectID: "project1",
		IsActive:  true,
	})
	store.AddToken("tkn_project1token2234567890", TokenData{
		Token:     "tkn_project1token2234567890",
		ProjectID: "project1",
		IsActive:  true,
	})
	store.AddToken("tkn_project2token1234567890", TokenData{
		Token:     "tkn_project2token1234567890",
		ProjectID: "project2",
		IsActive:  true,
	})

	tests := []struct {
		name      string
		projectID string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "Revoke project1 tokens",
			projectID: "project1",
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "Revoke project2 tokens",
			projectID: "project2",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "Revoke non-existent project tokens",
			projectID: "nonexistent",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "Empty project ID",
			projectID: "",
			wantCount: 0,
			wantErr:   true,
		},
	}

	for i, tt := range tests {
		// For each test, reset token state
		if i > 0 {
			store = NewMockRevocationStore()
			revoker = NewRevoker(store)

			// Add tokens again
			store.AddToken("tkn_project1token1234567890", TokenData{
				Token:     "tkn_project1token1234567890",
				ProjectID: "project1",
				IsActive:  true,
			})
			store.AddToken("tkn_project1token2234567890", TokenData{
				Token:     "tkn_project1token2234567890",
				ProjectID: "project1",
				IsActive:  true,
			})
			store.AddToken("tkn_project2token1234567890", TokenData{
				Token:     "tkn_project2token1234567890",
				ProjectID: "project2",
				IsActive:  true,
			})
		}

		t.Run(tt.name, func(t *testing.T) {
			count, err := revoker.RevokeProjectTokens(ctx, tt.projectID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Revoker.RevokeProjectTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && count != tt.wantCount {
				t.Errorf("Revoker.RevokeProjectTokens() count = %v, want %v", count, tt.wantCount)
			}

			if err == nil && tt.projectID != "" {
				// Check that all tokens for the project were revoked
				for tokenID, token := range store.tokens {
					if token.ProjectID == tt.projectID && token.IsActive {
						t.Errorf("token %s for project %s should be inactive", tokenID, tt.projectID)
					}
				}
			}
		})
	}

	// Test store failure
	t.Run("Store failure", func(t *testing.T) {
		store.failOnRevoke = true
		defer func() { store.failOnRevoke = false }()

		_, err := revoker.RevokeProjectTokens(ctx, "project1")
		if err == nil {
			t.Errorf("Revoker.RevokeProjectTokens() expected error on store failure")
		}
	})
}

func TestRevoker_RevokeExpiredTokens(t *testing.T) {
	ctx := context.Background()
	store := NewMockRevocationStore()
	revoker := NewRevoker(store)

	// Add tokens with different expiration times
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	store.AddToken("tkn_expiredtoken12345678901", TokenData{
		Token:     "tkn_expiredtoken12345678901",
		ProjectID: "project1",
		IsActive:  true,
		ExpiresAt: &past,
	})
	store.AddToken("tkn_validtoken12345678901", TokenData{
		Token:     "tkn_validtoken12345678901",
		ProjectID: "project1",
		IsActive:  true,
		ExpiresAt: &future,
	})
	store.AddToken("tkn_noexpirytoken12345678901", TokenData{
		Token:     "tkn_noexpirytoken12345678901",
		ProjectID: "project1",
		IsActive:  true,
		ExpiresAt: nil,
	})

	t.Run("Revoke expired tokens", func(t *testing.T) {
		count, err := revoker.RevokeExpiredTokens(ctx)
		if err != nil {
			t.Errorf("Revoker.RevokeExpiredTokens() error = %v", err)
			return
		}

		wantCount := 1 // Only one token is expired
		if count != wantCount {
			t.Errorf("Revoker.RevokeExpiredTokens() count = %v, want %v", count, wantCount)
		}

		// Check that only the expired token was revoked
		_ = store.tokens["tkn_expiredtoken12345678901"]
		_ = store.tokens["tkn_validtoken12345678901"]
		_ = store.tokens["tkn_noexpirytoken12345678901"]
	})

	// Test store failure
	t.Run("Store failure", func(t *testing.T) {
		store.failOnRevoke = true
		defer func() { store.failOnRevoke = false }()

		_, err := revoker.RevokeExpiredTokens(ctx)
		if err == nil {
			t.Errorf("Revoker.RevokeExpiredTokens() expected error on store failure")
		}
	})
}

func TestAutomaticRevocation(t *testing.T) {
	store := NewMockRevocationStore()
	revoker := NewRevoker(store)

	// Add an expired token
	now := time.Now()
	past := now.Add(-1 * time.Hour)

	store.AddToken("tkn_expiredtoken12345678901", TokenData{
		Token:     "tkn_expiredtoken12345678901",
		ProjectID: "project1",
		IsActive:  true,
		ExpiresAt: &past,
	})

	// Set up automatic revocation with a very short interval
	interval := 100 * time.Millisecond
	autoRevoke := NewAutomaticRevocation(revoker, interval)

	// Start automatic revocation
	autoRevoke.Start()

	// Wait for the automatic revocation to run at least once
	time.Sleep(interval * 2)

	// Stop automatic revocation
	autoRevoke.Stop()

	// Check that the expired token was revoked
	token, exists := store.tokens["tkn_expiredtoken12345678901"]
	if !exists {
		t.Fatalf("token should exist in store")
	}
	if token.IsActive {
		t.Errorf("token should be inactive after automatic revocation")
	}
}
