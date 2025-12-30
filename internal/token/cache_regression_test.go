package token

import (
	"context"
	"testing"
	"time"
)

type tokenStringOnlyStore struct {
	data map[string]TokenData
}

func newTokenStringOnlyStore() *tokenStringOnlyStore {
	return &tokenStringOnlyStore{data: make(map[string]TokenData)}
}

func (s *tokenStringOnlyStore) GetTokenByID(ctx context.Context, id string) (TokenData, error) {
	return TokenData{}, ErrTokenNotFound
}

func (s *tokenStringOnlyStore) GetTokenByToken(ctx context.Context, tokenString string) (TokenData, error) {
	td, ok := s.data[tokenString]
	if !ok {
		return TokenData{}, ErrTokenNotFound
	}
	return td, nil
}

func (s *tokenStringOnlyStore) IncrementTokenUsage(ctx context.Context, tokenString string) error {
	return nil
}

func (s *tokenStringOnlyStore) CreateToken(ctx context.Context, token TokenData) error {
	return nil
}

func (s *tokenStringOnlyStore) UpdateToken(ctx context.Context, token TokenData) error {
	return nil
}

func (s *tokenStringOnlyStore) ListTokens(ctx context.Context) ([]TokenData, error) {
	return nil, nil
}

func (s *tokenStringOnlyStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}

func TestCachedValidator_PopulatesCacheViaGetTokenByToken(t *testing.T) {
	ctx := context.Background()
	store := newTokenStringOnlyStore()
	validator := NewValidator(store)

	cv := NewCachedValidator(validator, CacheOptions{TTL: 1 * time.Minute, MaxSize: 10, EnableCleanup: false})

	now := time.Now()
	future := now.Add(1 * time.Hour)

	tok, _ := GenerateToken()
	store.data[tok] = TokenData{Token: tok, ProjectID: "p1", ExpiresAt: &future, IsActive: true, CreatedAt: now}

	_, _ = cv.ValidateToken(ctx, tok)
	_, _ = cv.ValidateToken(ctx, tok)

	hits, misses, _, _ := cv.GetCacheStats()
	if hits != 1 || misses != 1 {
		t.Fatalf("unexpected cache stats: hits=%d misses=%d", hits, misses)
	}
}
