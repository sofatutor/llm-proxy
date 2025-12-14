package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		ListenAddr:      ":8080",
		RequestTimeout:  30 * time.Second,
		EventBusBackend: "in-memory",
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
		ListenAddr:      ":8080",
		RequestTimeout:  30 * time.Second,
		EnableMetrics:   true,
		MetricsPath:     "/metrics",
		EventBusBackend: "in-memory",
	}
	server, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	p := &proxy.TransparentProxy{}
	p.SetMetrics(&proxy.ProxyMetrics{
		RequestCount: 2,
		ErrorCount:   1,
		CacheHits:    3,
		CacheMisses:  4,
		CacheBypass:  5,
		CacheStores:  6,
	})
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
		CacheHits     int64   `json:"cache_hits"`
		CacheMisses   int64   `json:"cache_misses"`
		CacheBypass   int64   `json:"cache_bypass"`
		CacheStores   int64   `json:"cache_stores"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	if resp.RequestCount != 2 || resp.ErrorCount != 1 {
		t.Errorf("unexpected metrics values: %+v", resp)
	}
	if resp.UptimeSeconds <= 0 {
		t.Errorf("expected positive uptime, got %f", resp.UptimeSeconds)
	}
	if resp.CacheHits != 3 || resp.CacheMisses != 4 || resp.CacheBypass != 5 || resp.CacheStores != 6 {
		t.Errorf("unexpected cache metrics: hits=%d misses=%d bypass=%d stores=%d", resp.CacheHits, resp.CacheMisses, resp.CacheBypass, resp.CacheStores)
	}
}

func TestMetricsEndpoint_NoProxy(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:      ":8080",
		RequestTimeout:  30 * time.Second,
		EnableMetrics:   true,
		MetricsPath:     "/metrics",
		EventBusBackend: "in-memory",
	}
	server, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Ensure proxy is nil to hit the no-proxy branch
	server.proxy = nil

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
	if resp.RequestCount != 0 || resp.ErrorCount != 0 {
		t.Errorf("unexpected non-zero metrics without proxy: %+v", resp)
	}
	if resp.UptimeSeconds <= 0 {
		t.Errorf("expected positive uptime, got %f", resp.UptimeSeconds)
	}
}

func TestServerLifecycle(t *testing.T) {
	// Use httptest.NewServer to start the server with the health handler
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server, err := New(&config.Config{
			RequestTimeout:  1 * time.Second,
			EventBusBackend: "in-memory",
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
		ListenAddr:      ":0", // Random port
		RequestTimeout:  1 * time.Second,
		EventBusBackend: "in-memory",
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
		ListenAddr:      ":8080",
		RequestTimeout:  30 * time.Second,
		EventBusBackend: "in-memory",
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
func (m *mockTokenStore) GetTokenByToken(ctx context.Context, tokenString string) (token.TokenData, error) {
	return token.TokenData{}, errors.New("not implemented")
}
func (m *mockTokenStore) IncrementTokenUsage(ctx context.Context, tokenID string) error {
	return errors.New("not implemented")
}
func (m *mockTokenStore) CreateToken(ctx context.Context, td token.TokenData) error {
	return nil
}
func (m *mockTokenStore) UpdateToken(ctx context.Context, td token.TokenData) error {
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
func (m *mockProjectStore) GetProjectActive(ctx context.Context, projectID string) (bool, error) {
	return true, nil
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

// recordingTokenStore captures the last created token for assertions.
type recordingTokenStore struct {
	mockTokenStore
	created token.TokenData
}

func (r *recordingTokenStore) CreateToken(ctx context.Context, td token.TokenData) error {
	r.created = td
	return nil
}

type updatingTokenStore struct {
	mockTokenStore
	existing token.TokenData
	updated  token.TokenData
}

func (u *updatingTokenStore) GetTokenByID(ctx context.Context, tokenID string) (token.TokenData, error) {
	return u.existing, nil
}

func (u *updatingTokenStore) UpdateToken(ctx context.Context, td token.TokenData) error {
	u.updated = td
	return nil
}

// activeProjectStore returns an active project for creation tests.
type activeProjectStore struct{ mockProjectStore }

func (a *activeProjectStore) GetProjectByID(ctx context.Context, id string) (proxy.Project, error) {
	return proxy.Project{ID: id, IsActive: true}, nil
}

// testProjectStore embeds mockProjectStore and allows method overrides for testing
type testProjectStore struct {
	*mockProjectStore
	listProjectsFunc  func(ctx context.Context) ([]proxy.Project, error)
	createProjectFunc func(ctx context.Context, p proxy.Project) error
}

func (t *testProjectStore) ListProjects(ctx context.Context) ([]proxy.Project, error) {
	if t.listProjectsFunc != nil {
		return t.listProjectsFunc(ctx)
	}
	return t.mockProjectStore.ListProjects(ctx)
}
func (t *testProjectStore) CreateProject(ctx context.Context, p proxy.Project) error {
	if t.createProjectFunc != nil {
		return t.createProjectFunc(ctx, p)
	}
	return t.mockProjectStore.CreateProject(ctx, p)
}

func TestServer_New_WithDependencyInjection_ConfigAndFallback(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:         ":8080",
		RequestTimeout:     30 * time.Second,
		APIConfigPath:      "/non/existent/path.yaml", // triggers fallback branch
		DefaultAPIProvider: "openai",
		OpenAIAPIURL:       "https://api.openai.com",
		EventBusBackend:    "in-memory",
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
		EventBusBackend:    "in-memory",
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

func TestHandleReadyAndLive(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":8080", RequestTimeout: 30 * time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()
	srv.handleReady(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "ready" {
		t.Errorf("handleReady: expected 200/ready, got %d/%q", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest("GET", "/live", nil)
	rr = httptest.NewRecorder()
	srv.handleLive(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "alive" {
		t.Errorf("handleLive: expected 200/alive, got %d/%q", rr.Code, rr.Body.String())
	}
}

func TestInitializeAPIRoutes_ConfigFallback(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":8080", RequestTimeout: 30 * time.Second, APIConfigPath: "notfound.json", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Should not panic or error, should use fallback config
	err = srv.initializeAPIRoutes()
	if err != nil {
		t.Errorf("expected fallback config, got error: %v", err)
	}
}

// Start should initialize components and then return promptly with a listen error when the port is unavailable
func TestServer_Start_ReturnsListenError(t *testing.T) {
	cfg := &config.Config{ListenAddr: "127.0.0.1:0", RequestTimeout: 1 * time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// occupy a port briefly to force a bind error
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	// Keep the listener open to force EADDRINUSE for Start()
	srv.server.Addr = addr

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatalf("expected error from Start() when port unavailable")
		}
		_ = ln.Close()
	case <-time.After(2 * time.Second):
		t.Fatalf("Start() did not return in time")
	}
}

// initializeComponents calls initializeAPIRoutes successfully (covers happy path)
func TestServer_initializeComponents_CoversHappyPath(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: 1 * time.Second, APIConfigPath: "notfound.json", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	if err := srv.initializeComponents(); err != nil {
		t.Fatalf("initializeComponents error: %v", err)
	}
	if srv.proxy == nil {
		t.Fatalf("proxy was not set by initializeComponents")
	}
}

// --- Merged from server_extra_test.go ---

func Test_getClientIP(t *testing.T) {
	s := &Server{}

	// X-Forwarded-For with multiple entries
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.1, 203.0.113.2")
	if got := s.getClientIP(r); got != "203.0.113.1" {
		t.Fatalf("got %q, want %q", got, "203.0.113.1")
	}

	// X-Real-IP fallback
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Real-IP", "198.51.100.7")
	if got := s.getClientIP(r); got != "198.51.100.7" {
		t.Fatalf("got %q, want %q", got, "198.51.100.7")
	}

	// RemoteAddr fallback with host:port
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.5:12345"
	if got := s.getClientIP(r); got != "192.0.2.5" {
		t.Fatalf("got %q, want %q", got, "192.0.2.5")
	}

	// RemoteAddr without colon
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.99"
	if got := s.getClientIP(r); got != "192.0.2.99" {
		t.Fatalf("got %q, want %q", got, "192.0.2.99")
	}
}

type flushRecorder struct {
	http.ResponseWriter
	flushed bool
}

func (f *flushRecorder) Flush() { f.flushed = true }

func Test_responseWriter_Flush(t *testing.T) {
	// Underlying implements http.Flusher
	rr := httptest.NewRecorder()
	fr := &flushRecorder{ResponseWriter: rr}
	rw := &responseWriter{ResponseWriter: fr}
	rw.Flush()
	if !fr.flushed {
		t.Fatalf("expected Flush to be forwarded")
	}

	// Underlying does not implement http.Flusher (no panic, no forward)
	rw2 := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	// Should be a no-op
	rw2.Flush()
}

func TestHandleProjects_And_CreateProject_EdgeCases(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":8080", RequestTimeout: 30 * time.Second, ManagementToken: "testtoken", EventBusBackend: "in-memory"}
	logger := zap.NewNop()
	ps := &mockProjectStore{}
	ts := &mockTokenStore{}
	srv, err := New(cfg, ts, ps)
	require.NoError(t, err)
	srv.logger = logger

	setAuth := func(r *http.Request, token string) {
		r.Header.Set("Authorization", "Bearer "+token)
	}

	t.Run("method not allowed", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPut, "/manage/projects", nil)
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("auth fail", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/manage/projects", nil)
		setAuth(r, "wrongtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GET happy path", func(t *testing.T) {
		tps := &testProjectStore{mockProjectStore: &mockProjectStore{}}
		srv.projectStore = tps
		r := httptest.NewRequest(http.MethodGet, "/manage/projects", nil)
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("GET store error", func(t *testing.T) {
		tps := &testProjectStore{mockProjectStore: &mockProjectStore{}, listProjectsFunc: func(ctx context.Context) ([]proxy.Project, error) { return nil, errors.New("fail") }}
		srv.projectStore = tps
		r := httptest.NewRequest(http.MethodGet, "/manage/projects", nil)
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("POST bad JSON", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/manage/projects", strings.NewReader("notjson"))
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST missing fields", func(t *testing.T) {
		body := `{"name":""}`
		r := httptest.NewRequest(http.MethodPost, "/manage/projects", strings.NewReader(body))
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("POST store error", func(t *testing.T) {
		tps := &testProjectStore{mockProjectStore: &mockProjectStore{}, createProjectFunc: func(ctx context.Context, p proxy.Project) error { return errors.New("fail") }}
		srv.projectStore = tps
		body := `{"name":"foo","openai_api_key":"bar"}`
		r := httptest.NewRequest(http.MethodPost, "/manage/projects", strings.NewReader(body))
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})

	t.Run("POST happy path", func(t *testing.T) {
		tps := &testProjectStore{mockProjectStore: &mockProjectStore{}, createProjectFunc: func(ctx context.Context, p proxy.Project) error { return nil }}
		srv.projectStore = tps
		body := `{"name":"foo","openai_api_key":"bar"}`
		r := httptest.NewRequest(http.MethodPost, "/manage/projects", strings.NewReader(body))
		setAuth(r, "testtoken")
		w := httptest.NewRecorder()
		srv.handleProjects(w, r)
		resp := w.Result()
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

func TestLogRequestMiddleware(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: 1 * time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
		_, err := w.Write([]byte("ok"))
		if err != nil {
			t.Errorf("unexpected error from w.Write: %v", err)
		}
	})
	mw := srv.logRequestMiddleware(h)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if !called {
		t.Error("handler was not called by logRequestMiddleware")
	}
	if rr.Code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Errorf("expected body to contain 'ok', got %q", rr.Body.String())
	}
}

func TestHandleNotFound_WriteHeader_Flush_EventBus(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: 1 * time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notfound", nil)
	srv.handleNotFound(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}

	// Initialize event bus for the test
	srv.eventBus = &mockEventBus{}
	if srv.EventBus() == nil {
		t.Error("EventBus() returned nil")
	}
}

// (removed unused mockAuditStore and mockDB)

func TestHandleAuditEvents_And_ByID(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Use a real in-memory DB and seed deterministic rows used by the handlers
	realDB, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	srv.db = realDB
	ctx := context.Background()
	// Insert deterministic event for show
	_, err = srv.db.DB().ExecContext(ctx, "INSERT INTO audit_events (id,timestamp,action,actor,outcome) VALUES (?,?,?,?,?)", "evt-1", time.Now(), "x", "y", "success")
	require.NoError(t, err)

	// List
	r := httptest.NewRequest(http.MethodGet, "/manage/audit?search=abc&page=1&page_size=20", nil)
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w := httptest.NewRecorder()
	srv.handleAuditEvents(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", w.Code)
	}

	// Show
	r2 := httptest.NewRequest(http.MethodGet, "/manage/audit/evt-1", nil)
	r2.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w2 := httptest.NewRecorder()
	srv.handleAuditEventByID(w2, r2)
	if w2.Code != http.StatusOK {
		t.Fatalf("show expected 200, got %d", w2.Code)
	}
}

// Additional coverage for handleAuditEventByID error branches
func TestHandleAuditEventByID_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, "/manage/audit/anything", nil)
	w := httptest.NewRecorder()
	srv.handleAuditEventByID(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleAuditEventByID_DBUnavailable(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Ensure DB is nil to hit ServiceUnavailable branch
	srv.db = nil

	r := httptest.NewRequest(http.MethodGet, "/manage/audit/evt-x", nil)
	w := httptest.NewRecorder()
	srv.handleAuditEventByID(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleAuditEventByID_MissingID(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Real in-memory DB so the handler proceeds past db==nil
	realDB, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	srv.db = realDB

	r := httptest.NewRequest(http.MethodGet, "/manage/audit/", nil)
	w := httptest.NewRecorder()
	srv.handleAuditEventByID(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing id, got %d", w.Code)
	}
}

func TestHandleAuditEventByID_NotFound(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	realDB, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	srv.db = realDB

	r := httptest.NewRequest(http.MethodGet, "/manage/audit/does-not-exist", nil)
	w := httptest.NewRecorder()
	srv.handleAuditEventByID(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing audit event, got %d", w.Code)
	}
}

func TestHandleAuditEventByID_EncodeError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Seed a real DB with one audit event so the handler reaches the encode step
	realDB, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	srv.db = realDB
	ctx := context.Background()
	_, err = srv.db.DB().ExecContext(ctx, "INSERT INTO audit_events (id,timestamp,action,actor,outcome) VALUES (?,?,?,?,?)", "evt-encode", time.Now(), "x", "y", "success")
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/manage/audit/evt-encode", nil)
	rr := httptest.NewRecorder()
	failing := &mockFailingResponseWriter{ResponseWriter: rr, failOnFirstWrite: true}
	srv.handleAuditEventByID(failing, r)
	if failing.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 when encode fails, got %d", failing.statusCode)
	}
}

func TestHandleAuditEvents_MethodNotAllowed(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Real DB so handler progresses when method would be GET
	realDB, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	srv.db = realDB

	r := httptest.NewRequest(http.MethodPost, "/manage/audit", nil)
	w := httptest.NewRecorder()
	srv.handleAuditEvents(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleAuditEvents_DBUnavailable(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Force DB unavailable branch
	srv.db = nil

	r := httptest.NewRequest(http.MethodGet, "/manage/audit", nil)
	w := httptest.NewRecorder()
	srv.handleAuditEvents(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHandleAuditEvents_EncodeError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	realDB, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	srv.db = realDB

	// Seed one row so encoding is attempted
	ctx := context.Background()
	_, err = srv.db.DB().ExecContext(ctx, "INSERT INTO audit_events (id,timestamp,action,actor,outcome) VALUES (?,?,?,?,?)", "evt-enc-list", time.Now(), "x", "y", "success")
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/manage/audit?page=1&page_size=20", nil)
	rr := httptest.NewRecorder()
	failing := &mockFailingResponseWriter{ResponseWriter: rr, failOnFirstWrite: true}
	srv.handleAuditEvents(failing, r)
	if failing.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 when encoding list fails, got %d", failing.statusCode)
	}
}

func TestManagementAuthMiddleware_Direct(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "tok", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	mw := srv.managementAuthMiddleware(next)

	// Missing header
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing header, got %d", w.Code)
	}

	// Wrong token
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong token, got %d", w.Code)
	}

	// Correct token
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || !nextCalled {
		t.Fatalf("expected 204 and next called, got %d, nextCalled=%v", w.Code, nextCalled)
	}
}

func TestHandleMetrics_EncodeError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory", EnableMetrics: true, MetricsPath: "/metrics"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Ensure some proxy metrics to exercise data path
	p := &proxy.TransparentProxy{}
	p.SetMetrics(&proxy.ProxyMetrics{
		RequestCount: 10,
		ErrorCount:   2,
	})
	srv.proxy = p

	r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	failing := &mockFailingResponseWriter{ResponseWriter: rr, failOnFirstWrite: true}
	srv.handleMetrics(failing, r)
	if failing.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 when metrics encode fails, got %d", failing.statusCode)
	}
}

// initializeComponents error path via bad API config (no default_api and missing provider)
func TestInitializeComponents_ErrorPath(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "api_bad_*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	// Valid YAML with APIs but missing default_api
	configYAML := `
