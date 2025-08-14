package admin

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSessionSecret(t *testing.T) {
	cfg := &config.Config{AdminUI: config.AdminUIConfig{ManagementToken: "secret-token"}}
	secret := getSessionSecret(cfg)
	want := []byte("secret-tokenllmproxy-cookie-salt")
	if string(secret) != string(want) {
		t.Errorf("getSessionSecret() = %q, want %q", secret, want)
	}
}

func TestNewServer_Success(t *testing.T) {
	// Create a temporary template directory for testing
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "templates")
	err := os.MkdirAll(templateDir, 0755)
	require.NoError(t, err)

	// Create subdirectories and templates to avoid template loading errors
	subDirs := []string{"projects", "tokens", "audit"}
	for _, subDir := range subDirs {
		err = os.MkdirAll(filepath.Join(templateDir, subDir), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(templateDir, subDir, "test.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
		require.NoError(t, err)
	}

	// Create minimal template files to avoid template loading errors
	err = os.WriteFile(filepath.Join(templateDir, "base.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		LogLevel:     "info",
		LogFormat:    "json",
		LogFile:      "",
		AuditEnabled: false,
		AdminUI: config.AdminUIConfig{
			ListenAddr:      ":8081",
			ManagementToken: "test-token",
			APIBaseURL:      "http://localhost:8080",
			TemplateDir:     templateDir,
		},
	}

	server, err := NewServer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.engine)
	assert.NotNil(t, server.apiClient)
	assert.NotNil(t, server.logger)
	assert.NotNil(t, server.auditLogger)
	assert.Equal(t, cfg.AdminUI.ListenAddr, server.server.Addr)
}

func TestNewServer_NoManagementToken(t *testing.T) {
	// Create a temporary template directory for testing
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "templates")
	err := os.MkdirAll(templateDir, 0755)
	require.NoError(t, err)

	// Create subdirectories and templates to avoid template loading errors
	subDirs := []string{"projects", "tokens", "audit"}
	for _, subDir := range subDirs {
		err = os.MkdirAll(filepath.Join(templateDir, subDir), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(templateDir, subDir, "test.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
		require.NoError(t, err)
	}

	// Create minimal template files to avoid template loading errors
	err = os.WriteFile(filepath.Join(templateDir, "base.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		LogLevel:     "info",
		LogFormat:    "text",
		LogFile:      "",
		AuditEnabled: false,
		AdminUI: config.AdminUIConfig{
			ListenAddr:      ":8081",
			ManagementToken: "", // No token
			APIBaseURL:      "http://localhost:8080",
			TemplateDir:     templateDir,
		},
	}

	server, err := NewServer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.Nil(t, server.apiClient) // Should be nil when no token
}

func TestNewServer_AuditEnabled(t *testing.T) {
	// Create temporary directory for audit log
	tempDir := t.TempDir()
	auditFile := filepath.Join(tempDir, "audit.log")
	templateDir := filepath.Join(tempDir, "templates")
	err := os.MkdirAll(templateDir, 0755)
	require.NoError(t, err)

	// Create subdirectories and templates to avoid template loading errors
	subDirs := []string{"projects", "tokens", "audit"}
	for _, subDir := range subDirs {
		err = os.MkdirAll(filepath.Join(templateDir, subDir), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(templateDir, subDir, "test.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
		require.NoError(t, err)
	}

	// Create minimal template files to avoid template loading errors
	err = os.WriteFile(filepath.Join(templateDir, "base.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		LogLevel:       "debug", // Test debug mode
		LogFormat:      "json",
		LogFile:        "",
		AuditEnabled:   true,
		AuditLogFile:   auditFile,
		AuditCreateDir: true,
		AdminUI: config.AdminUIConfig{
			ListenAddr:      ":8081",
			ManagementToken: "test-token",
			APIBaseURL:      "http://localhost:8080",
			TemplateDir:     templateDir,
		},
	}

	server, err := NewServer(cfg)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.auditLogger)
}

func TestNewServer_AuditLoggerError(t *testing.T) {
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "templates")
	err := os.MkdirAll(templateDir, 0755)
	require.NoError(t, err)

	// Create subdirectories and templates to avoid template loading errors
	subDirs := []string{"projects", "tokens", "audit"}
	for _, subDir := range subDirs {
		err = os.MkdirAll(filepath.Join(templateDir, subDir), 0755)
		require.NoError(t, err)
		err = os.WriteFile(filepath.Join(templateDir, subDir, "test.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
		require.NoError(t, err)
	}

	// Create minimal template files to avoid template loading errors
	err = os.WriteFile(filepath.Join(templateDir, "base.html"), []byte(`<html><body>{{.}}</body></html>`), 0644)
	require.NoError(t, err)

	cfg := &config.Config{
		LogLevel:       "info",
		LogFormat:      "json",
		LogFile:        "",
		AuditEnabled:   true,
		AuditLogFile:   "/invalid/path/audit.log", // Invalid path
		AuditCreateDir: false,
		AdminUI: config.AdminUIConfig{
			ListenAddr:      ":8081",
			ManagementToken: "test-token",
			APIBaseURL:      "http://localhost:8080",
			TemplateDir:     templateDir,
		},
	}

	server, err := NewServer(cfg)
	assert.Error(t, err)
	assert.Nil(t, server)
	assert.Contains(t, err.Error(), "failed to initialize admin audit logger")
}

// testTemplateDir returns the absolute path to the test template directory, robust to CWD.
func testTemplateDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot get current filename")
	}
	// filename is .../internal/admin/server_test.go
	// testdata is .../internal/admin/testdata
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// createTestTemplate creates a template file with the given relative path and content,
// and registers a cleanup to remove it after the test.
func createTestTemplate(t *testing.T, relPath string, content string) string {
	t.Helper()
	fullPath := filepath.Join(testTemplateDir(), relPath)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", fullPath, err)
	}
	t.Cleanup(func() {
		if err := os.Remove(fullPath); err != nil {
			t.Errorf("failed to remove %s: %v", fullPath, err)
		}
	})
	return fullPath
}

func TestNewServer_Minimal(t *testing.T) {
	createTestTemplate(t, "base.html", "<html><body>base</body></html>")
	createTestTemplate(t, "tokens/dummy.html", "<html><body>dummy</body></html>")

	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      ":0",
			TemplateDir:     testTemplateDir(),
		},
		LogLevel: "info",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}
	if srv.config != cfg {
		t.Error("Server config not set correctly")
	}
	if srv.engine == nil {
		t.Error("Server engine not set")
	}
	if srv.server == nil {
		t.Error("Server http.Server not set")
	}
}

func TestServer_Shutdown_NoServer(t *testing.T) {
	s := &Server{}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Shutdown panicked: %v", r)
		}
	}()
	_ = s.Shutdown(context.Background())
	// Accept both nil and non-nil error, but must not panic
}

