// Package admin provides the HTTP server for the Admin UI.
// This package implements a separate web interface for managing
// projects and tokens via the Management API.
package admin

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sofatutor/llm-proxy/internal/config"
)

// Session represents a user session
type Session struct {
	ID         string
	Token      string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	RememberMe bool
}

// getSessionSecret derives a secret key from the management token and a salt
func getSessionSecret(cfg *config.Config) []byte {
	salt := "llmproxy-cookie-salt" // TODO: move to config/env
	return []byte(cfg.AdminUI.ManagementToken + salt)
}

// Server represents the Admin UI HTTP server.
// It provides a web interface for managing projects and tokens
// by communicating with the Management API.
type Server struct {
	server    *http.Server
	config    *config.Config
	engine    *gin.Engine
	apiClient *APIClient
}

// NewServer creates a new Admin UI server with the provided configuration.
// It initializes the Gin engine, sets up routes, and configures the HTTP server.
func NewServer(cfg *config.Config) (*Server, error) {
	// Set Gin mode based on log level
	if cfg.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Add middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	// Add session middleware
	store := cookie.NewStore(getSessionSecret(cfg))
	engine.Use(sessions.Sessions("llmproxy_session", store))

	// Create API client for communicating with Management API
	// Note: API client will be updated when user logs in
	var apiClient *APIClient
	if cfg.AdminUI.ManagementToken != "" {
		apiClient = NewAPIClient(cfg.AdminUI.APIBaseURL, cfg.AdminUI.ManagementToken)
	}

	s := &Server{
		config:    cfg,
		engine:    engine,
		apiClient: apiClient,
		server: &http.Server{
			Addr:         cfg.AdminUI.ListenAddr,
			Handler:      engine,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	// Setup routes
	s.setupRoutes()

	return s, nil
}

// Start starts the Admin UI server.
// This method blocks until the server is shut down or an error occurs.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server without interrupting active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// setupRoutes configures all the routes for the Admin UI
func (s *Server) setupRoutes() {
	// Serve static files (CSS, JS, images)
	s.engine.Static("/static", "./web/static")

	// Load HTML templates with custom functions
	s.engine.SetFuncMap(s.templateFuncs())
	s.engine.LoadHTMLFiles(
		"web/templates/base.html",
		"web/templates/dashboard.html",
		"web/templates/simple-dashboard.html",
		"web/templates/error.html",
		"web/templates/login.html",
		"web/templates/projects-list-complete.html",
		"web/templates/tokens-list-complete.html",
		"web/templates/projects/new.html",
		"web/templates/projects/show.html",
		"web/templates/projects/edit.html",
		"web/templates/tokens/new.html",
		"web/templates/tokens/created.html",
	)

	// Authentication routes (no middleware)
	auth := s.engine.Group("/auth")
	{
		auth.GET("/login", s.handleLoginForm)
		auth.POST("/login", s.handleLogin)
		auth.GET("/logout", s.handleLogout)  // Allow GET for direct URL access
		auth.POST("/logout", s.handleLogout) // Keep POST for form submission
	}

	// Root route - redirect to dashboard
	s.engine.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/dashboard")
	})

	// Protected routes with authentication middleware
	protected := s.engine.Group("/")
	protected.Use(s.authMiddleware())
	{
		// Dashboard
		protected.GET("/dashboard", s.handleDashboard)

		// Projects routes
		projects := protected.Group("/projects")
		{
			projects.GET("", s.handleProjectsList)
			projects.GET("/new", s.handleProjectsNew)
			projects.POST("", s.handleProjectsCreate)
			projects.GET("/:id", s.handleProjectsShow)
			projects.GET("/:id/edit", s.handleProjectsEdit)
			projects.PUT("/:id", s.handleProjectsUpdate)
			projects.DELETE("/:id", s.handleProjectsDelete)
		}

		// Tokens routes
		tokens := protected.Group("/tokens")
		{
			tokens.GET("", s.handleTokensList)
			tokens.GET("/new", s.handleTokensNew)
			tokens.POST("", s.handleTokensCreate)
			tokens.GET("/:token", s.handleTokensShow)
		}
	}

	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
			"service":   "admin-ui",
			"version":   "0.1.0",
		})
	})
}

