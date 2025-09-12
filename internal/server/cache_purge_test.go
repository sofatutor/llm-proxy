package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

func TestHandleCachePurge_Success(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    CachePurgeRequest
		expectedStatus int
		expectedType   string // "exact" or "prefix"
		checkResult    func(t *testing.T, response CachePurgeResponse)
	}{
		{
			name: "exact_purge_nonexistent_key",
			requestBody: CachePurgeRequest{
				Method: "GET",
				URL:    "/v1/models",
			},
			expectedStatus: http.StatusOK,
			expectedType:   "exact",
			checkResult: func(t *testing.T, response CachePurgeResponse) {
				if response.Deleted != false {
					t.Errorf("expected deleted false, got %v", response.Deleted)
				}
			},
		},
		{
			name: "prefix_purge_no_matches",
			requestBody: CachePurgeRequest{
				Method: "GET",
				URL:    "/v1/models",
				Prefix: "nonexistent:",
			},
			expectedStatus: http.StatusOK,
			expectedType:   "prefix",
			checkResult: func(t *testing.T, response CachePurgeResponse) {
				// Should be 0 for no matches
				if response.Deleted != 0.0 { // JSON numbers are float64
					t.Errorf("expected deleted 0, got %v", response.Deleted)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			server := createTestServerWithRealProxy(t)

			// Create request
			reqBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/manage/cache/purge", bytes.NewBuffer(reqBody))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")

			// Execute
			rr := httptest.NewRecorder()
			server.handleCachePurge(rr, req)

			// Verify response status
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Verify response body
			var response CachePurgeResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			tt.checkResult(t, response)
		})
	}
}

func TestHandleCachePurge_Errors(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		requestBody    string
		authHeader     string
		setupServer    func(*Server)
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "method_not_allowed",
			method:         "GET",
			requestBody:    `{"method":"GET","url":"/v1/models"}`,
			authHeader:     "Bearer test-token",
			setupServer:    func(s *Server) {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedError:  "method not allowed",
		},
		{
			name:           "invalid_json",
			method:         "POST",
			requestBody:    `invalid json`,
			authHeader:     "Bearer test-token",
			setupServer:    func(s *Server) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid JSON",
		},
		{
			name:           "missing_method",
			method:         "POST",
			requestBody:    `{"url":"/v1/models"}`,
			authHeader:     "Bearer test-token",
			setupServer:    func(s *Server) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "method and url are required",
		},
		{
			name:           "missing_url",
			method:         "POST",
			requestBody:    `{"method":"GET"}`,
			authHeader:     "Bearer test-token",
			setupServer:    func(s *Server) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "method and url are required",
		},
		{
			name:           "invalid_url",
			method:         "POST",
			requestBody:    `{"method":"GET","url":"://invalid"}`,
			authHeader:     "Bearer test-token",
			setupServer:    func(s *Server) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid URL",
		},
		{
			name:           "proxy_not_available",
			method:         "POST",
			requestBody:    `{"method":"GET","url":"/v1/models"}`,
			authHeader:     "Bearer test-token",
			setupServer:    func(s *Server) { s.proxy = nil },
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "proxy not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			server := createTestServerWithRealProxy(t)
			tt.setupServer(server)

			// Create request
			req := httptest.NewRequest(tt.method, "/manage/cache/purge", strings.NewReader(tt.requestBody))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			req.Header.Set("Content-Type", "application/json")

			// Execute
			rr := httptest.NewRecorder()
			server.handleCachePurge(rr, req)

			// Verify response status
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Verify error message
			responseBody := rr.Body.String()
			if !strings.Contains(responseBody, tt.expectedError) {
				t.Errorf("expected error containing %q, got %q", tt.expectedError, responseBody)
			}
		})
	}
}

func TestHandleCachePurge_CacheDisabled(t *testing.T) {
	// Create server with cache disabled
	server := createTestServerWithCacheDisabled(t)

	requestBody := CachePurgeRequest{
		Method: "GET",
		URL:    "/v1/models",
	}

	reqBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/manage/cache/purge", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	server.handleCachePurge(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	responseBody := rr.Body.String()
	if !strings.Contains(responseBody, "caching is disabled") {
		t.Errorf("expected error containing 'caching is disabled', got %q", responseBody)
	}
}

// Helper functions

func createTestServerWithRealProxy(t *testing.T) *Server {
	t.Helper()

	cfg := &config.Config{
		ManagementToken: "test-token",
		AuditEnabled:    false,
	}

	// Create a real proxy with caching enabled
	proxyConfig := proxy.ProxyConfig{
		TargetBaseURL:       "https://api.openai.com",
		AllowedEndpoints:    []string{"/v1/models"},
		AllowedMethods:      []string{"GET", "POST"},
		RequestTimeout:      30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		FlushInterval:       100 * time.Millisecond,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		HTTPCacheEnabled:    true, // Enable caching
		HTTPCacheDefaultTTL: 300 * time.Second,
	}

	tokenStore := &mockTokenStore{}
	projectStore := &mockProjectStore{}

	transparentProxy, err := proxy.NewTransparentProxyWithLogger(proxyConfig, 
		token.NewValidator(tokenStore), projectStore, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	server := &Server{
		config:      cfg,
		logger:      zap.NewNop(), // Add logger
		auditLogger: audit.NewNullLogger(),
		proxy:       transparentProxy,
	}

	return server
}

func createTestServerWithCacheDisabled(t *testing.T) *Server {
	t.Helper()

	cfg := &config.Config{
		ManagementToken: "test-token",
		AuditEnabled:    false,
	}

	// Create a proxy with caching disabled
	proxyConfig := proxy.ProxyConfig{
		TargetBaseURL:       "https://api.openai.com",
		AllowedEndpoints:    []string{"/v1/models"},
		AllowedMethods:      []string{"GET", "POST"},
		RequestTimeout:      30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		FlushInterval:       100 * time.Millisecond,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		HTTPCacheEnabled:    false, // Disable caching
	}

	tokenStore := &mockTokenStore{}
	projectStore := &mockProjectStore{}

	transparentProxy, err := proxy.NewTransparentProxyWithLogger(proxyConfig, 
		token.NewValidator(tokenStore), projectStore, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	server := &Server{
		config:      cfg,
		logger:      zap.NewNop(), // Add logger
		auditLogger: audit.NewNullLogger(),
		proxy:       transparentProxy,
	}

	return server
}

func mustParseURL(rawURL string) *url.URL {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u
}