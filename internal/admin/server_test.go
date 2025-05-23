package admin

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sofatutor/llm-proxy/internal/config"
)

func TestGetSessionSecret(t *testing.T) {
	cfg := &config.Config{AdminUI: config.AdminUIConfig{ManagementToken: "secret-token"}}
	secret := getSessionSecret(cfg)
	want := []byte("secret-tokenllmproxy-cookie-salt")
	if string(secret) != string(want) {
		t.Errorf("getSessionSecret() = %q, want %q", secret, want)
	}
}

func TestNewServer_Minimal(t *testing.T) {
	if _, err := os.Stat("web/templates/base.html"); err != nil {
		t.Skip("Skipping: required template file not found")
	}
	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      ":0",
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
	DashboardData any
	DashboardErr  error
}

func (m *mockAPIClient) GetDashboardData(ctx context.Context) (any, error) {
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

func (m *mockAPIClient) CreateToken(ctx context.Context, projectID string, durationHours int) (*TokenCreateResponse, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &TokenCreateResponse{Token: "tok-1234", ExpiresAt: time.Now().Add(time.Duration(durationHours) * time.Hour)}, nil
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

var _ APIClientInterface = (*mockAPIClient)(nil) // Ensure interface compliance

func TestServer_HandleDashboard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/simple-dashboard.html") // Use dummy template

	s.engine.GET("/dashboard", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{DashboardData: map[string]any{"foo": "bar"}}
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

func TestServer_HandleProjectsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-list-complete.html")

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

func TestServer_HandleTokensList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/tokens-list-complete.html")

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

func TestServer_HandleTokensNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/tokens-*.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/tokens-*.html")

	s.engine.POST("/tokens", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleTokensCreate(c)
	})

	form := strings.NewReader("project_id=1&duration_hours=24")
	req, _ := http.NewRequest("POST", "/tokens", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestServer_HandleProjectsShow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-show.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-edit.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-edit.html")

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