// --- Coverage for HTTP Handlers ---

// mockAPIClient implements APIClientInterface for handler tests
// Only implements methods needed for handler coverage

type mockAPIClient struct {
	DashboardData *DashboardData
	DashboardErr  error
}

func (m *mockAPIClient) GetDashboardData(ctx context.Context) (*DashboardData, error) {
	return m.DashboardData, m.DashboardErr
}

func (m *mockAPIClient) GetProjects(ctx context.Context, page, pageSize int) ([]Project, *Pagination, error) {
	if m.DashboardErr != nil {
		return nil, nil, m.DashboardErr
	}
	return []Project{{ID: "1", Name: "Test Project"}}, &Pagination{Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1, HasNext: false, HasPrev: false}, nil
}

func (m *mockAPIClient) GetTokens(ctx context.Context, projectID string, page, pageSize int) ([]Token, *Pagination, error) {
	if m.DashboardErr != nil {
		return nil, nil, m.DashboardErr
	}
	return []Token{{ProjectID: "1", IsActive: true}}, &Pagination{Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1, HasNext: false, HasPrev: false}, nil
}

func (m *mockAPIClient) CreateToken(ctx context.Context, projectID string, durationMinutes int) (*TokenCreateResponse, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &TokenCreateResponse{Token: "tok-1234", ExpiresAt: time.Now().Add(time.Duration(durationMinutes) * time.Minute)}, nil
}

func (m *mockAPIClient) GetProject(ctx context.Context, id string) (*Project, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &Project{ID: id, Name: "Test Project"}, nil
}

func (m *mockAPIClient) UpdateProject(ctx context.Context, id, name, apiKey string) (*Project, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &Project{ID: id, Name: name, OpenAIAPIKey: apiKey}, nil
}

func (m *mockAPIClient) DeleteProject(ctx context.Context, id string) error {
	return m.DashboardErr
}

func (m *mockAPIClient) CreateProject(ctx context.Context, name, apiKey string) (*Project, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &Project{ID: "new-id", Name: name, OpenAIAPIKey: apiKey}, nil
}

// Implement audit methods to satisfy APIClientInterface
func (m *mockAPIClient) GetAuditEvents(ctx context.Context, filters map[string]string, page, pageSize int) ([]AuditEvent, *Pagination, error) {
	if m.DashboardErr != nil {
		return nil, nil, m.DashboardErr
	}
	return []AuditEvent{{ID: "evt-1", Outcome: "success"}}, &Pagination{Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1}, nil
}

func (m *mockAPIClient) GetAuditEvent(ctx context.Context, id string) (*AuditEvent, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &AuditEvent{ID: id, Outcome: "success"}, nil
}

var _ APIClientInterface = (*mockAPIClient)(nil) // Ensure interface compliance

// capturingAuditClient records the filters passed to GetAuditEvents for assertions
type capturingAuditClient struct {
	mockAPIClient
	lastFilters map[string]string
}

func (m *capturingAuditClient) GetAuditEvents(ctx context.Context, filters map[string]string, page, pageSize int) ([]AuditEvent, *Pagination, error) {
	m.lastFilters = filters
	if m.DashboardErr != nil {
		return nil, nil, m.DashboardErr
	}
	return []AuditEvent{{ID: "evt-1", Outcome: "success"}}, &Pagination{Page: page, PageSize: pageSize, TotalItems: 1, TotalPages: 1}, nil
}

func TestServer_HandleDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dashboardFile := createTestTemplate(t, "dashboard.html", "<html><body>dashboard</body></html>")

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(dashboardFile)

	s.engine.GET("/dashboard", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardData: &DashboardData{TotalProjects: 1, TotalTokens: 2, ActiveTokens: 1, ExpiredTokens: 0, TotalRequests: 10, RequestsToday: 5, RequestsThisWeek: 7}}
		c.Set("apiClient", client)
		s.handleDashboard(c)
	})

	req, _ := http.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleDashboard_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dashboardFile := createTestTemplate(t, "dashboard.html", "<html><body>dashboard</body></html>")

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(dashboardFile)

	s.engine.GET("/dashboard", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleDashboard(c)
	})

	req, _ := http.NewRequest("GET", "/dashboard", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for dashboard error, got %d", w.Code)
	}
}

func TestServer_HandleDashboard_WithHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dashboardFile := createTestTemplate(t, "dashboard.html", "<html><body>dashboard</body></html>")

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(dashboardFile)

	s.engine.GET("/dashboard", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardData: &DashboardData{TotalProjects: 1, TotalTokens: 2, ActiveTokens: 1, ExpiredTokens: 0, TotalRequests: 10, RequestsToday: 5, RequestsThisWeek: 7}}
		c.Set("apiClient", client)
		s.handleDashboard(c)
	})

	// Test with X-Forwarded-For header
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1,10.0.0.1")
	req.Header.Set("User-Agent", "Test Browser")
	req.Header.Set("Referer", "https://example.com")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with X-Forwarded-For, got %d", w.Code)
	}
}

func TestServer_HandleDashboard_WithRealIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dashboardFile := createTestTemplate(t, "dashboard.html", "<html><body>dashboard</body></html>")

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(dashboardFile)

	s.engine.GET("/dashboard", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardData: &DashboardData{TotalProjects: 1, TotalTokens: 2, ActiveTokens: 1, ExpiredTokens: 0, TotalRequests: 10, RequestsToday: 5, RequestsThisWeek: 7}}
		c.Set("apiClient", client)
		s.handleDashboard(c)
	})

	// Test with X-Real-IP header (when X-Forwarded-For is not present)
	req, _ := http.NewRequest("GET", "/dashboard", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with X-Real-IP, got %d", w.Code)
	}
}

func TestServer_HandleProjectsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsFile := createTestTemplate(t, "projects/list.html", "<html><body>projects</body></html>")

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(projectsFile)

	s.engine.GET("/projects", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsList(c)
	})

	req, _ := http.NewRequest("GET", "/projects", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleProjectsList_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsFile := createTestTemplate(t, "projects/list.html", "<html><body>projects</body></html>")

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(projectsFile)

	s.engine.GET("/projects", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleProjectsList(c)
	})

	req, _ := http.NewRequest("GET", "/projects", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for projects list error, got %d", w.Code)
	}
}

