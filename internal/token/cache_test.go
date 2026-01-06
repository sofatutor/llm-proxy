package token

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// MockValidator implements TokenValidator for testing cache
type MockValidator struct {
	tokens          map[string]TokenData
	validationCount int
	trackingCount   int
	failOnValidate  bool
	failOnTracking  bool
}

func NewMockValidator() *MockValidator {
	return &MockValidator{
		tokens:         make(map[string]TokenData),
		failOnValidate: false,
		failOnTracking: false,
	}
}

func (m *MockValidator) ValidateToken(ctx context.Context, tokenID string) (string, error) {
	m.validationCount++

	if m.failOnValidate {
		return "", errors.New("mock validation failure")
	}

	token, exists := m.tokens[tokenID]
	if !exists {
		return "", ErrTokenNotFound
	}

	if !token.IsActive {
		return "", ErrTokenInactive
	}

	if IsExpired(token.ExpiresAt) {
		return "", ErrTokenExpired
	}

	if token.MaxRequests != nil && token.RequestCount >= *token.MaxRequests {
		return "", ErrTokenRateLimit
	}

	return token.ProjectID, nil
}

func (m *MockValidator) ValidateTokenWithTracking(ctx context.Context, tokenID string) (string, error) {
	m.trackingCount++

	if m.failOnTracking {
		return "", errors.New("mock tracking failure")
	}

	projectID, err := m.ValidateToken(ctx, tokenID)
	if err != nil {
		return "", err
	}

	token := m.tokens[tokenID]
	token.RequestCount++
	now := time.Now()
	token.LastUsedAt = &now
	m.tokens[tokenID] = token

	return projectID, nil
}

func (m *MockValidator) AddToken(tokenID string, data TokenData) {
	m.tokens[tokenID] = data
}

func TestCachedValidator_ValidateToken(t *testing.T) {
	ctx := context.Background()
	store := NewStrictMockTokenStore()
	validator := &StandardValidator{store: store}

	// Create cache with small TTL for testing
	cachedValidator := NewCachedValidator(validator, CacheOptions{
		TTL:           100 * time.Millisecond,
		MaxSize:       10,
		EnableCleanup: false,
	})

	// Add a valid token
	now := time.Now()
	future := now.Add(1 * time.Hour)
	maxReq := 100

	validToken, _ := GenerateToken()
	store.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})

	// Test case 1: First call should miss cache
	projectID, err := cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("First ValidateToken() error = %v", err)
	}
	if projectID != "project1" {
		t.Errorf("First ValidateToken() projectID = %v, want %v", projectID, "project1")
	}

	// Wait a moment to ensure cache is populated
	time.Sleep(10 * time.Millisecond)

	// Test case 2: Second call should hit cache
	projectID, err = cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("Second ValidateToken() error = %v", err)
	}
	if projectID != "project1" {
		t.Errorf("Second ValidateToken() projectID = %v, want %v", projectID, "project1")
	}

	// Check cache stats
	hits, misses, _, _ := cachedValidator.GetCacheStats()
	if hits != 1 {
		t.Errorf("Cache hits = %v, want 1", hits)
	}
	if misses != 1 {
		t.Errorf("Cache misses = %v, want 1", misses)
	}

	// Test case 3: Wait for cache expiration
	time.Sleep(120 * time.Millisecond)
	_, err = cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("After TTL, ValidateToken() error = %v", err)
	}

	// Test case 4: Invalid token should not be cached
	invalidToken, _ := GenerateToken()
	store.AddToken(invalidToken, TokenData{
		Token:     invalidToken,
		ProjectID: "project2",
		ExpiresAt: &future,
		IsActive:  false, // Inactive token
		CreatedAt: now,
	})

	_, err = cachedValidator.ValidateToken(ctx, invalidToken)
	if err == nil {
		t.Errorf("ValidateToken() with invalid token should return error")
	}
}

