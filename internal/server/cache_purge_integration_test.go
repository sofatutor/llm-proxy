package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

func TestCachePurgeIntegration(t *testing.T) {
	// Create a real server with cache enabled
	cfg := &config.Config{
		ListenAddr:      ":0",
		ManagementToken: "test-management-token",
		AuditEnabled:    false,
		EventBusBackend: "in-memory",
	}

	server, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	// Setup a real proxy with caching
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
		HTTPCacheEnabled:    true,
		HTTPCacheDefaultTTL: 300 * time.Second,
	}

	transparentProxy, err := proxy.NewTransparentProxyWithLogger(proxyConfig, 
		token.NewValidator(&mockTokenStore{}), &mockProjectStore{}, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	server.proxy = transparentProxy

	tests := []struct {
		name           string
		requestBody    CachePurgeRequest
		authToken      string
		expectedStatus int
		expectedResult interface{}
	}{
		{
			name: "successful_exact_purge_with_auth",
			requestBody: CachePurgeRequest{
				Method: "GET",
				URL:    "/v1/models",
			},
			authToken:      "test-management-token",
			expectedStatus: http.StatusOK,
			expectedResult: false, // Key doesn't exist
		},
		{
			name: "successful_prefix_purge_with_auth",
			requestBody: CachePurgeRequest{
				Method: "GET",
				URL:    "/v1/models",
				Prefix: "models:",
			},
			authToken:      "test-management-token",
			expectedStatus: http.StatusOK,
			expectedResult: 0.0, // No keys with prefix
		},
		{
			name: "unauthorized_request",
			requestBody: CachePurgeRequest{
				Method: "GET",
				URL:    "/v1/models",
			},
			authToken:      "wrong-token",
			expectedStatus: http.StatusUnauthorized,
			expectedResult: nil,
		},
		{
			name: "missing_auth_header",
			requestBody: CachePurgeRequest{
				Method: "GET",
				URL:    "/v1/models",
			},
			authToken:      "",
			expectedStatus: http.StatusUnauthorized,
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request with auth middleware
			reqBody, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/manage/cache/purge", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			if tt.authToken != "" {
				req.Header.Set("Authorization", "Bearer "+tt.authToken)
			}

			// Use the auth middleware wrapped handler
			handler := server.logRequestMiddleware(server.managementAuthMiddleware(http.HandlerFunc(server.handleCachePurge)))
			
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			// Verify response status
			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// For successful requests, verify response body
			if tt.expectedStatus == http.StatusOK {
				var response CachePurgeResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to unmarshal response: %v", err)
				}

				if response.Deleted != tt.expectedResult {
					t.Errorf("expected deleted %v, got %v", tt.expectedResult, response.Deleted)
				}
			}
		})
	}
}