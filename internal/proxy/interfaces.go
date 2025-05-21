package proxy

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// TokenValidator defines the interface for token validation
type TokenValidator interface {
	// ValidateToken validates a token and returns the associated project ID
	ValidateToken(ctx context.Context, token string) (string, error)

	// ValidateTokenWithTracking validates a token, increments its usage, and returns the project ID
	ValidateTokenWithTracking(ctx context.Context, token string) (string, error)
}

// ProjectStore defines the interface for retrieving project information
type ProjectStore interface {
	// GetAPIKeyForProject retrieves the API key for a project
	GetAPIKeyForProject(ctx context.Context, projectID string) (string, error)
}

// Proxy defines the interface for a transparent HTTP proxy
type Proxy interface {
	// Handler returns an http.Handler for the proxy
	Handler() http.Handler

	// Shutdown gracefully shuts down the proxy
	Shutdown(ctx context.Context) error
}

// ProxyConfig contains configuration for the proxy
type ProxyConfig struct {
	// TargetBaseURL is the base URL of the API to proxy to
	TargetBaseURL string

	// AllowedEndpoints is a whitelist of endpoints that can be accessed
	AllowedEndpoints []string

	// AllowedMethods is a whitelist of HTTP methods that can be used
	AllowedMethods []string

	// RequestTimeout is the maximum duration for a complete request
	RequestTimeout time.Duration

	// ResponseHeaderTimeout is the time to wait for response headers
	ResponseHeaderTimeout time.Duration

	// FlushInterval is how often to flush streaming responses
	FlushInterval time.Duration

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int

	// MaxIdleConnsPerHost is the maximum number of idle connections per host
	MaxIdleConnsPerHost int

	// IdleConnTimeout is how long to keep idle connections alive
	IdleConnTimeout time.Duration

	// LogLevel controls the verbosity of logging
	LogLevel string
}

// Validate checks that the ProxyConfig is valid and returns an error if not.
func (c *ProxyConfig) Validate() error {
	if c.TargetBaseURL == "" {
		return errors.New("TargetBaseURL must not be empty")
	}
	if len(c.AllowedMethods) == 0 {
		return errors.New("AllowedMethods must not be empty")
	}
	if len(c.AllowedEndpoints) == 0 {
		return errors.New("AllowedEndpoints must not be empty")
	}
	return nil
}

// ErrorResponse is the standard format for error responses
type ErrorResponse struct {
	Error       string `json:"error"`
	Description string `json:"description,omitempty"`
	Code        string `json:"code,omitempty"`
}

// Middleware defines a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// Chain applies a series of middleware to a handler
func Chain(h http.Handler, middleware ...Middleware) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}

// contextKey is a type for context keys
type contextKey string

const (
	// ctxKeyValidationError is the context key for validation errors
	ctxKeyValidationError contextKey = "validation_error"

	// ctxKeyRequestStart is the context key for request start time
	ctxKeyRequestStart contextKey = "request_start"

	// ctxKeyOriginalPath is the context key for the original request path
	ctxKeyOriginalPath contextKey = "original_path"
)
