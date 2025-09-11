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
	// GetProjectActive checks if a project is active
	GetProjectActive(ctx context.Context, projectID string) (bool, error)
	// Management API CRUD
	ListProjects(ctx context.Context) ([]Project, error)
	CreateProject(ctx context.Context, project Project) error
	GetProjectByID(ctx context.Context, projectID string) (Project, error)
	UpdateProject(ctx context.Context, project Project) error
	DeleteProject(ctx context.Context, projectID string) error
}

// ProjectActiveChecker defines the interface for checking project active status
type ProjectActiveChecker interface {
	// GetProjectActive checks if a project is active
	GetProjectActive(ctx context.Context, projectID string) (bool, error)
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

	// Project active guard configuration
	EnforceProjectActive bool // Whether to enforce project active status

	// --- HTTP cache (global, opt-in; set programmatically, not via YAML) ---
	// HTTPCacheEnabled toggles the proxy cache for GET/HEAD based on HTTP semantics
	HTTPCacheEnabled bool
	// HTTPCacheDefaultTTL is used only when upstream allows caching but omits explicit TTL
	HTTPCacheDefaultTTL time.Duration
	// HTTPCacheMaxObjectBytes is a guardrail for maximum cacheable response size
	HTTPCacheMaxObjectBytes int64

	// RedisCacheURL enables Redis-backed cache when non-empty (e.g., redis://localhost:6379/0)
	RedisCacheURL string
	// RedisCacheKeyPrefix allows namespacing cache keys (default: llmproxy:cache:)
	RedisCacheKeyPrefix string
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
	// ctxKeyRequestID is the context key for the request ID
	ctxKeyRequestID contextKey = "request_id"
	// ctxKeyProjectID is the context key for the project ID
	ctxKeyProjectID contextKey = "project_id"
	// ctxKeyLogger is the context key for a request-scoped logger
	ctxKeyLogger contextKey = "logger"
	// ctxKeyOriginalPath stores the original request path before proxy rewriting
	ctxKeyOriginalPath contextKey = "original_path"
	// ctxKeyValidationError carries token validation error (if any)
	ctxKeyValidationError contextKey = "validation_error"
	// Timing keys for observability
	ctxKeyProxyReceivedAt    contextKey = "proxy_received_at"
	ctxKeyProxySentBackendAt contextKey = "proxy_sent_backend_at"
	ctxKeyProxyFirstRespAt   contextKey = "proxy_first_resp_at"
	ctxKeyProxyFinalRespAt   contextKey = "proxy_final_resp_at"
	// ctxKeyRequestStart marks the time when a handler started processing
	ctxKeyRequestStart contextKey = "request_start"
)

// Project represents a project for the management API and proxy
// (copied from database/models.go)
type Project struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	OpenAIAPIKey  string     `json:"openai_api_key"`
	IsActive      bool       `json:"is_active"`
	DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
