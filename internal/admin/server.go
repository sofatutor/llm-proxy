// Package admin provides the HTTP server for the Admin UI.
// This package implements a separate web interface for managing
// projects and tokens via the Management API.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/obfuscate"
	"go.uber.org/zap"
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

// APIClientInterface abstracts the API client for testability
//go:generate mockgen -destination=mock_api_client.go -package=admin . APIClientInterface
// Only the methods needed by handlers are included

type APIClientInterface interface {
	GetDashboardData(ctx context.Context) (*DashboardData, error)
	GetProjects(ctx context.Context, page, pageSize int) ([]Project, *Pagination, error)
	GetTokens(ctx context.Context, projectID string, page, pageSize int) ([]Token, *Pagination, error)
	CreateToken(ctx context.Context, projectID string, durationMinutes int) (*TokenCreateResponse, error)
	GetProject(ctx context.Context, projectID string) (*Project, error)
	UpdateProject(ctx context.Context, projectID string, name string, openAIAPIKey string, isActive *bool) (*Project, error)
	DeleteProject(ctx context.Context, projectID string) error
	CreateProject(ctx context.Context, name string, openAIAPIKey string) (*Project, error)
	GetAuditEvents(ctx context.Context, filters map[string]string, page, pageSize int) ([]AuditEvent, *Pagination, error)
	GetAuditEvent(ctx context.Context, id string) (*AuditEvent, error)
	GetToken(ctx context.Context, tokenID string) (*Token, error)
	UpdateToken(ctx context.Context, tokenID string, isActive *bool, maxRequests *int) (*Token, error)
	RevokeToken(ctx context.Context, tokenID string) error
	RevokeProjectTokens(ctx context.Context, projectID string) error
}

// Server represents the Admin UI HTTP server.
// It provides a web interface for managing projects and tokens
// by communicating with the Management API.
type Server struct {
	server    *http.Server
	config    *config.Config
	engine    *gin.Engine
	apiClient *APIClient
	logger    *zap.Logger

	// For testability: allow injection of token validation logic
	ValidateTokenWithAPI func(context.Context, string) bool

	// Audit logger for admin actions
	auditLogger *audit.Logger
}