func TestServer_HandleTokensList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "list.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens</body></html>"), 0644)
	defer func() {
		err := os.Remove(tokensFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "list.html"))

	s.engine.GET("/tokens", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleTokensList(c)
	})

	req, _ := http.NewRequest("GET", "/tokens", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleTokensList_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "list.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens</body></html>"), 0644)
	defer func() {
		err := os.Remove(tokensFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "list.html"))

	s.engine.GET("/tokens", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleTokensList(c)
	})

	req, _ := http.NewRequest("GET", "/tokens", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for tokens list error, got %d", w.Code)
	}
}

func TestServer_HandleTokensNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "new.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens new</body></html>"), 0644)
	defer func() {
		err := os.Remove(tokensFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "*.html"))

	s.engine.GET("/tokens/new", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleTokensNew(c)
	})

	req, _ := http.NewRequest("GET", "/tokens/new", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleTokensCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "new.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens new</body></html>"), 0644)
	defer func() {
		err := os.Remove(tokensFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "*.html"))

	s.engine.POST("/tokens", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleTokensCreate(c)
	})

	form := strings.NewReader("project_id=1&duration_minutes=1440")
	req, _ := http.NewRequest("POST", "/tokens", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleTokensCreate_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "new.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens new</body></html>"), 0644)
	defer func() {
		err := os.Remove(tokensFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "*.html"))

	s.engine.POST("/tokens", func(c *gin.Context) {
		client := &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleTokensCreate(c)
	})

	form := strings.NewReader("") // missing fields
	req, _ := http.NewRequest("POST", "/tokens", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}
}

func TestServer_HandleTokensCreate_APIError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "new.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens new</body></html>"), 0644)
	defer func() {
		if err := os.Remove(tokensFile); err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "*.html"))

	// Bind succeeds, but API returns error → expect 500 branch
	s.engine.POST("/tokens", func(c *gin.Context) {
		client := &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleTokensCreate(c)
	})

	form := strings.NewReader("project_id=1&duration_minutes=60")
	req, _ := http.NewRequest("POST", "/tokens", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for API error after valid bind, got %d", w.Code)
	}
}

func TestServer_HandleProjectsShow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsShowFile := filepath.Join(testTemplateDir(), "projects-show.html")
	_ = os.WriteFile(projectsShowFile, []byte("<html><body>projects show</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsShowFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsShowFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-show.html"))

	s.engine.GET("/projects/:id", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsShow(c)
	})

	req, _ := http.NewRequest("GET", "/projects/1", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleProjectsEdit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsEditFile := filepath.Join(testTemplateDir(), "projects-edit.html")
	_ = os.WriteFile(projectsEditFile, []byte("<html><body>projects edit</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsEditFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsEditFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-edit.html"))

	s.engine.GET("/projects/:id/edit", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsEdit(c)
	})

	req, _ := http.NewRequest("GET", "/projects/1/edit", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleProjectsUpdate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsEditFile := filepath.Join(testTemplateDir(), "projects-edit.html")
	_ = os.WriteFile(projectsEditFile, []byte("<html><body>projects edit</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsEditFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsEditFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-edit.html"))

	s.engine.PUT("/projects/:id", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsUpdate(c)
	})

	form := strings.NewReader("name=Updated+Project&openai_api_key=key-1234")
	req, _ := http.NewRequest("PUT", "/projects/1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}
}

func TestServer_HandleProjectsUpdate_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsEditFile := filepath.Join(testTemplateDir(), "projects-edit.html")
	_ = os.WriteFile(projectsEditFile, []byte("<html><body>projects edit</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsEditFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsEditFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-edit.html"))

	s.engine.PUT("/projects/:id", func(c *gin.Context) {
		client := &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleProjectsUpdate(c)
	})

	form := strings.NewReader("") // missing fields
	req, _ := http.NewRequest("PUT", "/projects/1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", w.Code)
	}

	form2 := strings.NewReader("name=Updated&openai_api_key=key-1234")
	req2, _ := http.NewRequest("PUT", "/projects/1", form2)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	s.engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for API error, got %d", w2.Code)
	}
}

func TestServer_HandleProjectsDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.DELETE("/projects/:id", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsDelete(c)
	})

	req, _ := http.NewRequest("DELETE", "/projects/1", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}
}

func TestServer_HandleProjectsDelete_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})

	s.engine.DELETE("/projects/:id", func(c *gin.Context) {
		client := &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleProjectsDelete(c)
	})

	req, _ := http.NewRequest("DELETE", "/projects/1", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for API error, got %d", w.Code)
	}
}

func TestServer_HandleTokensShow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})

	s.engine.GET("/tokens/:token", func(c *gin.Context) {
		s.handleTokensShow(c)
	})

	req, _ := http.NewRequest("GET", "/tokens/abc123", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}
}

func TestServer_HandleProjectsNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsNewFile := filepath.Join(testTemplateDir(), "projects-new.html")
	_ = os.WriteFile(projectsNewFile, []byte("<html><body>projects new</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsNewFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsNewFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-new.html"))

	s.engine.GET("/projects/new", func(c *gin.Context) {
		s.handleProjectsNew(c)
	})

	req, _ := http.NewRequest("GET", "/projects/new", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleProjectsCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsNewFile := filepath.Join(testTemplateDir(), "projects-new.html")
	_ = os.WriteFile(projectsNewFile, []byte("<html><body>projects new</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsNewFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsNewFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-new.html"))

	s.engine.POST("/projects", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsCreate(c)
	})

	form := strings.NewReader("name=New+Project&openai_api_key=key-1234")
	req, _ := http.NewRequest("POST", "/projects", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}
}

func TestServer_HandleProjectsCreate_APIError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsNewFile := filepath.Join(testTemplateDir(), "projects-new.html")
	_ = os.WriteFile(projectsNewFile, []byte("<html><body>projects new</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsNewFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsNewFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-new.html"))

	s.engine.POST("/projects", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleProjectsCreate(c)
	})

	form := strings.NewReader("name=Test+Project&openai_api_key=key-1234")
	req, _ := http.NewRequest("POST", "/projects", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for API error, got %d", w.Code)
	}
}

func TestServer_HandleProjectsShow_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsShowFile := filepath.Join(testTemplateDir(), "projects-show.html")
	_ = os.WriteFile(projectsShowFile, []byte("<html><body>projects show</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsShowFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsShowFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-show.html"))

	s.engine.GET("/projects/:id", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleProjectsShow(c)
	})

	req, _ := http.NewRequest("GET", "/projects/1", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for not found, got %d", w.Code)
	}
}