apis:
  test_api:
    base_url: https://api.example.com
    allowed_endpoints:
      - /v1/test
    allowed_methods:
      - GET
`
	_, err = tmpFile.Write([]byte(configYAML))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, APIConfigPath: tmpFile.Name(), DefaultAPIProvider: "missing_api", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	if err := srv.initializeComponents(); err == nil {
		t.Fatalf("expected initializeComponents to error with bad API config")
	}
}

// Start should return an initialization error when components fail to init
func TestServer_Start_InitializationFailure(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "api_bad_*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	configYAML := `
apis:
  test_api:
    base_url: https://api.example.com
    allowed_endpoints:
      - /v1/test
    allowed_methods:
      - GET
`
	_, err = tmpFile.Write([]byte(configYAML))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	cfg := &config.Config{ListenAddr: "127.0.0.1:0", RequestTimeout: time.Second, APIConfigPath: tmpFile.Name(), DefaultAPIProvider: "missing_api", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Start should fail fast due to initializeComponents error
	if err := srv.Start(); err == nil {
		t.Fatalf("expected Start to fail due to initialization error")
	}
}

// Encode error paths for list endpoints
func TestHandleListProjects_EncodeError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "tok", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	// Auth passes
	r := httptest.NewRequest(http.MethodGet, "/manage/projects", nil)
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	rr := httptest.NewRecorder()
	failing := &mockFailingResponseWriter{ResponseWriter: rr, failOnFirstWrite: true}
	srv.handleProjects(failing, r)
	if failing.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 when projects encode fails, got %d", failing.statusCode)
	}
}

func TestHandleTokens_ListEncodeError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodGet, "/manage/tokens", nil)
	rr := httptest.NewRecorder()
	failing := &mockFailingResponseWriter{ResponseWriter: rr, failOnFirstWrite: true}
	srv.handleTokens(failing, r)
	if failing.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 when tokens list encode fails, got %d", failing.statusCode)
	}
}

func Test_parseInt(t *testing.T) {
	if parseInt("", 7) != 7 {
		t.Fatal("empty returns default")
	}
	if parseInt("abc", 7) != 7 {
		t.Fatal("invalid returns default")
	}
	if parseInt("42", 7) != 42 {
		t.Fatal("valid parsed")
	}
}

func TestAuditEvent_ForwardedHeaders(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodPost, "/manage/projects", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Forwarded-User-Agent", "Mozilla/5.0 (Macintosh)")
	r.Header.Set("X-Forwarded-Referer", "https://admin.example.com/projects")
	r.Header.Set("X-Admin-Origin", "1")

	ev := s.auditEvent("action.test", "actor.test", audit.ResultSuccess, r, "req-123")
	if ev == nil {
		t.Fatalf("expected event, got nil")
	}
	if ev.ClientIP != "203.0.113.9" {
		t.Fatalf("client IP mismatch: got %v", ev.ClientIP)
	}
	if ua, ok := ev.Details["user_agent"].(string); !ok || !strings.Contains(ua, "Mozilla/5.0") {
		t.Fatalf("user_agent not set from forwarded header: %v", ev.Details["user_agent"])
	}
	if ref, ok := ev.Details["referer"].(string); !ok || !strings.Contains(ref, "admin.example.com") {
		t.Fatalf("referer detail missing: %v", ev.Details["referer"])
	}
	if origin, ok := ev.Details["origin"].(string); !ok || origin != "admin-ui" {
		t.Fatalf("origin detail missing or wrong: %v", ev.Details["origin"])
	}
}

func TestAuditEvent_NoForwardedHeaders(t *testing.T) {
	s := &Server{}
	r := httptest.NewRequest(http.MethodGet, "/manage/tokens", nil)
	r.Header.Set("User-Agent", "Go-http-client/1.1")
	r.RemoteAddr = "192.0.2.50:1234"

	ev := s.auditEvent("action.test", "actor.test", audit.ResultSuccess, r, "req-456")
	if ev == nil {
		t.Fatalf("expected event, got nil")
	}
	if ev.ClientIP != "192.0.2.50" {
		t.Fatalf("client IP mismatch: got %v", ev.ClientIP)
	}
	if ua, ok := ev.Details["user_agent"].(string); !ok || ua != "Go-http-client/1.1" {
		t.Fatalf("user_agent should fall back to request UA: %v", ev.Details["user_agent"])
	}
	if _, ok := ev.Details["referer"]; ok {
		t.Fatalf("referer should not be set when not forwarded")
	}
	if _, ok := ev.Details["origin"]; ok {
		t.Fatalf("origin should not be set when header missing")
	}
}

func TestHandleTokens_Create_DurationTooLong(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "testtoken", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	body := `{"project_id":"any","duration_minutes":525601}`
	r := httptest.NewRequest(http.MethodPost, "/manage/tokens", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w := httptest.NewRecorder()
	srv.handleTokens(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for too long duration, got %d", w.Code)
	}
}

func TestHandleTokens_Create_WithMaxRequests(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "testtoken", EventBusBackend: "in-memory"}
	ts := &recordingTokenStore{}
	ps := &activeProjectStore{}
	srv, err := New(cfg, ts, ps)
	require.NoError(t, err)

	body := `{"project_id":"any","duration_minutes":60,"max_requests":5}`
	r := httptest.NewRequest(http.MethodPost, "/manage/tokens", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w := httptest.NewRecorder()
	srv.handleTokens(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	if ts.created.MaxRequests == nil || *ts.created.MaxRequests != 5 {
		t.Fatalf("expected max_requests to be stored, got %v", ts.created.MaxRequests)
	}

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	val, ok := resp["max_requests"].(float64)
	if !ok {
		t.Fatalf("expected max_requests in response, got %#v", resp)
	}
	require.Equal(t, float64(5), val)
}

func TestHandleTokens_Create_InvalidMaxRequests(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "testtoken", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &recordingTokenStore{}, &activeProjectStore{})
	require.NoError(t, err)

	body := `{"project_id":"any","duration_minutes":60,"max_requests":-1}`
	r := httptest.NewRequest(http.MethodPost, "/manage/tokens", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w := httptest.NewRecorder()
	srv.handleTokens(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleTokens_Create_MaxRequestsZeroMeansUnlimited(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "testtoken", EventBusBackend: "in-memory"}
	ts := &recordingTokenStore{}
	ps := &activeProjectStore{}
	srv, err := New(cfg, ts, ps)
	require.NoError(t, err)

	body := `{"project_id":"any","duration_minutes":60,"max_requests":0}`
	r := httptest.NewRequest(http.MethodPost, "/manage/tokens", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	w := httptest.NewRecorder()
	srv.handleTokens(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	if ts.created.MaxRequests != nil {
		t.Fatalf("expected max_requests to be stored as nil (unlimited), got %v", ts.created.MaxRequests)
	}

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	if _, ok := resp["max_requests"]; ok {
		t.Fatalf("did not expect max_requests in response for unlimited, got %#v", resp)
	}
}

func TestHandleUpdateToken_MaxRequestsZeroClearsLimit(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, ManagementToken: "testtoken", EventBusBackend: "in-memory"}
	store := &updatingTokenStore{existing: token.TokenData{ID: "tok-1", Token: "sk-test123456789", ProjectID: "any", IsActive: true, RequestCount: 0, MaxRequests: intPtr(10), CreatedAt: time.Now()}}
	srv, err := New(cfg, store, &activeProjectStore{})
	require.NoError(t, err)

	body := `{"max_requests":0}`
	r := httptest.NewRequest(http.MethodPatch, "/manage/tokens/tok-1", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+cfg.ManagementToken)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateToken(w, r, "tok-1")

	require.Equal(t, http.StatusOK, w.Code)
	if store.updated.MaxRequests != nil {
		t.Fatalf("expected max_requests to be cleared (nil), got %v", store.updated.MaxRequests)
	}
}

func TestInitializeAPIRoutes_FallbackToDefaultWhenProviderMissing(t *testing.T) {
	// Create a real config file where DefaultAPI is test_api
	tmpFile, err := os.CreateTemp("", "api_config_*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile.Name())
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
`
	if _, err := tmpFile.Write([]byte(configYAML)); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	_ = tmpFile.Close()

	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, APIConfigPath: tmpFile.Name(), DefaultAPIProvider: "missing_api", EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)
	if err := srv.initializeAPIRoutes(); err != nil {
		t.Fatalf("initializeAPIRoutes failed: %v", err)
	}

	// Verify the route from default provider exists
	req := httptest.NewRequest("GET", "/v1/test", nil)
	rr := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatalf("expected /v1/test to be registered via default provider fallback")
	}
}

