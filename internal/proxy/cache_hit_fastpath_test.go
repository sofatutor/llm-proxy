package proxy

import (
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
