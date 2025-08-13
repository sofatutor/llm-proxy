package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIRoutesInitialization tests the initialization of API routes
func TestAPIRoutesInitialization(t *testing.T) {
	// Create a temporary API config file
	tmpFile, err := os.CreateTemp("", "api_config_*.yaml")
	require.NoError(t, err, "Failed to create temp file")
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("failed to remove temp file: %v", err)
		}
	}()

	// Write test config
	testConfig := `
default_api: test_api
apis:
  test_api:
    base_url: https://api.example.com
    allowed_endpoints:
      - /v1/test
    allowed_methods:
      - GET
      - POST
    timeouts:
      request: 30s
      response_header: 15s
      idle_connection: 60s
      flush_interval: 100ms
    connection:
      max_idle_conns: 100
      max_idle_conns_per_host: 10
`
	_, err = tmpFile.Write([]byte(testConfig))
	require.NoError(t, err, "Failed to write to temp file")
	err = tmpFile.Close()
	require.NoError(t, err, "Failed to close temp file")

	// Create test configuration
	cfg := &config.Config{
		ListenAddr:         ":8080",
		RequestTimeout:     30 * time.Second,
		APIConfigPath:      tmpFile.Name(),
		DefaultAPIProvider: "test_api",
		EventBusBackend:    "in-memory",
	}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Initialize API routes
	err = srv.initializeAPIRoutes()
	require.NoError(t, err, "Failed to initialize API routes")

	// Create test server
	testServer := httptest.NewServer(srv.server.Handler)
	defer testServer.Close()

	// Test allowed endpoint
	respAllowed, err := http.Get(testServer.URL + "/v1/test")
	require.NoError(t, err, "Failed to make request to allowed endpoint")
	defer func() {
		if err := respAllowed.Body.Close(); err != nil {
			t.Fatalf("failed to close respAllowed body: %v", err)
		}
	}()

	// Since we're not setting up a complete proxy with mocked token validator,
	// we expect authentication failures, not 404
	assert.NotEqual(t, http.StatusNotFound, respAllowed.StatusCode, "Expected endpoint to be found")

	// Test disallowed endpoint
	respDisallowed, err := http.Get(testServer.URL + "/v1/not_allowed")
	require.NoError(t, err, "Failed to make request to disallowed endpoint")
	defer func() {
		if err := respDisallowed.Body.Close(); err != nil {
			t.Fatalf("failed to close respDisallowed body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusNotFound, respDisallowed.StatusCode, "Expected 404 for disallowed endpoint")

	// Test disallowed method
	req, err := http.NewRequest("DELETE", testServer.URL+"/v1/test", nil)
	require.NoError(t, err, "Failed to create DELETE request")

	client := &http.Client{}
	respMethod, err := client.Do(req)
	require.NoError(t, err, "Failed to make DELETE request")
	defer func() {
		if err := respMethod.Body.Close(); err != nil {
			t.Fatalf("failed to close respMethod body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusMethodNotAllowed, respMethod.StatusCode, "Expected 405 for disallowed method")
}

// TestDefaultOpenAIFallback tests fallback to default OpenAI configuration
func TestDefaultOpenAIFallback(t *testing.T) {
	// Create test configuration with non-existent config path
	cfg := &config.Config{
		ListenAddr:         ":8080",
		RequestTimeout:     30 * time.Second,
		APIConfigPath:      "/non/existent/path.yaml",
		DefaultAPIProvider: "openai",
		OpenAIAPIURL:       "https://api.openai.com",
		EventBusBackend:    "in-memory",
	}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Initialize API routes - should fall back to default OpenAI config
	err = srv.initializeAPIRoutes()
	require.NoError(t, err, "Failed to initialize API routes with fallback")

	// Create test server
	testServer := httptest.NewServer(srv.server.Handler)
	defer testServer.Close()

	// Test common OpenAI endpoint
	respAllowed, err := http.Get(testServer.URL + "/v1/models")
	require.NoError(t, err, "Failed to make request to OpenAI endpoint")
	defer func() {
		if err := respAllowed.Body.Close(); err != nil {
			t.Fatalf("failed to close respAllowed body: %v", err)
		}
	}()

	// We expect authentication issues, not 404
	assert.NotEqual(t, http.StatusNotFound, respAllowed.StatusCode, "Expected OpenAI endpoint to be found")

	// Test disallowed endpoint
	respDisallowed, err := http.Get(testServer.URL + "/v1/not_an_openai_endpoint")
	require.NoError(t, err, "Failed to make request to disallowed endpoint")
	defer func() {
		if err := respDisallowed.Body.Close(); err != nil {
			t.Fatalf("failed to close respDisallowed body: %v", err)
		}
	}()

	assert.Equal(t, http.StatusNotFound, respDisallowed.StatusCode, "Expected 404 for disallowed endpoint")
}
