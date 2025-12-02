package encryption

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
)

// mockTokenStore is a mock implementation of token.TokenStore for testing.
type mockTokenStore struct {
	mu              sync.RWMutex
	tokens          map[string]token.TokenData
	getByIDError    error
	incrementError  error
	createError     error
	updateError     error
	listError       error
	getByProjectErr error
	incrementCalls  []string
	createCalls     []token.TokenData
	updateCalls     []token.TokenData
}

func newMockTokenStore() *mockTokenStore {
	return &mockTokenStore{
		tokens:         make(map[string]token.TokenData),
		incrementCalls: []string{},
		createCalls:    []token.TokenData{},
		updateCalls:    []token.TokenData{},
	}
}

func (m *mockTokenStore) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getByIDError != nil {
		return token.TokenData{}, m.getByIDError
	}
	td, ok := m.tokens[tokenID]
	if !ok {
		return token.TokenData{}, errors.New("token not found")
	}
	return td, nil
}

func (m *mockTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.incrementError != nil {
		return m.incrementError
	}
	m.incrementCalls = append(m.incrementCalls, tokenID)
	if td, ok := m.tokens[tokenID]; ok {
		td.RequestCount++
		m.tokens[tokenID] = td
	}
	return nil
}

func (m *mockTokenStore) CreateToken(ctx context.Context, td token.TokenData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createError != nil {
		return m.createError
	}
	m.createCalls = append(m.createCalls, td)
	m.tokens[td.Token] = td
	return nil
}

func (m *mockTokenStore) UpdateToken(ctx context.Context, td token.TokenData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateError != nil {
		return m.updateError
	}
	m.updateCalls = append(m.updateCalls, td)
	if _, ok := m.tokens[td.Token]; !ok {
		return errors.New("token not found")
	}
	m.tokens[td.Token] = td
	return nil
}

func (m *mockTokenStore) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listError != nil {
		return nil, m.listError
	}
	result := make([]token.TokenData, 0, len(m.tokens))
	for _, td := range m.tokens {
		result = append(result, td)
	}
	return result, nil
}

func (m *mockTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getByProjectErr != nil {
		return nil, m.getByProjectErr
	}
	result := make([]token.TokenData, 0)
	for _, td := range m.tokens {
		if td.ProjectID == projectID {
			result = append(result, td)
		}
	}
	return result, nil
}

// mockRevocationStore is a mock implementation of token.RevocationStore.
type mockRevocationStore struct {
	mu                 sync.RWMutex
	revokedTokens      map[string]bool
	deletedTokens      map[string]bool
	revokeError        error
	deleteError        error
	revokeBatchError   error
	revokeProjectError error
	revokeExpiredError error
	revokeCalls        []string
	deleteCalls        []string
	batchRevokeCalls   [][]string
	projectRevokeCalls []string
}

func newMockRevocationStore() *mockRevocationStore {
	return &mockRevocationStore{
		revokedTokens:      make(map[string]bool),
		deletedTokens:      make(map[string]bool),
		revokeCalls:        []string{},
		deleteCalls:        []string{},
		batchRevokeCalls:   [][]string{},
		projectRevokeCalls: []string{},
	}
}

func (m *mockRevocationStore) RevokeToken(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revokeError != nil {
		return m.revokeError
	}
	m.revokeCalls = append(m.revokeCalls, tokenID)
	m.revokedTokens[tokenID] = true
	return nil
}

func (m *mockRevocationStore) DeleteToken(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.deleteError != nil {
		return m.deleteError
	}
	m.deleteCalls = append(m.deleteCalls, tokenID)
	m.deletedTokens[tokenID] = true
	return nil
}

func (m *mockRevocationStore) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revokeBatchError != nil {
		return 0, m.revokeBatchError
	}
	m.batchRevokeCalls = append(m.batchRevokeCalls, tokenIDs)
	for _, id := range tokenIDs {
		m.revokedTokens[id] = true
	}
	return len(tokenIDs), nil
}

func (m *mockRevocationStore) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revokeProjectError != nil {
		return 0, m.revokeProjectError
	}
	m.projectRevokeCalls = append(m.projectRevokeCalls, projectID)
	return 5, nil // Return some count for testing
}

