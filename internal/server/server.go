// Package server implements the HTTP server for the LLM Proxy.
// It handles request routing, lifecycle management, and provides
// health check endpoints and core API functionality.
package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
)

// Server represents the HTTP server for the LLM Proxy.
// It encapsulates the underlying http.Server along with application configuration
// and handles request routing and server lifecycle management.
type Server struct {
	server *http.Server
	config *config.Config
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

// New creates a new HTTP server with the provided configuration.
// It initializes the server with appropriate timeouts and registers
// all necessary route handlers.
//
// The server is not started until the Start method is called.
func New(cfg *config.Config) *Server {
	mux := http.NewServeMux()

	s := &Server{
		config: cfg,
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

	return s
}

// Start initializes all required components and starts the HTTP server.
// This method blocks until the server is shut down or an error occurs.
//
// It returns an error if the server fails to start or encounters an
// unrecoverable error during operation.
func (s *Server) Start() error {
	// TODO: Initialize required components
	// - Database connection
	// - Logging
	// - Proxy routes
	// - Admin routes
	// - Metrics

	log.Printf("Server starting on %s\n", s.config.ListenAddr)
	
	return s.server.ListenAndServe()
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