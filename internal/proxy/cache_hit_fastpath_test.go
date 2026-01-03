package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestProxy_CacheHitAvoidsTrackingAndAPIKeyLookup(t *testing.T) {
	validator := &MockTokenValidator{}
	validator.On("ValidateToken", mock.Anything, "valid-token").Return("project-1", nil).Once()

	// Intentionally do NOT set expectations for ValidateTokenWithTracking.
	// If it is called, the mock will fail.

	store := &MockProjectStore{}
	// Intentionally do NOT set expectations for GetAPIKeyForProject.
	// A cache hit should not need upstream auth.

	cfg := ProxyConfig{
		TargetBaseURL:         "http://example.invalid",
		AllowedEndpoints:      []string{"/v1/"},
		AllowedMethods:        []string{http.MethodGet},
		HTTPCacheEnabled:      true,
		HTTPCacheDefaultTTL:   60,
		EnforceProjectActive:  false,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	p, err := NewTransparentProxyWithLogger(cfg, validator, store, zap.NewNop())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/v1/test", nil)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")

	key := CacheKeyFromRequest(req)
	p.cache.Set(key, cachedResponse{
		statusCode: http.StatusOK,
		headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"public, max-age=60"},
		},
		body:      []byte(`{"ok":true}`),
		expiresAt: time.Now().Add(1 * time.Minute),
	})

	rr := httptest.NewRecorder()
	p.Handler().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "llm-proxy; hit", rr.Header().Get("Cache-Status"))
	require.Equal(t, "hit", rr.Header().Get("X-PROXY-CACHE"))
	require.NotEmpty(t, rr.Header().Get("X-PROXY-CACHE-KEY"))
}

func TestProxy_POSTCacheHitAvoidsTracking(t *testing.T) {
	validator := &MockTokenValidator{}
	// First call during cache population should use tracking
	// Second call (cache hit) should use ValidateToken without tracking
	validator.On("ValidateToken", mock.Anything, "valid-token").Return("project-1", nil).Once()

	// Intentionally do NOT set expectations for ValidateTokenWithTracking on cache hit.
	// If it is called during the cache hit, the mock will fail.

	store := &MockProjectStore{}
	// Intentionally do NOT set expectations for GetAPIKeyForProject during cache hit.

	cfg := ProxyConfig{
		TargetBaseURL:           "http://example.invalid",
		AllowedEndpoints:        []string{"/v1/"},
		AllowedMethods:          []string{http.MethodPost},
		HTTPCacheEnabled:        true,
		HTTPCacheDefaultTTL:     60,
		HTTPCacheMaxObjectBytes: 1024 * 1024,
		EnforceProjectActive:    false,
		MaxIdleConns:            10,
		MaxIdleConnsPerHost:     10,
		IdleConnTimeout:         30 * time.Second,
		ResponseHeaderTimeout:   5 * time.Second,
	}

	p, err := NewTransparentProxyWithLogger(cfg, validator, store, zap.NewNop())
	require.NoError(t, err)

	// First request: populate the cache
	// This request will call ValidateTokenWithTracking (expected)
	reqBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"test"}]}`)
	req1 := httptest.NewRequest(http.MethodPost, "http://example.com/v1/chat/completions", bytes.NewReader(reqBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer valid-token")
	req1.Header.Set("Cache-Control", "public, max-age=300") // Client cache opt-in
	req1.ContentLength = int64(len(reqBody))

	// Pre-populate cache with this exact request
	// We compute the hash manually to create the cache key
	sum := sha256.Sum256(reqBody)
	req1.Header.Set("X-Body-Hash", hex.EncodeToString(sum[:]))
	key := CacheKeyFromRequest(req1)
	p.cache.Set(key, cachedResponse{
		statusCode: http.StatusOK,
		headers: http.Header{
			"Content-Type":  []string{"application/json"},
			"Cache-Control": []string{"public, max-age=300"},
		},
		body:      []byte(`{"id":"test","choices":[{"message":{"content":"cached"}}]}`),
		expiresAt: time.Now().Add(5 * time.Minute),
	})

	// Second request: hit the cache
	// Create a fresh request with the same body (do NOT set X-Body-Hash - let the proxy compute it)
	req2 := httptest.NewRequest(http.MethodPost, "http://example.com/v1/chat/completions", bytes.NewReader(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer valid-token")
	req2.Header.Set("Cache-Control", "public, max-age=300") // Client cache opt-in
	req2.ContentLength = int64(len(reqBody))

	// Make the second request - should hit cache without calling ValidateTokenWithTracking
	rr := httptest.NewRecorder()
	p.Handler().ServeHTTP(rr, req2)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "llm-proxy; hit", rr.Header().Get("Cache-Status"))
	require.Contains(t, rr.Body.String(), "cached")

	// Verify mock expectations were met (ValidateToken called once, ValidateTokenWithTracking never called)
	validator.AssertExpectations(t)
}