func (m *mockRevocationStore) RevokeExpiredTokens(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.revokeExpiredError != nil {
		return 0, m.revokeExpiredError
	}
	return 3, nil // Return some count for testing
}

// mockRateLimitStore is a mock implementation of token.RateLimitStore.
type mockRateLimitStore struct {
	mu               sync.RWMutex
	tokens           map[string]token.TokenData
	getByIDError     error
	incrementError   error
	resetError       error
	updateLimitError error
	incrementCalls   []string
	resetCalls       []string
	updateLimitCalls []struct {
		tokenID     string
		maxRequests *int
	}
}

func newMockRateLimitStore() *mockRateLimitStore {
	return &mockRateLimitStore{
		tokens:         make(map[string]token.TokenData),
		incrementCalls: []string{},
		resetCalls:     []string{},
		updateLimitCalls: []struct {
			tokenID     string
			maxRequests *int
		}{},
	}
}

func (m *mockRateLimitStore) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.getByIDError != nil {
		return token.TokenData{}, m.getByIDError
	}
	td, ok := m.tokens[tokenID]
	if !ok {
		return token.TokenData{}, errors.New("token not found")
	}
	return td, nil
}

func (m *mockRateLimitStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.incrementError != nil {
		return m.incrementError
	}
	m.incrementCalls = append(m.incrementCalls, tokenID)
	if td, ok := m.tokens[tokenID]; ok {
		td.RequestCount++
		m.tokens[tokenID] = td
	}
	return nil
}

func (m *mockRateLimitStore) ResetTokenUsage(ctx context.Context, tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resetError != nil {
		return m.resetError
	}
	m.resetCalls = append(m.resetCalls, tokenID)
	if td, ok := m.tokens[tokenID]; ok {
		td.RequestCount = 0
		m.tokens[tokenID] = td
	}
	return nil
}

func (m *mockRateLimitStore) UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updateLimitError != nil {
		return m.updateLimitError
	}
	m.updateLimitCalls = append(m.updateLimitCalls, struct {
		tokenID     string
		maxRequests *int
	}{tokenID, maxRequests})
	if td, ok := m.tokens[tokenID]; ok {
		td.MaxRequests = maxRequests
		m.tokens[tokenID] = td
	}
	return nil
}

// ========== SecureTokenStore Tests ==========

func TestNewSecureTokenStore(t *testing.T) {
	mock := newMockTokenStore()

	t.Run("with hasher", func(t *testing.T) {
		hasher := NewTokenHasher()
		store := NewSecureTokenStore(mock, hasher)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})

	t.Run("nil hasher uses NullTokenHasher", func(t *testing.T) {
		store := NewSecureTokenStore(mock, nil)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})
}

func TestSecureTokenStore_CreateAndGetToken(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token-12345"
	td := token.TokenData{
		Token:     originalToken,
		ProjectID: "proj-1",
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	// Create token
	if err := store.CreateToken(ctx, td); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Verify the token was hashed before storage
	if len(mock.createCalls) != 1 {
		t.Fatalf("expected 1 create call, got %d", len(mock.createCalls))
	}

	createdToken := mock.createCalls[0]
	if createdToken.Token == originalToken {
		t.Error("token should be hashed in storage")
	}

	// SHA-256 produces 64 hex characters
	if len(createdToken.Token) != 64 {
		t.Errorf("hashed token should be 64 chars, got %d", len(createdToken.Token))
	}

	// Get token using original plaintext token
	retrieved, err := store.GetTokenByID(ctx, originalToken)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	// The retrieved token will have the hashed value
	if retrieved.Token != createdToken.Token {
		t.Errorf("retrieved token should have hashed value")
	}
	if retrieved.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %q, want %q", retrieved.ProjectID, "proj-1")
	}
}