// NewServer creates a new Admin UI server with the provided configuration.
// It initializes the Gin engine, sets up routes, and configures the HTTP server.
func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize logger
	logger, err := logging.NewLogger(cfg.LogLevel, cfg.LogFormat, cfg.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize admin server logger: %w", err)
	}

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

	// Add method override middleware for HTML forms
	engine.Use(func(c *gin.Context) {
		if c.Request.Method == "POST" && c.Request.FormValue("_method") != "" {
			c.Request.Method = c.Request.FormValue("_method")
		}
		c.Next()
	})

	// Add session middleware
	store := cookie.NewStore(getSessionSecret(cfg))
	engine.Use(sessions.Sessions("llmproxy_session", store))

	// Create API client for communicating with Management API
	// Note: API client will be updated when user logs in
	var apiClient *APIClient
	if cfg.AdminUI.ManagementToken != "" {
		apiClient = NewAPIClient(cfg.AdminUI.APIBaseURL, cfg.AdminUI.ManagementToken)
	}

	// Initialize audit logger
	var auditLogger *audit.Logger
	if cfg.AuditEnabled && cfg.AuditLogFile != "" {
		auditConfig := audit.LoggerConfig{
			FilePath:  cfg.AuditLogFile,
			CreateDir: cfg.AuditCreateDir,
		}
		var err error
		auditLogger, err = audit.NewLogger(auditConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize admin audit logger: %w", err)
		}
	} else {
		auditLogger = audit.NewNullLogger()
	}

	s := &Server{
		config:      cfg,
		engine:      engine,
		apiClient:   apiClient,
		logger:      logger,
		auditLogger: auditLogger,
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

	// Load HTML templates with custom functions using glob patterns
	s.engine.SetFuncMap(s.templateFuncs())
	td := s.config.AdminUI.TemplateDir

	// Load all templates - both root level and subdirectories
	templGlob := template.Must(template.New("").Funcs(s.templateFuncs()).ParseGlob(filepath.Join(td, "*.html")))
	templGlob = template.Must(templGlob.ParseGlob(filepath.Join(td, "*/*.html")))
	s.engine.SetHTMLTemplate(templGlob)

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
			// HTML forms submit via POST with _method override; Gin matches routes before middleware,
			// so provide POST fallback routes that dispatch to the correct handlers.
			projects.POST("/:id", s.handleProjectsPostOverride)
			projects.PUT("/:id", s.handleProjectsUpdate)
			projects.DELETE("/:id", s.handleProjectsDelete)
			projects.POST("/:id/revoke-tokens", s.handleProjectsBulkRevoke)
		}

		// Tokens routes
		tokens := protected.Group("/tokens")
		{
			tokens.GET("", s.handleTokensList)
			tokens.GET("/new", s.handleTokensNew)
			tokens.POST("", s.handleTokensCreate)
			tokens.GET("/:token", s.handleTokensShow)
			tokens.GET("/:token/edit", s.handleTokensEdit)
			// POST fallback for HTML forms with _method override
			tokens.POST("/:token", s.handleTokensPostOverride)
			tokens.PUT("/:token", s.handleTokensUpdate)
			tokens.DELETE("/:token", s.handleTokensRevoke)
		}

		// Audit routes
		audit := protected.Group("/audit")
		{
			audit.GET("", s.handleAuditList)
			audit.GET("/:id", s.handleAuditShow)
		}
	}

	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		adminHealth := gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
			"service":   "admin-ui",
			"version":   "0.1.0",
		}

		backendURL := s.config.AdminUI.APIBaseURL
		if backendURL == "" {
			backendURL = "http://localhost:8080"
		}
		if !strings.HasSuffix(backendURL, "/health") {
			backendURL = strings.TrimRight(backendURL, "/") + "/health"
		}
		client := &http.Client{Timeout: 2 * time.Second}
		backendHealth := gin.H{
			"status": "down",
			"error":  "Backend unavailable",
		}
		if resp, err := client.Get(backendURL); err == nil && resp.StatusCode == 200 {
			var backendData map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&backendData); err == nil {
				backendHealth = backendData
			}
			err := resp.Body.Close()
			if err != nil {
				s.logger.Warn("failed to close backend health response body", zap.Error(err))
			}
		}
		c.JSON(http.StatusOK, gin.H{
			"admin":   adminHealth,
			"backend": backendHealth,
		})
	})
}

// Dashboard handlers
func (s *Server) handleDashboard(c *gin.Context) {
	// Get API client from context
	apiClientIface := c.MustGet("apiClient").(APIClientInterface)

	// Get dashboard data from Management API with forwarded browser metadata
	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	dashboardData, err := apiClientIface.GetDashboardData(ctx)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load dashboard data: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"title":  "Dashboard",
		"active": "dashboard",
		"data":   dashboardData,
	})
}

// Project handlers
func (s *Server) handleProjectsList(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	page := getPageFromQuery(c, 1)
	pageSize := getPageSizeFromQuery(c, 10)

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	// Forward best-effort original client IP from headers
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	projects, pagination, err := apiClient.GetProjects(ctx, page, pageSize)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load projects: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "projects/list.html", gin.H{
		"title":      "Projects",
		"active":     "projects",
		"projects":   projects,
		"pagination": pagination,
	})
}

func (s *Server) handleProjectsNew(c *gin.Context) {
	c.HTML(http.StatusOK, "projects/new.html", gin.H{
		"title":  "Create Project",
		"active": "projects",
	})
}

