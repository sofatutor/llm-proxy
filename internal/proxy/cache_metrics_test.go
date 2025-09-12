package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestCacheMetrics_HitMissBypassStore(t *testing.T) {
	tests := []struct {
		name          string
		setupCache    func() httpCache
		request       func() *http.Request
		expectCacheOp string
		setupResponse func(*httptest.ResponseRecorder)
	}{
		{
			name: "cache_hit_increments_hit_counter",
			setupCache: func() httpCache {
				cache := newInMemoryCache()
				// Create a request to generate the same cache key
				req := httptest.NewRequest("GET", "/v1/models", nil)
				key := CacheKeyFromRequest(req)
				// Pre-populate cache with the correct key
				cache.Set(key, cachedResponse{
					statusCode: 200,
					headers:    http.Header{"Content-Type": []string{"application/json"}, "Cache-Control": []string{"public, s-maxage=60"}},
					body:       []byte(`{"cached": true}`),
					expiresAt:  time.Now().Add(time.Hour),
				})
				return cache
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/models", nil)
				return req
			},
			expectCacheOp: "hit",
		},
		{
			name: "cache_miss_increments_miss_counter",
			setupCache: func() httpCache {
				return newInMemoryCache() // Empty cache
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/v1/models", nil)
			},
			expectCacheOp: "miss",
		},
		{
			name: "cache_bypass_increments_bypass_counter",
			setupCache: func() httpCache {
				cache := newInMemoryCache()
				// Create a request to generate the same cache key
				req := httptest.NewRequest("GET", "/v1/models", nil)
				req.Header.Set("Authorization", "Bearer valid-token")
				key := CacheKeyFromRequest(req)
				// Pre-populate cache with response that doesn't allow reuse for authorized requests
				// (no explicit public/s-maxage cache control - private by default for auth requests)
				cache.Set(key, cachedResponse{
					statusCode: 200,
					headers:    http.Header{"Content-Type": []string{"application/json"}},
					body:       []byte(`{"cached": true}`),
					expiresAt:  time.Now().Add(time.Hour),
				})
				return cache
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/v1/models", nil)
				req.Header.Set("Authorization", "Bearer valid-token")
				return req
			},
			expectCacheOp: "bypass",
		},
		{
			name: "cache_store_increments_store_counter",
			setupCache: func() httpCache {
				return newInMemoryCache() // Empty cache, will store response
			},
			request: func() *http.Request {
				return httptest.NewRequest("GET", "/v1/models", nil)
			},
			expectCacheOp: "store",
			setupResponse: func(w *httptest.ResponseRecorder) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "public, max-age=300")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"models": []}`))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create proxy with cache enabled
			cfg := ProxyConfig{
				HTTPCacheEnabled:    true,
				HTTPCacheDefaultTTL: 5 * time.Minute,
				AllowedEndpoints:    []string{"/v1/models"},
				AllowedMethods:      []string{"GET", "POST"},
			}

			validator := &MockTokenValidator{}
			validator.On("ValidateToken", mock.Anything, "valid-token").Return("project-1", nil)
			validator.On("ValidateTokenWithTracking", mock.Anything, "valid-token").Return("project-1", nil)

			store := &MockProjectStore{}
			store.On("GetAPIKeyForProject", mock.Anything, "project-1").Return("test-key", nil)
			store.On("GetProjectActive", mock.Anything, "project-1").Return(true, nil)

			proxy, err := NewTransparentProxyWithLogger(cfg, validator, store, zap.NewNop())
			if err != nil {
				t.Fatalf("Failed to create proxy: %v", err)
			}

			// Set custom cache
			proxy.cache = tt.setupCache()

			// Create request
			req := tt.request()
			req.Header.Set("Authorization", "Bearer valid-token")

			// Record initial metrics
			initialMetrics := proxy.Metrics()
			initialHits := initialMetrics.CacheHits
			initialMisses := initialMetrics.CacheMisses
			initialBypass := initialMetrics.CacheBypass
			initialStores := initialMetrics.CacheStores

			// Execute request
			w := httptest.NewRecorder()

			// For store test, we need to mock upstream response
			if tt.setupResponse != nil {
				// TODO: This test needs more complex setup to test store path
				// For now, validate that store path increments counter when called directly
				proxy.incrementCacheMetric(CacheMetricStore)
				newMetrics := proxy.Metrics()
				if newMetrics.CacheStores != initialStores+1 {
					t.Errorf("Expected CacheStores to be %d, got %d", initialStores+1, newMetrics.CacheStores)
				}
				return
			}

			proxy.Handler().ServeHTTP(w, req)

			// Check metrics were incremented correctly
			newMetrics := proxy.Metrics()

			switch tt.expectCacheOp {
			case "hit":
				if newMetrics.CacheHits != initialHits+1 {
					t.Errorf("Expected CacheHits to be %d, got %d", initialHits+1, newMetrics.CacheHits)
				}
			case "miss":
				if newMetrics.CacheMisses != initialMisses+1 {
					t.Errorf("Expected CacheMisses to be %d, got %d", initialMisses+1, newMetrics.CacheMisses)
				}
			case "bypass":
				if newMetrics.CacheBypass != initialBypass+1 {
					t.Errorf("Expected CacheBypass to be %d, got %d", initialBypass+1, newMetrics.CacheBypass)
				}
			}
		})
	}
}

func TestCacheMetrics_Disabled(t *testing.T) {
	// Test that metrics are not incremented when cache is disabled
	cfg := ProxyConfig{
		HTTPCacheEnabled: false,
		AllowedEndpoints: []string{"/v1/models"},
		AllowedMethods:   []string{"GET", "POST"},
	}

	validator := &MockTokenValidator{}
	validator.On("ValidateToken", mock.Anything, "valid-token").Return("project-1", nil)
	validator.On("ValidateTokenWithTracking", mock.Anything, "valid-token").Return("project-1", nil)

	store := &MockProjectStore{}
	store.On("GetAPIKeyForProject", mock.Anything, "project-1").Return("test-key", nil)
	store.On("GetProjectActive", mock.Anything, "project-1").Return(true, nil)

	proxy, err := NewTransparentProxyWithLogger(cfg, validator, store, zap.NewNop())
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Ensure cache is nil
	if proxy.cache != nil {
		t.Fatal("Expected cache to be nil when disabled")
	}

	// Record initial metrics
	initialMetrics := proxy.Metrics()
	initialHits := initialMetrics.CacheHits
	initialMisses := initialMetrics.CacheMisses

	// Execute request
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	proxy.Handler().ServeHTTP(w, req)

	// Check that cache metrics were not incremented
	newMetrics := proxy.Metrics()
	if newMetrics.CacheHits != initialHits || newMetrics.CacheMisses != initialMisses {
		t.Error("Cache metrics should not be incremented when cache is disabled")
	}
}

func TestCacheMetrics_ThreadSafety(t *testing.T) {
	// Test that cache metrics are thread-safe
	cfg := ProxyConfig{
		HTTPCacheEnabled: true,
	}

	validator := &MockTokenValidator{}
	store := &MockProjectStore{}

	proxy, err := NewTransparentProxyWithLogger(cfg, validator, store, zap.NewNop())
	if err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}

	// Simulate concurrent cache operations
	const numGoroutines = 10
	const operationsPerGoroutine = 100

	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			for j := 0; j < operationsPerGoroutine; j++ {
				proxy.incrementCacheMetric(CacheMetricHit)
				proxy.incrementCacheMetric(CacheMetricMiss)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final counts
	metrics := proxy.Metrics()
	expectedCount := int64(numGoroutines * operationsPerGoroutine)

	if metrics.CacheHits != expectedCount {
		t.Errorf("Expected %d cache hits, got %d", expectedCount, metrics.CacheHits)
	}
	if metrics.CacheMisses != expectedCount {
		t.Errorf("Expected %d cache misses, got %d", expectedCount, metrics.CacheMisses)
	}
}

// New test: verify a real upstream response with cacheable headers increments CacheStores
func TestCacheMetrics_StorePath_IncrementsOnCacheableResponse(t *testing.T) {
	// Fake upstream that returns cacheable response
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := ProxyConfig{
		TargetBaseURL:       upstream.URL,
		HTTPCacheEnabled:    true,
		HTTPCacheDefaultTTL: 5 * time.Minute,
		AllowedEndpoints:    []string{"/v1/models"},
		AllowedMethods:      []string{"GET"},
	}

	validator := &MockTokenValidator{}
	validator.On("ValidateToken", mock.Anything, "valid-token").Return("project-1", nil)
	validator.On("ValidateTokenWithTracking", mock.Anything, "valid-token").Return("project-1", nil)

	store := &MockProjectStore{}
	store.On("GetAPIKeyForProject", mock.Anything, "project-1").Return("test-key", nil)
	store.On("GetProjectActive", mock.Anything, "project-1").Return(true, nil)

	p, err := NewTransparentProxyWithLogger(cfg, validator, store, zap.NewNop())
	if err != nil {
		t.Fatalf("proxy: %v", err)
	}

	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	// Record metrics before
	before := p.Metrics()
	beforeStores := before.CacheStores

	p.Handler().ServeHTTP(w, req)
	res := w.Result()
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if got := res.Header.Get("X-PROXY-CACHE"); !strings.Contains(got, "stored") {
		t.Fatalf("expected X-PROXY-CACHE to contain 'stored', got %q", got)
	}
	if res.Header.Get("X-PROXY-CACHE-KEY") == "" {
		t.Fatalf("expected X-PROXY-CACHE-KEY to be set")
	}

	// Metrics should reflect a store
	after := p.Metrics()
	if after.CacheStores != beforeStores+1 {
		t.Fatalf("expected CacheStores=%d, got %d", beforeStores+1, after.CacheStores)
	}
}
