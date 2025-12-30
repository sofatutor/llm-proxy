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
	"path"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/middleware"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

// TransparentProxy implements the Proxy interface for transparent proxying
type TransparentProxy struct {
	config               ProxyConfig
	tokenValidator       TokenValidator
	projectStore         ProjectStore
	logger               *zap.Logger
	auditLogger          AuditLogger
	metrics              *ProxyMetrics
	proxy                *httputil.ReverseProxy
	httpServer           *http.Server
	shuttingDown         bool
	mu                   sync.RWMutex
	allowedMethodsHeader string // cached comma-separated allowed methods
	obsMiddleware        *middleware.ObservabilityMiddleware
	cache                httpCache
	cacheStatsAggregator *CacheStatsAggregator
}

// ProxyMetrics tracks proxy usage statistics
type ProxyMetrics struct {
	RequestCount      int64
	ErrorCount        int64
	TotalResponseTime time.Duration
	// Cache metrics (provider-agnostic counters)
	CacheHits   int64 // Cache hits (responses served from cache)
	CacheMisses int64 // Cache misses (responses fetched from upstream)
	CacheBypass int64 // Cache bypassed (e.g., due to authorization)
	CacheStores int64 // Cache stores (responses stored in cache)
	mu          sync.Mutex
}

// CacheMetricType represents the kind of cache metric to increment.
type CacheMetricType int

const (
	CacheMetricHit CacheMetricType = iota
	CacheMetricMiss
	CacheMetricBypass
	CacheMetricStore
)

// Metrics returns a copy of the current proxy metrics.
// Returns a value copy to ensure thread-safety when reading metrics.
func (p *TransparentProxy) Metrics() ProxyMetrics {
	// Defensive nil guard for p.metrics
	if p.metrics == nil {
		return ProxyMetrics{}
	}
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()
	// Return a copy to avoid race conditions when accessing fields
	return ProxyMetrics{
		RequestCount:      p.metrics.RequestCount,
		ErrorCount:        p.metrics.ErrorCount,
		TotalResponseTime: p.metrics.TotalResponseTime,
		CacheHits:         p.metrics.CacheHits,
		CacheMisses:       p.metrics.CacheMisses,
		CacheBypass:       p.metrics.CacheBypass,
		CacheStores:       p.metrics.CacheStores,
	}
}

// SetMetrics overwrites the current metrics (primarily for testing).
func (p *TransparentProxy) SetMetrics(m *ProxyMetrics) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metrics = m
}

// Cache returns the HTTP cache instance for management operations.
// Returns nil if caching is disabled.
func (p *TransparentProxy) Cache() httpCache {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cache
}

// SetCacheStatsAggregator sets the cache stats aggregator for per-token cache hit tracking.
func (p *TransparentProxy) SetCacheStatsAggregator(agg *CacheStatsAggregator) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cacheStatsAggregator = agg
}

// isVaryCompatible reports whether a cached response with a given Vary header
// is valid for the current request and lookup key.
func isVaryCompatible(r *http.Request, cr cachedResponse, lookupKey string) bool {
	if cr.vary == "" || cr.vary == "*" {
		return true
	}
	varyKey := CacheKeyFromRequestWithVary(r, cr.vary)
	return varyKey == lookupKey
}

// storageKeyForResponse returns the cache storage key to use for a response,
// based on the upstream Vary header. Falls back to the lookup key when Vary is empty or '*'.
func storageKeyForResponse(r *http.Request, varyHeader string, lookupKey string) string {
	if varyHeader != "" && varyHeader != "*" {
		return CacheKeyFromRequestWithVary(r, varyHeader)
	}
	return lookupKey
}