// Dashboard handlers
func (s *Server) handleDashboard(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	// Get dashboard data from Management API
	dashboardData, err := apiClient.GetDashboardData(c.Request.Context())
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load dashboard data: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "simple-dashboard.html", gin.H{
		"title": "Dashboard",
		"data":  dashboardData,
	})
}

// Project handlers
func (s *Server) handleProjectsList(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	page := getPageFromQuery(c, 1)
	pageSize := getPageSizeFromQuery(c, 10)

	projects, pagination, err := apiClient.GetProjects(c.Request.Context(), page, pageSize)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load projects: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "projects-list-complete.html", gin.H{
		"title":      "Projects",
		"projects":   projects,
		"pagination": pagination,
	})
}

func (s *Server) handleProjectsNew(c *gin.Context) {
	c.HTML(http.StatusOK, "projects/new.html", gin.H{
		"title": "Create Project",
	})
}

func (s *Server) handleProjectsCreate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	var req struct {
		Name         string `form:"name" binding:"required"`
		OpenAIAPIKey string `form:"openai_api_key" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "projects/new.html", gin.H{
			"title": "Create Project",
			"error": "Please fill in all required fields",
		})
		return
	}

	project, err := apiClient.CreateProject(c.Request.Context(), req.Name, req.OpenAIAPIKey)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "projects/new.html", gin.H{
			"title": "Create Project",
			"error": fmt.Sprintf("Failed to create project: %v", err),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%s", project.ID))
}

func (s *Server) handleProjectsShow(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	id := c.Param("id")

	project, err := apiClient.GetProject(c.Request.Context(), id)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "Project not found",
		})
		return
	}

	c.HTML(http.StatusOK, "projects/show.html", gin.H{
		"title":   "Project Details",
		"project": project,
	})
}

func (s *Server) handleProjectsEdit(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	id := c.Param("id")

	project, err := apiClient.GetProject(c.Request.Context(), id)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "Project not found",
		})
		return
	}

	c.HTML(http.StatusOK, "projects/edit.html", gin.H{
		"title":   "Edit Project",
		"project": project,
	})
}

func (s *Server) handleProjectsUpdate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	id := c.Param("id")

	var req struct {
		Name         string `form:"name"`
		OpenAIAPIKey string `form:"openai_api_key"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Invalid form data",
		})
		return
	}

	project, err := apiClient.UpdateProject(c.Request.Context(), id, req.Name, req.OpenAIAPIKey)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to update project: %v", err),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%s", project.ID))
}

func (s *Server) handleProjectsDelete(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	id := c.Param("id")

	err := apiClient.DeleteProject(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to delete project: %v", err),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, "/projects")
}

// Token handlers
func (s *Server) handleTokensList(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	page := getPageFromQuery(c, 1)
	pageSize := getPageSizeFromQuery(c, 10)
	projectID := c.Query("project_id")

	tokens, pagination, err := apiClient.GetTokens(c.Request.Context(), projectID, page, pageSize)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load tokens: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens-list-complete.html", gin.H{
		"tokens":     tokens,
		"pagination": pagination,
		"projectId":  projectID,
	})
}

func (s *Server) handleTokensNew(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	projects, _, err := apiClient.GetProjects(c.Request.Context(), 1, 100)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load projects: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens/new.html", gin.H{
		"title":    "Generate Token",
		"projects": projects,
	})
}

