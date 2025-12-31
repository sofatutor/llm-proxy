package token

import (
	"context"
	"sync"
	"testing"
	"time"
)

type mockUsageStatsStore struct {
	mu         sync.Mutex
	called     int
	lastDeltas map[string]int
	ch         chan struct{}
}

func newMockUsageStatsStore() *mockUsageStatsStore {
	return &mockUsageStatsStore{ch: make(chan struct{}, 1)}
}

func (m *mockUsageStatsStore) IncrementTokenUsageBatch(ctx context.Context, deltas map[string]int, lastUsedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.called++
	m.lastDeltas = make(map[string]int, len(deltas))
	for k, v := range deltas {
		m.lastDeltas[k] = v
	}
	select {
	case m.ch <- struct{}{}:
	default:
	}
	return nil
}

type countingTokenStore struct {
	tokens map[string]TokenData

	mu                  sync.Mutex
	getByTokenCalls     int
	incrementUsageCalls int
}

func newCountingTokenStore() *countingTokenStore {
	return &countingTokenStore{tokens: make(map[string]TokenData)}
}

func (s *countingTokenStore) GetTokenByID(ctx context.Context, id string) (TokenData, error) {
	return TokenData{}, ErrTokenNotFound
}

func (s *countingTokenStore) GetTokenByToken(ctx context.Context, tokenString string) (TokenData, error) {
	s.mu.Lock()
	s.getByTokenCalls++
	s.mu.Unlock()
	if td, ok := s.tokens[tokenString]; ok {
		return td, nil
	}
	return TokenData{}, ErrTokenNotFound
}

func (s *countingTokenStore) IncrementTokenUsage(ctx context.Context, tokenString string) error {
	s.mu.Lock()
	s.incrementUsageCalls++
	s.mu.Unlock()
	return nil
}

func (s *countingTokenStore) CreateToken(ctx context.Context, token TokenData) error { return nil }
func (s *countingTokenStore) UpdateToken(ctx context.Context, token TokenData) error { return nil }
func (s *countingTokenStore) ListTokens(ctx context.Context) ([]TokenData, error)    { return nil, nil }
func (s *countingTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]TokenData, error) {
	return nil, nil
}

func TestStandardValidator_ValidateTokenWithTracking_Unlimited_UsesAsyncAggregator(t *testing.T) {
	store := newCountingTokenStore()
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	store.tokens[tokenString] = TokenData{Token: tokenString, ProjectID: "p1", IsActive: true}

	usageStore := newMockUsageStatsStore()
	agg := NewUsageStatsAggregator(UsageStatsAggregatorConfig{BufferSize: 10, FlushInterval: 10 * time.Millisecond, BatchSize: 1}, usageStore, nil)
	agg.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = agg.Stop(ctx)
	})

	validator := NewValidator(store)
	validator.SetUsageStatsAggregator(agg)

	projectID, err := validator.ValidateTokenWithTracking(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() error = %v", err)
	}
	if projectID != "p1" {
		t.Fatalf("projectID = %q, want %q", projectID, "p1")
	}

	select {
	case <-usageStore.ch:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for async usage flush")
	}

	store.mu.Lock()
	incCalls := store.incrementUsageCalls
	store.mu.Unlock()
	if incCalls != 0 {
		t.Fatalf("IncrementTokenUsage calls = %d, want 0 (async)", incCalls)
	}

	usageStore.mu.Lock()
	delta := usageStore.lastDeltas[tokenString]
	usageStore.mu.Unlock()
	if delta != 1 {
		t.Fatalf("usage delta = %d, want 1", delta)
	}
}

func TestStandardValidator_ValidateTokenWithTracking_MaxRequestsZero_UsesAsyncAggregator(t *testing.T) {
	store := newCountingTokenStore()
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	max := 0
	store.tokens[tokenString] = TokenData{Token: tokenString, ProjectID: "p1", IsActive: true, MaxRequests: &max}

	usageStore := newMockUsageStatsStore()
	agg := NewUsageStatsAggregator(UsageStatsAggregatorConfig{BufferSize: 10, FlushInterval: 10 * time.Millisecond, BatchSize: 1}, usageStore, nil)
	agg.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = agg.Stop(ctx)
	})

	validator := NewValidator(store)
	validator.SetUsageStatsAggregator(agg)

	projectID, err := validator.ValidateTokenWithTracking(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() error = %v", err)
	}
	if projectID != "p1" {
		t.Fatalf("projectID = %q, want %q", projectID, "p1")
	}

	select {
	case <-usageStore.ch:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for async usage flush")
	}

	store.mu.Lock()
	incCalls := store.incrementUsageCalls
	store.mu.Unlock()
	if incCalls != 0 {
		t.Fatalf("IncrementTokenUsage calls = %d, want 0 (async)", incCalls)
	}

	usageStore.mu.Lock()
	delta := usageStore.lastDeltas[tokenString]
	usageStore.mu.Unlock()
	if delta != 1 {
		t.Fatalf("usage delta = %d, want 1", delta)
	}
}