func TestServer_HandleProjectsEdit_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	projectsEditFile := filepath.Join(testTemplateDir(), "projects-edit.html")
	_ = os.WriteFile(projectsEditFile, []byte("<html><body>projects edit</body></html>"), 0644)
	defer func() {
		err := os.Remove(projectsEditFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", projectsEditFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "projects-edit.html"))

	s.engine.GET("/projects/:id/edit", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleProjectsEdit(c)
	})

	req, _ := http.NewRequest("GET", "/projects/1/edit", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for not found, got %d", w.Code)
	}
}

func TestServer_HandleProjectsNew_Error(t *testing.T) {
	// This handler does not have an error branch in the current implementation
	// so this test is a placeholder for future error handling
}

func TestServer_HandleTokensNew_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokensFile := filepath.Join(testTemplateDir(), "tokens", "new.html")
	_ = os.WriteFile(tokensFile, []byte("<html><body>tokens new</body></html>"), 0644)
	defer func() {
		err := os.Remove(tokensFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", tokensFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "tokens", "new.html"))

	s.engine.GET("/tokens/new", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleTokensNew(c)
	})

	req, _ := http.NewRequest("GET", "/tokens/new", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for API error, got %d", w.Code)
	}
}

func TestServer_HandleAuditList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// minimal templates
	_ = os.MkdirAll(filepath.Join(testTemplateDir(), "audit"), 0755)
	_ = os.WriteFile(filepath.Join(testTemplateDir(), "audit", "list.html"), []byte("<html><body>audit list</body></html>"), 0644)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(testTemplateDir(), "audit", "list.html")) })

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "audit", "*.html"))

	var capClient *capturingAuditClient
	s.engine.GET("/audit", func(c *gin.Context) {
		capClient = &capturingAuditClient{}
		var client APIClientInterface = capClient
		c.Set("apiClient", client)
		s.handleAuditList(c)
	})

	req, _ := http.NewRequest("GET", "/audit", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// With search param: ensure it is forwarded to API client
	req2, _ := http.NewRequest("GET", "/audit?search=req-123", nil)
	w2 := httptest.NewRecorder()
	s.engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 with search, got %d", w2.Code)
	}
	if capClient == nil || capClient.lastFilters == nil || capClient.lastFilters["search"] != "req-123" {
		t.Fatalf("expected search filter to be forwarded, got %#v", capClient)
	}
}

func TestServer_HandleAuditList_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = os.MkdirAll(filepath.Join(testTemplateDir(), "audit"), 0755)
	_ = os.WriteFile(filepath.Join(testTemplateDir(), "audit", "list.html"), []byte("<html><body>audit list</body></html>"), 0644)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(testTemplateDir(), "audit", "list.html")) })

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "audit", "*.html"))

	s.engine.GET("/audit", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errFake}
		c.Set("apiClient", client)
		s.handleAuditList(c)
	})

	req, _ := http.NewRequest("GET", "/audit", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestServer_HandleAuditShow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = os.MkdirAll(filepath.Join(testTemplateDir(), "audit"), 0755)
	_ = os.WriteFile(filepath.Join(testTemplateDir(), "audit", "show.html"), []byte("<html><body>audit show</body></html>"), 0644)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(testTemplateDir(), "audit", "show.html")) })

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "audit", "*.html"))

	s.engine.GET("/audit/:id", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleAuditShow(c)
	})

	req, _ := http.NewRequest("GET", "/audit/evt-1", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleAuditShow_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_ = os.MkdirAll(filepath.Join(testTemplateDir(), "audit"), 0755)
	_ = os.WriteFile(filepath.Join(testTemplateDir(), "audit", "show.html"), []byte("<html><body>audit show</body></html>"), 0644)
	t.Cleanup(func() { _ = os.Remove(filepath.Join(testTemplateDir(), "audit", "show.html")) })

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "audit", "*.html"))

	s.engine.GET("/audit/:id", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardErr: errors.New("not found")}
		c.Set("apiClient", client)
		s.handleAuditShow(c)
	})

	req, _ := http.NewRequest("GET", "/audit/missing", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func Test_obfuscateToken_short(t *testing.T) {
	token := "short"
	obfuscated := obfuscateToken(token)
	if obfuscated != "****" {
		t.Errorf("obfuscateToken(short) = %q, want ****", obfuscated)
	}
}

func Test_parsePositiveInt(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"42", 42, false},
		{"0", 0, false},
		{"-1", -1, false}, // negative returns -1, not 0
		{"abc", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := parsePositiveInt(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parsePositiveInt(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("parsePositiveInt(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func Test_getPageFromQuery_and_getPageSizeFromQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		query       string
		pageKey     string
		pageSizeKey string
		wantPage    int
		wantSize    int
	}{
		{"page=3&size=20", "page", "size", 3, 20},
		{"page=abc&size=xyz", "page", "size", 1, 10},
		{"page=0&size=0", "page", "size", 1, 10},
		{"", "page", "size", 1, 10},
		{"page=5", "page", "size", 5, 10},
		{"size=50", "page", "size", 1, 50},
		{"size=200", "page", "size", 1, 10}, // size > 100 capped
	}
	for _, tt := range tests {
		r, _ := http.NewRequest("GET", "/?"+tt.query, nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		page := getPageFromQuery(c, 1)
		size := getPageSizeFromQuery(c, 10)
		if page != tt.wantPage {
			t.Errorf("getPageFromQuery(%q) = %d, want %d", tt.query, page, tt.wantPage)
		}
		if size != tt.wantSize {
			t.Errorf("getPageSizeFromQuery(%q) = %d, want %d", tt.query, size, tt.wantSize)
		}
	}
}

func TestServer_templateFuncs(t *testing.T) {
	s := &Server{}
	funcs := s.templateFuncs()

	if got := funcs["add"].(func(int, int) int)(2, 3); got != 5 {
		t.Errorf("add(2,3) = %d, want 5", got)
	}
	if got := funcs["sub"].(func(int, int) int)(5, 3); got != 2 {
		t.Errorf("sub(5,3) = %d, want 2", got)
	}
	if got := funcs["inc"].(func(int) int)(7); got != 8 {
		t.Errorf("inc(7) = %d, want 8", got)
	}
	if got := funcs["dec"].(func(int) int)(7); got != 6 {
		t.Errorf("dec(7) = %d, want 6", got)
	}
	seq := funcs["seq"].(func(int, int) []int)(2, 4)
	if len(seq) != 3 || seq[0] != 2 || seq[2] != 4 {
		t.Errorf("seq(2,4) = %v, want [2 3 4]", seq)
	}
	if now := funcs["now"].(func() time.Time)(); now.IsZero() {
		t.Error("now() returned zero time")
	}
	if !funcs["eq"].(func(any, any) bool)(1, 1) {
		t.Error("eq(1,1) = false, want true")
	}
	if funcs["ne"].(func(any, any) bool)(1, 1) {
		t.Error("ne(1,1) = true, want false")
	}
	if !funcs["lt"].(func(any, any) bool)(1, 2) {
		t.Error("lt(1,2) = false, want true")
	}
	if !funcs["gt"].(func(any, any) bool)(2, 1) {
		t.Error("gt(2,1) = false, want true")
	}
	if !funcs["le"].(func(any, any) bool)(2, 2) {
		t.Error("le(2,2) = false, want true")
	}
	if !funcs["ge"].(func(any, any) bool)(2, 2) {
		t.Error("ge(2,2) = false, want true")
	}
	if !funcs["and"].(func(bool, bool) bool)(true, true) {
		t.Error("and(true,true) = false, want true")
	}
	if funcs["or"].(func(bool, bool) bool)(false, false) {
		t.Error("or(false,false) = true, want false")
	}
	if !funcs["not"].(func(bool) bool)(false) {
		t.Error("not(false) = false, want true")
	}
	if got := funcs["obfuscateAPIKey"].(func(string) string)("sk-1234567890abcdef"); !strings.HasPrefix(got, "sk-1234") {
		t.Errorf("obfuscateAPIKey() = %q, want prefix sk-1234", got)
	}
	if got := funcs["obfuscateToken"].(func(string) string)("tok-12345678"); !strings.HasPrefix(got, "tok-") {
		t.Errorf("obfuscateToken() = %q, want prefix tok-", got)
	}
}

func Test_getFormFieldNames(t *testing.T) {
	form := map[string][]string{
		"field1": {"value1"},
		"field2": {"value2"},
		"Field3": {"true"},
	}
	fields := getFormFieldNames(form)
	if len(fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(fields))
	}
	found := map[string]bool{"field1": false, "field2": false, "Field3": false}
	for _, f := range fields {
		if _, ok := found[f]; ok {
			found[f] = true
		}
	}
	for k, v := range found {
		if !v {
			t.Errorf("missing field name: %s", k)
		}
	}
}

func Test_obfuscateToken(t *testing.T) {
	token := "tok-1234567890abcdef"
	obfuscated := obfuscateToken(token)
	if !strings.HasPrefix(obfuscated, "tok-") {
		t.Errorf("obfuscateToken() = %q, want prefix tok-", obfuscated)
	}
	if len(obfuscated) < 8 {
		t.Errorf("obfuscateToken() = %q, too short", obfuscated)
	}
}

func TestServer_HandleLoginForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loginFile := filepath.Join(testTemplateDir(), "login.html")
	_ = os.WriteFile(loginFile, []byte("<html><body>login</body></html>"), 0644)
	defer func() {
		err := os.Remove(loginFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", loginFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "login.html"))

	s.engine.GET("/auth/login", func(c *gin.Context) {
		s.handleLoginForm(c)
	})

	req, _ := http.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleLogout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})

	// Add sessions middleware with a dummy cookie store
	store := cookie.NewStore([]byte("secret"))
	s.engine.Use(sessions.Sessions("mysession", store))

	s.engine.GET("/auth/logout", func(c *gin.Context) {
		s.handleLogout(c)
	})

	req, _ := http.NewRequest("GET", "/auth/logout", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}
}

func TestServer_HandleLogout_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})

	baseStore := cookie.NewStore([]byte("secret"))
	s.engine.Use(sessions.Sessions("mysession", baseStore))

	// Patch the session's Save method by using a custom handler
	s.engine.GET("/auth/logout", func(c *gin.Context) {
		sess := sessions.Default(c)
		sess.Clear()
		// Simulate Save error by returning an error from Save
		// This is not directly possible, so we just call handleLogout and ensure it still redirects
		s.handleLogout(c)
	})

	req, _ := http.NewRequest("GET", "/auth/logout", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}
}