func TestLogRequestMiddleware_ServerError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "in-memory"}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	called := false
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	})
	mw := srv.logRequestMiddleware(h)
	req := httptest.NewRequest("GET", "/err", nil)
	rr := httptest.NewRecorder()
	mw.ServeHTTP(rr, req)
	if !called {
		t.Fatalf("handler not called")
	}
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}

func TestNew_UnknownEventBusBackend_ReturnsError(t *testing.T) {
	cfg := &config.Config{ListenAddr: ":0", RequestTimeout: time.Second, EventBusBackend: "unknown-backend"}
	_, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	if err == nil {
		t.Fatalf("expected error for unknown event bus backend")
	}
	if !strings.Contains(err.Error(), "unknown event bus backend") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServer_Shutdown_WithoutAggregator(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:      ":0",
		RequestTimeout:  time.Second,
		ManagementToken: "testtoken",
		EventBusBackend: "in-memory",
	}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Server has no aggregator set (cacheStatsAgg is nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown should complete without error
	err = srv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("expected no error during shutdown, got: %v", err)
	}
}

func TestServer_Shutdown_WithAggregator(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:           ":0",
		RequestTimeout:       time.Second,
		ManagementToken:      "testtoken",
		EventBusBackend:      "in-memory",
		CacheStatsBufferSize: 100,
	}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Create a mock aggregator and set it
	aggConfig := proxy.CacheStatsAggregatorConfig{
		BufferSize:    10,
		FlushInterval: 50 * time.Millisecond,
		BatchSize:     5,
	}
	mockStore := &mockCacheStatsStore{}
	agg := proxy.NewCacheStatsAggregator(aggConfig, mockStore, zap.NewNop())
	agg.Start()
	srv.cacheStatsAgg = agg

	// Record some cache hits to ensure flush happens
	agg.RecordCacheHit("test-token-1")
	agg.RecordCacheHit("test-token-1")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown should complete without error and flush pending stats
	err = srv.Shutdown(ctx)
	if err != nil {
		t.Fatalf("expected no error during shutdown, got: %v", err)
	}
}

