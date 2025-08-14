package token

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

// CompleteStore implements TokenStore, RevocationStore and RateLimitStore for testing
type CompleteStore struct {
	MockStore
}

// CreateToken adds a new token to the store
func (s *CompleteStore) CreateToken(ctx context.Context, token TokenData) error {
	if _, exists := s.tokens[token.Token]; exists {
		return errors.New("token already exists")
	}

	s.tokens[token.Token] = token
	return nil
}

func (s *CompleteStore) UpdateToken(ctx context.Context, token TokenData) error {
	if _, exists := s.tokens[token.Token]; !exists {
		return errors.New("token not found")
	}

	s.tokens[token.Token] = token
	return nil
}

func (s *CompleteStore) GetTokenUnsafe(tokenID string) (TokenData, bool) {
	return s.MockStore.GetTokenUnsafe(tokenID)
}

func (s *CompleteStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}

func (s *CompleteStore) ListTokens(ctx context.Context) ([]TokenData, error) {
	return nil, nil
}

type mockManagerStore struct {
	tokenData TokenData
	err       error
}

func (m *mockManagerStore) GetTokenByID(ctx context.Context, tokenID string) (TokenData, error) {
	return m.tokenData, m.err
}

// Implement other required methods as no-ops
func (m *mockManagerStore) IncrementTokenUsage(ctx context.Context, tokenID string) error { return nil }
func (m *mockManagerStore) ResetTokenUsage(ctx context.Context, tokenID string) error     { return nil }
func (m *mockManagerStore) UpdateTokenLimit(ctx context.Context, tokenID string, maxRequests *int) error {
	return nil
}
func (m *mockManagerStore) CreateToken(ctx context.Context, token TokenData) error { return nil }
func (m *mockManagerStore) UpdateToken(ctx context.Context, token TokenData) error { return nil }
func (m *mockManagerStore) DeleteToken(ctx context.Context, tokenID string) error  { return nil }
func (m *mockManagerStore) RevokeToken(ctx context.Context, tokenID string) error  { return nil }
func (m *mockManagerStore) RevokeBatchTokens(ctx context.Context, tokenIDs []string) (int, error) {
	return 0, nil
}
func (m *mockManagerStore) RevokeProjectTokens(ctx context.Context, projectID string) (int, error) {
	return 0, nil
}
func (m *mockManagerStore) RevokeExpiredTokens(ctx context.Context) (int, error) { return 0, nil }
func (m *mockManagerStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}
func (m *mockManagerStore) ListTokens(ctx context.Context) ([]TokenData, error) {
	return nil, nil
}

func TestManager_CreateToken(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, true)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a token with expiration
	expiration := 1 * time.Hour
	options := TokenOptions{
		Expiration: expiration,
	}

	token, err := manager.CreateToken(ctx, "test-project", options)
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	// Verify token properties
	if token.ProjectID != "test-project" {
		t.Errorf("Token project ID = %v, want %v", token.ProjectID, "test-project")
	}

	if token.ExpiresAt == nil {
		t.Errorf("Token ExpiresAt should not be nil")
	} else {
		// Check expiration is approximately now + expiration
		now := time.Now()
		expectedExpiry := now.Add(expiration)
		diff := expectedExpiry.Sub(*token.ExpiresAt)
		if diff < -2*time.Second || diff > 2*time.Second {
			t.Errorf("Token expiration = %v, want approximately %v (diff: %v)",
				*token.ExpiresAt, expectedExpiry, diff)
		}
	}

	if !token.IsActive {
		t.Errorf("Token should be active")
	}

	if token.RequestCount != 0 {
		t.Errorf("Token request count should be 0, got %v", token.RequestCount)
	}
}

