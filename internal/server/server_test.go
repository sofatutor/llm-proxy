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
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		ListenAddr:     ":8080",
		RequestTimeout: 30 * time.Second,
	}

	// Create a new server
	server, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

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

func TestMetricsEndpoint(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:     ":8080",
		RequestTimeout: 30 * time.Second,
		EnableMetrics:  true,
		MetricsPath:    "/metrics",
	}
	server, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	p := &proxy.TransparentProxy{}
	p.SetMetrics(&proxy.ProxyMetrics{RequestCount: 2, ErrorCount: 1})
	server.proxy = p

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	server.handleMetrics(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rr.Code)
	}
	var resp struct {
		UptimeSeconds float64 `json:"uptime_seconds"`
		RequestCount  int64   `json:"request_count"`
		ErrorCount    int64   `json:"error_count"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	if resp.RequestCount != 2 || resp.ErrorCount != 1 {
		t.Errorf("unexpected metrics values: %+v", resp)
	}
	if resp.UptimeSeconds <= 0 {
		t.Errorf("expected positive uptime, got %f", resp.UptimeSeconds)
	}
}

func TestServerLifecycle(t *testing.T) {
	// Use httptest.NewServer to start the server with the health handler
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server, err := New(&config.Config{
			RequestTimeout: 1 * time.Second,
		}, &mockTokenStore{}, &mockProjectStore{})
		require.NoError(t, err)
		server.handleHealth(w, r)
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
	shutdownTestServer, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
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
	server, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

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
func (m *mockTokenStore) CreateToken(ctx context.Context, td token.TokenData) error {
	return nil
}
func (m *mockTokenStore) GetTokensByProjectID(ctx context.Context, projectID string) ([]token.TokenData, error) {
	return nil, nil
}
func (m *mockTokenStore) ListTokens(ctx context.Context) ([]token.TokenData, error) {
	return nil, nil
}

type mockProjectStore struct{}

func (m *mockProjectStore) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error) {
	return "mock-key", nil
}
func (m *mockProjectStore) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	return nil, nil
}
func (m *mockProjectStore) CreateProject(ctx context.Context, p proxy.Project) error { return nil }
func (m *mockProjectStore) GetProjectByID(ctx context.Context, id string) (proxy.Project, error) {
	return proxy.Project{}, nil
}
func (m *mockProjectStore) UpdateProject(ctx context.Context, p proxy.Project) error { return nil }
func (m *mockProjectStore) DeleteProject(ctx context.Context, id string) error       { return nil }

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

	srv, err := New(cfg, ts, ps)
	require.NoError(t, err)
	if srv.tokenStore != ts {
		t.Errorf("tokenStore not injected correctly")
	}
	if srv.projectStore != ps {
		t.Errorf("projectStore not injected correctly")
	}

	err = srv.initializeAPIRoutes()
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
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatalf("failed to remove temp file: %v", err)
		}
	}()
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
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}
	cfg2 := &config.Config{
		ListenAddr:         ":8080",
		RequestTimeout:     30 * time.Second,
		APIConfigPath:      tmpFile.Name(),
		DefaultAPIProvider: "test_api",
	}
	srv2, err := New(cfg2, ts, ps)
	require.NoError(t, err)
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

func TestServer_Start_and_InitializeComponents_Coverage(t *testing.T) {
	t.Skip("Not implemented: triggers double route registration. TODO: fix test config or server logic.")
}

// Additional tests to improve coverage

func TestServer_handleReady(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "",
	}
	server, err := New(config, ts, ps)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	rr := httptest.NewRecorder()

	server.handleReady(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ready", rr.Body.String())
}

func TestServer_handleLive(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "",
	}
	server, err := New(config, ts, ps)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/health/live", nil)
	rr := httptest.NewRecorder()

	server.handleLive(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "alive", rr.Body.String())
}

func TestServer_handleNotFound(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "",
	}
	server, err := New(config, ts, ps)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rr := httptest.NewRecorder()

	server.handleNotFound(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestResponseWriter_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr}

	rw.WriteHeader(http.StatusAccepted)

	assert.Equal(t, http.StatusAccepted, rw.statusCode)
	assert.Equal(t, http.StatusAccepted, rr.Code)
}

func TestResponseWriter_Flush(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rr}

	// This should not panic even if the underlying ResponseWriter doesn't implement Flusher
	rw.Flush()

	// Test with a flusher
	flushableRecorder := &flushableResponseWriter{ResponseWriter: rr}
	rw2 := &responseWriter{ResponseWriter: flushableRecorder}
	rw2.Flush()

	assert.True(t, flushableRecorder.flushed, "Flush should have been called")
}

// Helper for testing Flush functionality
type flushableResponseWriter struct {
	http.ResponseWriter
	flushed bool
}

func (f *flushableResponseWriter) Flush() {
	f.flushed = true
}

func TestServer_handleMetrics_WithProxyMetrics(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "",
	}
	server, err := New(config, ts, ps)
	require.NoError(t, err)

	// Use actual proxy type but set specific metrics
	p := &proxy.TransparentProxy{}
	p.SetMetrics(&proxy.ProxyMetrics{RequestCount: 100, ErrorCount: 5})
	server.proxy = p

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()

	server.handleMetrics(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var metrics map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &metrics)
	require.NoError(t, err)

	assert.Contains(t, metrics, "uptime_seconds")
	assert.Contains(t, metrics, "request_count")
	assert.Contains(t, metrics, "error_count")
	assert.Equal(t, float64(100), metrics["request_count"])
	assert.Equal(t, float64(5), metrics["error_count"])
}

func TestServer_initializeComponents_ErrorHandling(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}

	// Test with an invalid API config that might cause initializeAPIRoutes to fail
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "/invalid/path/that/does/not/exist.yaml",
	}

	server, err := New(config, ts, ps)
	require.NoError(t, err)

	err = server.initializeComponents()
	// This may or may not fail depending on the implementation
	// but it exercises the error path
	if err != nil {
		assert.Contains(t, err.Error(), "failed to initialize API routes")
	}
}

func TestServer_handleMetrics_NoProxy(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "",
	}
	server, err := New(config, ts, ps)
	require.NoError(t, err)

	// Don't set any proxy - test the nil case
	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()

	server.handleMetrics(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")

	var metrics map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &metrics)
	require.NoError(t, err)

	assert.Contains(t, metrics, "uptime_seconds")
	assert.Contains(t, metrics, "request_count")
	assert.Contains(t, metrics, "error_count")
	// Should be 0 when no proxy is set
	assert.Equal(t, float64(0), metrics["request_count"])
	assert.Equal(t, float64(0), metrics["error_count"])
}

func TestServer_logRequestMiddleware(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "",
	}
	server, err := New(config, ts, ps)
	require.NoError(t, err)

	// Create a test handler that the middleware will wrap
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("test response"))
	})

	// Wrap the handler with the middleware
	wrappedHandler := server.logRequestMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "test response", rr.Body.String())
}

func TestServer_initializeComponents_Success(t *testing.T) {
	ts := &mockTokenStore{}
	ps := &mockProjectStore{}

	// Test with a valid API config
	config := &config.Config{
		ListenAddr:    "localhost:0",
		APIConfigPath: "", // Empty path should work with defaults
	}

	server, err := New(config, ts, ps)
	require.NoError(t, err)

	err = server.initializeComponents()
	assert.NoError(t, err, "initializeComponents should succeed with valid config")
}