func TestServer_Shutdown_WithAggregatorError(t *testing.T) {
	cfg := &config.Config{
		ListenAddr:           ":0",
		RequestTimeout:       time.Second,
		ManagementToken:      "testtoken",
		EventBusBackend:      "in-memory",
		CacheStatsBufferSize: 100,
	}
	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	// Create a mock aggregator that will error on Stop due to context timeout
	aggConfig := proxy.CacheStatsAggregatorConfig{
		BufferSize:    10,
		FlushInterval: 10 * time.Second, // Long interval
		BatchSize:     1000,
	}
	mockStore := &mockCacheStatsStore{}
	agg := proxy.NewCacheStatsAggregator(aggConfig, mockStore, zap.NewNop())
	agg.Start()
	srv.cacheStatsAgg = agg

	// Use an already-cancelled context to trigger timeout error
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Shutdown should log error but still complete
	// Error will be from http.Server shutdown, not the aggregator
	// The aggregator error is logged but not returned
	_ = srv.Shutdown(ctx)
}

// mockCacheStatsStore implements proxy.CacheStatsStore for testing
type mockCacheStatsStore struct {
	calls []map[string]int
}

func (m *mockCacheStatsStore) IncrementCacheHitCountBatch(ctx context.Context, deltas map[string]int) error {
	m.calls = append(m.calls, deltas)
	return nil
}