// incrementCacheMetric safely increments the specified cache metric counter.
func (p *TransparentProxy) incrementCacheMetric(metric CacheMetricType) {
	p.metrics.mu.Lock()
	defer p.metrics.mu.Unlock()

	switch metric {
	case CacheMetricHit:
		p.metrics.CacheHits++
	case CacheMetricMiss:
		p.metrics.CacheMisses++
	case CacheMetricBypass:
		p.metrics.CacheBypass++
	case CacheMetricStore:
		p.metrics.CacheStores++
	}
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

// NewTransparentProxyWithObservability creates a new proxy with observability middleware.
func NewTransparentProxyWithObservability(config ProxyConfig, validator TokenValidator, store ProjectStore, obsCfg middleware.ObservabilityConfig) (*TransparentProxy, error) {
	logger, err := logging.NewLogger(config.LogLevel, config.LogFormat, config.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return NewTransparentProxyWithLoggerAndObservability(config, validator, store, logger, obsCfg)
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

	// Precompute allowed methods header
	allowedMethodsHeader := "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	if len(config.AllowedMethods) > 0 {
		allowedMethodsHeader = strings.Join(config.AllowedMethods, ", ")
	}

	proxy := &TransparentProxy{
		config:               config,
		tokenValidator:       validator,
		projectStore:         store,
		logger:               logger,
		metrics:              &ProxyMetrics{},
		allowedMethodsHeader: allowedMethodsHeader,
	}

	// Initialize HTTP cache (enabled only when HTTPCacheEnabled is true)
	if !config.HTTPCacheEnabled {
		logger.Info("HTTP cache disabled")
		proxy.cache = nil
	} else {
		if config.RedisCacheURL != "" {
			if opt, err := redis.ParseURL(config.RedisCacheURL); err == nil {
				client := redis.NewClient(opt)
				proxy.cache = newRedisCache(client, config.RedisCacheKeyPrefix)
				logger.Info("HTTP cache enabled", zap.String("backend", "redis"))
			} else {
				proxy.cache = newInMemoryCache()
				logger.Warn("Failed to parse RedisCacheURL; falling back to in-memory cache", zap.Error(err))
			}
		} else {
			proxy.cache = newInMemoryCache()
			logger.Info("HTTP cache enabled", zap.String("backend", "in-memory"))
		}
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

// NewTransparentProxyWithLoggerAndObservability creates a proxy with observability middleware using an existing logger.
func NewTransparentProxyWithLoggerAndObservability(config ProxyConfig, validator TokenValidator, store ProjectStore, logger *zap.Logger, obsCfg middleware.ObservabilityConfig) (*TransparentProxy, error) {
	p, err := NewTransparentProxyWithLogger(config, validator, store, logger)
	if err != nil {
		return nil, err
	}
	p.obsMiddleware = middleware.NewObservabilityMiddleware(obsCfg, logger)
	return p, nil
}

// NewTransparentProxyWithAudit creates a proxy with audit logging capabilities.
func NewTransparentProxyWithAudit(config ProxyConfig, validator TokenValidator, store ProjectStore, logger *zap.Logger, auditLogger AuditLogger, obsCfg middleware.ObservabilityConfig) (*TransparentProxy, error) {
	p, err := NewTransparentProxyWithLoggerAndObservability(config, validator, store, logger, obsCfg)
	if err != nil {
		return nil, err
	}
	p.auditLogger = auditLogger
	return p, nil
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

	// Verbose upstream request logging
	if !p.logger.Core().Enabled(zap.DebugLevel) {
		return
	}
	headers := make(map[string][]string)
	for k, v := range req.Header {
		headers[k] = v
	}
	p.logger.Debug("Upstream request",
		zap.String("request_id", requestID),
		zap.String("method", req.Method),
		zap.String("url", req.URL.String()),
		zap.Any("headers", headers),
	)

	// --- PATCH: Add X-UPSTREAM-REQUEST-START header ---
	upstreamStart := time.Now().UnixNano()
	req.Header.Set("X-UPSTREAM-REQUEST-START", fmt.Sprintf("%d", upstreamStart))
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

// calculateCacheTTL determines the effective TTL for caching a response.
// It prefers TTL from response headers (when the response is cacheable),
// otherwise falls back to client-forced TTL from the request. It returns the
// chosen TTL and whether it came from the response headers.
func calculateCacheTTL(res *http.Response, req *http.Request, defaultTTL time.Duration) (time.Duration, bool) {
	if res == nil || req == nil {
		return 0, false
	}
	respTTL := cacheTTLFromHeaders(res, defaultTTL)
	if respTTL > 0 {
		if !isResponseCacheable(res) {
			return 0, false
		}
		return respTTL, true
	}
	forcedTTL := requestForcedCacheTTL(req)
	if forcedTTL > 0 {
		return forcedTTL, false
	}
	return 0, false
}

func (p *TransparentProxy) modifyResponse(res *http.Response) error {
	// For streaming responses, return early without side effects
	if isStreaming(res) {
		return nil
	}
	// Set proxy headers
	res.Header.Set("X-Proxy", "llm-proxy")

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

	// --- PATCH: Copy X-UPSTREAM-REQUEST-START from request to response ---
	if res.Request != nil {
		if v := res.Request.Header.Get("X-UPSTREAM-REQUEST-START"); v != "" {
			res.Header.Set("X-UPSTREAM-REQUEST-START", v)
		}
	}

	// Store in cache when enabled and request is cacheable
	if p.cache != nil && res.Request != nil {
		req := res.Request
		if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodPost {
			// Only cache successful responses
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				res.Header.Set("X-CACHE-DEBUG", "status-not-cacheable")
				return nil
			}

			// Calculate effective TTL
			ttl, fromResponse := calculateCacheTTL(res, req, p.config.HTTPCacheDefaultTTL)
			if ttl <= 0 {
				res.Header.Set("X-CACHE-DEBUG", fmt.Sprintf("ttl-zero-ttl=%v-from-resp=%v", ttl, fromResponse))
				return nil
			}

			// Ensure Cache-Status preserves miss set by handler
			if res.Header.Get("Cache-Status") == "" {
				res.Header.Set("Cache-Status", "llm-proxy; miss")
			}

			key := CacheKeyFromRequest(req)
			// Compute storage key via helper to respect Vary
			storageKey := storageKeyForResponse(req, res.Header.Get("Vary"), key)

			if !isStreaming(res) {
				bodyBytes, err := io.ReadAll(res.Body)
				if err == nil {
					_ = res.Body.Close()
					res.Body = io.NopCloser(bytes.NewReader(bodyBytes))

					if p.config.HTTPCacheMaxObjectBytes == 0 || int64(len(bodyBytes)) <= p.config.HTTPCacheMaxObjectBytes {
						headers := cloneHeadersForCache(res.Header)
						if !fromResponse {
							headers.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(ttl.Seconds())))
						}
						// Store the Vary header for per-response cache key generation
						varyValue := res.Header.Get("Vary")
						cr := cachedResponse{
							statusCode: res.StatusCode,
							headers:    headers,
							body:       bodyBytes,
							expiresAt:  time.Now().Add(ttl),
							vary:       varyValue,
						}
						p.cache.Set(storageKey, cr)
						res.Header.Set("X-PROXY-CACHE", "stored")
						res.Header.Set("X-PROXY-CACHE-KEY", storageKey)
						p.incrementCacheMetric(CacheMetricStore)
						if !fromResponse {
							res.Header.Set("Cache-Status", "llm-proxy; stored (forced)")
						} else {
							res.Header.Set("Cache-Status", "llm-proxy; stored")
						}
					}
				} else {
					res.Header.Set("X-CACHE-DEBUG", fmt.Sprintf("read-body-error=%v", err))
				}
			} else {
				res.Header.Set("X-CACHE-DEBUG", "streaming-response")
				maxBytes := p.config.HTTPCacheMaxObjectBytes
				if maxBytes <= 0 {
					maxBytes = 2 * 1024 * 1024 // default 2MB
				}
				headers := cloneHeadersForCache(res.Header)
				if !fromResponse {
					headers.Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(ttl.Seconds())))
				}
				// Store the Vary header for per-response cache key generation
				varyValue := res.Header.Get("Vary")
				// Compute storage key via helper
				storageKey := storageKeyForResponse(req, varyValue, key)
				expiresAt := time.Now().Add(ttl)
				orig := res.Body
				res.Body = newStreamingCapture(orig, maxBytes, func(buf []byte) {
					if len(buf) == 0 {
						return
					}
					if int64(len(buf)) > maxBytes {
						return
					}
					p.cache.Set(storageKey, cachedResponse{
						statusCode: res.StatusCode,
						headers:    headers,
						body:       append([]byte(nil), buf...),
						expiresAt:  expiresAt,
						vary:       varyValue,
					})
					p.incrementCacheMetric(CacheMetricStore)
				})
			}
		}
	}

	// Set miss status if no cache status was set
	if res.Header.Get("Cache-Status") == "" {
		res.Header.Set("Cache-Status", "llm-proxy; miss")
	}

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
		p.handleValidationError(w, r, validationErr)
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
func (p *TransparentProxy) handleValidationError(w http.ResponseWriter, r *http.Request, err error) {
	// Get request ID and token directly from the request
	requestID, _ := r.Context().Value(ctxKeyRequestID).(string)
	var obfuscatedToken string
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && strings.HasPrefix(authHeader, "Bearer ") {
		tok := strings.TrimSpace(authHeader[7:])
		obfuscatedToken = token.ObfuscateToken(tok)
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
		zap.Error(err),
		zap.String("token", obfuscatedToken),
	)
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
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Short-circuit OPTIONS requests: no auth required, respond with 204 and CORS headers
		if r.Method == http.MethodOptions {
			// Set CORS headers for preflight requests
			if origin := r.Header.Get("Origin"); origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				if reqHeaders := r.Header.Get("Access-Control-Request-Headers"); reqHeaders != "" {
					w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With")
				}
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-Proxy-ID, X-LLM-Proxy-Remote-Duration, X-LLM-Proxy-Remote-Duration-Ms")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Generate request ID and add to context
		requestID := uuid.New().String()
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, requestID)
		// Also add to shared logging context so middlewares can read it
		ctx = logging.WithRequestID(ctx, requestID)
		r = r.WithContext(ctx)

		// Set request header so observability/file logger can capture it (response header is still set in ModifyResponse only)
		r.Header.Set("X-Request-ID", requestID)

		// Record when proxy receives the request
		receivedAt := time.Now().UTC()
		ctx = context.WithValue(ctx, ctxKeyProxyReceivedAt, receivedAt)
		r = r.WithContext(ctx)

		// --- Token extraction and validation (moved from director) ---
		authHeader := r.Header.Get("Authorization")
		tokenStr := extractTokenFromHeader(authHeader)
		if tokenStr == "" {
			p.handleValidationError(w, r, errors.New("missing or invalid authorization header"))
			return
		}
		projectID, err := p.tokenValidator.ValidateTokenWithTracking(r.Context(), tokenStr)
		if err != nil {
			p.handleValidationError(w, r, err)
			return
		}
		ctx = context.WithValue(r.Context(), ctxKeyProjectID, projectID)
		ctx = context.WithValue(ctx, ctxKeyTokenID, tokenStr)
		r = r.WithContext(ctx)
		apiKey, err := p.projectStore.GetAPIKeyForProject(r.Context(), projectID)
		if err != nil {
			p.handleValidationError(w, r, fmt.Errorf("failed to get API key: %w", err))
			return
		}
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

		// Enforce project active status using shared helper (if enabled)
		if allowed, status, er := shouldAllowProject(r.Context(), p.config.EnforceProjectActive, p.projectStore, projectID, p.auditLogger, r); !allowed {
			writeErrorResponse(w, status, er)
			return
		}

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

			// --- PATCH: Add X-UPSTREAM-REQUEST-START header ---
			upstreamStart := time.Now().UnixNano()
			req.Header.Set("X-UPSTREAM-REQUEST-START", fmt.Sprintf("%d", upstreamStart))
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

			// --- PATCH: Add X-UPSTREAM-REQUEST-STOP header ---
			upstreamStop := time.Now().UnixNano()
			res.Header.Set("X-UPSTREAM-REQUEST-STOP", fmt.Sprintf("%d", upstreamStop))

			return p.modifyResponse(res)
		}

		p.proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			logProxyTimings(p.logger, r.Context())
			p.errorHandler(w, r, err)
		}

		// Compute X-Body-Hash once for methods with bodies to support POST/PUT/PATCH caching
		if r.Body != nil && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch) {
			bodyBytes, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			// Import hashing lazily to avoid overhead elsewhere
			sum := sha256.Sum256(bodyBytes)
			r.Header.Set("X-Body-Hash", hex.EncodeToString(sum[:]))
		}

		// Simple cache lookup with conditional handling (ETag/Last-Modified)
		if p.cache != nil && (r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodPost) {
			// Allow GET/HEAD lookups by default when cache is enabled, since reuse will still be gated by canServeCachedForRequest.
			// Require explicit client opt-in for POST lookups.
			optIn := hasClientCacheOptIn(r)
			allowedLookup := (r.Method == http.MethodGet || r.Method == http.MethodHead) || (r.Method == http.MethodPost && optIn)
			if !allowedLookup {
				// Cache is enabled but this request type/method is not cacheable - count as miss
				p.recordCacheMiss()
				p.proxy.ServeHTTP(rw, r)
				return
			}
			key := CacheKeyFromRequest(r)
			if cr, ok := p.cache.Get(key); ok {
				// Validate Vary compatibility using helper
				if !isVaryCompatible(r, cr, key) {
					p.recordCacheMiss()
					// Note: don't set miss status here; let modifyResponse handle cache status
					p.proxy.ServeHTTP(rw, r)
					return
				}

				if !canServeCachedForRequest(r, cr.headers) {
					// Authorization present but cached response not explicitly shared-cacheable
					w.Header().Set("Cache-Status", "llm-proxy; bypass")
					w.Header().Set("X-PROXY-CACHE", "bypass")
					w.Header().Set("X-PROXY-CACHE-KEY", key)
					p.incrementCacheMetric(CacheMetricBypass)
					p.proxy.ServeHTTP(rw, r)
					return
				}
				// Origin revalidation path: if client requests revalidation (no-cache/max-age=0),
				// send conditional request upstream using cached validators (ETag/Last-Modified).
				if wantsRevalidation(r) {
					condReq := r.Clone(r.Context())
					if etag := cr.headers.Get("ETag"); etag != "" {
						condReq.Header.Set("If-None-Match", etag)
					}
					if lm := cr.headers.Get("Last-Modified"); lm != "" {
						condReq.Header.Set("If-Modified-Since", lm)
					}
					// Forward conditionally to upstream; let modifyResponse handle store/refresh
					// Don't increment miss here since this is a conditional revalidation
					p.proxy.ServeHTTP(rw, condReq)
					return
				}
				// If the client provided conditionals, respond 304 when validators match
				if r.Method == http.MethodGet || r.Method == http.MethodHead {
					if conditionalRequestMatches(r, cr.headers) {
						for hk, hv := range cr.headers {
							for _, v := range hv {
								w.Header().Add(hk, v)
							}
						}
						// Set fresh timing headers for conditional cache hit
						setFreshCacheTimingHeadersFromRequest(w, r, time.Now())
						w.Header().Set("Cache-Status", "llm-proxy; conditional-hit")
						w.Header().Set("X-PROXY-CACHE", "conditional-hit")
						w.Header().Set("X-PROXY-CACHE-KEY", key)
						p.recordCacheHit(r) // Conditional hit counts as cache hit
						w.WriteHeader(http.StatusNotModified)
						return
					}
				}
				for hk, hv := range cr.headers {
					for _, v := range hv {
						w.Header().Add(hk, v)
					}
				}
				// Set fresh timing headers for cache hit (for accurate latency measurement)
				setFreshCacheTimingHeadersFromRequest(w, r, time.Now())
				w.Header().Set("Cache-Status", "llm-proxy; hit")
				w.Header().Set("X-PROXY-CACHE", "hit")
				w.Header().Set("X-PROXY-CACHE-KEY", key)
				p.recordCacheHit(r)
				w.WriteHeader(cr.statusCode)
				if r.Method != http.MethodHead {
					_, _ = w.Write(cr.body)
				}
				return
			}
			// Cache miss - no entry found
			p.recordCacheMiss()
			// Note: don't set miss status here; let modifyResponse handle cache status
			// w.Header().Set("Cache-Status", "llm-proxy; miss")
			// Do not set X-PROXY-CACHE(-KEY) on miss; only set definitive headers on hit/bypass/conditional-hit or store path
		} else if p.cache != nil {
			// Cache is enabled but method is not cacheable (e.g., DELETE, OPTIONS, etc.) - count as miss
			p.recordCacheMiss()
		}

		p.proxy.ServeHTTP(rw, r)
	})

	var handler http.Handler = baseHandler
	handler = p.ValidateRequestMiddleware()(handler)

	if p.obsMiddleware != nil {
		handler = p.obsMiddleware.Middleware()(handler)
	}
	handler = CircuitBreakerMiddleware(5, 30*time.Second, func(status int) bool {
		return status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
	})(handler)

	return handler
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

