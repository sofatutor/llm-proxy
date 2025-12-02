package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// TestSetCacheStatsAggregator tests the SetCacheStatsAggregator method.
func TestSetCacheStatsAggregator(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    100,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     10,
	}

	agg := NewCacheStatsAggregator(config, store, logger)

	// Create a minimal proxy config
	proxyConfig := ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedEndpoints: []string{"/v1/test"},
		AllowedMethods:   []string{"GET", "POST"},
	}

	// Create a minimal proxy
	proxy := &TransparentProxy{
		config:  proxyConfig,
		logger:  logger,
		metrics: &ProxyMetrics{},
	}

	// Initially nil
	if proxy.cacheStatsAggregator != nil {
		t.Error("expected cacheStatsAggregator to be nil initially")
	}

	// Set the aggregator
	proxy.SetCacheStatsAggregator(agg)

	// Verify it was set
	if proxy.cacheStatsAggregator == nil {
		t.Error("expected cacheStatsAggregator to be set")
	}
	if proxy.cacheStatsAggregator != agg {
		t.Error("expected cacheStatsAggregator to match the provided aggregator")
	}

	// Setting to nil should work
	proxy.SetCacheStatsAggregator(nil)
	if proxy.cacheStatsAggregator != nil {
		t.Error("expected cacheStatsAggregator to be nil after setting to nil")
	}
}

// TestRecordCacheHit tests the recordCacheHit helper method.
func TestRecordCacheHit(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    100,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     10,
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = agg.Stop(ctx)
	}()

	// Create a minimal proxy
	proxyConfig := ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedEndpoints: []string{"/v1/test"},
		AllowedMethods:   []string{"GET", "POST"},
	}

	proxy := &TransparentProxy{
		config:               proxyConfig,
		logger:               logger,
		metrics:              &ProxyMetrics{},
		cacheStatsAggregator: agg,
	}

	t.Run("with token in context", func(t *testing.T) {
		// Create a request with token ID in context
		req := httptest.NewRequest("GET", "/v1/test", nil)
		ctx := context.WithValue(req.Context(), ctxKeyTokenID, "test-token-123")
		req = req.WithContext(ctx)

		// Record cache hit
		proxy.recordCacheHit(req)

		// Verify metric was incremented
		if proxy.metrics.CacheHits != 1 {
			t.Errorf("expected CacheHits to be 1, got %d", proxy.metrics.CacheHits)
		}

		// Wait for aggregator to process
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("without token in context", func(t *testing.T) {
		// Reset metrics
		proxy.metrics.CacheHits = 0

		// Create a request without token ID in context
		req := httptest.NewRequest("GET", "/v1/test", nil)

		// Record cache hit
		proxy.recordCacheHit(req)

		// Verify metric was incremented
		if proxy.metrics.CacheHits != 1 {
			t.Errorf("expected CacheHits to be 1, got %d", proxy.metrics.CacheHits)
		}

		// The aggregator should NOT have received an event (no token)
	})

	t.Run("with empty token in context", func(t *testing.T) {
		// Reset metrics
		proxy.metrics.CacheHits = 0

		// Create a request with empty token ID in context
		req := httptest.NewRequest("GET", "/v1/test", nil)
		ctx := context.WithValue(req.Context(), ctxKeyTokenID, "")
		req = req.WithContext(ctx)

		// Record cache hit
		proxy.recordCacheHit(req)

		// Verify metric was incremented
		if proxy.metrics.CacheHits != 1 {
			t.Errorf("expected CacheHits to be 1, got %d", proxy.metrics.CacheHits)
		}

		// The aggregator should NOT have received an event (empty token)
	})

	t.Run("without aggregator", func(t *testing.T) {
		// Create a proxy without aggregator
		proxyNoAgg := &TransparentProxy{
			config:  proxyConfig,
			logger:  logger,
			metrics: &ProxyMetrics{},
		}

		req := httptest.NewRequest("GET", "/v1/test", nil)
		ctx := context.WithValue(req.Context(), ctxKeyTokenID, "test-token-456")
		req = req.WithContext(ctx)

		// Record cache hit - should not panic
		proxyNoAgg.recordCacheHit(req)

		// Verify metric was incremented
		if proxyNoAgg.metrics.CacheHits != 1 {
			t.Errorf("expected CacheHits to be 1, got %d", proxyNoAgg.metrics.CacheHits)
		}
	})
}

// TestRecordCacheHit_AggregatorRecordsTokenHit verifies that cache hits are recorded to the aggregator.
func TestRecordCacheHit_AggregatorRecordsTokenHit(t *testing.T) {
	store := &mockCacheStatsStore{}
	logger := zaptest.NewLogger(t)

	config := CacheStatsAggregatorConfig{
		BufferSize:    100,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     5, // Small batch size to trigger flush
	}

	agg := NewCacheStatsAggregator(config, store, logger)
	agg.Start()

	proxyConfig := ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedEndpoints: []string{"/v1/test"},
		AllowedMethods:   []string{"GET", "POST"},
	}

	proxy := &TransparentProxy{
		config:               proxyConfig,
		logger:               logger,
		metrics:              &ProxyMetrics{},
		cacheStatsAggregator: agg,
	}

	// Record 5 cache hits for the same token (should trigger batch flush)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/v1/test", nil)
		ctx := context.WithValue(req.Context(), ctxKeyTokenID, "aggregator-test-token")
		req = req.WithContext(ctx)
		proxy.recordCacheHit(req)
	}

	// Wait for flush
	time.Sleep(100 * time.Millisecond)

	// Stop aggregator to ensure final flush
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := agg.Stop(ctx); err != nil {
		t.Fatalf("failed to stop aggregator: %v", err)
	}

	// Verify the store received the batch
	calls := store.getCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one flush call to the store")
	}

	// Count total hits for the token
	totalHits := 0
	for _, call := range calls {
		totalHits += call["aggregator-test-token"]
	}

	if totalHits != 5 {
		t.Errorf("expected 5 cache hits recorded for token, got %d", totalHits)
	}
}

// TestTokenIDContextKey tests that the token ID context key is properly defined.
func TestTokenIDContextKey(t *testing.T) {
	// Create a request and add token ID to context
	req, _ := http.NewRequest("GET", "/test", nil)
	tokenID := "test-token-abc123"
	ctx := context.WithValue(req.Context(), ctxKeyTokenID, tokenID)
	req = req.WithContext(ctx)

	// Retrieve token ID from context
	retrieved, ok := req.Context().Value(ctxKeyTokenID).(string)
	if !ok {
		t.Error("expected to retrieve token ID from context")
	}
	if retrieved != tokenID {
		t.Errorf("expected token ID %q, got %q", tokenID, retrieved)
	}
}