func TestStandardValidator_ValidateTokenWithTracking_Limited_IsSynchronous(t *testing.T) {
	store := newCountingTokenStore()
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	max := 10
	store.tokens[tokenString] = TokenData{Token: tokenString, ProjectID: "p1", IsActive: true, MaxRequests: &max, RequestCount: 0}

	usageStore := newMockUsageStatsStore()
	agg := NewUsageStatsAggregator(UsageStatsAggregatorConfig{BufferSize: 10, FlushInterval: 10 * time.Millisecond, BatchSize: 1}, usageStore, nil)
	agg.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = agg.Stop(ctx)
	})

	validator := NewValidator(store)
	validator.SetUsageStatsAggregator(agg)

	_, err = validator.ValidateTokenWithTracking(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() error = %v", err)
	}

	store.mu.Lock()
	incCalls := store.incrementUsageCalls
	store.mu.Unlock()
	if incCalls != 1 {
		t.Fatalf("IncrementTokenUsage calls = %d, want 1", incCalls)
	}
}

func TestCachedValidator_ValidateTokenWithTracking_UnlimitedCached_DoesNotInvalidateCache(t *testing.T) {
	store := newCountingTokenStore()
	tokenString, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	store.tokens[tokenString] = TokenData{Token: tokenString, ProjectID: "p1", IsActive: true}

	usageStore := newMockUsageStatsStore()
	agg := NewUsageStatsAggregator(UsageStatsAggregatorConfig{BufferSize: 10, FlushInterval: 10 * time.Millisecond, BatchSize: 1}, usageStore, nil)
	agg.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = agg.Stop(ctx)
	})

	validator := NewValidator(store)
	validator.SetUsageStatsAggregator(agg)
	cv := NewCachedValidator(validator, CacheOptions{TTL: time.Minute, MaxSize: 100, EnableCleanup: false})

	// Populate cache.
	_, err = cv.ValidateToken(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	store.mu.Lock()
	getCallsBefore := store.getByTokenCalls
	store.mu.Unlock()

	_, err = cv.ValidateTokenWithTracking(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() error = %v", err)
	}

	select {
	case <-usageStore.ch:
		// ok
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for async usage flush")
	}

	hits, misses, evictions, _ := cv.GetCacheStats()
	if hits != 1 {
		t.Fatalf("cache hits = %d, want 1", hits)
	}
	if misses != 1 {
		t.Fatalf("cache misses = %d, want 1", misses)
	}
	if evictions != 0 {
		t.Fatalf("cache evictions = %d, want 0", evictions)
	}

	store.mu.Lock()
	getCallsAfter := store.getByTokenCalls
	incCalls := store.incrementUsageCalls
	store.mu.Unlock()

	if incCalls != 0 {
		t.Fatalf("IncrementTokenUsage calls = %d, want 0 (async)", incCalls)
	}
	if getCallsAfter != getCallsBefore {
		t.Fatalf("GetTokenByToken calls increased (%d -> %d), want unchanged for cached unlimited", getCallsBefore, getCallsAfter)
	}

	// Expired cache entries should be treated as misses and evicted.
	cv.cacheMutex.Lock()
	entry := cv.cache[tokenString]
	entry.ValidUntil = time.Now().Add(-time.Second)
	cv.cache[tokenString] = entry
	cv.cacheMutex.Unlock()

	_, err = cv.ValidateTokenWithTracking(context.Background(), tokenString)
	if err != nil {
		t.Fatalf("ValidateTokenWithTracking() after expiry error = %v", err)
	}

	hits, misses, evictions, _ = cv.GetCacheStats()
	if hits != 1 {
		t.Fatalf("cache hits = %d, want 1", hits)
	}
	if misses != 2 {
		t.Fatalf("cache misses = %d, want 2", misses)
	}
	if evictions != 1 {
		t.Fatalf("cache evictions = %d, want 1", evictions)
	}
}