func TestServer_HandleLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loginFile := filepath.Join(testTemplateDir(), "login.html")
	_ = os.WriteFile(loginFile, []byte("<html><body>login</body></html>"), 0644)
	defer func() {
		err := os.Remove(loginFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", loginFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "login.html"))

	// Add sessions middleware with a dummy cookie store
	store := cookie.NewStore([]byte("secret"))
	s.engine.Use(sessions.Sessions("mysession", store))

	// Inject ValidateTokenWithAPI for test
	s.ValidateTokenWithAPI = func(_ context.Context, token string) bool {
		return token == "valid-token"
	}

	s.engine.POST("/auth/login", func(c *gin.Context) {
		s.handleLogin(c)
	})

	// Valid login
	form := strings.NewReader("management_token=valid-token")
	req, _ := http.NewRequest("POST", "/auth/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 for valid login, got %d", w.Code)
	}

	// Missing token
	form2 := strings.NewReader("")
	req2, _ := http.NewRequest("POST", "/auth/login", form2)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	s.engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing token, got %d", w2.Code)
	}

	// Invalid token
	form3 := strings.NewReader("management_token=invalid-token")
	req3, _ := http.NewRequest("POST", "/auth/login", form3)
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w3 := httptest.NewRecorder()
	s.engine.ServeHTTP(w3, req3)
	if w3.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", w3.Code)
	}
}

var errFake = &fakeError{"simulated API error"}

type fakeError struct{ msg string }

func (e *fakeError) Error() string { return e.msg }

func TestNewServer_ErrorCases(t *testing.T) {
	// Ensure at least one HTML file exists for the glob in root and subdir
	baseFile := filepath.Join(testTemplateDir(), "base.html")
	_ = os.WriteFile(baseFile, []byte("<html><body>base</body></html>"), 0644)
	defer func() {
		err := os.Remove(baseFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", baseFile, err)
		}
	}()
	tokensDir := filepath.Join(testTemplateDir(), "tokens")
	if err := os.MkdirAll(tokensDir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", tokensDir, err)
	}
	dummySubFile := filepath.Join(tokensDir, "new.html")
	_ = os.WriteFile(dummySubFile, []byte("<html><body>dummy</body></html>"), 0644)
	defer func() {
		err := os.Remove(dummySubFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", dummySubFile, err)
		}
	}()

	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      "",
			TemplateDir:     testTemplateDir(),
		},
		LogLevel: "info",
	}
	_, err := NewServer(cfg)
	if err != nil {
		t.Errorf("NewServer() with empty ListenAddr should not error, got %v", err)
	}

	cfg2 := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "",
			ListenAddr:      ":0",
			TemplateDir:     testTemplateDir(),
		},
		LogLevel: "info",
	}
	_, err2 := NewServer(cfg2)
	if err2 != nil {
		t.Errorf("NewServer() with empty ManagementToken should not error, got %v", err2)
	}
}

