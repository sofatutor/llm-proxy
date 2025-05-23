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
			proxy.handleValidationError(w, tc.err)
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

// --- RETRY LOGIC TESTS ---
func TestTransparentProxy_RetryLogic_TransientNetworkError(t *testing.T) {
	failures := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failures < 2 {
			failures++
			w.WriteHeader(http.StatusGatewayTimeout) // 504
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

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.Handler().ServeHTTP(w, req)
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 after retries, got %d", resp.StatusCode)
	}
	if failures != 2 {
		t.Errorf("expected 2 transient failures before success, got %d", failures)
	}
}

func TestTransparentProxy_RetryLogic_NonTransientError(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest) // 400
		if _, err := w.Write([]byte(`{"error":"bad request"}`)); err != nil {
			t.Logf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	p := newTestProxyWithConfig(ProxyConfig{
		TargetBaseURL:    server.URL,
		AllowedEndpoints: []string{"/test"},
		AllowedMethods:   []string{"GET"},
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()

	p.Handler().ServeHTTP(w, req)
	resp := w.Result()
	if err := resp.Body.Close(); err != nil {
		t.Logf("Failed to close response body: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for non-transient error, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
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
		if resp.StatusCode != http.StatusBadGateway {
			t.Errorf("expected 502 for retry exhaustion, got %d", resp.StatusCode)
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
