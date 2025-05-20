// Package server implements the HTTP server
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
)

// Server represents the HTTP server
type Server struct {
	server *http.Server
	config *config.Config
}

// HealthResponse is the response body for the health check endpoint
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

// Version is the application version
const Version = "0.1.0"

// New creates a new HTTP server
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

// Start starts the HTTP server
func (s *Server) Start() error {
	// TODO: Initialize required components
	// - Database connection
	// - Logging
	// - Proxy routes
	// - Admin routes
	// - Metrics

	fmt.Printf("Server starting on %s\n", s.config.ListenAddr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// handleHealth is the health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}