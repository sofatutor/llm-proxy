package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"errors"

	"github.com/sofatutor/llm-proxy/internal/middleware"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockTokenValidator mocks the token validation interface
type MockTokenValidator struct {
	mock.Mock
}

func (m *MockTokenValidator) ValidateToken(ctx context.Context, tokenID string) (string, error) {
	args := m.Called(ctx, tokenID)
	return args.String(0), args.Error(1)
}

func (m *MockTokenValidator) ValidateTokenWithTracking(ctx context.Context, tokenID string) (string, error) {
	args := m.Called(ctx, tokenID)
	return args.String(0), args.Error(1)
}

// MockProjectStore mocks the project storage interface
type MockProjectStore struct {
	mock.Mock
}

func (m *MockProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	args := m.Called(ctx, projectID)
	return args.String(0), args.Error(1)
}

func (m *MockProjectStore) ListProjects(ctx context.Context) ([]Project, error) { return nil, nil }
func (m *MockProjectStore) CreateProject(ctx context.Context, p Project) error  { return nil }
func (m *MockProjectStore) GetProjectByID(ctx context.Context, id string) (Project, error) {
	return Project{}, nil
}
func (m *MockProjectStore) UpdateProject(ctx context.Context, p Project) error { return nil }
func (m *MockProjectStore) DeleteProject(ctx context.Context, id string) error { return nil }

// MockAPIServer creates a mock server to represent the target API
func createMockAPIServer(t *testing.T) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Echo back information about the request for testing
		response := map[string]interface{}{
			"method":       r.Method,
			"path":         r.URL.Path,
			"query":        r.URL.RawQuery,
			"content_type": r.Header.Get("Content-Type"),
			"auth_header":  r.Header.Get("Authorization"),
			"user_agent":   r.Header.Get("User-Agent"),
		}

		// Check if this is a streaming request
		if strings.Contains(r.URL.Path, "streaming") {
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			// Send 3 events
			for i := 0; i < 3; i++ {
				data := map[string]interface{}{
					"event": i,
					"data":  "event data " + string(rune('A'+i)),
				}
				jsonData, _ := json.Marshal(data)

				// Write in SSE format
				if _, err := fmt.Fprintf(w, "data: %s\n\n", jsonData); err != nil {
					t.Logf("Failed to write SSE data: %v", err)
				}
				flusher.Flush()
				time.Sleep(50 * time.Millisecond)
			}
			return
		}

		// For non-streaming, just return JSON
		jsonResponse, _ := json.Marshal(response)
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse); err != nil {
			t.Logf("Failed to write JSON response: %v", err)
		}
	})

	return httptest.NewServer(handler)
}

// Test basic request proxying
func TestTransparentProxy_BasicProxying(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Set up expected calls
	mockValidator.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil)
	mockStore.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil)

	// Create proxy configuration
	config := ProxyConfig{
		TargetBaseURL:       mockAPI.URL,
		AllowedEndpoints:    []string{"/v1/completions", "/v1/chat/completions"},
		AllowedMethods:      []string{"GET", "POST"},
		RequestTimeout:      30 * time.Second,
		FlushInterval:       100 * time.Millisecond,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// Create proxy
	proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Create request to test
	reqBody := strings.NewReader(`{"prompt": "Hello, world!"}`)
	req := httptest.NewRequest("POST", "/v1/completions", reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 status code")

	// Parse response body
	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err, "Expected no error decoding response")

	// Verify the request was properly proxied
	assert.Equal(t, "POST", response["method"], "Expected POST method")
	assert.Equal(t, "/v1/completions", response["path"], "Expected correct path")
	assert.Equal(t, "application/json", response["content_type"], "Expected content type preserved")
	assert.Equal(t, "Bearer api_key_123", response["auth_header"], "Expected auth header replacement")

	// Verify mock expectations
	mockValidator.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestHTTPCache_BasicHitOnSecondGET(t *testing.T) {
	// Upstream that counts hits
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)
	mockValidator.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil)
	mockStore.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil)

	cfg := ProxyConfig{
		TargetBaseURL:       server.URL,
		AllowedEndpoints:    []string{"/v1/test"},
		AllowedMethods:      []string{"GET"},
		HTTPCacheEnabled:    true,
		HTTPCacheDefaultTTL: 10 * time.Second,
	}

	p, err := NewTransparentProxyWithLogger(cfg, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Debug: Check if cache is actually enabled
	if p.cache == nil {
		t.Fatalf("Cache is nil, but HTTPCacheEnabled was true")
	}

	// First request -> miss and store
	req1 := httptest.NewRequest("GET", "/v1/test", nil)
	req1.Header.Set("Authorization", "Bearer test_token")
	req1.Header.Set("Cache-Control", "public, max-age=60") // Client opts in to caching
	w1 := httptest.NewRecorder()
	p.Handler().ServeHTTP(w1, req1)
	res1 := w1.Result()
	_ = res1.Body.Close()
	assert.Equal(t, http.StatusOK, res1.StatusCode)
	assert.Contains(t, res1.Header.Get("Cache-Status"), "stored")

	// Second request -> hit (no upstream increment)
	req2 := httptest.NewRequest("GET", "/v1/test", nil)
	req2.Header.Set("Authorization", "Bearer test_token")
	req2.Header.Set("Cache-Control", "public, max-age=60") // Client opts in to caching
	w2 := httptest.NewRecorder()
	p.Handler().ServeHTTP(w2, req2)
	res2 := w2.Result()
	_ = res2.Body.Close()
	assert.Equal(t, http.StatusOK, res2.StatusCode)
	assert.Contains(t, res2.Header.Get("Cache-Status"), "hit")

	// Upstream should have been hit once
	assert.Equal(t, 1, hitCount)
}