func TestManager_TokenValidation(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, true)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a token
	options := TokenOptions{
		Expiration: 1 * time.Hour,
	}

	token, err := manager.CreateToken(ctx, "test-project", options)
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	// Validate token
	projectID, err := manager.ValidateToken(ctx, token.Token)
	if err != nil {
		t.Errorf("ValidateToken() error = %v", err)
	}
	if projectID != "test-project" {
		t.Errorf("ValidateToken() projectID = %v, want %v", projectID, "test-project")
	}

	// Check if token is valid
	if !manager.IsTokenValid(ctx, token.Token) {
		t.Errorf("IsTokenValid() = false, want true")
	}

	// Validate with tracking
	projectID, err = manager.ValidateTokenWithTracking(ctx, token.Token)
	if err != nil {
		t.Errorf("ValidateTokenWithTracking() error = %v", err)
	}
	if projectID != "test-project" {
		t.Errorf("ValidateTokenWithTracking() projectID = %v, want %v", projectID, "test-project")
	}

	// Get token info
	info, err := manager.GetTokenInfo(ctx, token.Token)
	if err != nil {
		t.Errorf("GetTokenInfo() error = %v", err)
	}
	if info.Token != token.Token {
		t.Errorf("GetTokenInfo().Token = %v, want %v", info.Token, token.Token)
	}
	if !info.IsValid {
		t.Errorf("GetTokenInfo().IsValid = false, want true")
	}

	// Get token stats
	stats, err := manager.GetTokenStats(ctx, token.Token)
	if err != nil {
		t.Errorf("GetTokenStats() error = %v", err)
	}
	if stats.RequestCount != 1 {
		t.Errorf("GetTokenStats().RequestCount = %v, want 1", stats.RequestCount)
	}
	if !stats.IsValid {
		t.Errorf("GetTokenStats().IsValid = false, want true")
	}
}

func TestManager_TokenRevocation(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, false) // No caching
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create tokens
	options := TokenOptions{
		Expiration: 1 * time.Hour,
	}

	token1, err := manager.CreateToken(ctx, "project1", options)
	if err != nil {
		t.Fatalf("CreateToken(1) error = %v", err)
	}

	token2, err := manager.CreateToken(ctx, "project1", options)
	if err != nil {
		t.Fatalf("CreateToken(2) error = %v", err)
	}

	expiredOptions := TokenOptions{
		Expiration: -1 * time.Hour, // Already expired
	}
	expiredToken, err := manager.CreateToken(ctx, "project1", expiredOptions)
	if err != nil {
		t.Fatalf("CreateToken(expired) error = %v", err)
	}
	// Force the expired token's ExpiresAt to a time well in the past
	past := time.Now().Add(-2 * time.Hour)
	store.mutex.Lock()
	tok := store.tokens[expiredToken.Token]
	tok.ExpiresAt = &past
	store.tokens[expiredToken.Token] = tok
	store.mutex.Unlock()

	// Revoke the first token
	err = manager.RevokeToken(ctx, token1.Token)
	if err != nil {
		t.Errorf("RevokeToken() error = %v", err)
	}

	// Check token1 is no longer valid
	if manager.IsTokenValid(ctx, token1.Token) {
		t.Errorf("IsTokenValid() for revoked token = true, want false")
	}

	// Check token2 is still valid
	if !manager.IsTokenValid(ctx, token2.Token) {
		t.Errorf("IsTokenValid() for valid token = false, want true")
	}

	// Revoke expired tokens
	count, err := manager.RevokeExpiredTokens(ctx)
	if err != nil {
		t.Errorf("RevokeExpiredTokens() error = %v", err)
	}
	if count != 1 {
		t.Errorf("RevokeExpiredTokens() count = %v, want 1", count)
	}

	// Ensure expired token is now inactive
	tok, _ = store.GetTokenUnsafe(expiredToken.Token)
	if tok.IsActive {
		t.Errorf("Expired token should be inactive after revocation")
	}

	// Revoke all tokens for the project
	count, err = manager.RevokeProjectTokens(ctx, "project1")
	if err != nil {
		t.Errorf("RevokeProjectTokens() error = %v", err)
	}
	if count != 1 { // Only one active token remains after revoking the expired token
		t.Errorf("RevokeProjectTokens() count = %v, want 1", count)
	}

	// All tokens should now be invalid
	if manager.IsTokenValid(ctx, token2.Token) {
		t.Errorf("IsTokenValid() after project revocation = true, want false")
	}
}

