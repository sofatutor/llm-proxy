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

	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

// Add context keys for timing
type ctxKey string

const (
	ctxKeyProxyReceivedAt    ctxKey = "proxy_received_at"
	ctxKeyProxySentBackendAt ctxKey = "proxy_sent_backend_at"
	ctxKeyProxyFirstRespAt   ctxKey = "proxy_first_response_at"
	ctxKeyProxyFinalRespAt   ctxKey = "proxy_final_response_at"
	ctxKeyRequestID          ctxKey = "request_id"
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

// NewTransparentProxy creates a new proxy instance with an internally
// configured logger based on the provided ProxyConfig.
func NewTransparentProxy(config ProxyConfig, validator TokenValidator, store ProjectStore) (*TransparentProxy, error) {
	logger, err := logging.NewLogger(config.LogLevel, config.LogFormat, config.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return NewTransparentProxyWithLogger(config, validator, store, logger)
}

// NewTransparentProxyWithLogger allows providing a custom logger. If logger is nil
// a new one is created based on the ProxyConfig.
func NewTransparentProxyWithLogger(config ProxyConfig, validator TokenValidator, store ProjectStore, logger *zap.Logger) (*TransparentProxy, error) {
	if logger == nil {
		var err error
		logger, err = logging.NewLogger(config.LogLevel, config.LogFormat, config.LogFile)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize logger: %w", err)
		}
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

	return proxy, nil
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

	// Add proxy identification headers
	req.Header.Set("X-Proxy", "llm-proxy")
	req.Header.Set("X-Proxy-Version", version)
	if pid, ok := req.Context().Value(ctxKeyProjectID).(string); ok {
		req.Header.Set("X-Proxy-ID", pid)
	}

	// Preserve or strip certain headers
	p.processRequestHeaders(req)

	requestID, _ := req.Context().Value(ctxKeyRequestID).(string)
	p.logger.Debug("Proxying request",
		zap.String("request_id", requestID),
		zap.String("method", req.Method),
		zap.String("path", req.URL.Path),
		zap.String("project_id", req.Header.Get("X-Proxy-ID")))
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

	// Calculate and add response time header
	startTime, ok := req.Context().Value(ctxKeyRequestStart).(time.Time)
	if ok {
		remoteCallDuration := time.Since(startTime)
		res.Header.Set("X-LLM-Proxy-Remote-Duration", remoteCallDuration.String())
		res.Header.Set("X-LLM-Proxy-Remote-Duration-Ms", fmt.Sprintf("%.2f", float64(remoteCallDuration.Milliseconds())))
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

	// Only parse as JSON if Content-Type is application/json and not compressed
	contentType := res.Header.Get("Content-Type")
	contentEncoding := res.Header.Get("Content-Encoding")
	if !strings.Contains(contentType, "application/json") || (contentEncoding != "" && contentEncoding != "identity") {
		p.logger.Debug("Skipping metadata extraction: not JSON or compressed",
			zap.String("content_type", contentType),
			zap.String("content_encoding", contentEncoding))
		return nil
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
		p.logger.Debug("Failed to extract response metadata",
			zap.Error(err),
			zap.String("content_type", contentType),
			zap.String("content_encoding", contentEncoding))
		return nil
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
	requestID, _ := r.Context().Value(ctxKeyRequestID).(string)
	p.logger.Error("Proxy error",
		zap.String("request_id", requestID),
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
	// Try to get request ID from context if available
	var requestID string
	if w != nil {
		if req, ok := w.(interface{ Request() *http.Request }); ok && req.Request() != nil {
			requestID, _ = req.Request().Context().Value(ctxKeyRequestID).(string)
		}
	}

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

	p.logger.Error("Validation error",
		zap.String("request_id", requestID),
		zap.Error(err))
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
	return p.ValidateRequestMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate request ID and add to context
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, requestID)
		r = r.WithContext(ctx)

		// Set X-Request-ID header
		w.Header().Set("X-Request-ID", requestID)

		// Record when proxy receives the request
		receivedAt := time.Now().UTC()
		ctx = context.WithValue(ctx, ctxKeyProxyReceivedAt, receivedAt)
		r = r.WithContext(ctx)

		// --- Token extraction and validation (moved from director) ---
		authHeader := r.Header.Get("Authorization")
		tokenStr := extractTokenFromHeader(authHeader)
		if tokenStr == "" {
			p.handleValidationError(w, errors.New("missing or invalid authorization header"))
			return
		}
		projectID, err := p.tokenValidator.ValidateTokenWithTracking(r.Context(), tokenStr)
		if err != nil {
			p.handleValidationError(w, err)
			return
		}
		ctx = context.WithValue(r.Context(), ctxKeyProjectID, projectID)
		r = r.WithContext(ctx)
		apiKey, err := p.projectStore.GetAPIKeyForProject(r.Context(), projectID)
		if err != nil {
			p.handleValidationError(w, fmt.Errorf("failed to get API key: %w", err))
			return
		}
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

		// Wrap the ResponseWriter to allow us to set headers at first/last byte
		rw := &timingResponseWriter{ResponseWriter: w}

		// Instrument the reverse proxy (director now only rewrites URL/host)
		p.proxy.Director = func(req *http.Request) {
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
			// Add proxy identification headers
			req.Header.Set("X-Proxy", "llm-proxy")
			req.Header.Set("X-Proxy-Version", version)
			if pid, ok := req.Context().Value(ctxKeyProjectID).(string); ok {
				req.Header.Set("X-Proxy-ID", pid)
			}
			// Preserve or strip certain headers
			p.processRequestHeaders(req)
			requestID, _ := req.Context().Value(ctxKeyRequestID).(string)
			p.logger.Debug("Proxying request",
				zap.String("request_id", requestID),
				zap.String("method", req.Method),
				zap.String("path", req.URL.Path),
				zap.String("project_id", req.Header.Get("X-Proxy-ID")))
		}

		p.proxy.ModifyResponse = func(res *http.Response) error {
			firstRespAt := time.Now().UTC()
			ctx := context.WithValue(res.Request.Context(), ctxKeyProxyFirstRespAt, firstRespAt)
			res.Request = res.Request.WithContext(ctx)
			ctx = context.WithValue(res.Request.Context(), ctxKeyProxyFinalRespAt, firstRespAt)
			res.Request = res.Request.WithContext(ctx)
			setTimingHeaders(res, res.Request.Context())
			requestID, _ := res.Request.Context().Value(ctxKeyRequestID).(string)
			if requestID != "" {
				res.Header.Set("X-Request-ID", requestID)
			}
			logProxyTimings(p.logger, res.Request.Context())
			return p.modifyResponse(res)
		}

		p.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			logProxyTimings(p.logger, r.Context())
			p.errorHandler(w, r, err)
		}

		p.proxy.ServeHTTP(rw, r)
	}))
}

