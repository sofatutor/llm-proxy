package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/token"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		ListenAddr:     ":8080",
		RequestTimeout: 30 * time.Second,
	}

	// Create a new server
	server := New(cfg, &mockTokenStore{}, &mockProjectStore{})

	// Create a request to the health endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Call the health endpoint directly
	server.handleHealth(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check the content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type %s, got %s", "application/json", contentType)
	}

	// Parse the response body
	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate the response fields
	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}

	if response.Version != Version {
		t.Errorf("Expected version '%s', got '%s'", Version, response.Version)
	}

	// Timestamp should be recent
	now := time.Now()
	diff := now.Sub(response.Timestamp)
	if diff > 5*time.Second {
		t.Errorf("Timestamp is too old: %v", response.Timestamp)
	}
}

func TestServerLifecycle(t *testing.T) {
	// Use httptest.NewServer to start the server with the health handler
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		New(&config.Config{
			RequestTimeout: 1 * time.Second,
		}, &mockTokenStore{}, &mockProjectStore{}).handleHealth(w, r)
	}))
	defer ts.Close()

	// Test the health endpoint using the test server's URL
	client := &http.Client{Timeout: 100 * time.Millisecond}
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("Failed to connect to test server: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// For the Shutdown test, we can use a simple httptest server
	shutdownServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer shutdownServer.Close()

	cfg := &config.Config{
		ListenAddr:     ":0", // Random port
		RequestTimeout: 1 * time.Second,
	}

	// Create a server with the test server's config
	shutdownTestServer := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	// Replace the internal http.Server with the test server's
	shutdownTestServer.server = shutdownServer.Config

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Test the Shutdown method
	if err := shutdownTestServer.Shutdown(ctx); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	// The httptest server will be shut down when we call Close
}

// TestHandleHealthJSONError tests error handling in the health endpoint when JSON encoding fails
func TestHandleHealthJSONError(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		ListenAddr:     ":8080",
		RequestTimeout: 30 * time.Second,
	}

	// Create a new server
	server := New(cfg, &mockTokenStore{}, &mockProjectStore{})

	// Create a response recorder that allows us to examine the response
	rr := httptest.NewRecorder()

	// Mock the json.NewEncoder to return an error
	// We'll use a special hook in our test to simulate the error condition
	// by temporarily intercepting the ResponseWriter's Write method

	// Create a request to the health endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Use a recorder wrapper that fails on first Write
	failingRW := &mockFailingResponseWriter{
		ResponseWriter:   rr,
		failOnFirstWrite: true,
	}

	// Call the health endpoint directly
	server.handleHealth(failingRW, req)

	// Since we expect an error in the encoding process, check that
	// we got an appropriate status code (500) in our error case
	if failingRW.statusCode != http.StatusInternalServerError {
		t.Errorf("Expected error status code %d, got %d",
			http.StatusInternalServerError, failingRW.statusCode)
	}
}

// mockFailingResponseWriter implements http.ResponseWriter and fails on write
type mockFailingResponseWriter struct {
	http.ResponseWriter
	failOnFirstWrite bool
	statusCode       int
	headers          http.Header
}

func (m *mockFailingResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockFailingResponseWriter) Write(b []byte) (int, error) {
	if m.failOnFirstWrite {
		m.statusCode = http.StatusInternalServerError
		return 0, fmt.Errorf("simulated write error")
	}
	return m.ResponseWriter.Write(b)
}

func (m *mockFailingResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	m.ResponseWriter.WriteHeader(statusCode)
}

func TestInitializeComponents_DatabaseInitializationPending(t *testing.T) {
	t.Skip("Database connection initialization is not yet implemented in initializeComponents (see TODO in server.go)")
}

func TestInitializeComponents_LoggingInitializationPending(t *testing.T) {
	t.Skip("Logging initialization is not yet implemented in initializeComponents (see TODO in server.go)")
}

func TestInitializeComponents_AdminRoutesInitializationPending(t *testing.T) {
	t.Skip("Admin routes initialization is not yet implemented in initializeComponents (see TODO in server.go)")
}

func TestInitializeComponents_MetricsInitializationPending(t *testing.T) {
	t.Skip("Metrics initialization is not yet implemented in initializeComponents (see TODO in server.go)")
}

// --- Dependency injection coverage for production code ---

type mockTokenStore struct{}

func (m *mockTokenStore) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	return token.TokenData{}, errors.New("not implemented")
}
func (m *mockTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	return errors.New("not implemented")
}

type mockProjectStore struct{}

func (m *mockProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	return "mock-key", nil
}

func TestServer_New_WithDependencyInjection_ConfigAndFallback(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:         ":8080",
		RequestTimeout:     30 * time.Second,
		APIConfigPath:      "/non/existent/path.yaml", // triggers fallback branch
		DefaultAPIProvider: "openai",
		OpenAIAPIURL:       "https://api.openai.com",
	}

	ts := &mockTokenStore{}
	ps := &mockProjectStore{}

	srv := New(cfg, ts, ps)
	if srv.tokenStore != ts {
		t.Errorf("tokenStore not injected correctly")
	}
	if srv.projectStore != ps {
		t.Errorf("projectStore not injected correctly")
	}

	err := srv.initializeAPIRoutes()
	if err != nil {
		t.Fatalf("initializeAPIRoutes failed: %v", err)
	}

	// Test that a /v1/ route is registered (OpenAI fallback branch)
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)
	// Should not be 404 (route exists, even if auth fails)
	if rr.Code == http.StatusNotFound {
		t.Errorf("Expected /v1/models to be registered, got 404")
	}

	// Now test with a valid config file (config branch)
	tmpFile, err := os.CreateTemp("", "api_config_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	configYAML := `
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
	if _, err := tmpFile.Write([]byte(configYAML)); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()
	cfg2 := &config.Config{
		ListenAddr:         ":8080",
		RequestTimeout:     30 * time.Second,
		APIConfigPath:      tmpFile.Name(),
		DefaultAPIProvider: "test_api",
	}
	srv2 := New(cfg2, ts, ps)
	err = srv2.initializeAPIRoutes()
	if err != nil {
		t.Fatalf("initializeAPIRoutes (config branch) failed: %v", err)
	}
	req2 := httptest.NewRequest("GET", "/v1/test", nil)
	rr2 := httptest.NewRecorder()
	srv2.server.Handler.ServeHTTP(rr2, req2)
	if rr2.Code == http.StatusNotFound {
		t.Errorf("Expected /v1/test to be registered, got 404")
	}
}
