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

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
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

func (m *mockAPIClient) CreateProject(ctx context.Context, name, apiKey string) (*Project, error) {
	if m.DashboardErr != nil {
		return nil, m.DashboardErr
	}
	return &Project{ID: "new-id", Name: name, OpenAIAPIKey: apiKey}, nil
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

func TestServer_HandleDashboard_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/simple-dashboard.html")

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

func TestServer_HandleProjectsList_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-list-complete.html")

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

func TestServer_HandleTokensList_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/tokens-list-complete.html")

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

func TestServer_HandleTokensCreate_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/tokens-*.html")

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

	form2 := strings.NewReader("project_id=1&duration_hours=24")
	req2, _ := http.NewRequest("POST", "/tokens", form2)
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	s.engine.ServeHTTP(w2, req2)
	if w2.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for API error, got %d", w2.Code)
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

func TestServer_HandleProjectsUpdate_Errors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-edit.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-new.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-new.html")

	s.engine.POST("/projects", func(c *gin.Context) {
		var client APIClientInterface = &mockAPIClient{}
		c.Set("apiClient", client)
		s.handleProjectsCreate(c)
	})

	form := strings.NewReader("name=Test+Project&openai_api_key=key-1234")
	req, _ := http.NewRequest("POST", "/projects", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.engine.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", w.Code)
	}

	// Error case: missing fields
	formErr := strings.NewReader("")
	reqErr, _ := http.NewRequest("POST", "/projects", formErr)
	reqErr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wErr := httptest.NewRecorder()
	s.engine.ServeHTTP(wErr, reqErr)
	if wErr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for error case, got %d", wErr.Code)
	}
}

func TestServer_HandleProjectsCreate_APIError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-new.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-show.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/projects-edit.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/tokens-new.html")

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
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/login.html")

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

func TestServer_HandleLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New()}
	s.engine.SetFuncMap(template.FuncMap{})
	s.engine.LoadHTMLGlob("testdata/login.html")

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
	if _, err := os.Stat("web/templates/base.html"); err != nil {
		t.Skip("Skipping: required template file not found")
	}
	// Invalid config: missing ListenAddr
	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      "",
		},
		LogLevel: "info",
	}
	_, err := NewServer(cfg)
	if err != nil {
		t.Errorf("NewServer() with empty ListenAddr should not error, got %v", err)
	}

	// Invalid config: missing ManagementToken
	cfg2 := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "",
			ListenAddr:      ":0",
		},
		LogLevel: "info",
	}
	_, err2 := NewServer(cfg2)
	if err2 != nil {
		t.Errorf("NewServer() with empty ManagementToken should not error, got %v", err2)
	}
}

func TestServer_Start_Error(t *testing.T) {
	if _, err := os.Stat("web/templates/base.html"); err != nil {
		t.Skip("Skipping: required template file not found")
	}
	// Start with invalid address to force error
	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      "invalid:address",
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
	// Call setupRoutes again to cover the method
	srv.setupRoutes()
}

func TestServer_Start_Coverage(t *testing.T) {
	if _, err := os.Stat("web/templates/base.html"); err != nil {
		t.Skip("Skipping: required template file not found")
	}
	cfg := &config.Config{
		AdminUI: config.AdminUIConfig{
			APIBaseURL:      "http://localhost:1234",
			ManagementToken: "token",
			ListenAddr:      ":0", // Use port 0 for automatic port assignment
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
		srv.Start()
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
	if _, err := os.Stat("web/templates/base.html"); err != nil {
		t.Skip("Skipping: required template file not found")
	}
	gin.SetMode(gin.TestMode)
	s := &Server{engine: gin.New(), config: &config.Config{AdminUI: config.AdminUIConfig{APIBaseURL: "http://localhost:1234"}}}
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
	cfg := &config.Config{AdminUI: config.AdminUIConfig{APIBaseURL: "http://localhost:1234"}}
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
