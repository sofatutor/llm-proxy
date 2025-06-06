package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
)

// MockTokenStore is an in-memory implementation of TokenStore for testing and development
type MockTokenStore struct {
	tokens     map[string]Token
	mutex      sync.RWMutex
	projectIDs map[string]string // token -> projectID mapping for quick lookup
}

// NewMockTokenStore creates a new MockTokenStore
func NewMockTokenStore() *MockTokenStore {
	return &MockTokenStore{
		tokens:     make(map[string]Token),
		projectIDs: make(map[string]string),
	}
}

// CreateToken creates a new token in the store
func (m *MockTokenStore) CreateToken(ctx context.Context, token Token) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.tokens[token.Token]; exists {
		return ErrTokenExists
	}

	m.tokens[token.Token] = token
	m.projectIDs[token.Token] = token.ProjectID
	return nil
}

// GetTokenByID retrieves a token by ID
func (m *MockTokenStore) GetTokenByID(ctx context.Context, tokenID string) (Token, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	token, exists := m.tokens[tokenID]
	if !exists {
		return Token{}, ErrTokenNotFound
	}

	return token, nil
}

// UpdateToken updates a token in the store
func (m *MockTokenStore) UpdateToken(ctx context.Context, token Token) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.tokens[token.Token]; !exists {
		return ErrTokenNotFound
	}

	m.tokens[token.Token] = token
	m.projectIDs[token.Token] = token.ProjectID
	return nil
}

// DeleteToken deletes a token from the store
func (m *MockTokenStore) DeleteToken(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.tokens[tokenID]; !exists {
		return ErrTokenNotFound
	}

	delete(m.tokens, tokenID)
	delete(m.projectIDs, tokenID)
	return nil
}

// ListTokens retrieves all tokens from the store
func (m *MockTokenStore) ListTokens(ctx context.Context) ([]Token, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	tokens := make([]Token, 0, len(m.tokens))
	for _, t := range m.tokens {
		tokens = append(tokens, t)
	}
	return tokens, nil
}

// GetTokensByProjectID retrieves all tokens for a project
func (m *MockTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]Token, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var tokens []Token
	for _, t := range m.tokens {
		if t.ProjectID == projectID {
			tokens = append(tokens, t)
		}
	}
	return tokens, nil
}

// IncrementTokenUsage increments the request count and updates the last_used_at timestamp
func (m *MockTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	t, exists := m.tokens[tokenID]
	if !exists {
		return ErrTokenNotFound
	}

	t.RequestCount++
	now := time.Now()
	t.LastUsedAt = &now
	m.tokens[tokenID] = t
	return nil
}

// CleanExpiredTokens deletes expired tokens from the store
func (m *MockTokenStore) CleanExpiredTokens(ctx context.Context) (int64, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	now := time.Now()
	var count int64
	for id, t := range m.tokens {
		if t.ExpiresAt != nil && t.ExpiresAt.Before(now) {
			delete(m.tokens, id)
			delete(m.projectIDs, id)
			count++
		}
	}
	return count, nil
}

// CreateMockToken creates a new token in the mock store with the given parameters
func (m *MockTokenStore) CreateMockToken(tokenID, projectID string, expiresIn time.Duration, isActive bool, maxRequests *int) (Token, error) {
	if tokenID == "" {
		return Token{}, errors.New("token ID cannot be empty")
	}
	if projectID == "" {
		return Token{}, errors.New("project ID cannot be empty")
	}

	var expiresAt *time.Time
	if expiresIn > 0 {
		expiry := time.Now().Add(expiresIn)
		expiresAt = &expiry
	}

	now := time.Now()
	token := Token{
		Token:        tokenID,
		ProjectID:    projectID,
		ExpiresAt:    expiresAt,
		IsActive:     isActive,
		RequestCount: 0,
		MaxRequests:  maxRequests,
		CreatedAt:    now,
	}

	err := m.CreateToken(context.Background(), token)
	return token, err
}

// TokenStoreAdapter adapts the database.DB to the token.TokenStore interface
type TokenStoreAdapter struct {
	store *MockTokenStore
}

// NewTokenStoreAdapter creates a new TokenStoreAdapter
func NewTokenStoreAdapter(store *MockTokenStore) *TokenStoreAdapter {
	return &TokenStoreAdapter{
		store: store,
	}
}

// GetTokenByID retrieves a token by ID
func (a *TokenStoreAdapter) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	t, err := a.store.GetTokenByID(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return token.TokenData{}, token.ErrTokenNotFound
		}
		return token.TokenData{}, err
	}
	return ExportTokenData(t), nil
}

// IncrementTokenUsage increments the request count and updates the last_used_at timestamp
func (a *TokenStoreAdapter) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	err := a.store.IncrementTokenUsage(ctx, tokenID)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return token.ErrTokenNotFound
		}
		return err
	}
	return nil
}

// CreateToken creates a new token in the store
func (a *TokenStoreAdapter) CreateToken(ctx context.Context, td token.TokenData) error {
	dbToken := ImportTokenData(td)
	return a.store.CreateToken(ctx, dbToken)
}

// ListTokens retrieves all tokens from the store
func (a *TokenStoreAdapter) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	dbTokens, err := a.store.ListTokens(ctx)
	if err != nil {
		return nil, err
	}
	tokens := make([]token.TokenData, len(dbTokens))
	for i, t := range dbTokens {
		tokens[i] = ExportTokenData(t)
	}
	return tokens, nil
}

// GetTokensByProjectID retrieves all tokens for a project
func (a *TokenStoreAdapter) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	dbTokens, err := a.store.GetTokensByProjectID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	tokens := make([]token.TokenData, len(dbTokens))
	for i, t := range dbTokens {
		tokens[i] = ExportTokenData(t)
	}
	return tokens, nil
}

func TestMockTokenStore_EdgeCases(t *testing.T) {
	store := NewMockTokenStore()
	ctx := context.Background()

	t.Run("GetTokenByID not found", func(t *testing.T) {
		_, err := store.GetTokenByID(ctx, "notfound")
		if err == nil {
			t.Error("expected error for notfound token")
		}
	})

	t.Run("IncrementTokenUsage not found", func(t *testing.T) {
		err := store.IncrementTokenUsage(ctx, "notfound")
		if err == nil {
			t.Error("expected error for notfound token")
		}
	})

	t.Run("ListTokens empty", func(t *testing.T) {
		ts, err := store.ListTokens(ctx)
		if err != nil {
			t.Error(err)
		}
		if len(ts) != 0 {
			t.Errorf("expected 0 tokens, got %d", len(ts))
		}
	})

	t.Run("GetTokensByProjectID empty", func(t *testing.T) {
		ts, err := store.GetTokensByProjectID(ctx, "pid")
		if err != nil {
			t.Error(err)
		}
		if len(ts) != 0 {
			t.Errorf("expected 0 tokens, got %d", len(ts))
		}
	})
}
