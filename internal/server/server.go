// Package server implements the HTTP server for the LLM Proxy.
// It handles request routing, lifecycle management, and provides
// health check endpoints and core API functionality.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/middleware"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

type ctxKey string

const ctxKeyRequestID ctxKey = "request_id"

// Server represents the HTTP server for the LLM Proxy.
// It encapsulates the underlying http.Server along with application configuration
// and handles request routing and server lifecycle management.
type Server struct {
	server       *http.Server
	config       *config.Config
	tokenStore   token.TokenStore
	projectStore proxy.ProjectStore
	logger       *zap.Logger
	proxy        *proxy.TransparentProxy
	metrics      Metrics
}

// HealthResponse is the response body for the health check endpoint.
// It provides basic information about the server status and version.
type HealthResponse struct {
	Status    string    `json:"status"`    // Service status, "ok" for a healthy system
	Timestamp time.Time `json:"timestamp"` // Current server time
	Version   string    `json:"version"`   // Application version number
}

// Metrics holds runtime metrics for the server.
type Metrics struct {
	StartTime    time.Time
	RequestCount int64
	ErrorCount   int64
}

// Version is the application version, following semantic versioning.
const Version = "0.1.0"

// maxDurationMinutes is the maximum allowed duration for a token (365 days)
const maxDurationMinutes = 525600

// New creates a new HTTP server with the provided configuration and store implementations.
// It initializes the server with appropriate timeouts and registers all necessary route handlers.
// The server is not started until the Start method is called.
func New(cfg *config.Config, tokenStore token.TokenStore, projectStore proxy.ProjectStore) (*Server, error) {
	mux := http.NewServeMux()

	logger, err := logging.NewLogger(cfg.LogLevel, cfg.LogFormat, cfg.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	metrics := Metrics{StartTime: time.Now()}

	s := &Server{
		config:       cfg,
		tokenStore:   tokenStore,
		projectStore: projectStore,
		logger:       logger,
		metrics:      metrics,
		server: &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      mux,
			ReadTimeout:  cfg.RequestTimeout,
			WriteTimeout: cfg.RequestTimeout,
			IdleTimeout:  cfg.RequestTimeout * 2,
		},
	}

	// Register routes
	mux.HandleFunc("/health", s.logRequestMiddleware(s.handleHealth))
	mux.HandleFunc("/ready", s.logRequestMiddleware(s.handleReady))
	mux.HandleFunc("/live", s.logRequestMiddleware(s.handleLive))
	mux.HandleFunc("/manage/projects", s.logRequestMiddleware(s.handleProjects))
	mux.HandleFunc("/manage/projects/", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleProjectByID)))
	mux.HandleFunc("/manage/tokens", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleTokens)))

	// Add catch-all handler for unmatched routes to ensure logging
	mux.HandleFunc("/", s.logRequestMiddleware(s.handleNotFound))

	if cfg.EnableMetrics {
		path := cfg.MetricsPath
		if path == "" {
			path = "/metrics"
		}
		mux.HandleFunc(path, s.logRequestMiddleware(s.handleMetrics))
	}

	return s, nil
}

// Start initializes all required components and starts the HTTP server.
// This method blocks until the server is shut down or an error occurs.
//
// It returns an error if the server fails to start or encounters an
// unrecoverable error during operation.
func (s *Server) Start() error {
	// Initialize required components
	if err := s.initializeComponents(); err != nil {
		return fmt.Errorf("failed to initialize components: %w", err)
	}

	log.Printf("Server starting on %s\n", s.config.ListenAddr)

	return s.server.ListenAndServe()
}

// initializeComponents sets up all the required components for the server
func (s *Server) initializeComponents() error {
	// Initialize API routes from configuration
	if err := s.initializeAPIRoutes(); err != nil {
		return fmt.Errorf("failed to initialize API routes: %w", err)
	}

	// Pending: database, logging, admin, and metrics initialization.
	// See server_test.go for test stubs covering these responsibilities.

	return nil
}