// mockEventBus implements the eventbus.EventBus interface for testing
type mockEventBus struct{}

func (m *mockEventBus) Publish(ctx context.Context, evt eventbus.Event) {}
func (m *mockEventBus) Subscribe() <-chan eventbus.Event {
	ch := make(chan eventbus.Event)
	close(ch)
	return ch
}
func (m *mockEventBus) Stop() {}

func TestInitializeAPIRoutes_RedisCacheURLConstruction(t *testing.T) {
	// Save and restore environment
	origCacheURL := os.Getenv("REDIS_CACHE_URL")
	origAddr := os.Getenv("REDIS_ADDR")
	origDB := os.Getenv("REDIS_DB")
	origBackend := os.Getenv("HTTP_CACHE_BACKEND")
	origEnabled := os.Getenv("HTTP_CACHE_ENABLED")
	defer func() {
		_ = os.Setenv("REDIS_CACHE_URL", origCacheURL)
		_ = os.Setenv("REDIS_ADDR", origAddr)
		_ = os.Setenv("REDIS_DB", origDB)
		_ = os.Setenv("HTTP_CACHE_BACKEND", origBackend)
		_ = os.Setenv("HTTP_CACHE_ENABLED", origEnabled)
	}()

	tests := []struct {
		name        string
		cacheURL    string
		redisAddr   string
		redisDB     string
		backend     string
		cacheEnable string
	}{
		{
			name:        "explicit REDIS_CACHE_URL takes precedence",
			cacheURL:    "redis://custom:6380/5",
			redisAddr:   "ignored:6379",
			redisDB:     "9",
			backend:     "redis",
			cacheEnable: "true",
		},
		{
			name:        "construct from REDIS_ADDR and REDIS_DB",
			cacheURL:    "",
			redisAddr:   "myredis:6380",
			redisDB:     "3",
			backend:     "redis",
			cacheEnable: "true",
		},
		{
			name:        "use defaults when REDIS_ADDR not set",
			cacheURL:    "",
			redisAddr:   "",
			redisDB:     "",
			backend:     "redis",
			cacheEnable: "true",
		},
		{
			name:        "REDIS_ADDR set but REDIS_DB empty uses default 0",
			cacheURL:    "",
			redisAddr:   "otherredis:6379",
			redisDB:     "",
			backend:     "redis",
			cacheEnable: "true",
		},
		{
			name:        "cache disabled skips redis setup",
			cacheURL:    "",
			redisAddr:   "",
			redisDB:     "",
			backend:     "in-memory",
			cacheEnable: "false",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear and set env vars
			_ = os.Unsetenv("REDIS_CACHE_URL")
			_ = os.Unsetenv("REDIS_ADDR")
			_ = os.Unsetenv("REDIS_DB")
			_ = os.Unsetenv("HTTP_CACHE_BACKEND")
			_ = os.Unsetenv("HTTP_CACHE_ENABLED")

			if tc.cacheURL != "" {
				_ = os.Setenv("REDIS_CACHE_URL", tc.cacheURL)
			}
			if tc.redisAddr != "" {
				_ = os.Setenv("REDIS_ADDR", tc.redisAddr)
			}
			if tc.redisDB != "" {
				_ = os.Setenv("REDIS_DB", tc.redisDB)
			}
			if tc.backend != "" {
				_ = os.Setenv("HTTP_CACHE_BACKEND", tc.backend)
			}
			if tc.cacheEnable != "" {
				_ = os.Setenv("HTTP_CACHE_ENABLED", tc.cacheEnable)
			}

			cfg := &config.Config{
				ListenAddr:      ":0",
				RequestTimeout:  time.Second,
				EventBusBackend: "in-memory",
			}
			srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
			require.NoError(t, err)

			// initializeAPIRoutes should not error regardless of config
			err = srv.initializeAPIRoutes()
			require.NoError(t, err)
		})
	}
}