func TestServer_Start_Error(t *testing.T) {
	// Ensure at least one HTML file exists for the glob in root and subdir
	baseFile := filepath.Join(testTemplateDir(), "base.html")
	_ = os.WriteFile(baseFile, []byte("<html><body>base</body></html>"), 0644)
	defer func() {
		err := os.Remove(baseFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", baseFile, err)
		}
	}()
	tokensDir := filepath.Join(testTemplateDir(), "tokens")
	if err := os.MkdirAll(tokensDir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", tokensDir, err)
	}
	dummySubFile := filepath.Join(tokensDir, "new.html")
	_ = os.WriteFile(dummySubFile, []byte("<html><body>dummy</body></html>"), 0644)
	defer func() {
		err := os.Remove(dummySubFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", dummySubFile, err)
		}
	}()

	// Start with invalid address to force error
	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      "invalid:address",
			TemplateDir:     testTemplateDir(),
		},
		LogLevel: "info",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	// Start should return error immediately
	err = srv.Start()
	if err == nil {
		t.Error("Start() with invalid address should return error")
	}
}

func TestServer_setupRoutes_Coverage(t *testing.T) {
	// Ensure at least one HTML file exists for the glob in root and subdir
	baseFile := filepath.Join(testTemplateDir(), "base.html")
	_ = os.WriteFile(baseFile, []byte("<html><body>base</body></html>"), 0644)
	defer func() {
		err := os.Remove(baseFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", baseFile, err)
		}
	}()
	tokensDir := filepath.Join(testTemplateDir(), "tokens")
	if err := os.MkdirAll(tokensDir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", tokensDir, err)
	}
	dummySubFile := filepath.Join(tokensDir, "new.html")
	_ = os.WriteFile(dummySubFile, []byte("<html><body>dummy</body></html>"), 0644)
	defer func() {
		err := os.Remove(dummySubFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", dummySubFile, err)
		}
	}()

	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      ":0",
			TemplateDir:     testTemplateDir(),
		},
		LogLevel: "info",
	}
	// Use a fresh gin.Engine to avoid double route registration
	srv := &Server{
		config: cfg,
		engine: gin.New(),
	}
	srv.setupRoutes()
}

func TestServer_Start_Coverage(t *testing.T) {
	// Ensure at least one HTML file exists for the glob in root and subdir
	baseFile := filepath.Join(testTemplateDir(), "base.html")
	_ = os.WriteFile(baseFile, []byte("<html><body>base</body></html>"), 0644)
	defer func() {
		err := os.Remove(baseFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", baseFile, err)
		}
	}()
	tokensDir := filepath.Join(testTemplateDir(), "tokens")
	if err := os.MkdirAll(tokensDir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", tokensDir, err)
	}
	dummySubFile := filepath.Join(tokensDir, "new.html")
	_ = os.WriteFile(dummySubFile, []byte("<html><body>dummy</body></html>"), 0644)
	defer func() {
		err := os.Remove(dummySubFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", dummySubFile, err)
		}
	}()

	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      ":0",
			TemplateDir:     testTemplateDir(),
		},
		LogLevel: "info",
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	// Start server in goroutine and immediately shut it down
	go func() {
		// This will block, so we run it in a goroutine
		err := srv.Start()
		_ = err // ignore error for coverage
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Shut it down
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestServer_authMiddleware_NoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New(), config: &config.Config{AdminUI: config.AdminUIConfig{APIBaseURL: "http://localhost:1234", TemplateDir: "internal/admin/testdata"}}}
	// Add sessions middleware with a dummy cookie store
	store := cookie.NewStore([]byte("secret"))
	s.engine.Use(sessions.Sessions("mysession", store))
	s.engine.Use(s.authMiddleware())
	s.engine.GET("/protected", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for missing session, got %d", w.Code)
	}
}

func TestServer_authMiddleware_WithSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{AdminUI: config.AdminUIConfig{APIBaseURL: "http://localhost:1234", TemplateDir: "internal/admin/testdata"}}
	s := &Server{engine: gin.New(), config: cfg}

	// Add sessions middleware with a dummy cookie store
	store := cookie.NewStore([]byte("secret"))
	s.engine.Use(sessions.Sessions("mysession", store))

	// Add a test route without auth middleware for setting session
	s.engine.GET("/set-session", func(c *gin.Context) {
		sess := sessions.Default(c)
		sess.Set("token", "test-token")
		if err := sess.Save(); err != nil {
			t.Errorf("failed to save session: %v", err)
		}
		c.String(http.StatusOK, "session set")
	})

	// Add protected route group with auth middleware
	protected := s.engine.Group("/")
	protected.Use(s.authMiddleware())
	protected.GET("/protected", func(c *gin.Context) {
		client, exists := c.Get("apiClient")
		if !exists || client == nil {
			c.String(http.StatusInternalServerError, "apiClient not set")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	// First, create a session by hitting the login route
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/set-session", nil)
	s.engine.ServeHTTP(w, r)

	// Extract session cookie
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie was set")
	}

	// Now, make a request to the protected route with the session cookie
	w2 := httptest.NewRecorder()
	r2, _ := http.NewRequest("GET", "/protected", nil)
	for _, cookie := range cookies {
		r2.AddCookie(cookie)
	}
	s.engine.ServeHTTP(w2, r2)
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 for valid session, got %d. Response: %s", w2.Code, w2.Body.String())
	}
}

func TestServer_validateTokenWithAPI(t *testing.T) {
	// Success case: returns 200
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/manage/projects" && r.Header.Get("Authorization") == "Bearer valid-token" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	s := &Server{config: &config.Config{AdminUI: config.AdminUIConfig{APIBaseURL: ts.URL}}}
	ok := s.validateTokenWithAPI(context.Background(), "valid-token")
	if !ok {
		t.Error("validateTokenWithAPI should return true for 200 OK")
	}

	// Error case: returns 401
	fail := s.validateTokenWithAPI(context.Background(), "bad-token")
	if fail {
		t.Error("validateTokenWithAPI should return false for non-2xx status")
	}

	// Error case: request creation fails (invalid URL)
	s2 := &Server{config: &config.Config{AdminUI: config.AdminUIConfig{APIBaseURL: "://bad-url"}}}
	if s2.validateTokenWithAPI(context.Background(), "any") {
		t.Error("validateTokenWithAPI should return false for request error")
	}
}