func TestCachedValidator_ValidateTokenWithTracking(t *testing.T) {
	ctx := context.Background()
	mockValidator := NewMockValidator()

	cachedValidator := NewCachedValidator(mockValidator, CacheOptions{
		TTL:           1 * time.Minute,
		MaxSize:       10,
		EnableCleanup: false,
	})

	// Add a valid token
	now := time.Now()
	future := now.Add(1 * time.Hour)
	maxReq := 100

	validToken, _ := GenerateToken()
	mockValidator.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 50,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})

	// Test case 1: Validate with tracking should bypass cache
	projectID, err := cachedValidator.ValidateTokenWithTracking(ctx, validToken)
	if err != nil {
		t.Errorf("ValidateTokenWithTracking() error = %v", err)
	}
	if projectID != "project1" {
		t.Errorf("ValidateTokenWithTracking() projectID = %v, want %v", projectID, "project1")
	}
	if mockValidator.trackingCount != 1 {
		t.Errorf("ValidateTokenWithTracking() should call underlying validator, trackingCount = %v, want 1", mockValidator.trackingCount)
	}

	// Test case 2: Cache should be invalidated
	mockValidator.AddToken(validToken, TokenData{
		Token:        validToken,
		ProjectID:    "project1-updated", // Changed project ID
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 51, // Incremented
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	})

	// Regular validate should bypass cache after tracking
	mockValidator.validationCount = 0 // Reset for clarity
	projectID, err = cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("ValidateToken() after tracking error = %v", err)
	}
	if projectID != "project1-updated" {
		t.Errorf("ValidateToken() after tracking projectID = %v, want %v", projectID, "project1-updated")
	}
	if mockValidator.validationCount != 1 {
		t.Errorf("ValidateToken() after tracking should not use cache, validationCount = %v, want 1", mockValidator.validationCount)
	}
}

type countingStore struct {
	mu sync.Mutex

	tokens map[string]TokenData

	getByTokenCalls int
	incCalls        int
}

func newCountingStore() *countingStore {
	return &countingStore{tokens: make(map[string]TokenData)}
}

func (s *countingStore) GetTokenByID(ctx context.Context, id string) (TokenData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	td, ok := s.tokens[id]
	if !ok {
		return TokenData{}, ErrTokenNotFound
	}
	return td, nil
}

func (s *countingStore) GetTokenByToken(ctx context.Context, tokenString string) (TokenData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getByTokenCalls++
	td, ok := s.tokens[tokenString]
	if !ok {
		return TokenData{}, ErrTokenNotFound
	}
	return td, nil
}

func (s *countingStore) IncrementTokenUsage(ctx context.Context, tokenString string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incCalls++

	td, ok := s.tokens[tokenString]
	if !ok {
		return ErrTokenNotFound
	}
	if !td.IsActive {
		return ErrTokenInactive
	}
	if IsExpired(td.ExpiresAt) {
		return ErrTokenExpired
	}
	if td.MaxRequests != nil && *td.MaxRequests > 0 && td.RequestCount >= *td.MaxRequests {
		return ErrTokenRateLimit
	}
	td.RequestCount++
	now := time.Now()
	td.LastUsedAt = &now
	s.tokens[tokenString] = td
	return nil
}

func (s *countingStore) CreateToken(ctx context.Context, token TokenData) error { return nil }
func (s *countingStore) UpdateToken(ctx context.Context, token TokenData) error { return nil }
func (s *countingStore) ListTokens(ctx context.Context) ([]TokenData, error)    { return nil, nil }
func (s *countingStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}

func TestCachedValidator_ValidateTokenWithTracking_LimitedToken_UsesCacheAndAvoidsExtraReads(t *testing.T) {
	ctx := context.Background()
	store := newCountingStore()
	validator := &StandardValidator{store: store}
	cv := NewCachedValidator(validator, CacheOptions{TTL: time.Minute, MaxSize: 10, EnableCleanup: false})

	now := time.Now()
	future := now.Add(1 * time.Hour)
	maxReq := 100
	tok, _ := GenerateToken()

	store.tokens[tok] = TokenData{
		Token:        tok,
		ProjectID:    "p1",
		ExpiresAt:    &future,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  &maxReq,
		CreatedAt:    now,
	}

	// First call: expect underlying validation+tracking and then cache population (may re-read once).
	got, err := cv.ValidateTokenWithTracking(ctx, tok)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() error = %v", err)
	}
	if got != "p1" {
		t.Fatalf("ValidateTokenWithTracking() = %q, want %q", got, "p1")
	}

	// Second call: should hit cache and only do a usage increment (no GetTokenByToken read).
	got, err = cv.ValidateTokenWithTracking(ctx, tok)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() second error = %v", err)
	}
	if got != "p1" {
		t.Fatalf("ValidateTokenWithTracking() second = %q, want %q", got, "p1")
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.incCalls != 2 {
		t.Fatalf("IncrementTokenUsage calls = %d, want 2", store.incCalls)
	}
	// Cache population after first tracking currently performs one extra GetTokenByToken read.
	// The key property we assert: it does not keep reading on every subsequent request.
	if store.getByTokenCalls > 2 {
		t.Fatalf("GetTokenByToken calls = %d, want <= 2", store.getByTokenCalls)
	}
}