func (s *Server) handleProjectsCreate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	var req struct {
		Name         string `form:"name" binding:"required"`
		OpenAIAPIKey string `form:"openai_api_key" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "projects/new.html", gin.H{
			"title":  "Create Project",
			"active": "projects",
			"error":  "Please fill in all required fields",
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	project, err := apiClient.CreateProject(ctx, req.Name, req.OpenAIAPIKey)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "projects/new.html", gin.H{
			"title":  "Create Project",
			"active": "projects",
			"error":  fmt.Sprintf("Failed to create project: %v", err),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%s", project.ID))
}

func (s *Server) handleProjectsShow(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	id := c.Param("id")

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	project, err := apiClient.GetProject(ctx, id)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "Project not found",
		})
		return
	}

	c.HTML(http.StatusOK, "projects/show.html", gin.H{
		"title":   "Project Details",
		"active":  "projects",
		"project": project,
	})
}

func (s *Server) handleProjectsEdit(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	id := c.Param("id")

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	project, err := apiClient.GetProject(ctx, id)
	if err != nil {
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"error": "Project not found",
		})
		return
	}

	c.HTML(http.StatusOK, "projects/edit.html", gin.H{
		"title":   "Edit Project",
		"active":  "projects",
		"project": project,
	})
}

func (s *Server) handleProjectsUpdate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	id := c.Param("id")

	var req struct {
		Name         string `form:"name" binding:"required"`
		OpenAIAPIKey string `form:"openai_api_key" binding:"required"`
		IsActive     *bool  `form:"is_active"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Invalid form data",
		})
		return
	}

	// Checkbox handling: check if the posted value is "true"
	isActive := c.PostForm("is_active") == "true"
	isActivePtr := &isActive

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	project, err := apiClient.UpdateProject(ctx, id, req.Name, req.OpenAIAPIKey, isActivePtr)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to update project: %v", err),
		})
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%s", project.ID))
}

// handleProjectsPostOverride routes POST requests with _method overrides to the appropriate handler.
// It ensures form submissions to /projects/:id work even though Gin resolves routes before middleware.
func (s *Server) handleProjectsPostOverride(c *gin.Context) {
	// Parse form to access _method
	if err := c.Request.ParseForm(); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Failed to parse form data",
		})
		return
	}

	method := c.PostForm("_method")
	switch strings.ToUpper(method) {
	case http.MethodPut:
		s.handleProjectsUpdate(c)
		return
	case http.MethodDelete:
		s.handleProjectsDelete(c)
		return
	default:
		// No override provided; treat as bad request
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Unsupported method override for project action",
		})
		return
	}
}

func (s *Server) handleProjectsDelete(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	id := c.Param("id")

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	err := apiClient.DeleteProject(ctx, id)
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
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	page := getPageFromQuery(c, 1)
	pageSize := getPageSizeFromQuery(c, 10)
	projectID := c.Query("project_id")

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	tokens, pagination, err := apiClient.GetTokens(ctx, projectID, page, pageSize)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load tokens: %v", err),
		})
		return
	}

	// Fetch all projects to create a lookup map for project names
	projects, _, err := apiClient.GetProjects(ctx, 1, 1000) // Get up to 1000 projects
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load projects: %v", err),
		})
		return
	}

	// Create project ID to name lookup map
	projectNames := make(map[string]string)
	for _, project := range projects {
		projectNames[project.ID] = project.Name
	}

	c.HTML(http.StatusOK, "tokens/list.html", gin.H{
		"title":        "Tokens",
		"active":       "tokens",
		"tokens":       tokens,
		"pagination":   pagination,
		"projectId":    projectID,
		"projectNames": projectNames,
		"now":          time.Now(),
		"currentTime":  time.Now(),
	})
}

func (s *Server) handleTokensNew(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	projects, _, err := apiClient.GetProjects(ctx, 1, 100)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": fmt.Sprintf("Failed to load projects: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens/new.html", gin.H{
		"title":    "Generate Token",
		"active":   "tokens",
		"projects": projects,
	})
}

