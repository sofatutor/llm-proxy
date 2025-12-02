// Package encryption provides a secure token store wrapper that hashes tokens.
package encryption

import (
	"context"

	"github.com/sofatutor/llm-proxy/internal/token"
)

// SecureTokenStore wraps a TokenStore and hashes tokens before storage.
// This prevents tokens from being exposed if the database is compromised.
type SecureTokenStore struct {
	store  token.TokenStore
	hasher TokenHasherInterface
}

// NewSecureTokenStore creates a new SecureTokenStore.
// If hasher is nil, a NullTokenHasher is used (no hashing).
func NewSecureTokenStore(store token.TokenStore, hasher TokenHasherInterface) *SecureTokenStore {
	if hasher == nil {
		hasher = NewNullTokenHasher()
	}
	return &SecureTokenStore{
		store:  store,
		hasher: hasher,
	}
}

// GetTokenByID retrieves a token by its ID (the original plaintext token).
// The token is hashed before lookup, and the returned TokenData will
// have the hashed token value (not the original).
func (s *SecureTokenStore) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.GetTokenByID(ctx, hashedToken)
}

// IncrementTokenUsage increments the usage count for a token.
// The token is hashed before the operation.
func (s *SecureTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.IncrementTokenUsage(ctx, hashedToken)
}

// CreateToken creates a new token in the store.
// The token value is hashed before storage.
func (s *SecureTokenStore) CreateToken(ctx context.Context, td token.TokenData) error {
	// Hash the token value
	td.Token = s.hasher.CreateLookupKey(td.Token)
	return s.store.CreateToken(ctx, td)
}

// UpdateToken updates an existing token.
// The token value is hashed before the operation.
func (s *SecureTokenStore) UpdateToken(ctx context.Context, td token.TokenData) error {
	// Hash the token value if it's not already hashed
	if !IsHashed(td.Token) && len(td.Token) != 64 {
		td.Token = s.hasher.CreateLookupKey(td.Token)
	}
	return s.store.UpdateToken(ctx, td)
}

// ListTokens retrieves all tokens from the store.
// Note: The returned tokens will have hashed token values.
func (s *SecureTokenStore) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	return s.store.ListTokens(ctx)
}

// GetTokensByProjectID retrieves all tokens for a project.
// Note: The returned tokens will have hashed token values.
func (s *SecureTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	return s.store.GetTokensByProjectID(ctx, projectID)
}

// Compile-time interface check
var _ token.TokenStore = (*SecureTokenStore)(nil)

// SecureRevocationStore wraps a RevocationStore and hashes tokens before operations.
type SecureRevocationStore struct {
	store  token.RevocationStore
	hasher TokenHasherInterface
}

// NewSecureRevocationStore creates a new SecureRevocationStore.
func NewSecureRevocationStore(store token.RevocationStore, hasher TokenHasherInterface) *SecureRevocationStore {
	if hasher == nil {
		hasher = NewNullTokenHasher()
	}
	return &SecureRevocationStore{
		store:  store,
		hasher: hasher,
	}
}

// RevokeToken revokes a token by its ID.
func (s *SecureRevocationStore) RevokeToken(ctx context.Context, tokenID string) error {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.RevokeToken(ctx, hashedToken)
}

// DeleteToken deletes a token by its ID.
func (s *SecureRevocationStore) DeleteToken(ctx context.Context, tokenID string) error {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.DeleteToken(ctx, hashedToken)
}

// RevokeBatchTokens revokes multiple tokens at once.
func (s *SecureRevocationStore) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	hashedTokens := make([]string, len(tokenIDs))
	for i, t := range tokenIDs {
		hashedTokens[i] = s.hasher.CreateLookupKey(t)
	}
	return s.store.RevokeBatchTokens(ctx, hashedTokens)
}

// RevokeProjectTokens revokes all tokens for a project.
func (s *SecureRevocationStore) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	return s.store.RevokeProjectTokens(ctx, projectID)
}

// RevokeExpiredTokens revokes all expired tokens.
func (s *SecureRevocationStore) RevokeExpiredTokens(ctx context.Context) (int, error) {
	return s.store.RevokeExpiredTokens(ctx)
}

// Compile-time interface check
var _ token.RevocationStore = (*SecureRevocationStore)(nil)

// SecureRateLimitStore wraps a RateLimitStore and hashes tokens before operations.
type SecureRateLimitStore struct {
	store  token.RateLimitStore
	hasher TokenHasherInterface
}

// NewSecureRateLimitStore creates a new SecureRateLimitStore.
func NewSecureRateLimitStore(store token.RateLimitStore, hasher TokenHasherInterface) *SecureRateLimitStore {
	if hasher == nil {
		hasher = NewNullTokenHasher()
	}
	return &SecureRateLimitStore{
		store:  store,
		hasher: hasher,
	}
}

// GetTokenByID retrieves a token by its ID.
func (s *SecureRateLimitStore) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.GetTokenByID(ctx, hashedToken)
}

// IncrementTokenUsage increments the usage count for a token.
func (s *SecureRateLimitStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.IncrementTokenUsage(ctx, hashedToken)
}

// ResetTokenUsage resets the usage count for a token to zero.
func (s *SecureRateLimitStore) ResetTokenUsage(ctx context.Context, tokenID string) error {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.ResetTokenUsage(ctx, hashedToken)
}

// UpdateTokenLimit updates the maximum allowed requests for a token.
func (s *SecureRateLimitStore) UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	hashedToken := s.hasher.CreateLookupKey(tokenID)
	return s.store.UpdateTokenLimit(ctx, hashedToken, maxRequests)
}

// Compile-time interface check
var _ token.RateLimitStore = (*SecureRateLimitStore)(nil)
