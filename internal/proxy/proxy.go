package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// Store project ID in context for logging and billing
	*req = *req.WithContext(context.WithValue(req.Context(), ctxKeyProjectID, projectID))

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
	req.Header.Set("X-Proxy-Version", version)
	req.Header.Set("X-Proxy-ID", projectID)

	// Preserve or strip certain headers
	p.processRequestHeaders(req)

	p.logger.Debug("Proxying request",
		zap.String("method", req.Method),
		zap.String("path", req.URL.Path),
		zap.String("project_id", projectID))
}

// processRequestHeaders handles the manipulation of request headers
func (p *TransparentProxy) processRequestHeaders(req *http.Request) {
	// Headers to remove for security/privacy reasons
	headersToRemove := []string{
		"X-Forwarded-For",          // We'll set this ourselves if needed
		"X-Real-IP",                // Remove client IP for privacy
		"CF-Connecting-IP",         // Cloudflare headers
		"CF-IPCountry",             // Cloudflare headers
		"X-Client-IP",              // Other proxies
		"X-Original-Forwarded-For", // Chain of proxies
	}

	// Remove headers that shouldn't be passed to the upstream
	for _, header := range headersToRemove {
		req.Header.Del(header)
	}

	// Set X-Forwarded-For if configured to do so
	if p.config.SetXForwardedFor {
		// Get the client IP
		clientIP := req.RemoteAddr
		// Remove port if present
		if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
			clientIP = clientIP[:idx]
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}

	// If Content-Length is 0 and there's a body, let Go calculate the correct Content-Length
	if req.ContentLength == 0 && req.Body != nil {
		req.Header.Del("Content-Length")
	}

	// Ensure proper Accept header for SSE streaming if needed
	if isStreamingRequest(req) && req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/event-stream")
	}
}

// modifyResponse processes the response before returning it to the client
func (p *TransparentProxy) modifyResponse(res *http.Response) error {
	// Add proxy headers
	res.Header.Set("X-Proxy", "llm-proxy")
	res.Header.Set("X-Proxy-Version", version)

	// Get request from response
	req := res.Request
	if req == nil {
		p.logger.Warn("Response has no request")
		return nil
	}

	// Get project ID from context
	projectID, _ := req.Context().Value(ctxKeyProjectID).(string)
	if projectID != "" {
		res.Header.Set("X-Proxy-ID", projectID)
	}

	// For streaming responses, we just pass through
	if isStreaming(res) {
		return nil
	}

	// Process response body to extract metadata for non-streaming responses
	if res.StatusCode == http.StatusOK &&
		strings.Contains(res.Header.Get("Content-Type"), "application/json") &&
		res.Body != nil {
		// Extract metadata without consuming the response body
		if err := p.extractResponseMetadata(res); err != nil {
			p.logger.Warn("Failed to extract response metadata", zap.Error(err))
		}
	}

	// Update metrics
	p.metrics.mu.Lock()
	p.metrics.RequestCount++
	if res.StatusCode >= 400 {
		p.metrics.ErrorCount++
	}
	p.metrics.mu.Unlock()

	return nil
}

// extractResponseMetadata extracts metadata from the response body without consuming it
func (p *TransparentProxy) extractResponseMetadata(res *http.Response) error {
	// Check if we need to process the response
	if res.Body == nil {
		return errors.New("response body is nil")
	}

	// We need to read the body to extract metadata, but we must also
	// preserve it for the client. This is done by creating a new Reader
	// that allows us to read the body twice.
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Replace the body with a new ReadCloser that can be read again
	err = res.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to close response body: %w", err)
	}
	res.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Parse the body to extract metadata
	metadata, err := p.parseOpenAIResponseMetadata(bodyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse response metadata: %w", err)
	}

	// Add metadata to response headers
	for k, v := range metadata {
		res.Header.Set(fmt.Sprintf("X-OpenAI-%s", k), v)
	}

	return nil
}

// parseOpenAIResponseMetadata extracts metadata from OpenAI API responses
func (p *TransparentProxy) parseOpenAIResponseMetadata(bodyBytes []byte) (map[string]string, error) {
	metadata := make(map[string]string)

	// Try to parse as JSON
	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return metadata, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	// Look for usage information
	if usage, ok := result["usage"].(map[string]interface{}); ok {
		// Extract token counts
		if promptTokens, ok := usage["prompt_tokens"].(float64); ok {
			metadata["Prompt-Tokens"] = fmt.Sprintf("%.0f", promptTokens)
		}
		if completionTokens, ok := usage["completion_tokens"].(float64); ok {
			metadata["Completion-Tokens"] = fmt.Sprintf("%.0f", completionTokens)
		}
		if totalTokens, ok := usage["total_tokens"].(float64); ok {
			metadata["Total-Tokens"] = fmt.Sprintf("%.0f", totalTokens)
		}
	}

	// Extract model information
	if model, ok := result["model"].(string); ok {
		metadata["Model"] = model
	}

	// Extract other potentially useful metadata
	if id, ok := result["id"].(string); ok {
		metadata["ID"] = id
	}
	if created, ok := result["created"].(float64); ok {
		metadata["Created"] = fmt.Sprintf("%.0f", created)
	}

	return metadata, nil
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
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		p.logger.Error("Failed to encode error response", zap.Error(err))
	}
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
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		p.logger.Error("Failed to encode error response", zap.Error(err))
	}
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
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Error: "Method not allowed",
					Code:  "method_not_allowed",
				}); err != nil {
					p.logger.Error("Failed to encode error response", zap.Error(err))
				}
				return
			}

			// Check if endpoint is allowed
			if !p.isEndpointAllowed(r.URL.Path) {
				p.logger.Warn("Endpoint not allowed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path))
				w.WriteHeader(http.StatusNotFound)
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Error: "Endpoint not found",
					Code:  "endpoint_not_found",
				}); err != nil {
					p.logger.Error("Failed to encode error response", zap.Error(err))
				}
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

// isStreamingRequest checks if a request is intended for streaming
func isStreamingRequest(req *http.Request) bool {
	// Check for SSE Accept header
	if strings.Contains(req.Header.Get("Accept"), "text/event-stream") {
		return true
	}

	// Check query parameters for stream=true (common in OpenAI APIs)
	if req.URL.Query().Get("stream") == "true" {
		return true
	}

	// Check the request body for streaming flag
	// This is a heuristic and may need refinement for specific APIs
	// For OpenAI, the common pattern is POST with JSON containing "stream": true
	// But checking this would require reading the body, which we want to avoid
	// We'll just rely on the Accept header and query params for now

	return false
}