func (s *Server) handleTokensCreate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	var req struct {
		ProjectID       string `form:"project_id" binding:"required"`
		DurationMinutes int    `form:"duration_minutes" binding:"required,min=1,max=525600"`
	}

	if err := c.ShouldBind(&req); err != nil {
		// forward context as well for consistency in audit logs
		projCtx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
		if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
			projCtx = context.WithValue(projCtx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
		} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
			projCtx = context.WithValue(projCtx, ctxKeyForwardedIP, ip)
		}
		if ref := c.Request.Referer(); ref != "" {
			projCtx = context.WithValue(projCtx, ctxKeyForwardedReferer, ref)
		}
		projects, _, _ := apiClient.GetProjects(projCtx, 1, 100)
		c.HTML(http.StatusBadRequest, "tokens/new.html", gin.H{
			"title":    "Generate Token",
			"active":   "tokens",
			"projects": projects,
			"error":    "Please fill in all required fields correctly",
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}
	token, err := apiClient.CreateToken(ctx, req.ProjectID, req.DurationMinutes)
	if err != nil {
		// forward context as well for consistency in audit logs
		projCtx := ctx
		projects, _, _ := apiClient.GetProjects(projCtx, 1, 100)
		c.HTML(http.StatusInternalServerError, "tokens/new.html", gin.H{
			"title":    "Generate Token",
			"active":   "tokens",
			"projects": projects,
			"error":    fmt.Sprintf("Failed to create token: %v", err),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens/created.html", gin.H{
		"title":  "Token Created",
		"active": "tokens",
		"token":  token,
	})
}

func (s *Server) handleTokensShow(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	tokenID := c.Param("token")
	if tokenID == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Token ID is required",
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}

	token, err := apiClient.GetToken(ctx, tokenID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"error":   "Token not found",
				"details": fmt.Sprintf("Token %s was not found", tokenID),
			})
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error":   "Failed to load token",
				"details": err.Error(),
			})
		}
		return
	}

	// Get project for the token
	project, err := apiClient.GetProject(ctx, token.ProjectID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":   "Failed to load project",
			"details": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens/show.html", gin.H{
		"title":       "Token Details",
		"active":      "tokens",
		"token":       token,
		"project":     project,
		"tokenID":     tokenID,
		"now":         time.Now(),
		"currentTime": time.Now(),
	})
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
		"stringOr": func(value any, fallback string) string {
			// Safely dereference optional strings for templates
			switch v := value.(type) {
			case *string:
				if v != nil && *v != "" {
					return *v
				}
			case string:
				if v != "" {
					return v
				}
			}
			return fallback
		},
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
		"obfuscateAPIKey": func(apiKey string) string { return obfuscate.ObfuscateTokenGeneric(apiKey) },
		"obfuscateToken":  func(token string) string { return obfuscate.ObfuscateTokenSimple(token) },
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"pageRange": func(current, total int) []int {
			// Show up to 7 page numbers around current page
			start := current - 3
			end := current + 3

			if start < 1 {
				start = 1
			}
			if end > total {
				end = total
			}

			// Adjust if we have fewer than 7 pages to show
			if end-start < 6 && total > 6 {
				if start == 1 {
					end = start + 6
					if end > total {
						end = total
					}
				} else if end == total {
					start = end - 6
					if start < 1 {
						start = 1
					}
				}
			}

			pages := make([]int, 0, end-start+1)
			for i := start; i <= end; i++ {
				pages = append(pages, i)
			}
			return pages
		},
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				return nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil
				}
				dict[key] = values[i+1]
			}
			return dict
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
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	var req struct {
		ManagementToken string `form:"management_token" binding:"required"`
		RememberMe      bool   `form:"remember_me"`
	}

	if err := c.Request.ParseForm(); err == nil {
		// Only log field presence, not values
		logger.Debug("raw POST form fields", zap.Strings("fields", getFormFieldNames(c.Request.PostForm)))
	}

	if err := c.ShouldBind(&req); err != nil {
		logger.Warn("ShouldBind error", zap.Error(err))
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"title": "Sign In",
			"error": "Please enter your management token",
		})
		return
	}

	logger.Info("login attempt", zap.String("token", obfuscateToken(req.ManagementToken)), zap.Bool("remember_me", req.RememberMe))

	// Use injected or default token validation
	validate := s.ValidateTokenWithAPI
	if validate == nil {
		validate = s.validateTokenWithAPI
	}
	if !validate(c.Request.Context(), req.ManagementToken) {
		logger.Error("token validation failed", zap.String("token", obfuscateToken(req.ManagementToken)))
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
		logger.Error("session save error", zap.Error(err))
	}

	logger.Info("session saved", zap.String("token", obfuscateToken(req.ManagementToken)), zap.Bool("remember_me", req.RememberMe))

	// Redirect to dashboard
	c.Redirect(http.StatusSeeOther, "/dashboard")
}