func (s *Server) handleTokensCreate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(*APIClient)

	var req struct {
		ProjectID     string `form:"project_id" binding:"required"`
		DurationHours int    `form:"duration_hours" binding:"required,min=1,max=8760"` // Max 1 year
	}

	if err := c.ShouldBind(&req); err != nil {
		projects, _, _ := apiClient.GetProjects(c.Request.Context(), 1, 100)
		c.HTML(http.StatusBadRequest, "tokens/new.html", gin.H{
			"title":    "Generate Token",
			"projects": projects,
			"error":    "Please fill in all required fields correctly",
		})
		return
	}

	token, err := apiClient.CreateToken(c.Request.Context(), req.ProjectID, req.DurationHours)
	if err != nil {
		projects, _, _ := apiClient.GetProjects(c.Request.Context(), 1, 100)
		c.HTML(http.StatusInternalServerError, "tokens/new.html", gin.H{
			"title":    "Generate Token",
			"projects": projects,
			"error":    fmt.Sprintf("Failed to create token: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens/created.html", gin.H{
		"title": "Token Created",
		"token": token,
	})
}

func (s *Server) handleTokensShow(c *gin.Context) {
	// Note: This would require a new Management API endpoint to get token details
	// For now, redirect to tokens list
	c.Redirect(http.StatusSeeOther, "/tokens")
}

// Helper functions
func getPageFromQuery(c *gin.Context, defaultValue int) int {
	if page := c.Query("page"); page != "" {
		if p, err := parsePositiveInt(page); err == nil && p > 0 {
			return p
		}
	}
	return defaultValue
}

func getPageSizeFromQuery(c *gin.Context, defaultValue int) int {
	if size := c.Query("size"); size != "" {
		if s, err := parsePositiveInt(size); err == nil && s > 0 && s <= 100 {
			return s
		}
	}
	return defaultValue
}

func parsePositiveInt(s string) (int, error) {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return 0, err
	}
	return result, nil
}

// templateFuncs returns custom template functions for HTML templates
func (s *Server) templateFuncs() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"inc": func(a int) int {
			return a + 1
		},
		"dec": func(a int) int {
			return a - 1
		},
		"seq": func(start, end int) []int {
			if start > end {
				return []int{}
			}
			seq := make([]int, end-start+1)
			for i := range seq {
				seq[i] = start + i
			}
			return seq
		},
		"now": func() time.Time {
			return time.Now()
		},
		"eq": func(a, b any) bool {
			return a == b
		},
		"ne": func(a, b any) bool {
			return a != b
		},
		"lt": func(a, b any) bool {
			switch v := a.(type) {
			case int:
				if b2, ok := b.(int); ok {
					return v < b2
				}
			case int64:
				if b2, ok := b.(int64); ok {
					return v < b2
				}
			case time.Time:
				if b2, ok := b.(time.Time); ok {
					return v.Before(b2)
				}
			}
			return false
		},
		"gt": func(a, b any) bool {
			switch v := a.(type) {
			case int:
				if b2, ok := b.(int); ok {
					return v > b2
				}
			case int64:
				if b2, ok := b.(int64); ok {
					return v > b2
				}
			case time.Time:
				if b2, ok := b.(time.Time); ok {
					return v.After(b2)
				}
			}
			return false
		},
		"le": func(a, b any) bool {
			switch v := a.(type) {
			case int:
				if b2, ok := b.(int); ok {
					return v <= b2
				}
			case int64:
				if b2, ok := b.(int64); ok {
					return v <= b2
				}
			case time.Time:
				if b2, ok := b.(time.Time); ok {
					return v.Before(b2) || v.Equal(b2)
				}
			}
			return false
		},
		"ge": func(a, b any) bool {
			switch v := a.(type) {
			case int:
				if b2, ok := b.(int); ok {
					return v >= b2
				}
			case int64:
				if b2, ok := b.(int64); ok {
					return v >= b2
				}
			case time.Time:
				if b2, ok := b.(time.Time); ok {
					return v.After(b2) || v.Equal(b2)
				}
			}
			return false
		},
		"and": func(a, b bool) bool {
			return a && b
		},
		"or": func(a, b bool) bool {
			return a || b
		},
		"not": func(a bool) bool {
			return !a
		},
		"obfuscateAPIKey": func(apiKey string) string {
			if len(apiKey) <= 12 {
				if len(apiKey) <= 4 {
					return strings.Repeat("*", len(apiKey))
				}
				return apiKey[:2] + strings.Repeat("*", len(apiKey)-2)
			}
			return apiKey[:8] + "..." + apiKey[len(apiKey)-4:]
		},
		"obfuscateToken": func(token string) string {
			if len(token) <= 8 {
				return "****"
			}
			return token[:4] + "****" + token[len(token)-4:]
		},
	}
}

