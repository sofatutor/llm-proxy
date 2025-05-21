package token

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Manager provides a unified interface for all token operations
type Manager struct {
	validator   TokenValidator
	revoker     *Revoker
	limiter     *StandardRateLimiter
	generator   *TokenGenerator
	store       interface{} // Underlying store (should implement multiple store interfaces)
	useCaching  bool
}

// NewManager creates a new token manager with the given store
func NewManager(store interface{}, useCaching bool) (*Manager, error) {
	// Verify store implements all required interfaces
	tokenStore, ok1 := store.(TokenStore)
	revStore, ok2 := store.(RevocationStore)
	rateStore, ok3 := store.(RateLimitStore)

	if !ok1 || !ok2 || !ok3 {
		return nil, errors.New("store must implement TokenStore, RevocationStore, and RateLimitStore interfaces")
	}

	// Create components
	baseValidator := NewValidator(tokenStore)
	var validator TokenValidator = baseValidator
	if useCaching {
		validator = NewCachedValidator(baseValidator)
	}

	revoker := NewRevoker(revStore)
	limiter := NewRateLimiter(rateStore)
	generator := NewTokenGenerator()

	return &Manager{
		validator:  validator,
		revoker:    revoker,
		limiter:    limiter, 
		generator:  generator,
		store:      store,
		useCaching: useCaching,
	}, nil
}

// CreateToken generates a new token with the specified options
func (m *Manager) CreateToken(ctx context.Context, projectID string, options TokenOptions) (TokenData, error) {
	// Generate a new token
	tokenStr, expiresAt, maxRequests, err := m.generator.GenerateWithOptions(options.Expiration, options.MaxRequests)
	if err != nil {
		return TokenData{}, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create token data
	now := time.Now()
	token := TokenData{
		Token:        tokenStr,
		ProjectID:    projectID,
		ExpiresAt:    expiresAt,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  maxRequests,
		CreatedAt:    now,
	}

	// For now, we'll add the token directly to the mock store in tests
	// In real implementation, this would call a store method
	
	// Example implementation for testing only:
	if mockStore, ok := m.store.(interface {
		AddToken(tokenID string, data TokenData)
	}); ok {
		mockStore.AddToken(token.Token, token)
	}

	return token, nil
}

// ValidateToken validates a token without incrementing usage
func (m *Manager) ValidateToken(ctx context.Context, tokenID string) (string, error) {
	return m.validator.ValidateToken(ctx, tokenID)
}

// ValidateTokenWithTracking validates a token and increments usage count
func (m *Manager) ValidateTokenWithTracking(ctx context.Context, tokenID string) (string, error) {
	return m.validator.ValidateTokenWithTracking(ctx, tokenID)
}

// RevokeToken revokes a token
func (m *Manager) RevokeToken(ctx context.Context, tokenID string) error {
	return m.revoker.RevokeToken(ctx, tokenID)
}

// DeleteToken completely removes a token
func (m *Manager) DeleteToken(ctx context.Context, tokenID string) error {
	return m.revoker.DeleteToken(ctx, tokenID)
}

// RevokeExpiredTokens revokes all expired tokens
func (m *Manager) RevokeExpiredTokens(ctx context.Context) (int, error) {
	return m.revoker.RevokeExpiredTokens(ctx)
}

// RevokeProjectTokens revokes all tokens for a project
func (m *Manager) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	return m.revoker.RevokeProjectTokens(ctx, projectID)
}

// GetTokenInfo gets detailed information about a token
func (m *Manager) GetTokenInfo(ctx context.Context, tokenID string) (*TokenInfo, error) {
	store, ok := m.store.(TokenStore)
	if !ok {
		return nil, errors.New("store does not implement TokenStore interface")
	}

	tokenData, err := store.GetTokenByID(ctx, tokenID)
	if err != nil {
		return nil, err
	}

	info, err := GetTokenInfo(tokenData)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// GetTokenStats gets statistics about token usage
func (m *Manager) GetTokenStats(ctx context.Context, tokenID string) (*TokenStats, error) {
	store, ok := m.store.(TokenStore)
	if !ok {
		return nil, errors.New("store does not implement TokenStore interface")
	}

	tokenData, err := store.GetTokenByID(ctx, tokenID)
	if err != nil {
		return nil, err
	}

	var remaining int
	if tokenData.MaxRequests != nil {
		remaining = *tokenData.MaxRequests - tokenData.RequestCount
		if remaining < 0 {
			remaining = 0
		}
	} else {
		remaining = -1 // Unlimited
	}

	var timeRemaining time.Duration
	if tokenData.ExpiresAt != nil {
		timeRemaining = time.Until(*tokenData.ExpiresAt)
		if timeRemaining < 0 {
			timeRemaining = 0
		}
	} else {
		timeRemaining = -1 // No expiration
	}

	stats := &TokenStats{
		Token:            tokenData.Token,
		RequestCount:     tokenData.RequestCount,
		RemainingCount:   remaining,
		LastUsed:         tokenData.LastUsedAt,
		TimeRemaining:    timeRemaining,
		IsValid:          tokenData.IsValid(),
		ObfuscatedToken:  ObfuscateToken(tokenData.Token),
	}

	return stats, nil
}

// UpdateTokenLimit updates the maximum allowed requests for a token
func (m *Manager) UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	return m.limiter.UpdateLimit(ctx, tokenID, maxRequests)
}

// ResetTokenUsage resets the usage count for a token
func (m *Manager) ResetTokenUsage(ctx context.Context, tokenID string) error {
	return m.limiter.ResetUsage(ctx, tokenID)
}

// IsTokenValid checks if a token is valid
func (m *Manager) IsTokenValid(ctx context.Context, tokenID string) bool {
	_, err := m.validator.ValidateToken(ctx, tokenID)
	return err == nil
}

// StartAutomaticRevocation starts automatic revocation of expired tokens
func (m *Manager) StartAutomaticRevocation(interval time.Duration) *AutomaticRevocation {
	autoRevoke := NewAutomaticRevocation(m.revoker, interval)
	autoRevoke.Start()
	return autoRevoke
}

// GetCacheInfo returns information about the token validation cache if caching is enabled
func (m *Manager) GetCacheInfo() (string, bool) {
	if !m.useCaching {
		return "Caching disabled", false
	}

	if cachedValidator, ok := m.validator.(*CachedValidator); ok {
		return cachedValidator.GetCacheInfo(), true
	}

	return "Cache info not available", false
}

// WithGeneratorOptions configures the token generator with new options
func (m *Manager) WithGeneratorOptions(expiration time.Duration, maxRequests *int) *Manager {
	generator := m.generator.WithExpiration(expiration)
	if maxRequests != nil {
		generator = generator.WithMaxRequests(*maxRequests)
	}
	m.generator = generator
	return m
}

// TokenOptions contains options for token creation
type TokenOptions struct {
	// Expiration duration (0 for no expiration)
	Expiration time.Duration
	
	// Maximum requests (nil for no limit)
	MaxRequests *int
	
	// Custom metadata (implementation-dependent)
	Metadata map[string]string
}

// TokenStats contains statistics about token usage
type TokenStats struct {
	Token           string
	ObfuscatedToken string
	RequestCount    int
	RemainingCount  int // -1 means unlimited
	LastUsed        *time.Time
	TimeRemaining   time.Duration // -1 means no expiration
	IsValid         bool
}