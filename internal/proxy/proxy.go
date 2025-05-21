package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

// TransparentProxy implements the Proxy interface for transparent proxying
type TransparentProxy struct {
	config         ProxyConfig
	tokenValidator TokenValidator
	projectStore   ProjectStore
	logger         *zap.Logger
	metrics        *ProxyMetrics
	proxy          *httputil.ReverseProxy
	httpServer     *http.Server
	shuttingDown   bool
	mu             sync.RWMutex
}

// ProxyMetrics tracks proxy usage statistics
type ProxyMetrics struct {
	RequestCount      int64
	ErrorCount        int64
	TotalResponseTime time.Duration
	mu                sync.Mutex
}

// NewTransparentProxy creates a new proxy instance
func NewTransparentProxy(config ProxyConfig, validator TokenValidator, store ProjectStore) *TransparentProxy {
	// Initialize logger
	logger, _ := zap.NewProduction()
	if config.LogLevel == "debug" {
		logger, _ = zap.NewDevelopment()
	}

	proxy := &TransparentProxy{
		config:         config,
		tokenValidator: validator,
		projectStore:   store,
		logger:         logger,
		metrics:        &ProxyMetrics{},
	}

	// Initialize the reverse proxy
	targetURL, err := url.Parse(config.TargetBaseURL)
	if err != nil {
		logger.Fatal("Invalid target URL", zap.Error(err))
	}

	reverseProxy := &httputil.ReverseProxy{
		Director:       proxy.director,
		ModifyResponse: proxy.modifyResponse,
		ErrorHandler:   proxy.errorHandler,
		Transport:      proxy.createTransport(),
		FlushInterval:  config.FlushInterval,
	}

	proxy.proxy = reverseProxy

	return proxy
}

// director is the Director function for the reverse proxy
func (p *TransparentProxy) director(req *http.Request) {
	// Store original path in context for logging
	*req = *req.WithContext(context.WithValue(req.Context(), ctxKeyOriginalPath, req.URL.Path))

	targetURL, err := url.Parse(p.config.TargetBaseURL)
	if err != nil {
		p.logger.Error("Failed to parse target URL", zap.Error(err))
		return
	}

	// Update request URL
	req.URL.Scheme = targetURL.Scheme
	req.URL.Host = targetURL.Host
	req.Host = targetURL.Host

	// Keep original path - this is a transparent proxy

	// Extract token from Authorization header
	authHeader := req.Header.Get("Authorization")
	token := extractTokenFromHeader(authHeader)
	if token == "" {
		*req = *req.WithContext(context.WithValue(req.Context(), 
			ctxKeyValidationError, errors.New("missing or invalid authorization header")))
		return
	}

	// Validate token with tracking (increments usage)
	projectID, err := p.tokenValidator.ValidateTokenWithTracking(req.Context(), token)
	if err != nil {
		*req = *req.WithContext(context.WithValue(req.Context(), 
			ctxKeyValidationError, err))
		return
	}

	// Get API key for project
	apiKey, err := p.projectStore.GetAPIKeyForProject(req.Context(), projectID)
	if err != nil {
		*req = *req.WithContext(context.WithValue(req.Context(), 
			ctxKeyValidationError, fmt.Errorf("failed to get API key: %w", err)))
		return
	}

	// Replace Authorization header with API key
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// Add proxy identification headers
	req.Header.Set("X-Proxy", "llm-proxy")

	p.logger.Debug("Proxying request",
		zap.String("method", req.Method),
		zap.String("path", req.URL.Path),
		zap.String("project_id", projectID))
}

// modifyResponse processes the response before returning it to the client
func (p *TransparentProxy) modifyResponse(res *http.Response) error {
	// Add proxy headers
	res.Header.Set("X-Proxy", "llm-proxy")

	// For streaming responses, we just pass through
	if isStreaming(res) {
		return nil
	}

	// Update metrics
	p.metrics.mu.Lock()
	p.metrics.RequestCount++
	if res.StatusCode >= 400 {
		p.metrics.ErrorCount++
	}
	p.metrics.mu.Unlock()

	// For regular responses, try to extract metadata if possible
	if strings.Contains(res.Header.Get("Content-Type"), "application/json") {
		// For metadata extraction in a real implementation, we would
		// read the body, extract metadata, then restore it for the client
		// This would be API-specific, so we don't implement it in this generic proxy
	}

	return nil
}

// errorHandler handles errors that occur during proxying
func (p *TransparentProxy) errorHandler(w http.ResponseWriter, r *http.Request, err error) {
	// Check if there was a validation error
	if validationErr, ok := r.Context().Value(ctxKeyValidationError).(error); ok {
		p.handleValidationError(w, validationErr)
		return
	}

	// Handle different error types
	p.logger.Error("Proxy error", 
		zap.Error(err),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path))

	statusCode := http.StatusBadGateway
	errorResponse := ErrorResponse{
		Error: "Proxy error",
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		statusCode = http.StatusGatewayTimeout
		errorResponse.Error = "Request timeout"
		errorResponse.Code = "timeout"

	case errors.Is(err, context.Canceled):
		statusCode = http.StatusRequestTimeout
		errorResponse.Error = "Request canceled"
		errorResponse.Code = "canceled"

	default:
		// Use default values
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}