// Authentication handlers

func (s *Server) handleLoginForm(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"title": "Sign In",
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		ManagementToken string `form:"management_token" binding:"required"`
		RememberMe      bool   `form:"remember_me"`
	}

	if err := c.Request.ParseForm(); err == nil {
		// Only log field presence, not values
		log.Printf("Raw POST form fields: %v", getFormFieldNames(c.Request.PostForm))
	}

	if err := c.ShouldBind(&req); err != nil {
		log.Printf("ShouldBind error: %v", err)
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"title": "Sign In",
			"error": "Please enter your management token",
		})
		return
	}

	log.Printf("Login attempt: token=%q rememberMe=%v", obfuscateToken(req.ManagementToken), req.RememberMe)

	// Validate token against the Management API
	if !s.validateTokenWithAPI(c.Request.Context(), req.ManagementToken) {
		log.Printf("Token validation failed for token=%q", obfuscateToken(req.ManagementToken))
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"title": "Sign In",
			"error": "Invalid management token. Please check your token and try again.",
		})
		return
	}

	// Set session cookie
	session := sessions.Default(c)
	session.Set("token", req.ManagementToken)
	if req.RememberMe {
		session.Options(sessions.Options{
			MaxAge:   30 * 24 * 60 * 60, // 30 days
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
		})
	} else {
		session.Options(sessions.Options{
			MaxAge:   0,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,
		})
	}
	if err := session.Save(); err != nil {
		log.Printf("Session save error: %v", err)
	}

	log.Printf("Session saved for token=%q, rememberMe=%v", obfuscateToken(req.ManagementToken), req.RememberMe)

	// Redirect to dashboard
	c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (s *Server) handleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		log.Printf("Session save error: %v", err)
	}
	c.Redirect(http.StatusSeeOther, "/auth/login")
}

// authMiddleware checks for valid session
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		token := session.Get("token")
		if token == nil {
			c.Redirect(http.StatusSeeOther, "/auth/login")
			c.Abort()
			return
		}

		// Optionally, validate token again or cache validation
		apiClient := NewAPIClient(s.config.AdminUI.APIBaseURL, token.(string))
		c.Set("apiClient", apiClient)
		c.Next()
	}
}

// validateTokenWithAPI validates a token against the Management API
func (s *Server) validateTokenWithAPI(ctx context.Context, token string) bool {
	// Create a HTTP client to validate the token
	client := &http.Client{Timeout: 10 * time.Second}

	// Try to access a simple management endpoint with the token
	req, err := http.NewRequestWithContext(ctx, "GET", s.config.AdminUI.APIBaseURL+"/manage/projects", nil)
	if err != nil {
		return false
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing response body: %v", err)
		}
	}()

	// Valid token should return 200 OK (or other success status)
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// getFormFieldNames returns a slice of form field names for logging
func getFormFieldNames(form map[string][]string) []string {
	fields := make([]string, 0, len(form))
	for k := range form {
		fields = append(fields, k)
	}
	return fields
}

// obfuscateToken returns a partially masked version of a token for logging
func obfuscateToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return token[:4] + "****" + token[len(token)-4:]
}