func (s *Server) handleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		s.logger.Error("session save error", zap.Error(err))
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
			s.logger.Warn("Error closing response body", zap.Error(err))
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
	return obfuscate.ObfuscateTokenSimple(token)
}

// Audit handlers
func (s *Server) handleAuditList(c *gin.Context) {
	// Get API client from context
	apiClientIface := c.MustGet("apiClient").(APIClientInterface)

	// Parse query parameters for filtering
	filters := make(map[string]string)
	query := c.Request.URL.Query()

	// Filter parameters
	if action := query.Get("action"); action != "" {
		filters["action"] = action
	}
	if outcome := query.Get("outcome"); outcome != "" {
		filters["outcome"] = outcome
	}
	if projectID := query.Get("project_id"); projectID != "" {
		filters["project_id"] = projectID
	}
	if actor := query.Get("actor"); actor != "" {
		filters["actor"] = actor
	}
	if clientIP := query.Get("client_ip"); clientIP != "" {
		filters["client_ip"] = clientIP
	}
	if requestID := query.Get("request_id"); requestID != "" {
		filters["request_id"] = requestID
	}
	if method := query.Get("method"); method != "" {
		filters["method"] = method
	}
	if path := query.Get("path"); path != "" {
		filters["path"] = path
	}
	if search := query.Get("search"); search != "" {
		filters["search"] = search
	}
	if startTime := query.Get("start_time"); startTime != "" {
		filters["start_time"] = startTime
	}
	if endTime := query.Get("end_time"); endTime != "" {
		filters["end_time"] = endTime
	}

	// Parse pagination
	page := 1
	if pageStr := query.Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	pageSize := 20
	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 100 {
			pageSize = ps
		}
	}

	// Get audit events with forwarded browser metadata
	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}

	events, pagination, err := apiClientIface.GetAuditEvents(ctx, filters, page, pageSize)
	if err != nil {
		log.Printf("Failed to get audit events: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":   "Failed to load audit events",
			"details": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "audit/list.html", gin.H{
		"title":      "Audit Events",
		"active":     "audit",
		"events":     events,
		"pagination": pagination,
		"filters":    filters,
		"query":      query,
	})
}

func (s *Server) handleAuditShow(c *gin.Context) {
	// Get API client from context
	apiClientIface := c.MustGet("apiClient").(APIClientInterface)

	// Get audit event ID from URL
	id := c.Param("id")
	if id == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":   "Invalid audit event ID",
			"details": "Audit event ID is required",
		})
		return
	}

	// Get audit event with forwarded browser metadata
	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}

	event, err := apiClientIface.GetAuditEvent(ctx, id)
	if err != nil {
		log.Printf("Failed to get audit event %s: %v", id, err)
		if strings.Contains(err.Error(), "not found") {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"error":   "Audit event not found",
				"details": fmt.Sprintf("Audit event with ID %s was not found", id),
			})
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error":   "Failed to load audit event",
				"details": err.Error(),
			})
		}
		return
	}

	c.HTML(http.StatusOK, "audit/show.html", gin.H{
		"title":  "Audit Event",
		"active": "audit",
		"event":  event,
	})
}