func TestManager_TokenLimits(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, true)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create token with no limits
	unlimited, err := manager.CreateToken(ctx, "project1", TokenOptions{})
	if err != nil {
		t.Fatalf("CreateToken(unlimited) error = %v", err)
	}

	// Set a limit on the token
	maxReq := 3
	err = manager.UpdateTokenLimit(ctx, unlimited.Token, &maxReq)
	if err != nil {
		t.Errorf("UpdateTokenLimit() error = %v", err)
	}

	// Get token stats to verify limit
	stats, err := manager.GetTokenStats(ctx, unlimited.Token)
	if err != nil {
		t.Errorf("GetTokenStats() error = %v", err)
	}
	if stats.RemainingCount != maxReq {
		t.Errorf("GetTokenStats().RemainingCount = %v, want %v", stats.RemainingCount, maxReq)
	}

	// Use token multiple times
	for i := 0; i < maxReq; i++ {
		_, err := manager.ValidateTokenWithTracking(ctx, unlimited.Token)
		if err != nil {
			t.Errorf("ValidateTokenWithTracking() iteration %d error = %v", i, err)
		}
	}

	// One more validation should fail due to rate limit
	_, err = manager.ValidateTokenWithTracking(ctx, unlimited.Token)
	if err == nil {
		t.Errorf("ValidateTokenWithTracking() after limit should return error")
	}

	// Reset usage
	err = manager.ResetTokenUsage(ctx, unlimited.Token)
	if err != nil {
		t.Errorf("ResetTokenUsage() error = %v", err)
	}

	// Token should be valid again
	if !manager.IsTokenValid(ctx, unlimited.Token) {
		t.Errorf("IsTokenValid() after reset = false, want true")
	}

	// Get stats again
	stats, err = manager.GetTokenStats(ctx, unlimited.Token)
	if err != nil {
		t.Errorf("GetTokenStats() after reset error = %v", err)
	}
	if stats.RequestCount != 0 {
		t.Errorf("GetTokenStats().RequestCount after reset = %v, want 0", stats.RequestCount)
	}
	if stats.RemainingCount != maxReq {
		t.Errorf("GetTokenStats().RemainingCount after reset = %v, want %v", stats.RemainingCount, maxReq)
	}
}

func TestManager_CacheInfo(t *testing.T) {
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	// Create manager with caching
	managerWithCache, err := NewManager(store, true)
	if err != nil {
		t.Fatalf("NewManager(with cache) error = %v", err)
	}

	info, enabled := managerWithCache.GetCacheInfo()
	if !enabled {
		t.Errorf("GetCacheInfo() enabled = false, want true")
	}
	if info == "" {
		t.Errorf("GetCacheInfo() info is empty")
	}

	// Create manager without caching
	managerNoCache, err := NewManager(store, false)
	if err != nil {
		t.Fatalf("NewManager(no cache) error = %v", err)
	}

	info, enabled = managerNoCache.GetCacheInfo()
	if enabled {
		t.Errorf("GetCacheInfo() enabled = true, want false")
	}
	if info != "Caching disabled" {
		t.Errorf("GetCacheInfo() info = %v, want 'Caching disabled'", info)
	}
}

func TestManager_CustomGeneratorOptions(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Set custom generator options
	customExpiration := 48 * time.Hour
	customMaxReq := 500
	manager = manager.WithGeneratorOptions(customExpiration, &customMaxReq)

	// Create token with default options (should use custom defaults)
	token, err := manager.CreateToken(ctx, "project1", TokenOptions{})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	// Verify expiration is approximately now + customExpiration
	if token.ExpiresAt == nil {
		t.Errorf("Token ExpiresAt should not be nil")
	} else {
		now := time.Now()
		expectedExpiry := now.Add(customExpiration)
		diff := expectedExpiry.Sub(*token.ExpiresAt)
		if diff < -2*time.Second || diff > 2*time.Second {
			t.Errorf("Token expiration = %v, want approximately %v (diff: %v)",
				*token.ExpiresAt, expectedExpiry, diff)
		}
	}

	// Verify max requests
	if token.MaxRequests == nil {
		t.Errorf("Token MaxRequests should not be nil")
	} else if *token.MaxRequests != customMaxReq {
		t.Errorf("Token MaxRequests = %v, want %v", *token.MaxRequests, customMaxReq)
	}
}