// recordCacheMiss centralizes cache miss accounting to reduce duplication and
// ensure consistent metric semantics across all miss paths.
func (p *TransparentProxy) recordCacheMiss() {
	p.incrementCacheMetric(CacheMetricMiss)
}

// recordCacheHit records a cache hit for metrics and per-token tracking.
func (p *TransparentProxy) recordCacheHit(r *http.Request) {
	p.incrementCacheMetric(CacheMetricHit)
	// Record per-token cache hit if aggregator is configured
	if p.cacheStatsAggregator != nil {
		if tokenID, ok := r.Context().Value(ctxKeyTokenID).(string); ok && tokenID != "" {
			p.cacheStatsAggregator.RecordCacheHit(tokenID)
		}
	}
}

func setFreshCacheTimingHeadersFromRequest(w http.ResponseWriter, r *http.Request, now time.Time) {
	receivedAt := now
	if v := r.Context().Value(ctxKeyProxyReceivedAt); v != nil {
		if t, ok := v.(time.Time); ok && !t.IsZero() {
			receivedAt = t
		}
	}

	w.Header().Set("X-Proxy-Received-At", receivedAt.UTC().Format(time.RFC3339Nano))
	// For cache hits/conditional-hits, the response body is served from cache,
	// but there may still be non-trivial work before we can write the response
	// (auth, DB lookups, cache lookup, etc.).
	w.Header().Set("X-Proxy-First-Response-At", now.UTC().Format(time.RFC3339Nano))
	w.Header().Set("X-Proxy-Final-Response-At", now.UTC().Format(time.RFC3339Nano))
	w.Header().Set("Date", now.UTC().Format(http.TimeFormat))
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
			// Ensure request_id is set in context as the very first step
			requestID, ok := r.Context().Value(ctxKeyRequestID).(string)
			if !ok || requestID == "" {
				requestID = uuid.New().String()
				r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, requestID))
			}

			// --- Validation Scope: Only token, path, and method are validated here ---
			// Do not add API-specific validation or transformation logic here.

			// Check if method is allowed
			if !p.isMethodAllowed(r.Method) {
				p.logger.Warn("Method not allowed",
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path))
				w.WriteHeader(http.StatusMethodNotAllowed)
				if requestID != "" {
					w.Header().Set("X-Request-ID", requestID)
				}
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

			// --- End of validation scope ---

			// --- Begin param whitelist validation ---
			if r.Method == http.MethodPost && len(p.config.ParamWhitelist) > 0 && r.Header.Get("Content-Type") == "application/json" {
				// Read and buffer the body for validation and later proxying
				var bodyBytes []byte
				if r.Body != nil {
					bodyBytes, _ = io.ReadAll(r.Body)
				}
				if len(bodyBytes) > 0 {
					var bodyMap map[string]interface{}
					if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
						for param, allowed := range p.config.ParamWhitelist {
							if val, ok := bodyMap[param]; ok {
								valStr := ""
								switch v := val.(type) {
								case string:
									valStr = v
								case float64:
									valStr = fmt.Sprintf("%v", v)
								default:
									valStr = fmt.Sprintf("%v", v)
								}
								found := false
								// Support glob expressions in allowed values
								for _, allowedVal := range allowed {
									if ok, _ := path.Match(allowedVal, valStr); ok {
										found = true
										break
									}
								}
								if !found {
									w.WriteHeader(http.StatusBadRequest)
									w.Header().Set("Content-Type", "application/json")
									if err := json.NewEncoder(w).Encode(ErrorResponse{
										Error: fmt.Sprintf("Parameter '%s' value '%s' is not allowed. Allowed patterns: %v", param, valStr, allowed),
										Code:  "param_not_allowed",
									}); err != nil {
										p.logger.Error("Failed to encode error response", zap.Error(err))
									}
									return
								}
							}
						}
					}
					// Restore the body for downstream handlers
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}
			// --- End param whitelist validation ---

			// --- Begin CORS origin validation ---
			origin := r.Header.Get("Origin")
			originRequired := false
			for _, h := range p.config.RequiredHeaders {
				if strings.EqualFold(h, "origin") {
					originRequired = true
					break
				}
			}
			if originRequired {
				if origin == "" {
					w.WriteHeader(http.StatusBadRequest)
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(ErrorResponse{
						Error: "Origin header required",
						Code:  "origin_required",
					}); err != nil {
						p.logger.Error("Failed to encode error response", zap.Error(err))
					}
					return
				}
				if len(p.config.AllowedOrigins) > 0 {
					allowed := false
					for _, o := range p.config.AllowedOrigins {
						if o == origin {
							allowed = true
							break
						}
					}
					if !allowed {
						w.WriteHeader(http.StatusForbidden)
						w.Header().Set("Content-Type", "application/json")
						if err := json.NewEncoder(w).Encode(ErrorResponse{
							Error: "Origin not allowed",
							Code:  "origin_not_allowed",
						}); err != nil {
							p.logger.Error("Failed to encode error response", zap.Error(err))
						}
						return
					}
				}
			} else if origin != "" && len(p.config.AllowedOrigins) > 0 {
				allowed := false
				for _, o := range p.config.AllowedOrigins {
					if o == origin {
						allowed = true
						break
					}
				}
				if !allowed {
					w.WriteHeader(http.StatusForbidden)
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(ErrorResponse{
						Error: "Origin not allowed",
						Code:  "origin_not_allowed",
					}); err != nil {
						p.logger.Error("Failed to encode error response", zap.Error(err))
					}
					return
				}
			}
			// --- End CORS origin validation ---

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
			p.metrics.RequestCount++
			// Increment error count for status codes >= 400
			if rec.statusCode >= 400 {
				p.metrics.ErrorCount++
			}
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

// Add Flush forwarding for streaming support
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
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