func TestServer_templateFuncs_EdgeCases(t *testing.T) {
	s := &Server{}
	funcs := s.templateFuncs()

	t.Run("add/sub/inc/dec/seq", func(t *testing.T) {
		_ = funcs["add"].(func(int, int) int)(0, 0)
		_ = funcs["sub"].(func(int, int) int)(0, 0)
		_ = funcs["inc"].(func(int) int)(0)
		_ = funcs["dec"].(func(int) int)(0)
		seq := funcs["seq"].(func(int, int) []int)(0, 0)
		if len(seq) != 1 || seq[0] != 0 {
			t.Errorf("seq(0,0) = %v, want [0]", seq)
		}
	})

	t.Run("now", func(t *testing.T) {
		_ = funcs["now"].(func() time.Time)()
	})

	t.Run("eq/ne/lt/gt/le/ge", func(t *testing.T) {
		eq := funcs["eq"].(func(any, any) bool)
		ne := funcs["ne"].(func(any, any) bool)
		lt := funcs["lt"].(func(any, any) bool)
		gt := funcs["gt"].(func(any, any) bool)
		le := funcs["le"].(func(any, any) bool)
		ge := funcs["ge"].(func(any, any) bool)
		if !eq(nil, nil) || eq(1, 2) {
			t.Error("eq failed")
		}
		if !ne(nil, 1) || !ne(1, nil) || ne(1, 1) {
			t.Error("ne failed")
		}
		if !lt(1, 2) || lt(2, 1) {
			t.Error("lt failed")
		}
		if !gt(2, 1) || gt(1, 2) {
			t.Error("gt failed")
		}
		if !le(2, 2) || !le(1, 2) || le(3, 2) {
			t.Error("le failed")
		}
		if !ge(2, 2) || !ge(3, 2) || ge(1, 2) {
			t.Error("ge failed")
		}
	})

	t.Run("and/or/not", func(t *testing.T) {
		and := funcs["and"].(func(bool, bool) bool)
		or := funcs["or"].(func(bool, bool) bool)
		not := funcs["not"].(func(bool) bool)
		if and(false, false) || and(true, false) || !and(true, true) {
			t.Error("and failed")
		}
		if !or(false, true) || !or(true, false) || or(false, false) == true {
			t.Error("or failed")
		}
		if not(true) || !not(false) {
			t.Error("not failed")
		}
	})

	t.Run("obfuscateAPIKey/obfuscateToken", func(t *testing.T) {
		_ = funcs["obfuscateAPIKey"].(func(string) string)("")
		_ = funcs["obfuscateToken"].(func(string) string)("")
	})
}

func TestServer_templateFuncs_MoreEdgeCases(t *testing.T) {
	s := &Server{}
	funcs := s.templateFuncs()

	t.Run("obfuscateAPIKey short", func(t *testing.T) {
		obf := funcs["obfuscateAPIKey"].(func(string) string)
		if got := obf("a"); got != "*" {
			t.Errorf("obfuscateAPIKey short: got %q, want *", got)
		}
		if got := obf(""); got != "" {
			t.Errorf("obfuscateAPIKey empty: got %q, want empty", got)
		}
	})

	t.Run("obfuscateToken short", func(t *testing.T) {
		obf := funcs["obfuscateToken"].(func(string) string)
		if got := obf("a"); got != "****" {
			t.Errorf("obfuscateToken short: got %q, want ****", got)
		}
		if got := obf(""); got != "****" {
			t.Errorf("obfuscateToken empty: got %q, want ****", got)
		}
	})

	t.Run("contains edge", func(t *testing.T) {
		contains := funcs["contains"].(func(string, string) bool)
		if contains("", "") != true {
			t.Error("contains empty strings should be true")
		}
		if contains("foo", "") != true {
			t.Error("contains any string, empty substr should be true")
		}
	})
}

func TestServer_templateFuncs_Int64AndTime(t *testing.T) {
	s := &Server{}
	funcs := s.templateFuncs()

	// int64 comparisons
	lt := funcs["lt"].(func(any, any) bool)
	gt := funcs["gt"].(func(any, any) bool)
	le := funcs["le"].(func(any, any) bool)
	ge := funcs["ge"].(func(any, any) bool)

	var a, b int64 = 1, 2
	if !lt(a, b) || lt(b, a) {
		t.Error("lt int64 failed")
	}
	if !gt(b, a) || gt(a, b) {
		t.Error("gt int64 failed")
	}
	if !le(a, b) || !le(a, a) || le(b, a) {
		t.Error("le int64 failed")
	}
	if !ge(b, a) || !ge(b, b) || ge(a, b) {
		t.Error("ge int64 failed")
	}

	// time.Time comparisons, including equality branches for le/ge
	t1 := time.Now()
	t2 := t1.Add(time.Second)
	if !lt(t1, t2) || lt(t2, t1) {
		t.Error("lt time failed")
	}
	if !gt(t2, t1) || gt(t1, t2) {
		t.Error("gt time failed")
	}
	if !le(t1, t2) || !le(t1, t1) || le(t2, t1) {
		t.Error("le time failed")
	}
	if !ge(t2, t1) || !ge(t2, t2) || ge(t1, t2) {
		t.Error("ge time failed")
	}
}

func TestServer_HandleLogin_RememberMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loginFile := filepath.Join(testTemplateDir(), "login.html")
	_ = os.WriteFile(loginFile, []byte("<html><body>login</body></html>"), 0644)
	defer func() {
		if err := os.Remove(loginFile); err != nil {
			t.Errorf("failed to remove %s: %v", loginFile, err)
		}
	}()

	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob(filepath.Join(testTemplateDir(), "login.html"))
	store := cookie.NewStore([]byte("secret"))
	s.engine.Use(sessions.Sessions("mysession", store))
	s.ValidateTokenWithAPI = func(_ context.Context, token string) bool { return token == "valid-token" }

	s.engine.POST("/auth/login", func(c *gin.Context) { s.handleLogin(c) })

	form := strings.NewReader("management_token=valid-token&remember_me=true")
	req, _ := http.NewRequest("POST", "/auth/login", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", w.Code)
	}
	// Ensure cookie reflects remember_me (Max-Age set to 30 days)
	cookies := w.Result().Header["Set-Cookie"]
	found := false
	for _, c := range cookies {
		if strings.Contains(c, "Max-Age=2592000") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected session cookie with Max-Age=2592000, got: %v", cookies)
	}
}