func TestSecureTokenStore_IncrementTokenUsage(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	// Add token directly to mock with hashed value
	mock.tokens[hashedToken] = token.TokenData{
		Token:        hashedToken,
		ProjectID:    "proj-1",
		IsActive:     true,
		RequestCount: 0,
	}

	// Increment using original token
	if err := store.IncrementTokenUsage(ctx, originalToken); err != nil {
		t.Fatalf("increment failed: %v", err)
	}

	// Verify the hashed token was used
	if len(mock.incrementCalls) != 1 {
		t.Fatalf("expected 1 increment call, got %d", len(mock.incrementCalls))
	}
	if mock.incrementCalls[0] != hashedToken {
		t.Errorf("increment should use hashed token")
	}
}

func TestSecureTokenStore_UpdateToken(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	// Add token directly to mock
	mock.tokens[hashedToken] = token.TokenData{
		Token:     hashedToken,
		ProjectID: "proj-1",
		IsActive:  true,
	}

	// Update with plaintext token (should be hashed)
	td := token.TokenData{
		Token:     originalToken,
		ProjectID: "proj-1",
		IsActive:  false,
	}
	if err := store.UpdateToken(ctx, td); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify the token was hashed
	if len(mock.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(mock.updateCalls))
	}
	if mock.updateCalls[0].Token != hashedToken {
		t.Errorf("update should use hashed token")
	}
}

func TestSecureTokenStore_UpdateAlreadyHashed(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	// Add token directly to mock
	mock.tokens[hashedToken] = token.TokenData{
		Token:     hashedToken,
		ProjectID: "proj-1",
		IsActive:  true,
	}

	// Update with already hashed token (should NOT double-hash)
	td := token.TokenData{
		Token:     hashedToken, // Already hashed (64 char hex)
		ProjectID: "proj-1",
		IsActive:  false,
	}
	if err := store.UpdateToken(ctx, td); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	// Verify the token was NOT double-hashed
	if mock.updateCalls[0].Token != hashedToken {
		t.Errorf("already hashed token should not be re-hashed")
	}
}

func TestSecureTokenStore_ListTokens(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	// Add some tokens
	mock.tokens["hash1"] = token.TokenData{Token: "hash1", ProjectID: "proj-1"}
	mock.tokens["hash2"] = token.TokenData{Token: "hash2", ProjectID: "proj-2"}

	tokens, err := store.ListTokens(ctx)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(tokens))
	}
}

func TestSecureTokenStore_GetTokensByProjectID(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	// Add some tokens
	mock.tokens["hash1"] = token.TokenData{Token: "hash1", ProjectID: "proj-1"}
	mock.tokens["hash2"] = token.TokenData{Token: "hash2", ProjectID: "proj-1"}
	mock.tokens["hash3"] = token.TokenData{Token: "hash3", ProjectID: "proj-2"}

	tokens, err := store.GetTokensByProjectID(ctx, "proj-1")
	if err != nil {
		t.Fatalf("get by project failed: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens for proj-1, got %d", len(tokens))
	}
}