// initializeAPIRoutes sets up the API proxy routes based on configuration
func (s *Server) initializeAPIRoutes() error {
	// Load API providers configuration
	apiConfig, err := proxy.LoadAPIConfigFromFile(s.config.APIConfigPath)
	if err != nil {
		// If the config file doesn't exist or has errors, fall back to a default OpenAI configuration
		log.Printf("Warning: Failed to load API config from %s: %v", s.config.APIConfigPath, err)
		log.Printf("Using default OpenAI configuration")

		// Create a default API configuration
		apiConfig = &proxy.APIConfig{
			DefaultAPI: "openai",
			APIs: map[string]*proxy.APIProviderConfig{
				"openai": {
					BaseURL: s.config.OpenAIAPIURL,
					AllowedEndpoints: []string{
						"/v1/chat/completions",
						"/v1/completions",
						"/v1/embeddings",
						"/v1/models",
						"/v1/edits",
						"/v1/fine-tunes",
						"/v1/files",
						"/v1/images/generations",
						"/v1/audio/transcriptions",
						"/v1/moderations",
					},
					AllowedMethods: []string{"GET", "POST", "DELETE"},
					Timeouts: proxy.TimeoutConfig{
						Request:        s.config.RequestTimeout,
						ResponseHeader: 30 * time.Second,
						IdleConnection: 90 * time.Second,
						FlushInterval:  100 * time.Millisecond,
					},
					Connection: proxy.ConnectionConfig{
						MaxIdleConns:        100,
						MaxIdleConnsPerHost: 20,
					},
				},
			},
		}
	}

	// Get proxy configuration for the default API provider
	proxyConfig, err := apiConfig.GetProxyConfigForAPI(s.config.DefaultAPIProvider)
	if err != nil {
		// If specified provider doesn't exist, use the default one
		log.Printf("Warning: %v", err)
		proxyConfig, err = apiConfig.GetProxyConfigForAPI(apiConfig.DefaultAPI)
		if err != nil {
			return fmt.Errorf("failed to get proxy configuration: %w", err)
		}
	}

	// Use the injected tokenStore and projectStore
	// (No more creation of mock stores or test data here)
	tokenValidator := token.NewValidator(s.tokenStore)
	cachedValidator := token.NewCachedValidator(tokenValidator)

	var bus eventbus.EventBus
	if s.config.ObservabilityEnabled {
		bus = eventbus.NewInMemoryEventBus(s.config.ObservabilityBufferSize)
	}
	obsCfg := middleware.ObservabilityConfig{Enabled: s.config.ObservabilityEnabled, EventBus: bus}

	proxyHandler, err := proxy.NewTransparentProxyWithLoggerAndObservability(*proxyConfig, cachedValidator, s.projectStore, s.logger, obsCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize proxy: %w", err)
	}
	s.proxy = proxyHandler

	// Register proxy routes
	s.server.Handler.(*http.ServeMux).Handle("/v1/", proxyHandler.Handler())

	log.Printf("Initialized proxy for %s with %d allowed endpoints",
		proxyConfig.TargetBaseURL, len(proxyConfig.AllowedEndpoints))

	return nil
}

// Shutdown gracefully shuts down the server without interrupting
// active connections. It waits for all connections to complete
// or for the provided context to be canceled, whichever comes first.
//
// The context should typically include a timeout to prevent
// the shutdown from blocking indefinitely.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleHealth is the HTTP handler for the health check endpoint.
// It responds with a JSON payload containing the server status,
// current timestamp, and application version.
//
// This endpoint can be used by load balancers, monitoring tools,
// and container orchestration systems to verify service health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   Version,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Error encoding health response: %v\n", err)
		return
	}
	// Status code 200 OK is set implicitly when the response is written successfully
}

// handleReady is used for readiness probes.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// handleLive is used for liveness probes.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("alive"))
}

// handleMetrics returns basic runtime metrics in JSON format.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	m := struct {
		UptimeSeconds float64 `json:"uptime_seconds"`
		RequestCount  int64   `json:"request_count"`
		ErrorCount    int64   `json:"error_count"`
	}{
		UptimeSeconds: time.Since(s.metrics.StartTime).Seconds(),
	}
	if s.proxy != nil {
		pm := s.proxy.Metrics()
		m.RequestCount = pm.RequestCount
		m.ErrorCount = pm.ErrorCount
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(m); err != nil {
		s.logger.Error("Failed to encode metrics", zap.Error(err))
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
	}
}