func TestCachedValidator_CacheEviction(t *testing.T) {
	ctx := context.Background()
	mockValidator := NewMockValidator()

	// Create cache with small size for testing eviction
	cachedValidator := NewCachedValidator(mockValidator, CacheOptions{
		TTL:           1 * time.Minute,
		MaxSize:       2, // Only 2 entries
		EnableCleanup: false,
	})

	// Add tokens
	now := time.Now()
	future := now.Add(1 * time.Hour)

	token1, _ := GenerateToken()
	token2, _ := GenerateToken()
	token3, _ := GenerateToken()

	mockValidator.AddToken(token1, TokenData{
		Token:     token1,
		ProjectID: "project1",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})
	mockValidator.AddToken(token2, TokenData{
		Token:     token2,
		ProjectID: "project2",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})
	mockValidator.AddToken(token3, TokenData{
		Token:     token3,
		ProjectID: "project3",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})

	_, _ = cachedValidator.ValidateToken(ctx, token1)
	_, _ = cachedValidator.ValidateToken(ctx, token2)
	_, _ = cachedValidator.ValidateToken(ctx, token3)

	_, _, _, size := cachedValidator.GetCacheStats()
	if size > 2 {
		t.Errorf("Cache size = %v, should not exceed max size 2", size)
	}

	// Clear cache
	cachedValidator.ClearCache()
	_, _, _, size = cachedValidator.GetCacheStats()
	if size != 0 {
		t.Errorf("After ClearCache(), cache size = %v, want 0", size)
	}
}

func TestCachedValidator_Cleanup(t *testing.T) {
	store := NewStrictMockTokenStore()
	validator := &StandardValidator{store: store}

	// Create cache with short TTL and cleanup
	cachedValidator := NewCachedValidator(validator, CacheOptions{
		TTL:             50 * time.Millisecond,
		MaxSize:         10,
		EnableCleanup:   true,
		CleanupInterval: 100 * time.Millisecond,
	})

	// Add a valid token
	now := time.Now()
	future := now.Add(1 * time.Hour)

	validToken, _ := GenerateToken()
	store.AddToken(validToken, TokenData{
		Token:     validToken,
		ProjectID: "project1",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})

	// Validate to put in cache
	ctx := context.Background()
	_, err := cachedValidator.ValidateToken(ctx, validToken)
	if err != nil {
		t.Errorf("ValidateToken() error = %v", err)
	}

	// Wait a moment to ensure cache is populated
	time.Sleep(10 * time.Millisecond)

	// Check initial cache size
	_, _, _, size := cachedValidator.GetCacheStats()
	if size != 1 {
		t.Errorf("Initial cache size = %v, want 1", size)
	}

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Check cache size after cleanup
	_, _, _, size = cachedValidator.GetCacheStats()
	if size != 0 {
		t.Errorf("After cleanup, cache size = %v, want 0", size)
	}
}

func TestCachedValidator_EvictOldest(t *testing.T) {
	ctx := context.Background()
	store := NewStrictMockTokenStore()
	validator := &StandardValidator{store: store}

	cachedValidator := NewCachedValidator(validator, CacheOptions{
		TTL:           1 * time.Minute,
		MaxSize:       2, // Only 2 entries allowed
		EnableCleanup: false,
	})

	now := time.Now()
	future := now.Add(1 * time.Hour)

	token1, _ := GenerateToken()
	token2, _ := GenerateToken()
	token3, _ := GenerateToken()

	store.AddToken(token1, TokenData{
		Token:     token1,
		ProjectID: "project1",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})
	store.AddToken(token2, TokenData{
		Token:     token2,
		ProjectID: "project2",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})
	store.AddToken(token3, TokenData{
		Token:     token3,
		ProjectID: "project3",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})

	// Add all three tokens to the cache
	_, _ = cachedValidator.ValidateToken(ctx, token1)
	_, _ = cachedValidator.ValidateToken(ctx, token2)
	_, _ = cachedValidator.ValidateToken(ctx, token3)

	// Check that cache size does not exceed maxCacheSize
	_, _, evictions, size := cachedValidator.GetCacheStats()
	if size > 2 {
		t.Errorf("Cache size = %v, should not exceed max size 2", size)
	}
	if evictions == 0 {
		t.Errorf("Evictions = %v, want > 0", evictions)
	}
}

