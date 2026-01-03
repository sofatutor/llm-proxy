package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
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
	// Only expect ValidateToken (no tracking) to be called during cache hit.
	// The cache is manually pre-populated, so no actual upstream request occurs.
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

	// Manually pre-populate cache (no actual HTTP request made)
	// This simulates a cached response from a previous request
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

func TestPrepareBodyHashForCaching(t *testing.T) {
	logger := zap.NewNop()
	maxBytes := int64(1024)

	t.Run("nil_body", func(t *testing.T) {
		req := &http.Request{Body: nil, ContentLength: 10}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.False(t, result)
	})

	t.Run("negative_content_length", func(t *testing.T) {
		body := bytes.NewReader([]byte("test"))
		req := &http.Request{Body: http.NoBody, ContentLength: -1}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.False(t, result)
		require.NotNil(t, req.Body)
		_ = body // avoid unused warning
	})

	t.Run("content_length_exceeds_max", func(t *testing.T) {
		body := bytes.NewReader([]byte("test"))
		req := &http.Request{Body: io.NopCloser(body), ContentLength: maxBytes + 1}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.False(t, result)
	})

	t.Run("successful_hash", func(t *testing.T) {
		bodyContent := []byte("test body")
		closed := false
		body := &closeTrackingReadCloser{r: bytes.NewReader(bodyContent), closed: &closed}
		req := &http.Request{
			Body:          body,
			ContentLength: int64(len(bodyContent)),
			Header:        http.Header{},
		}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.True(t, result)

		// Verify hash was set correctly
		expectedHash := sha256.Sum256(bodyContent)
		require.Equal(t, hex.EncodeToString(expectedHash[:]), req.Header.Get("X-Body-Hash"))

		// Verify body was restored and readable
		restoredBody, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, bodyContent, restoredBody)

		require.NoError(t, req.Body.Close())
		require.True(t, closed)
	})

	t.Run("body_exceeds_limit_during_read", func(t *testing.T) {
		// Body claims to be small but is actually larger than maxBytes
		bodyContent := make([]byte, maxBytes+100)
		for i := range bodyContent {
			bodyContent[i] = byte('a')
		}
		closed := false
		body := &closeTrackingReadCloser{r: bytes.NewReader(bodyContent), closed: &closed}
		req := &http.Request{
			Body:          body,
			ContentLength: 100, // Lies about size
			Header:        http.Header{},
		}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.False(t, result)
		require.Empty(t, req.Header.Get("X-Body-Hash"))

		// Verify body was fully restored (bytes we consumed + unread remainder)
		restoredBody, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, bodyContent, restoredBody)

		require.NoError(t, req.Body.Close())
		require.True(t, closed)
	})

	t.Run("read_error_restores_body", func(t *testing.T) {
		// Create a reader that returns an error after some bytes
		errorReader := &errorAfterNBytesReader{
			data:   []byte("partial"),
			n:      7,
			count:  0,
			closed: false,
		}
		req := &http.Request{
			Body:          errorReader,
			ContentLength: 100,
			Header:        http.Header{},
		}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.False(t, result)
		require.Empty(t, req.Header.Get("X-Body-Hash"))

		// Verify body was restored with whatever bytes were read before the error.
		restoredBody, err := io.ReadAll(req.Body)
		require.Error(t, err)
		require.Equal(t, []byte("partial"), restoredBody)

		require.NoError(t, req.Body.Close())
		require.True(t, errorReader.closed)
	})

	t.Run("exact_max_bytes", func(t *testing.T) {
		bodyContent := make([]byte, maxBytes)
		for i := range bodyContent {
			bodyContent[i] = byte('b')
		}
		body := bytes.NewReader(bodyContent)
		req := &http.Request{
			Body:          io.NopCloser(body),
			ContentLength: maxBytes,
			Header:        http.Header{},
		}
		result := prepareBodyHashForCaching(req, maxBytes, logger)
		require.True(t, result)

		// Verify hash was set
		require.NotEmpty(t, req.Header.Get("X-Body-Hash"))
	})
}

type closeTrackingReadCloser struct {
	r      *bytes.Reader
	closed *bool
}

func (r *closeTrackingReadCloser) Read(p []byte) (int, error) { return r.r.Read(p) }
func (r *closeTrackingReadCloser) Close() error {
	*r.closed = true
	return nil
}

// errorAfterNBytesReader is a helper that returns data then an error
type errorAfterNBytesReader struct {
	data   []byte
	n      int
	count  int
	closed bool
}

func (r *errorAfterNBytesReader) Read(p []byte) (n int, err error) {
	if r.count >= r.n {
		return 0, io.ErrUnexpectedEOF
	}
	n = copy(p, r.data[r.count:])
	r.count += n
	if r.count >= r.n {
		return n, io.ErrUnexpectedEOF
	}
	return n, nil
}

func (r *errorAfterNBytesReader) Close() error {
	r.closed = true
	return nil
}