func TestAdminHealthEndpoint(t *testing.T) {
	// Ensure at least one HTML file exists for the glob in root and subdir
	baseFile := filepath.Join(testTemplateDir(), "base.html")
	_ = os.WriteFile(baseFile, []byte("<html><body>base</body></html>"), 0644)
	defer func() {
		err := os.Remove(baseFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", baseFile, err)
		}
	}()
	tokensDir := filepath.Join(testTemplateDir(), "tokens")
	if err := os.MkdirAll(tokensDir, 0755); err != nil {
		t.Fatalf("failed to create dir %s: %v", tokensDir, err)
	}
	dummySubFile := filepath.Join(tokensDir, "new.html")
	_ = os.WriteFile(dummySubFile, []byte("<html><body>dummy</body></html>"), 0644)
	defer func() {
		err := os.Remove(dummySubFile)
		if err != nil {
			t.Errorf("failed to remove %s: %v", dummySubFile, err)
		}
	}()

	// Start a fake backend health server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok","timestamp":"2024-01-01T00:00:00Z","version":"0.1.0"}`)); err != nil {
			t.Errorf("failed to write health response: %v", err)
		}
	}))
	defer backend.Close()

	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			ListenAddr:      "127.0.0.1:0",
			TemplateDir:     testTemplateDir(),
			APIBaseURL:      backend.URL,
			ManagementToken: "test-token",
		},
	}
	server, err := NewServer(cfg)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	server.engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	admin, ok := resp["admin"].(map[string]interface{})
	require.True(t, ok)
	backendStatus, ok := resp["backend"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ok", admin["status"])
	assert.Equal(t, "ok", backendStatus["status"])
}

func TestServer_templateFuncs_AllFuncs(t *testing.T) {
	s := &Server{}
	funcs := s.templateFuncs()

	t.Run("add/sub/inc/dec/seq", func(t *testing.T) {
		add := funcs["add"].(func(int, int) int)
		sub := funcs["sub"].(func(int, int) int)
		inc := funcs["inc"].(func(int) int)
		dec := funcs["dec"].(func(int) int)
		seq := funcs["seq"].(func(int, int) []int)
		if add(2, 3) != 5 {
			t.Error("add failed")
		}
		if sub(5, 2) != 3 {
			t.Error("sub failed")
		}
		if inc(7) != 8 {
			t.Error("inc failed")
		}
		if dec(7) != 6 {
			t.Error("dec failed")
		}
		if got := seq(2, 4); len(got) != 3 || got[0] != 2 || got[2] != 4 {
			t.Errorf("seq failed: %v", got)
		}
	})

	t.Run("now", func(t *testing.T) {
		now := funcs["now"].(func() time.Time)
		if time.Since(now()) > time.Second {
			t.Error("now not close to current time")
		}
	})

	t.Run("eq/ne/lt/gt/le/ge", func(t *testing.T) {
		eq := funcs["eq"].(func(any, any) bool)
		ne := funcs["ne"].(func(any, any) bool)
		lt := funcs["lt"].(func(any, any) bool)
		gt := funcs["gt"].(func(any, any) bool)
		le := funcs["le"].(func(any, any) bool)
		ge := funcs["ge"].(func(any, any) bool)
		if !eq(1, 1) || eq(1, 2) {
			t.Error("eq failed")
		}
		if !ne(1, 2) || ne(1, 1) {
			t.Error("ne failed")
		}
		if !lt(1, 2) || lt(2, 1) {
			t.Error("lt failed")
		}
		if !gt(2, 1) || gt(1, 2) {
			t.Error("gt failed")
		}
		if !le(2, 2) || !le(1, 2) || le(3, 2) {
			t.Error("le failed")
		}
		if !ge(2, 2) || !ge(3, 2) || ge(1, 2) {
			t.Error("ge failed")
		}
	})

	t.Run("and/or/not", func(t *testing.T) {
		and := funcs["and"].(func(bool, bool) bool)
		or := funcs["or"].(func(bool, bool) bool)
		not := funcs["not"].(func(bool) bool)
		if !and(true, true) || and(true, false) {
			t.Error("and failed")
		}
		if !or(true, false) || or(false, false) {
			t.Error("or failed")
		}
		if !not(false) || not(true) {
			t.Error("not failed")
		}
	})

	t.Run("obfuscateAPIKey", func(t *testing.T) {
		obf := funcs["obfuscateAPIKey"].(func(string) string)
		if obf("1234567890123456") != "12345678...3456" {
			t.Error("obfuscateAPIKey long failed")
		}
		if obf("abcd") != "****" {
			t.Error("obfuscateAPIKey short failed")
		}
		if obf("abcdefghij") != "ab********" {
			t.Error("obfuscateAPIKey med failed")
		}
	})

	t.Run("obfuscateToken", func(t *testing.T) {
		obf := funcs["obfuscateToken"].(func(string) string)
		if obf("1234567890abcdef") != "1234****cdef" {
			t.Error("obfuscateToken long failed")
		}
		if obf("1234567") != "****" {
			t.Error("obfuscateToken short failed")
		}
	})

	t.Run("contains", func(t *testing.T) {
		contains := funcs["contains"].(func(string, string) bool)
		if !contains("hello world", "world") {
			t.Error("contains failed")
		}
		if contains("hello", "bye") {
			t.Error("contains false positive")
		}
	})
}

func TestServer_templateFuncs_ObfuscationAndContains_Edges(t *testing.T) {
	// Ensure template dir has at least one root and one nested template, otherwise NewServer panics
	td := t.TempDir()
	if err := os.WriteFile(filepath.Join(td, "base.html"), []byte("{{define \"base\"}}ok{{end}}"), 0o644); err != nil {
		t.Fatalf("write base.html: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(td, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(td, "sub", "nested.html"), []byte("{{define \"nested\"}}ok{{end}}"), 0o644); err != nil {
		t.Fatalf("write nested.html: %v", err)
	}

	cfg := &config.Config{AdminUI: config.AdminUIConfig{TemplateDir: td}}
	s, err := NewServer(cfg)
	require.NoError(t, err)

	funcs := s.templateFuncs()

	// obfuscateAPIKey short (<=4) → all asterisks
	if got := funcs["obfuscateAPIKey"].(func(string) string)("abc"); got != "***" {
		t.Fatalf("obfuscateAPIKey short = %q, want ***", got)
	}
	// obfuscateToken short (<=8) → ****
	if got := funcs["obfuscateToken"].(func(string) string)("short"); got != "****" {
		t.Fatalf("obfuscateToken short = %q, want ****", got)
	}
	// contains
	if !funcs["contains"].(func(string, string) bool)("hello", "ell") {
		t.Fatalf("contains should be true for substring")
	}
}
