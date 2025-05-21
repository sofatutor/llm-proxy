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
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

// Server represents the HTTP server for the LLM Proxy.
// It encapsulates the underlying http.Server along with application configuration
// and handles request routing and server lifecycle management.
type Server struct {
	server       *http.Server
	config       *config.Config
	tokenStore   token.TokenStore
	projectStore proxy.ProjectStore
	logger       *zap.Logger
}

// HealthResponse is the response body for the health check endpoint.
// It provides basic information about the server status and version.
type HealthResponse struct {
	Status    string    `json:"status"`    // Service status, "ok" for a healthy system
	Timestamp time.Time `json:"timestamp"` // Current server time
	Version   string    `json:"version"`   // Application version number
}

// Version is the application version, following semantic versioning.
const Version = "0.1.0"

// New creates a new HTTP server with the provided configuration and store implementations.
// It initializes the server with appropriate timeouts and registers all necessary route handlers.
// The server is not started until the Start method is called.
func New(cfg *config.Config, tokenStore token.TokenStore, projectStore proxy.ProjectStore) (*Server, error) {
	mux := http.NewServeMux()

	logger, err := logging.NewLogger(cfg.LogLevel, cfg.LogFormat, cfg.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	s := &Server{
		config:       cfg,
		tokenStore:   tokenStore,
		projectStore: projectStore,
		logger:       logger,
		server: &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      mux,
			ReadTimeout:  cfg.RequestTimeout,
			WriteTimeout: cfg.RequestTimeout,
			IdleTimeout:  cfg.RequestTimeout * 2,
		},
	}

	// Register routes
	mux.HandleFunc("/health", s.handleHealth)

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
	proxyHandler, err := proxy.NewTransparentProxyWithLogger(*proxyConfig, cachedValidator, s.projectStore, s.logger)
	if err != nil {
		return fmt.Errorf("failed to initialize proxy: %w", err)
	}

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