type timingResponseWriter struct {
	http.ResponseWriter
	firstByteOnce sync.Once
	firstByteAt   time.Time
	finalByteAt   time.Time
}

func (w *timingResponseWriter) Write(b []byte) (int, error) {
	now := time.Now().UTC()
	w.firstByteOnce.Do(func() {
		w.firstByteAt = now
		w.Header().Set("X-Proxy-First-Response-At", w.firstByteAt.Format(time.RFC3339Nano))
	})
	w.finalByteAt = now
	return w.ResponseWriter.Write(b)
}

func (w *timingResponseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func setTimingHeaders(res *http.Response, ctx context.Context) {
	if v := ctx.Value(ctxKeyProxyReceivedAt); v != nil {
		if t, ok := v.(time.Time); ok {
			res.Header.Set("X-Proxy-Received-At", t.Format(time.RFC3339Nano))
		}
	}
	if v := ctx.Value(ctxKeyProxySentBackendAt); v != nil {
		if t, ok := v.(time.Time); ok {
			res.Header.Set("X-Proxy-Sent-Backend-At", t.Format(time.RFC3339Nano))
		}
	}
	if v := ctx.Value(ctxKeyProxyFirstRespAt); v != nil {
		if t, ok := v.(time.Time); ok {
			res.Header.Set("X-Proxy-First-Response-At", t.Format(time.RFC3339Nano))
		}
	}
	if v := ctx.Value(ctxKeyProxyFinalRespAt); v != nil {
		if t, ok := v.(time.Time); ok {
			res.Header.Set("X-Proxy-Final-Response-At", t.Format(time.RFC3339Nano))
		}
	}
}

func logProxyTimings(logger *zap.Logger, ctx context.Context) {
	received, _ := ctx.Value(ctxKeyProxyReceivedAt).(time.Time)
	sent, _ := ctx.Value(ctxKeyProxySentBackendAt).(time.Time)
	first, _ := ctx.Value(ctxKeyProxyFirstRespAt).(time.Time)
	final, _ := ctx.Value(ctxKeyProxyFinalRespAt).(time.Time)
	requestID, _ := ctx.Value(ctxKeyRequestID).(string)
	if !received.IsZero() && !sent.IsZero() {
		logger.Debug("Proxy overhead (pre-backend)", zap.Duration("duration", sent.Sub(received)), zap.String("request_id", requestID))
	}
	if !sent.IsZero() && !first.IsZero() {
		logger.Debug("Backend latency (first byte)", zap.Duration("duration", first.Sub(sent)), zap.String("request_id", requestID))
	}
	if !first.IsZero() && !final.IsZero() {
		logger.Debug("Streaming duration", zap.Duration("duration", final.Sub(first)), zap.String("request_id", requestID))
	}
	if !received.IsZero() && !final.IsZero() {
		logger.Debug("Total proxy duration", zap.Duration("duration", final.Sub(received)), zap.String("request_id", requestID))
	}
}

// LoggingMiddleware logs request details
func (p *TransparentProxy) LoggingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID, _ := r.Context().Value(ctxKeyRequestID).(string)
			p.logger.Info("Request started",
				zap.String("request_id", requestID),
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
				zap.String("request_id", requestID),
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
