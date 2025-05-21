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
	"io/ioutil"

	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	proxy := NewTransparentProxy(config, mockValidator, mockStore)

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 status code")

	// Parse response body
	var response map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&response)
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
	proxy := NewTransparentProxy(config, mockValidator, mockStore)

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

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
	proxy := NewTransparentProxy(config, mockValidator, mockStore)

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Expected 401 status code for invalid token")

	// Parse error response
	var errResponse map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&errResponse)
	assert.NoError(t, err, "Expected no error decoding response")

	// Verify error message
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

	// Create mock dependencies
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Create proxy configuration with limited allowlist
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/completions"}, // Only completions allowed
		AllowedMethods:   []string{"GET", "POST"},
	}

	// Create proxy
	proxy := NewTransparentProxy(config, mockValidator, mockStore)

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Expected 404 status code for disallowed endpoint")
}

// Test disallowed method
func TestTransparentProxy_DisallowedMethod(t *testing.T) {
	// Create mock API server
	mockAPI := createMockAPIServer(t)
	defer mockAPI.Close()

	// Create mock dependencies
	mockValidator := new(MockTokenValidator)
	mockStore := new(MockProjectStore)

	// Create proxy configuration with limited methods
	config := ProxyConfig{
		TargetBaseURL:    mockAPI.URL,
		AllowedEndpoints: []string{"/v1/completions"},
		AllowedMethods:   []string{"GET"}, // Only GET allowed
	}

	// Create proxy
	proxy := NewTransparentProxy(config, mockValidator, mockStore)

	// Create request with disallowed method
	req := httptest.NewRequest("DELETE", "/v1/completions", nil)
	req.Header.Set("Authorization", "Bearer test_token")

	// Record response
	w := httptest.NewRecorder()

	// Process request
	proxy.Handler().ServeHTTP(w, req)

	// Check response - should be 405 Method Not Allowed
	resp := w.Result()
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

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
	proxy := NewTransparentProxy(config, mockValidator, mockStore)

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
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("Failed to close response body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Expected 200 status code for large request")

	// Verify mock expectations
	mockValidator.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestTransparentProxy_ErrorHandler(t *testing.T) {
	proxy := NewTransparentProxy(ProxyConfig{}, nil, nil)
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
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantMessage)
		})
	}
}

func TestTransparentProxy_HandleValidationError(t *testing.T) {
	proxy := NewTransparentProxy(ProxyConfig{}, nil, nil)
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
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			assert.Contains(t, string(body), tc.wantCode)
			assert.Contains(t, string(body), tc.wantMsg)
		})
	}
}

// Minimal mock http.Server for Shutdown test

type mockHTTPServer struct {
	shutdownCalled bool
}

func (m *mockHTTPServer) Shutdown(ctx context.Context) error {
	m.shutdownCalled = true
	return nil
}

// httpServerAdapter allows us to use mockHTTPServer as *http.Server for testing
type httpServerAdapter struct {
	*mockHTTPServer
}

func (h *httpServerAdapter) Shutdown(ctx context.Context) error {
	return h.mockHTTPServer.Shutdown(ctx)
}

func TestTransparentProxy_Shutdown(t *testing.T) {
	proxy := NewTransparentProxy(ProxyConfig{}, nil, nil)
	// Case: no httpServer
	err := proxy.Shutdown(context.Background())
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