// Token edit/revoke handlers

func (s *Server) handleTokensEdit(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	tokenID := c.Param("token")
	if tokenID == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Token ID is required",
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}

	token, err := apiClient.GetToken(ctx, tokenID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"error":   "Token not found",
				"details": fmt.Sprintf("Token %s was not found", tokenID),
			})
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error":   "Failed to load token",
				"details": err.Error(),
			})
		}
		return
	}

	// Get project for the token
	project, err := apiClient.GetProject(ctx, token.ProjectID)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error":   "Failed to load project",
			"details": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "tokens/edit.html", gin.H{
		"title":   "Edit Token",
		"active":  "tokens",
		"token":   token,
		"project": project,
		"tokenID": tokenID,
	})
}

func (s *Server) handleTokensUpdate(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	tokenID := c.Param("token")
	if tokenID == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Token ID is required",
		})
		return
	}

	var req struct {
		IsActive    *bool `form:"is_active"`
		MaxRequests *int  `form:"max_requests"`
	}

	if err := c.ShouldBind(&req); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error":   "Invalid form data",
			"details": err.Error(),
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}

	_, err := apiClient.UpdateToken(ctx, tokenID, req.IsActive, req.MaxRequests)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"error":   "Token not found",
				"details": fmt.Sprintf("Token %s was not found", tokenID),
			})
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error":   "Failed to update token",
				"details": err.Error(),
			})
		}
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/tokens/%s", tokenID))
}

func (s *Server) handleTokensRevoke(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	tokenID := c.Param("token")
	if tokenID == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Token ID is required",
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}

	err := apiClient.RevokeToken(ctx, tokenID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"error":   "Token not found",
				"details": fmt.Sprintf("Token %s was not found", tokenID),
			})
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error":   "Failed to revoke token",
				"details": err.Error(),
			})
		}
		return
	}

	c.Redirect(http.StatusSeeOther, "/tokens")
}

// handleTokensPostOverride routes POST requests with _method overrides for token actions.
func (s *Server) handleTokensPostOverride(c *gin.Context) {
	// Parse form to access _method
	if err := c.Request.ParseForm(); err != nil {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Failed to parse form data",
		})
		return
	}

	method := c.PostForm("_method")
	switch strings.ToUpper(method) {
	case http.MethodPut:
		s.handleTokensUpdate(c)
		return
	case http.MethodDelete:
		s.handleTokensRevoke(c)
		return
	default:
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Unsupported method override for token action",
		})
		return
	}
}

// Project bulk revoke handler

func (s *Server) handleProjectsBulkRevoke(c *gin.Context) {
	// Get API client from context
	apiClient := c.MustGet("apiClient").(APIClientInterface)

	projectID := c.Param("id")
	if projectID == "" {
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Project ID is required",
		})
		return
	}

	ctx := context.WithValue(c.Request.Context(), ctxKeyForwardedUA, c.Request.UserAgent())
	if ip := c.Request.Header.Get("X-Forwarded-For"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, strings.Split(ip, ",")[0])
	} else if ip := c.Request.Header.Get("X-Real-IP"); ip != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedIP, ip)
	}
	if ref := c.Request.Referer(); ref != "" {
		ctx = context.WithValue(ctx, ctxKeyForwardedReferer, ref)
	}

	err := apiClient.RevokeProjectTokens(ctx, projectID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			c.HTML(http.StatusNotFound, "error.html", gin.H{
				"error":   "Project not found",
				"details": fmt.Sprintf("Project %s was not found", projectID),
			})
		} else {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{
				"error":   "Failed to revoke project tokens",
				"details": err.Error(),
			})
		}
		return
	}

	c.Redirect(http.StatusSeeOther, fmt.Sprintf("/projects/%s", projectID))
}