// managementAuthMiddleware checks the management token in the Authorization header
func (s *Server) managementAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) || len(header) <= len(prefix) {
			http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}
		token := header[len(prefix):]
		if token != s.config.ManagementToken {
			http.Error(w, `{"error":"invalid management token"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// GET /manage/projects
// We only register /manage/projects (no trailing slash) for handleProjects. This ensures that both /manage/projects and /manage/projects/ are handled identically, and only /manage/projects/{id} is handled by handleProjectByID. This avoids ambiguity and double handling in Go's http.ServeMux.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("handleProjects: START", zap.String("method", r.Method), zap.String("path", r.URL.Path))
	// Normalize path: treat /manage/projects/ as /manage/projects
	if r.URL.Path == "/manage/projects/" {
		r.URL.Path = "/manage/projects"
	}
	// DEBUG: Log method and headers
	for k, v := range r.Header {
		if strings.EqualFold(k, "Authorization") {
			s.logger.Debug("handleProjects: header", zap.String("key", k), zap.String("value", "******"))
		} else {
			s.logger.Debug("handleProjects: header", zap.String("key", k), zap.Any("value", v))
		}
	}
	// Mask management token in logs
	maskedToken := "******"
	if len(s.config.ManagementToken) > 4 {
		maskedToken = s.config.ManagementToken[:4] + "******"
	}
	s.logger.Debug("handleProjects: config.ManagementToken", zap.String("ManagementToken", maskedToken))
	if !s.checkManagementAuth(w, r) {
		s.logger.Debug("handleProjects: END (auth failed)")
		return
	}
	ctx := r.Context()
	requestID := getRequestID(ctx)

	switch r.Method {
	case http.MethodGet:
		s.logger.Info("listing projects", zap.String("request_id", requestID))
		s.handleListProjects(w, r.WithContext(ctx))
	case http.MethodPost:
		s.logger.Info("creating project", zap.String("request_id", requestID))
		s.handleCreateProject(w, r.WithContext(ctx))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
	s.logger.Debug("handleProjects: END", zap.String("method", r.Method), zap.String("path", r.URL.Path))
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("handleListProjects: START")
	ctx := r.Context()
	projects, err := s.projectStore.ListProjects(ctx)
	if err != nil {
		s.logger.Error("failed to list projects", zap.Error(err))
		http.Error(w, `{"error":"failed to list projects"}`, http.StatusInternalServerError)
		s.logger.Debug("handleListProjects: END (error)")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(projects); err != nil {
		s.logger.Error("failed to encode projects response", zap.Error(err))
		s.logger.Debug("handleListProjects: END (encode error)")
	} else {
		s.logger.Debug("handleListProjects: END (success)")
	}
}

// POST /manage/projects
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	var req struct {
		Name         string `json:"name"`
		OpenAIAPIKey string `json:"openai_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("invalid request body", zap.Error(err), zap.String("request_id", requestID))
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.OpenAIAPIKey == "" {
		s.logger.Error("missing required fields", zap.String("name", req.Name), zap.String("openai_api_key", req.OpenAIAPIKey), zap.String("request_id", requestID))
		http.Error(w, `{"error":"name and openai_api_key are required"}`, http.StatusBadRequest)
		return
	}
	id := generateUUID()
	now := time.Now().UTC()
	project := proxy.Project{
		ID:           id,
		Name:         req.Name,
		OpenAIAPIKey: req.OpenAIAPIKey,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.projectStore.CreateProject(ctx, project); err != nil {
		s.logger.Error("failed to create project", zap.Error(err), zap.String("name", req.Name), zap.String("request_id", requestID))
		http.Error(w, `{"error":"failed to create project"}`, http.StatusInternalServerError)
		return
	}
	s.logger.Info("project created", zap.String("id", id), zap.String("name", req.Name), zap.String("request_id", requestID))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(project); err != nil {
		s.logger.Error("failed to encode project response", zap.Error(err))
	}
}

// GET /manage/projects/{id}
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := strings.TrimPrefix(r.URL.Path, "/manage/projects/")
	if id == "" || strings.Contains(id, "/") {
		s.logger.Error("invalid project id", zap.String("id", id))
		http.Error(w, `{"error":"invalid project id"}`, http.StatusBadRequest)
		return
	}
	project, err := s.projectStore.GetProjectByID(ctx, id)
	if err != nil {
		s.logger.Error("project not found", zap.String("id", id), zap.Error(err))
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(project); err != nil {
		s.logger.Error("failed to encode project response", zap.Error(err))
	}
}

// PATCH /manage/projects/{id}
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := strings.TrimPrefix(r.URL.Path, "/manage/projects/")
	if id == "" || strings.Contains(id, "/") {
		s.logger.Error("invalid project id for update", zap.String("id", id))
		http.Error(w, `{"error":"invalid project id"}`, http.StatusBadRequest)
		return
	}
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("invalid request body for update", zap.Error(err))
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	project, err := s.projectStore.GetProjectByID(ctx, id)
	if err != nil {
		s.logger.Error("project not found for update", zap.String("id", id), zap.Error(err))
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	if name, ok := req["name"]; ok {
		project.Name = name
	}
	if key, ok := req["openai_api_key"]; ok {
		project.OpenAIAPIKey = key
	}
	project.UpdatedAt = time.Now().UTC()
	if err := s.projectStore.UpdateProject(ctx, project); err != nil {
		s.logger.Error("failed to update project", zap.String("id", id), zap.Error(err))
		http.Error(w, `{"error":"failed to update project"}`, http.StatusInternalServerError)
		return
	}
	s.logger.Info("project updated", zap.String("id", id))
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(project); err != nil {
		s.logger.Error("failed to encode project response", zap.Error(err))
	}
}

// DELETE /manage/projects/{id}
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	id := strings.TrimPrefix(r.URL.Path, "/manage/projects/")
	if id == "" || strings.Contains(id, "/") {
		s.logger.Error("invalid project id for delete", zap.String("id", id), zap.String("request_id", requestID))
		http.Error(w, `{"error":"invalid project id"}`, http.StatusBadRequest)
		return
	}
	if err := s.projectStore.DeleteProject(ctx, id); err != nil {
		s.logger.Error("project not found for delete", zap.String("id", id), zap.Error(err), zap.String("request_id", requestID))
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}
	s.logger.Info("project deleted", zap.String("id", id), zap.String("request_id", requestID))
	w.WriteHeader(http.StatusNoContent)
}

// generateUUID generates a random UUID (v4)
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = time.Now().UTC().MarshalBinary() // for entropy
	for i := range b {
		b[i] = byte(65 + time.Now().UnixNano()%26)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Add this helper to *Server
func (s *Server) checkManagementAuth(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	maskedHeader := header
	if len(header) > 10 {
		maskedHeader = header[:10] + "..."
	}
	s.logger.Debug("checkManagementAuth: header", zap.String("header", maskedHeader))
	if !strings.HasPrefix(header, prefix) || len(header) <= len(prefix) {
		s.logger.Debug("checkManagementAuth: missing or invalid prefix")
		http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
		return false
	}
	token := header[len(prefix):]
	maskedToken := "******"
	if len(s.config.ManagementToken) > 4 {
		maskedToken = s.config.ManagementToken[:4] + "******"
	}
	s.logger.Debug("checkManagementAuth: token compare", zap.String("token", token), zap.String("expected", maskedToken))
	if token != s.config.ManagementToken {
		s.logger.Debug("checkManagementAuth: token mismatch")
		http.Error(w, `{"error":"invalid management token"}`, http.StatusUnauthorized)
		return false
	}
	s.logger.Debug("checkManagementAuth: token match")
	return true
}

// Add the handler function
func (s *Server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetProject(w, r)
	case http.MethodPatch:
		s.handleUpdateProject(w, r)
	case http.MethodDelete:
		s.handleDeleteProject(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Handler for /manage/tokens (POST: create, GET: list)
func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	switch r.Method {
	case http.MethodPost:
		var req struct {
			ProjectID       string `json:"project_id"`
			DurationMinutes int    `json:"duration_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.logger.Error("invalid token create request body", zap.Error(err), zap.String("request_id", requestID))
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		var duration time.Duration
		if req.DurationMinutes > 0 {
			if req.DurationMinutes > maxDurationMinutes {
				s.logger.Error("duration_minutes exceeds maximum allowed", zap.Int("duration_minutes", req.DurationMinutes), zap.String("request_id", requestID))
				http.Error(w, `{"error":"duration_minutes exceeds maximum allowed"}`, http.StatusBadRequest)
				return
			}
			duration = time.Duration(req.DurationMinutes) * time.Minute
		} else {
			s.logger.Error("missing required fields for token create", zap.String("project_id", req.ProjectID), zap.Int("duration_minutes", req.DurationMinutes), zap.String("request_id", requestID))
			http.Error(w, `{"error":"project_id and duration_minutes are required"}`, http.StatusBadRequest)
			return
		}
		if req.ProjectID == "" {
			s.logger.Error("missing project_id for token create", zap.String("request_id", requestID))
			http.Error(w, `{"error":"project_id is required"}`, http.StatusBadRequest)
			return
		}
		// Check project exists
		_, err := s.projectStore.GetProjectByID(ctx, req.ProjectID)
		if err != nil {
			s.logger.Error("project not found for token create", zap.String("project_id", req.ProjectID), zap.Error(err), zap.String("request_id", requestID))
			http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
			return
		}
		// Generate token
		tokenStr, expiresAt, _, err := token.NewTokenGenerator().GenerateWithOptions(duration, nil)
		if err != nil {
			s.logger.Error("failed to generate token", zap.Error(err), zap.String("request_id", requestID))
			http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
			return
		}
		now := time.Now().UTC()
		dbToken := token.TokenData{
			Token:        tokenStr,
			ProjectID:    req.ProjectID,
			ExpiresAt:    expiresAt,
			IsActive:     true,
			RequestCount: 0,
			CreatedAt:    now,
		}
		if err := s.tokenStore.CreateToken(ctx, dbToken); err != nil {
			s.logger.Error("failed to store token", zap.Error(err), zap.String("request_id", requestID))
			http.Error(w, `{"error":"failed to store token"}`, http.StatusInternalServerError)
			return
		}
		s.logger.Info("token created",
			zap.String("token", token.ObfuscateToken(tokenStr)),
			zap.String("project_id", req.ProjectID),
			zap.String("request_id", requestID),
		)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"token":      tokenStr,
			"expires_at": expiresAt,
		}); err != nil {
			s.logger.Error("failed to encode token response", zap.Error(err))
		}
	case http.MethodGet:
		projectID := r.URL.Query().Get("projectId")
		var tokens []token.TokenData
		var err error
		if projectID != "" {
			tokens, err = s.tokenStore.GetTokensByProjectID(ctx, projectID)
		} else {
			tokens, err = s.tokenStore.ListTokens(ctx)
		}
		if err != nil {
			s.logger.Error("failed to list tokens", zap.Error(err))
			http.Error(w, `{"error":"failed to list tokens"}`, http.StatusInternalServerError)
			return
		}
		s.logger.Info("tokens listed", zap.Int("count", len(tokens)))
		w.Header().Set("Content-Type", "application/json")

		// Create sanitized response without actual token values
		sanitizedTokens := make([]TokenListResponse, len(tokens))
		for i, token := range tokens {
			sanitizedTokens[i] = TokenListResponse{
				ProjectID:    token.ProjectID,
				ExpiresAt:    token.ExpiresAt,
				IsActive:     token.IsActive,
				RequestCount: token.RequestCount,
				MaxRequests:  token.MaxRequests,
				CreatedAt:    token.CreatedAt,
				LastUsedAt:   token.LastUsedAt,
			}
		}

		if err := json.NewEncoder(w).Encode(sanitizedTokens); err != nil {
			s.logger.Error("failed to encode tokens response", zap.Error(err))
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func getRequestID(ctx context.Context) string {
	if v := ctx.Value(ctxKeyRequestID); v != nil {
		if id, ok := v.(string); ok && id != "" {
			return id
		}
	}
	return uuid.New().String()
}

// logRequestMiddleware logs all incoming requests with timing information
func (s *Server) logRequestMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, requestID)

		// Create a response writer that captures status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		s.logger.Info("request started",
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
		)

		// Call the next handler
		next(rw, r.WithContext(ctx))

		duration := time.Since(startTime)
		s.logger.Info("request completed",
			zap.String("request_id", requestID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status_code", rw.statusCode),
			zap.Duration("duration", duration),
		)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Add Flush forwarding for streaming support
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// handleNotFound is a catch-all handler for unmatched routes
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("route not found",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("remote_addr", r.RemoteAddr),
	)
	http.NotFound(w, r)
}