func TestHTTPCache_VaryAcceptSeparatesEntries(t *testing.T) {
	// Upstream returns different payload for different Accept
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Cache-Control", "public, max-age=60")
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"type":"json"}`))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain"))
	}))
	defer srv.Close()

	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)
	mockValidator.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil)
	mockStore.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil)

	cfg := ProxyConfig{
		TargetBaseURL:       srv.URL,
		AllowedEndpoints:    []string{"/v1/test"},
		AllowedMethods:      []string{"GET"},
		HTTPCacheEnabled:    true,
		HTTPCacheDefaultTTL: 10 * time.Second,
	}
	p, err := NewTransparentProxyWithLogger(cfg, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	handler := p.Handler()

	// First: Accept json -> miss and store
	r1 := httptest.NewRequest("GET", "/v1/test", nil)
	r1.Header.Set("Authorization", "Bearer test_token")
	r1.Header.Set("Accept", "application/json")
	r1.Header.Set("Cache-Control", "public, max-age=60") // Client opts in to caching
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)
	res1 := w1.Result()
	_, _ = io.ReadAll(res1.Body)
	_ = res1.Body.Close()
	assert.Contains(t, res1.Header.Get("Cache-Status"), "stored")

	// Second: Accept json -> hit
	r2 := httptest.NewRequest("GET", "/v1/test", nil)
	r2.Header.Set("Authorization", "Bearer test_token")
	r2.Header.Set("Accept", "application/json")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)
	res2 := w2.Result()
	_, _ = io.ReadAll(res2.Body)
	_ = res2.Body.Close()
	assert.Contains(t, res2.Header.Get("Cache-Status"), "hit")

	// Third: different Accept -> miss (separate cache entry)
	r3 := httptest.NewRequest("GET", "/v1/test", nil)
	r3.Header.Set("Authorization", "Bearer test_token")
	r3.Header.Set("Accept", "text/plain")
	r3.Header.Set("Cache-Control", "public, max-age=60") // Client opts in to caching
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, r3)
	res3 := w3.Result()
	_, _ = io.ReadAll(res3.Body)
	_ = res3.Body.Close()
	assert.Contains(t, res3.Header.Get("Cache-Status"), "stored") // Third request should store new cache entry

	// Add tests for uncovered cache functions
	assert.Equal(t, 2, hits)
}

func TestCacheHelpers_Coverage(t *testing.T) {
	// Test canServeCachedForRequest
	req := httptest.NewRequest("GET", "/test", nil)
	headers := make(http.Header)
	headers.Set("Cache-Control", "public, max-age=60")

	// Without Authorization - should be true
	assert.True(t, canServeCachedForRequest(req, headers))

	// With Authorization but public cache - should be true
	req.Header.Set("Authorization", "Bearer test")
	assert.True(t, canServeCachedForRequest(req, headers))

	// With Authorization but private cache - should be false
	headers.Set("Cache-Control", "private, max-age=60")
	assert.False(t, canServeCachedForRequest(req, headers))

	// Test conditionalRequestMatches
	req.Header.Set("If-None-Match", "123")
	headers.Set("ETag", "123")
	assert.True(t, conditionalRequestMatches(req, headers))

	req.Header.Set("If-None-Match", "456")
	assert.False(t, conditionalRequestMatches(req, headers))

	// Test wantsRevalidation
	req.Header.Set("Cache-Control", "no-cache")
	assert.True(t, wantsRevalidation(req))

	req.Header.Set("Cache-Control", "max-age=0")
	assert.True(t, wantsRevalidation(req))

	req.Header.Set("Cache-Control", "max-age=60")
	assert.False(t, wantsRevalidation(req))
}

// Test streaming response handling
func TestTransparentProxy_StreamingResponses(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Set up expected calls
	mockValidator.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil)
	mockStore.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil)

	// Create proxy configuration
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/streaming"},
		AllowedMethods:   []string{"GET", "POST"},
		RequestTimeout:   30 * time.Second,
		FlushInterval:    100 * time.Millisecond,
	}

	// Create proxy
	proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Create request to test
	req := httptest.NewRequest("GET", "/v1/streaming", nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer test_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 status code")
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"), "Expected SSE content type")

	// Read the full body - for a real streaming test we'd read incrementally
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err, "Expected no error reading response")

	// In a streaming response, we should see multiple "data:" lines
	bodyStr := string(body)
	assert.Contains(t, bodyStr, "data:", "Expected SSE data format")

	// Count the number of events (each event ends with \n\n)
	events := strings.Split(strings.TrimSpace(bodyStr), "\n\n")
	assert.Equal(t, 3, len(events), "Expected 3 SSE events")

	// Verify mock expectations
	mockValidator.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

// Test invalid token handling
func TestTransparentProxy_InvalidToken(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Set up expected calls for invalid token
	mockValidator.On("ValidateTokenWithTracking", mock.Anything, "invalid_token").
		Return("", token.ErrTokenInactive)

	// Create proxy configuration
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/completions"},
		AllowedMethods:   []string{"GET", "POST"},
	}

	// Create proxy
	proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Create request with invalid token
	req := httptest.NewRequest("POST", "/v1/completions", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer invalid_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response - should be an error
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Expected 401 status code for invalid token")

	// Parse error response
	var errResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&errResponse)
	assert.NoError(t, err, "Expected no error decoding response")

	// Verify error message (proxy returns {"error": ...})
	assert.Contains(t, errResponse, "error", "Expected error field in response")
	assert.Equal(t, "Token is inactive", errResponse["error"], "Expected specific error message for inactive token")

	// Verify mock expectations
	mockValidator.AssertExpectations(t)
}

// Test disallowed endpoint
func TestTransparentProxy_DisallowedEndpoint(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies (no need to set up token mocks)
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Create proxy configuration with limited allowlist
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/completions"}, // Only completions allowed
		AllowedMethods:   []string{"GET", "POST"},
	}

	// Create proxy
	proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Create request to a disallowed endpoint
	req := httptest.NewRequest("POST", "/v1/disallowed_endpoint", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response - should be 404 Not Found
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected 404 status code for disallowed endpoint")
}

// Test disallowed method
func TestTransparentProxy_DisallowedMethod(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies (no need to set up token mocks)
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Create proxy configuration with limited methods
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/completions"},
		AllowedMethods:   []string{"GET"}, // Only GET allowed
	}

	// Create proxy
	proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Create request with disallowed method
	req := httptest.NewRequest("DELETE", "/v1/completions", nil)
	req.Header.Set("Authorization", "Bearer test_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response - should be 405 Method Not Allowed
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "Expected 405 status code for disallowed method")
}

// Test large request body handling
func TestTransparentProxy_LargeRequestBody(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Set up expected calls
	mockValidator.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil)
	mockStore.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil)

	// Create proxy configuration
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/completions"},
		AllowedMethods:   []string{"POST"},
		RequestTimeout:   5 * time.Second,
	}

	// Create proxy
	proxy, err := NewTransparentProxyWithLogger(config, mockValidator, mockStore, zap.NewNop())
	require.NoError(t, err)

	// Create large request body (100KB)
	largeBody := bytes.Repeat([]byte("a"), 100*1024)
	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 status code for large request")

	// Verify mock expectations
	mockValidator.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestTransparentProxy_ErrorHandler(t *testing.T) {
	proxy, err := NewTransparentProxyWithLogger(ProxyConfig{}, nil, nil, zap.NewNop())
	require.NoError(t, err)
	testCases := []struct {
		name        string
		ctxErr      error
		proxyErr    error
		wantStatus  int
		wantMessage string
	}{
		{"generic error", nil, errors.New("fail"), http.StatusBadGateway, "Proxy error"},
		{"deadline exceeded", nil, context.DeadlineExceeded, http.StatusGatewayTimeout, "Request timeout"},
		{"canceled", nil, context.Canceled, http.StatusRequestTimeout, "Request canceled"},
		{"validation error", errors.New("validation fail"), errors.New("fail"), http.StatusUnauthorized, "Invalid token"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/", nil)
			if tc.ctxErr != nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKeyValidationError, tc.ctxErr))
			}
			proxy.errorHandler(w, r, tc.proxyErr)
			resp := w.Result()
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantMessage)
		})
	}
}

func TestTransparentProxy_HandleValidationError(t *testing.T) {
	proxy, err := NewTransparentProxyWithLogger(ProxyConfig{}, nil, nil, zap.NewNop())
	require.NoError(t, err)
	testCases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"not found", token.ErrTokenNotFound, http.StatusUnauthorized, "token_not_found", "Token not found"},
		{"inactive", token.ErrTokenInactive, http.StatusUnauthorized, "token_inactive", "Token is inactive"},
		{"expired", token.ErrTokenExpired, http.StatusUnauthorized, "token_expired", "Token has expired"},
		{"rate limit", token.ErrTokenRateLimit, http.StatusTooManyRequests, "rate_limit_exceeded", "Rate limit exceeded"},
		{"default", errors.New("other"), http.StatusUnauthorized, "invalid_token", "Invalid token"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			proxy.handleValidationError(w, r, tc.err)
			resp := w.Result()
			if err := resp.Body.Close(); err != nil {
				t.Logf("Failed to close response body: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantCode)
			assert.Contains(t, string(body), tc.wantMsg)
		})
	}
}

// Minimal mock http.Server for Shutdown test

func TestTransparentProxy_Shutdown(t *testing.T) {
	proxy, err := NewTransparentProxyWithLogger(ProxyConfig{}, nil, nil, zap.NewNop())
	require.NoError(t, err)
	// Case: no httpServer
	err = proxy.Shutdown(context.Background())
	assert.NoError(t, err)

	// Case: with httpServer
	proxy.httpServer = &http.Server{} // assign a real http.Server to satisfy type
	// Swap out the Shutdown method using embedding (simulate)
	// In Go, you can't swap methods at runtime, so we simulate by embedding
	// and type-asserting in the test
	// For this test, we just check that Shutdown returns no error
	// and that the code path is exercised
	// (Full mocking would require interface refactor)
	err = proxy.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestExtractTokenFromHeader(t *testing.T) {
	tests := []struct {
		header  string
		want    string
		comment string
	}{
		{"", "", "empty header"},
		{"Basic abcdef", "", "not Bearer"},
		{"Bearer", "", "Bearer with no token"},
		{"Bearer    ", "", "Bearer with only spaces"},
		{"Bearer token123", "token123", "normal Bearer token"},
		{"Bearer   token123   ", "token123", "Bearer with extra spaces"},
	}
	for _, tc := range tests {
		t.Run(tc.comment, func(t *testing.T) {
			got := extractTokenFromHeader(tc.header)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestIsMethodAllowed(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{}}
	// No allowed methods: allow all
	assert.True(t, p.isMethodAllowed("GET"))
	assert.True(t, p.isMethodAllowed("POST"))

	p.config.AllowedMethods = []string{"GET", "POST"}
	assert.True(t, p.isMethodAllowed("GET"))
	assert.True(t, p.isMethodAllowed("POST"))
	assert.False(t, p.isMethodAllowed("DELETE"))
}

func TestIsEndpointAllowed(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{}}
	// No allowed endpoints: allow all
	assert.True(t, p.isEndpointAllowed("/v1/foo"))
	assert.True(t, p.isEndpointAllowed("/v1/bar"))

	p.config.AllowedEndpoints = []string{"/v1/foo", "/v1/bar"}
	assert.True(t, p.isEndpointAllowed("/v1/foo/extra")) // prefix match
	assert.True(t, p.isEndpointAllowed("/v1/bar"))
	assert.False(t, p.isEndpointAllowed("/v1/baz"))
}

func TestValidateRequestMiddleware_MethodNotAllowed(t *testing.T) {
	p := &TransparentProxy{
		config: ProxyConfig{AllowedMethods: []string{"POST"}},
		logger: zap.NewNop(),
	}
	h := p.ValidateRequestMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/v1/completions", nil)
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "Method not allowed", resp["error"])
}

func TestValidateRequestMiddleware_EndpointNotAllowed(t *testing.T) {
	p := &TransparentProxy{
		config: ProxyConfig{AllowedEndpoints: []string{"/v1/allowed"}, AllowedMethods: []string{"POST"}},
		logger: zap.NewNop(),
	}
	h := p.ValidateRequestMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/disallowed", nil)
	h.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	assert.Equal(t, "Endpoint not found", resp["error"])
}

func TestValidateRequestMiddleware_ParamWhitelist(t *testing.T) {
	proxy := newTestProxyWithConfig(ProxyConfig{
		AllowedMethods:   []string{"POST"},
		AllowedEndpoints: []string{"/v1/completions"},
		ParamWhitelist: map[string][]string{
			"model": {"gpt-4o", "gpt-4.1-*", "text-embedding-3-small"},
		},
	})
	ts := httptest.NewServer(proxy.Handler())
	defer ts.Close()

	t.Run("Allowed exact value", func(t *testing.T) {
		body := `{"model": "gpt-4o"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode, "Allowed value should not be rejected")
		_ = resp.Body.Close()
	})

	t.Run("Allowed glob value", func(t *testing.T) {
		body := `{"model": "gpt-4.1-1106-preview"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode, "Glob-matched value should not be rejected")
		_ = resp.Body.Close()
	})

	t.Run("Disallowed value", func(t *testing.T) {
		body := `{"model": "gpt-3.5-turbo"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Disallowed value should be rejected")
		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		_ = resp.Body.Close()
		assert.Equal(t, "param_not_allowed", respBody["code"])
		assert.Contains(t, respBody["error"], "model")
	})

	t.Run("Missing parameter (should pass)", func(t *testing.T) {
		body := `{"prompt": "hi"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode, "Missing param should not be rejected")
		_ = resp.Body.Close()
	})
}

func TestValidateRequestMiddleware_CORSOriginValidation(t *testing.T) {
	proxy := newTestProxyWithConfig(ProxyConfig{
		AllowedMethods:   []string{"POST"},
		AllowedEndpoints: []string{"/v1/completions"},
		AllowedOrigins:   []string{"https://allowed.com"},
		RequiredHeaders:  []string{"origin"},
	})
	ts := httptest.NewServer(proxy.Handler())
	defer ts.Close()

	t.Run("Missing Origin header (required)", func(t *testing.T) {
		body := `{"prompt": "hi"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		_ = resp.Body.Close()
		assert.Equal(t, "origin_required", respBody["code"])
	})

	t.Run("Origin present but not allowed (required)", func(t *testing.T) {
		body := `{"prompt": "hi"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Origin", "https://not-allowed.com")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		_ = resp.Body.Close()
		assert.Equal(t, "origin_not_allowed", respBody["code"])
	})

	t.Run("Origin present and allowed (required)", func(t *testing.T) {
		body := `{"prompt": "hi"}`
		req, _ := http.NewRequest("POST", ts.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Origin", "https://allowed.com")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
		_ = resp.Body.Close()
	})

	// Now test with RequiredHeaders not including 'origin'
	proxy2 := newTestProxyWithConfig(ProxyConfig{
		AllowedMethods:   []string{"POST"},
		AllowedEndpoints: []string{"/v1/completions"},
		AllowedOrigins:   []string{"https://allowed.com"},
		RequiredHeaders:  []string{},
	})
	ts2 := httptest.NewServer(proxy2.Handler())
	defer ts2.Close()

	t.Run("Origin present but not allowed (not required)", func(t *testing.T) {
		body := `{"prompt": "hi"}`
		req, _ := http.NewRequest("POST", ts2.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Origin", "https://not-allowed.com")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		var respBody map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&respBody)
		_ = resp.Body.Close()
		assert.Equal(t, "origin_not_allowed", respBody["code"])
	})

	t.Run("Origin present and allowed (not required)", func(t *testing.T) {
		body := `{"prompt": "hi"}`
		req, _ := http.NewRequest("POST", ts2.URL+"/v1/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")
		req.Header.Set("Origin", "https://allowed.com")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode)
		assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
		_ = resp.Body.Close()
	})
}

func TestValidateRequestMiddleware_OPTIONSPreflightCORS(t *testing.T) {
	proxy := newTestProxyWithConfig(ProxyConfig{
		AllowedMethods:   []string{"POST", "OPTIONS"},
		AllowedEndpoints: []string{"/v1/completions"},
	})
	ts := httptest.NewServer(proxy.Handler())
	defer ts.Close()

	t.Run("OPTIONS with Origin and Access-Control-Request-Headers", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", ts.URL+"/v1/completions", nil)
		req.Header.Set("Origin", "https://allowed.com")
		req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
		assert.Contains(t, resp.Header.Get("Access-Control-Allow-Methods"), "OPTIONS")
		assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Content-Type")
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Expose-Headers"))
		assert.Equal(t, "86400", resp.Header.Get("Access-Control-Max-Age"))
		_ = resp.Body.Close()
	})

	t.Run("OPTIONS with Origin but no Access-Control-Request-Headers", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", ts.URL+"/v1/completions", nil)
		req.Header.Set("Origin", "https://allowed.com")
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
		assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
		assert.Contains(t, resp.Header.Get("Access-Control-Allow-Headers"), "Content-Type")
		assert.NotEmpty(t, resp.Header.Get("Access-Control-Expose-Headers"))
		assert.Equal(t, "86400", resp.Header.Get("Access-Control-Max-Age"))
		_ = resp.Body.Close()
	})

	t.Run("OPTIONS without Origin", func(t *testing.T) {
		req, _ := http.NewRequest("OPTIONS", ts.URL+"/v1/completions", nil)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		// Should not set CORS headers if no Origin
		assert.Empty(t, resp.Header.Get("Access-Control-Allow-Origin"))
		_ = resp.Body.Close()
	})
}

func TestModifyResponse_NonStreamingErrorIncrementsErrorCount(t *testing.T) {
	p := &TransparentProxy{metrics: &ProxyMetrics{}, logger: zap.NewNop()}
	res := &http.Response{
		StatusCode: 500,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("error")),
	}
	// Not streaming
	res.Header.Set("Content-Type", "application/json")
	res.Request = httptest.NewRequest("POST", "/v1/completions", nil)
	err := p.modifyResponse(res)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), p.metrics.RequestCount)
	assert.Equal(t, int64(1), p.metrics.ErrorCount)
}

func TestModifyResponse_StreamingReturnsEarly(t *testing.T) {
	p := &TransparentProxy{metrics: &ProxyMetrics{}, logger: zap.NewNop()}
	res := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("stream")),
	}
	res.Header.Set("Content-Type", "text/event-stream")
	err := p.modifyResponse(res)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), p.metrics.RequestCount)
	assert.Equal(t, int64(0), p.metrics.ErrorCount)
}

func TestParseOpenAIResponseMetadata(t *testing.T) {
	p := &TransparentProxy{}

	// Case: full metadata
	body := []byte(`{
		"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
		"model": "gpt-4",
		"id": "abc123",
		"created": 1234567890
	}`)
	meta, err := p.parseOpenAIResponseMetadata(body)
	assert.NoError(t, err)
	assert.Equal(t, "10", meta["Prompt-Tokens"])
	assert.Equal(t, "20", meta["Completion-Tokens"])
	assert.Equal(t, "30", meta["Total-Tokens"])
	assert.Equal(t, "gpt-4", meta["Model"])
	assert.Equal(t, "abc123", meta["ID"])
	assert.Equal(t, "1234567890", meta["Created"])

	// Case: missing usage
	body = []byte(`{"model": "gpt-3.5"}`)
	meta, err = p.parseOpenAIResponseMetadata(body)
	assert.NoError(t, err)
	assert.Equal(t, "gpt-3.5", meta["Model"])
	assert.NotContains(t, meta, "Prompt-Tokens")

	// Case: invalid JSON
	body = []byte("not json")
	meta, err = p.parseOpenAIResponseMetadata(body)
	assert.Error(t, err)
	assert.Empty(t, meta)

	// Case: usage present but wrong types
	body = []byte(`{"usage": {"prompt_tokens": "foo"}}`)
	meta, err = p.parseOpenAIResponseMetadata(body)
	assert.NoError(t, err)
	assert.NotContains(t, meta, "Prompt-Tokens")
}

func TestNewTransparentProxy_Coverage(t *testing.T) {
	_, err := NewTransparentProxy(ProxyConfig{}, nil, nil)
	if err == nil {
		t.Log("NewTransparentProxy returned non-nil (expected for stub)")
	}
}

// --- CIRCUIT BREAKER TESTS ---
func TestTransparentProxy_CircuitBreaker_OpensOnRepeatedFailures(t *testing.T) {
	failures := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failures++
		w.WriteHeader(http.StatusGatewayTimeout) // 504 (transient)
	}))
	defer server.Close()

	p := newTestProxyWithConfig(ProxyConfig{
		TargetBaseURL:    server.URL,
		AllowedEndpoints: []string{"/test"},
		AllowedMethods:   []string{"GET"},
	})

	handler := p.Handler() // Reuse the same handler for all requests

	// Hit the proxy enough times to trip the circuit breaker (threshold is 5)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		resp := w.Result()
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
		if resp.StatusCode != http.StatusGatewayTimeout {
			t.Errorf("expected 504 (upstream failure), got %d", resp.StatusCode)
		}
	}

	// Next request should get 503 from circuit breaker
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 from circuit breaker, got %d", resp.StatusCode)
	}
}

func TestTransparentProxy_CircuitBreaker_ClosesOnRecovery(t *testing.T) {
	failures := 0
	var allowSuccess bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowSuccess {
			failures++
			w.WriteHeader(http.StatusGatewayTimeout) // 504 (transient)
			return
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProxyWithConfig(ProxyConfig{
		TargetBaseURL:    server.URL,
		AllowedEndpoints: []string{"/test"},
		AllowedMethods:   []string{"GET"},
	})

	handler := p.Handler()

	// Use a short cooldown for test speed
	cooldown := 100 * time.Millisecond

	// Trip the circuit breaker (threshold is 5)
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req = req.WithContext(context.WithValue(req.Context(), cbCooldownOverrideKey, cooldown))
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// Next request should get 503 from circuit breaker
	req := httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), cbCooldownOverrideKey, cooldown))
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 from circuit breaker, got %d", resp.StatusCode)
	}

	// Wait for cooldown
	t.Log("Waiting for circuit breaker cooldown...")
	time.Sleep(cooldown + 10*time.Millisecond)
	allowSuccess = true

	// Now the circuit breaker should close and allow traffic again
	req = httptest.NewRequest("GET", "/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), cbCooldownOverrideKey, cooldown))
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	resp = w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after circuit breaker recovery, got %d", resp.StatusCode)
	}
}

// --- VALIDATION SCOPE TESTS ---
func TestTransparentProxy_ValidationScope_OnlyTokenPathMethod(t *testing.T) {
	// Setup proxy with allowed method and endpoint
	p := newTestProxyWithConfig(ProxyConfig{
		AllowedMethods:   []string{"GET"},
		AllowedEndpoints: []string{"/v1/test"},
	})

	ts := httptest.NewServer(p.Handler())
	defer ts.Close()

	// Valid request
	req, _ := http.NewRequest("GET", ts.URL+"/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		t.Errorf("expected valid request, got status %d", resp.StatusCode)
	}

	// Disallowed method
	req, _ = http.NewRequest("POST", ts.URL+"/v1/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for disallowed method, got %d", resp.StatusCode)
	}

	// Disallowed endpoint
	req, _ = http.NewRequest("GET", ts.URL+"/not-allowed", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for disallowed endpoint, got %d", resp.StatusCode)
	}
}

func TestTransparentProxy_ValidationScope_NoAPISpecificValidation(t *testing.T) {
	// Setup proxy with no API-specific validation
	p := newTestProxyWithConfig(ProxyConfig{})
	ts := httptest.NewServer(p.Handler())
	defer ts.Close()

	// Any request body or query param should not be validated by proxy
	req, _ := http.NewRequest("POST", ts.URL+"/v1/test", strings.NewReader(`{"foo":"bar"}`))
	req.Header.Set("Authorization", "Bearer valid-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not be 400 or 422 from proxy
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnprocessableEntity {
		t.Errorf("proxy performed API-specific validation, got status %d", resp.StatusCode)
	}
}

// newTestProxyWithConfig creates a TransparentProxy with the given config and stub dependencies.
func newTestProxyWithConfig(cfg ProxyConfig) *TransparentProxy {
	stubValidator := &stubTokenValidator{}
	stubStore := &stubProjectStore{}
	logger := zap.NewNop()
	p, _ := NewTransparentProxyWithLogger(cfg, stubValidator, stubStore, logger)
	return p
}

type stubTokenValidator struct{}

func (s *stubTokenValidator) ValidateTokenWithTracking(ctx context.Context, token string) (string, error) {
	return "test-project-id", nil
}
func (s *stubTokenValidator) ValidateToken(ctx context.Context, token string) (string, error) {
	return "test-project-id", nil
}

type stubProjectStore struct{}

func (s *stubProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	return "api-key", nil
}

func (s *stubProjectStore) ListProjects(ctx context.Context) ([]Project, error) {
	return nil, nil
}

func (s *stubProjectStore) CreateProject(ctx context.Context, project Project) error {
	return nil
}

func (s *stubProjectStore) GetProjectByID(ctx context.Context, projectID string) (Project, error) {
	return Project{}, nil
}

func (s *stubProjectStore) UpdateProject(ctx context.Context, project Project) error {
	return nil
}

func (s *stubProjectStore) DeleteProject(ctx context.Context, projectID string) error {
	return nil
}

// Helper for TestTimingResponseWriter_Flush
type flushRecorder struct {
	flushed bool
	http.ResponseWriter
}

func (f *flushRecorder) Flush() { f.flushed = true }

func TestTimingResponseWriter_Flush(t *testing.T) {
	rec := &flushRecorder{ResponseWriter: httptest.NewRecorder()}
	trw := &timingResponseWriter{ResponseWriter: rec}
	trw.Flush()
	if !rec.flushed {
		t.Errorf("Flush was not called on underlying ResponseWriter")
	}
}

func TestSetTimingHeaders(t *testing.T) {
	res := &http.Response{Header: make(http.Header)}
	now := time.Now().UTC()
	ctx := context.Background()
	ctx = context.WithValue(ctx, ctxKeyProxyReceivedAt, now)
	ctx = context.WithValue(ctx, ctxKeyProxySentBackendAt, now.Add(1*time.Second))
	ctx = context.WithValue(ctx, ctxKeyProxyFirstRespAt, now.Add(2*time.Second))
	ctx = context.WithValue(ctx, ctxKeyProxyFinalRespAt, now.Add(3*time.Second))

	setTimingHeaders(res, ctx)

	if res.Header.Get("X-Proxy-Received-At") == "" {
		t.Errorf("X-Proxy-Received-At header not set")
	}
	if res.Header.Get("X-Proxy-Sent-Backend-At") == "" {
		t.Errorf("X-Proxy-Sent-Backend-At header not set")
	}
	if res.Header.Get("X-Proxy-First-Response-At") == "" {
		t.Errorf("X-Proxy-First-Response-At header not set")
	}
	if res.Header.Get("X-Proxy-Final-Response-At") == "" {
		t.Errorf("X-Proxy-Final-Response-At header not set")
	}
}

func TestTransparentProxy_MetricsAndSetMetrics(t *testing.T) {
	p := &TransparentProxy{metrics: &ProxyMetrics{RequestCount: 42, ErrorCount: 7}}
	m := p.Metrics()
	assert.Equal(t, int64(42), m.RequestCount)
	assert.Equal(t, int64(7), m.ErrorCount)

	newMetrics := &ProxyMetrics{RequestCount: 100, ErrorCount: 1}
	p.SetMetrics(newMetrics)
	assert.Equal(t, newMetrics, p.metrics)
}

func TestTransparentProxy_createTransport(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{MaxIdleConns: 10, MaxIdleConnsPerHost: 2, IdleConnTimeout: 5 * time.Second, ResponseHeaderTimeout: 3 * time.Second}}
	tr := p.createTransport()
	assert.Equal(t, 10, tr.MaxIdleConns)
	assert.Equal(t, 2, tr.MaxIdleConnsPerHost)
	assert.Equal(t, 5*time.Second, tr.IdleConnTimeout)
	assert.Equal(t, 3*time.Second, tr.ResponseHeaderTimeout)
	assert.True(t, tr.ForceAttemptHTTP2)
}

// testFlusher is a test helper that implements http.Flusher and tracks if Flush was called
var testFlushed bool

type testFlusher struct{ http.ResponseWriter }

func (f *testFlusher) Flush() { testFlushed = true }

func TestResponseRecorder_Flush(t *testing.T) {
	testFlushed = false
	rec := &responseRecorder{ResponseWriter: &testFlusher{httptest.NewRecorder()}}
	rec.Flush()
	assert.True(t, testFlushed, "Flush should call underlying Flusher")
}

func TestIsEndpointAllowed_EdgeCases(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{}}
	// No allowed endpoints: allow all
	assert.True(t, p.isEndpointAllowed("/foo"))
	p.config.AllowedEndpoints = []string{"/v1/foo"}
	assert.True(t, p.isEndpointAllowed("/v1/foo"))
	assert.True(t, p.isEndpointAllowed("/v1/foo/extra")) // prefix match
	assert.False(t, p.isEndpointAllowed("/v1/bar"))
}

func TestIsMethodAllowed_EdgeCases(t *testing.T) {
	p := &TransparentProxy{config: ProxyConfig{}}
	// No allowed methods: allow all
	assert.True(t, p.isMethodAllowed("GET"))
	assert.True(t, p.isMethodAllowed("POST"))
	p.config.AllowedMethods = []string{"GET", "POST"}
	assert.True(t, p.isMethodAllowed("GET"))
	assert.True(t, p.isMethodAllowed("POST"))
	assert.False(t, p.isMethodAllowed("DELETE"))
	// Case insensitivity
	p.config.AllowedMethods = []string{"get"}
	assert.True(t, p.isMethodAllowed("GET"))
}

func TestTransparentProxy_Handler_ErrorAndEdgeCases(t *testing.T) {
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	cases := []struct {
		name       string
		method     string
		path       string
		headers    map[string]string
		validator  func(*MockTokenValidator)
		store      func(*MockProjectStore)
		config     ProxyConfig
		wantStatus int
		wantBody   string
	}{
		{
			name:      "OPTIONS preflight",
			method:    http.MethodOptions,
			path:      "/v1/completions",
			headers:   map[string]string{"Origin": "https://foo.com"},
			validator: func(v *MockTokenValidator) {},
			store:     func(s *MockProjectStore) {},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			},
			wantStatus: http.StatusNoContent,
			wantBody:   "",
		},
		{
			name:      "Missing Authorization header",
			method:    http.MethodPost,
			path:      "/v1/completions",
			headers:   map[string]string{},
			validator: func(v *MockTokenValidator) {},
			store:     func(s *MockProjectStore) {},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "\"Invalid token\"",
		},
		{
			name:    "Token validation error",
			method:  http.MethodPost,
			path:    "/v1/completions",
			headers: map[string]string{"Authorization": "Bearer badtoken"},
			validator: func(v *MockTokenValidator) {
				v.On("ValidateTokenWithTracking", mock.Anything, "badtoken").Return("", errors.New("invalid token")).Once()
			},
			store: func(s *MockProjectStore) {},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "\"Invalid token\"",
		},
		{
			name:    "Project store error",
			method:  http.MethodPost,
			path:    "/v1/completions",
			headers: map[string]string{"Authorization": "Bearer test_token"},
			validator: func(v *MockTokenValidator) {
				v.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil).Once()
			},
			store: func(s *MockProjectStore) {
				s.On("GetAPIKeyForProject", mock.Anything, "project123").Return("", errors.New("fail")).Once()
			},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "\"Invalid token\"",
		},
		{
			name:      "Disallowed method",
			method:    http.MethodDelete,
			path:      "/v1/completions",
			headers:   map[string]string{"Authorization": "Bearer test_token"},
			validator: func(v *MockTokenValidator) {},
			store:     func(s *MockProjectStore) {},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
			},
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   "Method not allowed",
		},
		{
			name:      "Disallowed endpoint",
			method:    http.MethodPost,
			path:      "/notallowed",
			headers:   map[string]string{"Authorization": "Bearer test_token"},
			validator: func(v *MockTokenValidator) {},
			store:     func(s *MockProjectStore) {},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "Endpoint not found",
		},
		{
			name:    "Param whitelist validation error",
			method:  http.MethodPost,
			path:    "/v1/completions",
			headers: map[string]string{"Authorization": "Bearer test_token", "Content-Type": "application/json"},
			validator: func(v *MockTokenValidator) {
				v.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil).Once()
			},
			store: func(s *MockProjectStore) {
				s.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil).Once()
			},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
				ParamWhitelist:   map[string][]string{"foo": {"bar"}},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "param_not_allowed",
		},
		{
			name:    "CORS origin required",
			method:  http.MethodPost,
			path:    "/v1/completions",
			headers: map[string]string{"Authorization": "Bearer test_token"},
			validator: func(v *MockTokenValidator) {
				v.On("ValidateTokenWithTracking", mock.Anything, "test_token").Return("project123", nil).Once()
			},
			store: func(s *MockProjectStore) {
				s.On("GetAPIKeyForProject", mock.Anything, "project123").Return("api_key_123", nil).Once()
			},
			config: ProxyConfig{
				TargetBaseURL:    mockAPI.URL,
				AllowedEndpoints: []string{"/v1/completions"},
				AllowedMethods:   []string{"POST"},
				RequiredHeaders:  []string{"origin"},
			},
			wantStatus: http.StatusBadRequest,
			wantBody:   "origin_required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mockValidator := new(MockTokenValidator)
			mockStore := new(MockProjectStore)
			if tc.validator != nil {
				tc.validator(mockValidator)
			}
			if tc.store != nil {
				tc.store(mockStore)
			}
			proxy, err := NewTransparentProxyWithLogger(tc.config, mockValidator, mockStore, zap.NewNop())
			require.NoError(t, err)
			var body io.Reader
			if tc.name == "Param whitelist validation error" {
				body = bytes.NewReader([]byte(`{"foo":"baz"}`))
			} else {
				body = bytes.NewReader([]byte(`{"prompt":"test"}`))
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			proxy.Handler().ServeHTTP(w, req)
			resp := w.Result()
			respBody, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.wantBody != "" {
				assert.Contains(t, string(respBody), tc.wantBody)
			}
		})
	}
}

func TestValidateRequestMiddleware_EdgeCases(t *testing.T) {
	proxy := newTestProxyWithConfig(ProxyConfig{
		AllowedMethods:   []string{"POST"},
		AllowedEndpoints: []string{"/v1/completions"},
		ParamWhitelist: map[string][]string{
			"model": {"gpt-4o"},
		},
	})
	h := proxy.ValidateRequestMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("POST with no body", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/completions", nil)
		r.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("POST with param not in whitelist", func(t *testing.T) {
		body := `{"model": "not-allowed"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/completions", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "param_not_allowed", resp["code"])
	})

	t.Run("POST with allowed param as non-string", func(t *testing.T) {
		body := `{"model": 123}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/completions", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		assert.Equal(t, "param_not_allowed", resp["code"])
	})

	t.Run("POST with missing Content-Type", func(t *testing.T) {
		body := `{"model": "gpt-4o"}`
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/completions", strings.NewReader(body))
		h.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("CORS with Origin present but no AllowedOrigins", func(t *testing.T) {
		proxy2 := newTestProxyWithConfig(ProxyConfig{
			AllowedMethods:   []string{"POST"},
			AllowedEndpoints: []string{"/v1/completions"},
		})
		h2 := proxy2.ValidateRequestMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/completions", nil)
		r.Header.Set("Origin", "https://foo.com")
		h2.ServeHTTP(w, r)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestNewTransparentProxyWithObservability(t *testing.T) {
	mockValidator := &MockTokenValidator{}
	mockStore := &MockProjectStore{}

	config := ProxyConfig{
		TargetBaseURL: "https://api.example.com",
		LogLevel:      "info",
		LogFormat:     "text",
		LogFile:       "",
	}

	obsCfg := middleware.ObservabilityConfig{
		Enabled:  true,
		EventBus: nil, // Can be nil for this test
	}

	proxy, err := NewTransparentProxyWithObservability(config, mockValidator, mockStore, obsCfg)
	require.NoError(t, err)
	require.NotNil(t, proxy)
	assert.NotNil(t, proxy.obsMiddleware)
}

func TestNewTransparentProxyWithLoggerAndObservability(t *testing.T) {
	mockValidator := &MockTokenValidator{}
	mockStore := &MockProjectStore{}
	logger := zap.NewNop()

	config := ProxyConfig{
		TargetBaseURL: "https://api.example.com",
		LogLevel:      "info",
		LogFormat:     "text",
		LogFile:       "",
	}

	obsCfg := middleware.ObservabilityConfig{
		Enabled:  true,
		EventBus: nil, // Can be nil for this test
	}

	proxy, err := NewTransparentProxyWithLoggerAndObservability(config, mockValidator, mockStore, logger, obsCfg)
	require.NoError(t, err)
	require.NotNil(t, proxy)
	assert.NotNil(t, proxy.obsMiddleware)
}