// handleValidationError handles errors specific to token validation
func (p *TransparentProxy) handleValidationError(w http.ResponseWriter, err error) {
	statusCode := http.StatusUnauthorized
	errorResponse := ErrorResponse{
		Error: "Authentication error",
	}

	// Check for specific token errors
	switch {
	case errors.Is(err, token.ErrTokenNotFound):
		errorResponse.Error = "Token not found"
		errorResponse.Code = "token_not_found"

	case errors.Is(err, token.ErrTokenInactive):
		errorResponse.Error = "Token is inactive"
		errorResponse.Code = "token_inactive"

	case errors.Is(err, token.ErrTokenExpired):
		errorResponse.Error = "Token has expired"
		errorResponse.Code = "token_expired"

	case errors.Is(err, token.ErrTokenRateLimit):
		statusCode = http.StatusTooManyRequests
		errorResponse.Error = "Rate limit exceeded"
		errorResponse.Code = "rate_limit_exceeded"

	default:
		errorResponse.Error = "Invalid token"
		errorResponse.Description = err.Error()
		errorResponse.Code = "invalid_token"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(errorResponse)
}

// createTransport creates an HTTP transport with appropriate settings
func (p *TransparentProxy) createTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          p.config.MaxIdleConns,
		MaxIdleConnsPerHost:   p.config.MaxIdleConnsPerHost,
		IdleConnTimeout:       p.config.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: p.config.ResponseHeaderTimeout,
	}
}

// Handler returns the HTTP handler for the proxy
func (p *TransparentProxy) Handler() http.Handler {
	// Apply middleware chain
	return Chain(p.proxy,
		p.LoggingMiddleware(),
		p.ValidateRequestMiddleware(),
		p.TimeoutMiddleware(p.config.RequestTimeout),
		p.MetricsMiddleware(),
	)
}

// LoggingMiddleware logs request details
func (p *TransparentProxy) LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			p.logger.Info("Request started",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr))

			// Create a response recorder to capture response details
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Process request
			next.ServeHTTP(rec, r)

			// Log request completion
			duration := time.Since(start)
			p.logger.Info("Request completed",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rec.statusCode),
				zap.Duration("duration", duration))
		})
	}
}

// ValidateRequestMiddleware validates the incoming request against allowed endpoints and methods
func (p *TransparentProxy) ValidateRequestMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if method is allowed
			if !p.isMethodAllowed(r.Method) {
				p.logger.Warn("Method not allowed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path))
				w.WriteHeader(http.StatusMethodNotAllowed)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: "Method not allowed",
					Code:  "method_not_allowed",
				})
				return
			}

			// Check if endpoint is allowed
			if !p.isEndpointAllowed(r.URL.Path) {
				p.logger.Warn("Endpoint not allowed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path))
				w.WriteHeader(http.StatusNotFound)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(ErrorResponse{
					Error: "Endpoint not found",
					Code:  "endpoint_not_found",
				})
				return
			}

			// Continue to next middleware
			next.ServeHTTP(w, r)
		})
	}
}

// TimeoutMiddleware adds a timeout to requests
func (p *TransparentProxy) TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MetricsMiddleware collects metrics about requests
func (p *TransparentProxy) MetricsMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Store start time in context
			ctx := context.WithValue(r.Context(), ctxKeyRequestStart, start)
			
			// Create response recorder to capture status code
			rec := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			
			// Process request
			next.ServeHTTP(rec, r.WithContext(ctx))
			
			// Record metrics
			duration := time.Since(start)
			p.metrics.mu.Lock()
			p.metrics.TotalResponseTime += duration
			p.metrics.mu.Unlock()
		})
	}
}

// Shutdown gracefully shuts down the proxy
func (p *TransparentProxy) Shutdown(ctx context.Context) error {
	p.mu.Lock()
	p.shuttingDown = true
	p.mu.Unlock()

	p.logger.Info("Shutting down proxy")
	
	// If we have an HTTP server, shut it down
	if p.httpServer != nil {
		return p.httpServer.Shutdown(ctx)
	}
	
	return nil
}

// isMethodAllowed checks if a method is in the allowed list
func (p *TransparentProxy) isMethodAllowed(method string) bool {
	// If no allowed methods are specified, allow all methods
	if len(p.config.AllowedMethods) == 0 {
		return true
	}

	for _, allowed := range p.config.AllowedMethods {
		if strings.EqualFold(method, allowed) {
			return true
		}
	}

	return false
}

// isEndpointAllowed checks if an endpoint is in the allowed list
func (p *TransparentProxy) isEndpointAllowed(path string) bool {
	// If no allowed endpoints are specified, allow all endpoints
	if len(p.config.AllowedEndpoints) == 0 {
		return true
	}

	// Check if path matches any allowed endpoint
	for _, endpoint := range p.config.AllowedEndpoints {
		if strings.HasPrefix(path, endpoint) {
			return true
		}
	}

	return false
}

// responseRecorder is a wrapper for http.ResponseWriter that records the status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader records the status code and calls the wrapped ResponseWriter's WriteHeader method
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// extractTokenFromHeader extracts a token from an authorization header
func extractTokenFromHeader(authHeader string) string {
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

// isStreaming checks if a response is a streaming response
func isStreaming(res *http.Response) bool {
	// Check Content-Type for SSE
	if strings.Contains(res.Header.Get("Content-Type"), "text/event-stream") {
		return true
	}

	// Check for chunked transfer encoding
	return strings.Contains(
		strings.ToLower(res.Header.Get("Transfer-Encoding")), 
		"chunked",
	)
}