func TestManager_DeleteToken(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Create a token
	options := TokenOptions{
		Expiration: 1 * time.Hour,
	}
	token, err := manager.CreateToken(ctx, "test-project", options)
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}

	// Delete the token
	err = manager.DeleteToken(ctx, token.Token)
	if err != nil {
		t.Errorf("DeleteToken() error = %v", err)
	}

	// Try to validate the deleted token
	_, err = manager.ValidateToken(ctx, token.Token)
	if err == nil {
		t.Errorf("ValidateToken() should fail for deleted token")
	}
}

func TestManager_StartAutomaticRevocation(t *testing.T) {
	ctx := context.Background()
	store := &CompleteStore{
		MockStore: *NewMockStore(),
	}

	manager, err := NewManager(store, false)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	// Add an expired token
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	token, err := manager.CreateToken(ctx, "test-project", TokenOptions{Expiration: -1 * time.Hour})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	// Force expiration
	store.mutex.Lock()
	tok := store.tokens[token.Token]
	tok.ExpiresAt = &past
	store.tokens[token.Token] = tok
	store.mutex.Unlock()

	// Start automatic revocation
	auto := manager.StartAutomaticRevocation(100*time.Millisecond, zap.NewNop())
	defer auto.Stop()

	// Wait for revocation to run
	time.Sleep(250 * time.Millisecond)

	// Token should be revoked
	tok, _ = store.GetTokenUnsafe(token.Token)
	if tok.IsActive {
		t.Errorf("Token should be inactive after automatic revocation")
	}
}

func TestManager_GetTokenInfo_ErrorsAndEdgeCases(t *testing.T) {
	ctx := context.Background()
	m := &Manager{store: &mockManagerStore{err: errors.New("not found")}, validator: NewValidator(&mockManagerStore{err: errors.New("not found")})}
	_, err := m.GetTokenInfo(ctx, "missing")
	if err == nil {
		t.Error("expected error for missing token, got nil")
	}

	// Edge case: unlimited requests, no expiration
	token := TokenData{
		Token:        "sk-1234567890ABCDEFGHijklmn",
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  nil,
		ExpiresAt:    nil,
		CreatedAt:    time.Now(),
	}
	m2 := &Manager{store: &mockManagerStore{tokenData: token}, validator: NewValidator(&mockManagerStore{tokenData: token})}
	info, err := m2.GetTokenInfo(ctx, token.Token)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if info == nil || info.MaxRequests != nil || info.ExpiresAt != nil {
		t.Errorf("expected unlimited requests and no expiration, got %+v", info)
	}
}

func TestManager_GetTokenStats_ErrorsAndEdgeCases(t *testing.T) {
	ctx := context.Background()
	m := &Manager{store: &mockManagerStore{err: errors.New("not found")}, validator: NewValidator(&mockManagerStore{err: errors.New("not found")})}
	_, err := m.GetTokenStats(ctx, "missing")
	if err == nil {
		t.Error("expected error for missing token, got nil")
	}

	// Edge case: unlimited requests, no expiration
	token := TokenData{
		Token:        "sk-1234567890ABCDEFGHijklmn",
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  nil,
		ExpiresAt:    nil,
		CreatedAt:    time.Now(),
	}
	m2 := &Manager{store: &mockManagerStore{tokenData: token}, validator: NewValidator(&mockManagerStore{tokenData: token})}
	stats, err := m2.GetTokenStats(ctx, token.Token)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if stats == nil || stats.RemainingCount != -1 || stats.TimeRemaining != -1 {
		t.Errorf("expected unlimited requests and no expiration, got %+v", stats)
	}
}