func TestCachedValidator_EvictOldest_CorrectnessAndEfficiency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := NewStrictMockTokenStore()
	validator := &StandardValidator{store: store}
	maxSize := 100
	cv := NewCachedValidator(validator, CacheOptions{
		TTL:           10 * time.Minute,
		MaxSize:       maxSize,
		EnableCleanup: false,
	})

	tokenIDs := make([]string, 0, 200)
	for i := 0; i < 200; i++ {
		tokenID, _ := GenerateToken() // use project token generator
		tokenIDs = append(tokenIDs, tokenID)
		expiresAt := time.Now().Add(1 * time.Hour)
		store.AddToken(tokenID, TokenData{
			Token:     tokenID,
			ProjectID: "proj",
			ExpiresAt: &expiresAt,
			IsActive:  true,
		})
		_, _ = cv.ValidateToken(ctx, tokenID)
	}

	// After all insertions, only the newest 100 tokens should remain
	expected := tokenIDs[100:]
	cacheKeys := make(map[string]struct{})
	cv.cacheMutex.Lock()
	for k := range cv.cache {
		cacheKeys[k] = struct{}{}
	}
	cv.cacheMutex.Unlock()

	missing := []string{}
	for _, k := range expected {
		if _, ok := cacheKeys[k]; !ok {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Missing expected tokens in cache: %v", missing)
	}
	if len(cv.cache) != maxSize {
		t.Errorf("Cache size = %d, want %d", len(cv.cache), maxSize)
	}
}

func (m *StrictMockTokenStore) CreateToken(ctx context.Context, td TokenData) error {
	return nil
}

func (m *StrictMockTokenStore) UpdateToken(ctx context.Context, td TokenData) error {
	return nil
}

func (m *StrictMockTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}

func (m *StrictMockTokenStore) ListTokens(ctx context.Context) ([]TokenData, error) {
	return nil, nil
}

func TestCachedValidator_cacheToken_EdgeCases(t *testing.T) {
	ctx := context.Background()
	store := NewStrictMockTokenStore()
	validator := &StandardValidator{store: store}
	cv := NewCachedValidator(validator, CacheOptions{TTL: time.Minute, MaxSize: 10, EnableCleanup: false})

	now := time.Now()
	future := now.Add(time.Hour)
	validToken, _ := GenerateToken()
	store.AddToken(validToken, TokenData{
		Token:     validToken,
		ProjectID: "p1",
		ExpiresAt: &future,
		IsActive:  true,
		CreatedAt: now,
	})

	// Valid token: should be cached
	cv.cacheToken(ctx, validToken)
	cv.cacheMutex.RLock()
	_, found := cv.cache[validToken]
	cv.cacheMutex.RUnlock()
	if !found {
		t.Errorf("cacheToken did not cache valid token")
	}

	// Invalid token: inactive
	inactiveToken, _ := GenerateToken()
	store.AddToken(inactiveToken, TokenData{
		Token:     inactiveToken,
		ProjectID: "p2",
		ExpiresAt: &future,
		IsActive:  false,
		CreatedAt: now,
	})
	cv.cacheToken(ctx, inactiveToken)
	cv.cacheMutex.RLock()
	_, found = cv.cache[inactiveToken]
	cv.cacheMutex.RUnlock()
	if found {
		t.Errorf("cacheToken should not cache inactive token")
	}

	// Not found token
	cv.cacheToken(ctx, "notfound")
	cv.cacheMutex.RLock()
	_, found = cv.cache["notfound"]
	if found {
		// Print cache keys for debugging
		keys := make([]string, 0, len(cv.cache))
		for k := range cv.cache {
			keys = append(keys, k)
		}
		t.Errorf("cacheToken should not cache notfound token; cache keys: %v", keys)
	}
	cv.cacheMutex.RUnlock()

	// Non-StandardValidator: should not panic or cache
	cv2 := &CachedValidator{validator: &MockValidator{}, cache: make(map[string]CacheEntry)}
	cv2.cacheToken(ctx, validToken)
}

// Patch for test: redefine NewStrictMockTokenStore to return ErrTokenNotFound for unknown tokens

type StrictMockTokenStore struct {
	data map[string]TokenData
}

func NewStrictMockTokenStore() *StrictMockTokenStore {
	return &StrictMockTokenStore{data: make(map[string]TokenData)}
}

func (m *StrictMockTokenStore) AddToken(tokenID string, td TokenData) {
	m.data[tokenID] = td
}

func (m *StrictMockTokenStore) GetTokenByID(ctx context.Context, tokenID string) (TokenData, error) {
	td, ok := m.data[tokenID]
	if !ok {
		return TokenData{}, ErrTokenNotFound
	}
	return td, nil
}

// GetTokenByToken retrieves a token by its token string (for authentication)
func (m *StrictMockTokenStore) GetTokenByToken(ctx context.Context, tokenString string) (TokenData, error) {
	td, ok := m.data[tokenString]
	if !ok {
		return TokenData{}, ErrTokenNotFound
	}
	return td, nil
}

// Implement other methods as no-ops for compatibility
func (m *StrictMockTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	return nil
}
