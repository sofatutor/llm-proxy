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

// ProjectStore defines the interface for retrieving and managing project information
// (extended for management API)
type ProjectStore interface {
	// GetAPIKeyForProject retrieves the API key for a project
	GetAPIKeyForProject(ctx context.Context, projectID string) (string, error)
	// Management API CRUD
	ListProjects(ctx context.Context) ([]Project, error)
	CreateProject(ctx context.Context, project Project) error
	GetProjectByID(ctx context.Context, projectID string) (Project, error)
	UpdateProject(ctx context.Context, project Project) error
	DeleteProject(ctx context.Context, projectID string) error
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
	// LogFormat controls the log output format (json or console)
	LogFormat string
	// LogFile specifies a file path for logs (stdout if empty)
	LogFile string

	// SetXForwardedFor determines whether to set the X-Forwarded-For header
	SetXForwardedFor bool

	// ParamWhitelist is a map of parameter names to allowed values
	ParamWhitelist map[string][]string

	// AllowedOrigins is a list of allowed CORS origins for this provider
	AllowedOrigins []string

	// RequiredHeaders is a list of required request headers (case-insensitive)
	RequiredHeaders []string
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

// version is the current version of the proxy
const version = "0.1.0"

const (
	// ctxKeyValidationError is the context key for validation errors
	ctxKeyValidationError contextKey = "validation_error"

	// ctxKeyRequestStart is the context key for request start time
	ctxKeyRequestStart contextKey = "request_start"

	// ctxKeyOriginalPath is the context key for the original request path
	ctxKeyOriginalPath contextKey = "original_path"

	// ctxKeyProjectID is the context key for the project ID
	ctxKeyProjectID contextKey = "project_id"
)

// Project represents a project for the management API and proxy
// (copied from database/models.go)
type Project struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	OpenAIAPIKey string    `json:"openai_api_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