func TestSecureTokenStore_ErrorHandling(t *testing.T) {
	hasher := NewTokenHasher()
	ctx := context.Background()

	t.Run("GetTokenByID error", func(t *testing.T) {
		mock := newMockTokenStore()
		mock.getByIDError = errors.New("db error")
		store := NewSecureTokenStore(mock, hasher)

		_, err := store.GetTokenByID(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("IncrementTokenUsage error", func(t *testing.T) {
		mock := newMockTokenStore()
		mock.incrementError = errors.New("db error")
		store := NewSecureTokenStore(mock, hasher)

		err := store.IncrementTokenUsage(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("CreateToken error", func(t *testing.T) {
		mock := newMockTokenStore()
		mock.createError = errors.New("db error")
		store := NewSecureTokenStore(mock, hasher)

		err := store.CreateToken(ctx, token.TokenData{Token: "test"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("UpdateToken error", func(t *testing.T) {
		mock := newMockTokenStore()
		mock.updateError = errors.New("db error")
		mock.tokens["hash"] = token.TokenData{Token: "hash"}
		store := NewSecureTokenStore(mock, hasher)

		err := store.UpdateToken(ctx, token.TokenData{Token: "hash"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("ListTokens error", func(t *testing.T) {
		mock := newMockTokenStore()
		mock.listError = errors.New("db error")
		store := NewSecureTokenStore(mock, hasher)

		_, err := store.ListTokens(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("GetTokensByProjectID error", func(t *testing.T) {
		mock := newMockTokenStore()
		mock.getByProjectErr = errors.New("db error")
		store := NewSecureTokenStore(mock, hasher)

		_, err := store.GetTokensByProjectID(ctx, "proj-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSecureTokenStore_NullHasher(t *testing.T) {
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, nil) // Uses NullTokenHasher
	ctx := context.Background()

	originalToken := "sk-test-token"
	td := token.TokenData{
		Token:     originalToken,
		ProjectID: "proj-1",
		IsActive:  true,
	}

	// Create token
	if err := store.CreateToken(ctx, td); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// With NullTokenHasher, token should remain unchanged
	if mock.createCalls[0].Token != originalToken {
		t.Error("NullTokenHasher should not modify token")
	}
}

// ========== SecureRevocationStore Tests ==========

func TestNewSecureRevocationStore(t *testing.T) {
	mock := newMockRevocationStore()

	t.Run("with hasher", func(t *testing.T) {
		hasher := NewTokenHasher()
		store := NewSecureRevocationStore(mock, hasher)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})

	t.Run("nil hasher uses NullTokenHasher", func(t *testing.T) {
		store := NewSecureRevocationStore(mock, nil)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})
}

func TestSecureRevocationStore_RevokeToken(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRevocationStore()
	store := NewSecureRevocationStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	if err := store.RevokeToken(ctx, originalToken); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}

	// Verify hashed token was used
	if len(mock.revokeCalls) != 1 {
		t.Fatalf("expected 1 revoke call, got %d", len(mock.revokeCalls))
	}
	if mock.revokeCalls[0] != hashedToken {
		t.Errorf("revoke should use hashed token")
	}
	if !mock.revokedTokens[hashedToken] {
		t.Error("hashed token should be marked as revoked")
	}
}

func TestSecureRevocationStore_DeleteToken(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRevocationStore()
	store := NewSecureRevocationStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	if err := store.DeleteToken(ctx, originalToken); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify hashed token was used
	if len(mock.deleteCalls) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(mock.deleteCalls))
	}
	if mock.deleteCalls[0] != hashedToken {
		t.Errorf("delete should use hashed token")
	}
}

func TestSecureRevocationStore_RevokeBatchTokens(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRevocationStore()
	store := NewSecureRevocationStore(mock, hasher)
	ctx := context.Background()

	tokens := []string{"token-1", "token-2", "token-3"}
	hashedTokens := make([]string, len(tokens))
	for i, tok := range tokens {
		hashedTokens[i] = hasher.CreateLookupKey(tok)
	}

	count, err := store.RevokeBatchTokens(ctx, tokens)
	if err != nil {
		t.Fatalf("batch revoke failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}

	// Verify all tokens were hashed
	if len(mock.batchRevokeCalls) != 1 {
		t.Fatalf("expected 1 batch call, got %d", len(mock.batchRevokeCalls))
	}
	for i, hashed := range mock.batchRevokeCalls[0] {
		if hashed != hashedTokens[i] {
			t.Errorf("token %d should be hashed", i)
		}
	}
}

func TestSecureRevocationStore_RevokeProjectTokens(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRevocationStore()
	store := NewSecureRevocationStore(mock, hasher)
	ctx := context.Background()

	count, err := store.RevokeProjectTokens(ctx, "proj-1")
	if err != nil {
		t.Fatalf("project revoke failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count 5, got %d", count)
	}

	// Project ID should NOT be hashed (it's not a token)
	if len(mock.projectRevokeCalls) != 1 {
		t.Fatalf("expected 1 project call, got %d", len(mock.projectRevokeCalls))
	}
	if mock.projectRevokeCalls[0] != "proj-1" {
		t.Errorf("project ID should be passed as-is")
	}
}

func TestSecureRevocationStore_RevokeExpiredTokens(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRevocationStore()
	store := NewSecureRevocationStore(mock, hasher)
	ctx := context.Background()

	count, err := store.RevokeExpiredTokens(ctx)
	if err != nil {
		t.Fatalf("expired revoke failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}
}

func TestSecureRevocationStore_ErrorHandling(t *testing.T) {
	hasher := NewTokenHasher()
	ctx := context.Background()

	t.Run("RevokeToken error", func(t *testing.T) {
		mock := newMockRevocationStore()
		mock.revokeError = errors.New("db error")
		store := NewSecureRevocationStore(mock, hasher)

		err := store.RevokeToken(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("DeleteToken error", func(t *testing.T) {
		mock := newMockRevocationStore()
		mock.deleteError = errors.New("db error")
		store := NewSecureRevocationStore(mock, hasher)

		err := store.DeleteToken(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("RevokeBatchTokens error", func(t *testing.T) {
		mock := newMockRevocationStore()
		mock.revokeBatchError = errors.New("db error")
		store := NewSecureRevocationStore(mock, hasher)

		_, err := store.RevokeBatchTokens(ctx, []string{"token"})
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("RevokeProjectTokens error", func(t *testing.T) {
		mock := newMockRevocationStore()
		mock.revokeProjectError = errors.New("db error")
		store := NewSecureRevocationStore(mock, hasher)

		_, err := store.RevokeProjectTokens(ctx, "proj-1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("RevokeExpiredTokens error", func(t *testing.T) {
		mock := newMockRevocationStore()
		mock.revokeExpiredError = errors.New("db error")
		store := NewSecureRevocationStore(mock, hasher)

		_, err := store.RevokeExpiredTokens(ctx)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// ========== SecureRateLimitStore Tests ==========

func TestNewSecureRateLimitStore(t *testing.T) {
	mock := newMockRateLimitStore()

	t.Run("with hasher", func(t *testing.T) {
		hasher := NewTokenHasher()
		store := NewSecureRateLimitStore(mock, hasher)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})

	t.Run("nil hasher uses NullTokenHasher", func(t *testing.T) {
		store := NewSecureRateLimitStore(mock, nil)
		if store == nil {
			t.Error("expected store, got nil")
		}
	})
}

func TestSecureRateLimitStore_GetTokenByID(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRateLimitStore()
	store := NewSecureRateLimitStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	// Add token with hashed value
	mock.tokens[hashedToken] = token.TokenData{
		Token:        hashedToken,
		ProjectID:    "proj-1",
		RequestCount: 10,
	}

	td, err := store.GetTokenByID(ctx, originalToken)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if td.Token != hashedToken {
		t.Error("should retrieve token by hashed lookup")
	}
	if td.RequestCount != 10 {
		t.Errorf("RequestCount = %d, want 10", td.RequestCount)
	}
}

func TestSecureRateLimitStore_IncrementTokenUsage(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRateLimitStore()
	store := NewSecureRateLimitStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	mock.tokens[hashedToken] = token.TokenData{Token: hashedToken}

	if err := store.IncrementTokenUsage(ctx, originalToken); err != nil {
		t.Fatalf("increment failed: %v", err)
	}

	if len(mock.incrementCalls) != 1 {
		t.Fatalf("expected 1 increment call, got %d", len(mock.incrementCalls))
	}
	if mock.incrementCalls[0] != hashedToken {
		t.Error("increment should use hashed token")
	}
}

func TestSecureRateLimitStore_ResetTokenUsage(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRateLimitStore()
	store := NewSecureRateLimitStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	mock.tokens[hashedToken] = token.TokenData{Token: hashedToken, RequestCount: 100}

	if err := store.ResetTokenUsage(ctx, originalToken); err != nil {
		t.Fatalf("reset failed: %v", err)
	}

	if len(mock.resetCalls) != 1 {
		t.Fatalf("expected 1 reset call, got %d", len(mock.resetCalls))
	}
	if mock.resetCalls[0] != hashedToken {
		t.Error("reset should use hashed token")
	}
}

func TestSecureRateLimitStore_UpdateTokenLimit(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRateLimitStore()
	store := NewSecureRateLimitStore(mock, hasher)
	ctx := context.Background()

	originalToken := "sk-test-token"
	hashedToken := hasher.CreateLookupKey(originalToken)

	mock.tokens[hashedToken] = token.TokenData{Token: hashedToken}

	maxRequests := 1000
	if err := store.UpdateTokenLimit(ctx, originalToken, &maxRequests); err != nil {
		t.Fatalf("update limit failed: %v", err)
	}

	if len(mock.updateLimitCalls) != 1 {
		t.Fatalf("expected 1 update limit call, got %d", len(mock.updateLimitCalls))
	}
	if mock.updateLimitCalls[0].tokenID != hashedToken {
		t.Error("update limit should use hashed token")
	}
	if *mock.updateLimitCalls[0].maxRequests != 1000 {
		t.Errorf("maxRequests = %d, want 1000", *mock.updateLimitCalls[0].maxRequests)
	}
}

func TestSecureRateLimitStore_ErrorHandling(t *testing.T) {
	hasher := NewTokenHasher()
	ctx := context.Background()

	t.Run("GetTokenByID error", func(t *testing.T) {
		mock := newMockRateLimitStore()
		mock.getByIDError = errors.New("db error")
		store := NewSecureRateLimitStore(mock, hasher)

		_, err := store.GetTokenByID(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("IncrementTokenUsage error", func(t *testing.T) {
		mock := newMockRateLimitStore()
		mock.incrementError = errors.New("db error")
		store := NewSecureRateLimitStore(mock, hasher)

		err := store.IncrementTokenUsage(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("ResetTokenUsage error", func(t *testing.T) {
		mock := newMockRateLimitStore()
		mock.resetError = errors.New("db error")
		store := NewSecureRateLimitStore(mock, hasher)

		err := store.ResetTokenUsage(ctx, "token")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("UpdateTokenLimit error", func(t *testing.T) {
		mock := newMockRateLimitStore()
		mock.updateLimitError = errors.New("db error")
		store := NewSecureRateLimitStore(mock, hasher)

		maxReqs := 100
		err := store.UpdateTokenLimit(ctx, "token", &maxReqs)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestSecureRateLimitStore_NullHasher(t *testing.T) {
	mock := newMockRateLimitStore()
	store := NewSecureRateLimitStore(mock, nil) // Uses NullTokenHasher
	ctx := context.Background()

	originalToken := "sk-test-token"
	mock.tokens[originalToken] = token.TokenData{Token: originalToken}

	// With NullTokenHasher, should use original token as lookup key
	if err := store.IncrementTokenUsage(ctx, originalToken); err != nil {
		t.Fatalf("increment failed: %v", err)
	}

	if mock.incrementCalls[0] != originalToken {
		t.Error("NullTokenHasher should not modify token")
	}
}

// ========== Concurrency Tests ==========

func TestSecureTokenStore_Concurrency(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockTokenStore()
	store := NewSecureTokenStore(mock, hasher)
	ctx := context.Background()

	// Pre-populate with some tokens
	for i := 0; i < 10; i++ {
		tok := hasher.CreateLookupKey(fmt.Sprintf("token-%d", i))
		mock.tokens[tok] = token.TokenData{Token: tok, ProjectID: "proj-1"}
	}

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			tokenID := fmt.Sprintf("token-%d", id%10)

			// Mix of operations
			_, _ = store.GetTokenByID(ctx, tokenID)
			_ = store.IncrementTokenUsage(ctx, tokenID)
			_, _ = store.ListTokens(ctx)

			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestSecureRevocationStore_Concurrency(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRevocationStore()
	store := NewSecureRevocationStore(mock, hasher)
	ctx := context.Background()

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			tokenID := fmt.Sprintf("token-%d", id%10)
			_ = store.RevokeToken(ctx, tokenID)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestSecureRateLimitStore_Concurrency(t *testing.T) {
	hasher := NewTokenHasher()
	mock := newMockRateLimitStore()
	store := NewSecureRateLimitStore(mock, hasher)
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 10; i++ {
		tok := hasher.CreateLookupKey(fmt.Sprintf("token-%d", i))
		mock.tokens[tok] = token.TokenData{Token: tok}
	}

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func(id int) {
			tokenID := fmt.Sprintf("token-%d", id%10)
			_ = store.IncrementTokenUsage(ctx, tokenID)
			_ = store.ResetTokenUsage(ctx, tokenID)
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}
