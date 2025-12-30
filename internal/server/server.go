// Package server implements the HTTP server for the LLM Proxy.
// It handles request routing, lifecycle management, and provides
// health check endpoints and core API functionality.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/sofatutor/llm-proxy/internal/logging"
	"github.com/sofatutor/llm-proxy/internal/middleware"
	"github.com/sofatutor/llm-proxy/internal/obfuscate"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	"github.com/sofatutor/llm-proxy/internal/token"
	"go.uber.org/zap"
)

// Server represents the HTTP server for the LLM Proxy.
// It encapsulates the underlying http.Server along with application configuration
// and handles request routing and server lifecycle management.
type Server struct {
	server        *http.Server
	config        *config.Config
	tokenStore    token.TokenStore
	projectStore  proxy.ProjectStore
	logger        *zap.Logger
	proxy         *proxy.TransparentProxy
	metrics       Metrics
	eventBus      eventbus.EventBus
	auditLogger   *audit.Logger
	db            *database.DB
	cacheStatsAgg *proxy.CacheStatsAggregator
}

// HealthResponse is the response body for the health check endpoint.
// It provides basic information about the server status and version.
type HealthResponse struct {
	Status    string    `json:"status"`    // Service status, "ok" for a healthy system
	Timestamp time.Time `json:"timestamp"` // Current server time
	Version   string    `json:"version"`   // Application version number
}

// Metrics holds runtime metrics for the server.
type Metrics struct {
	StartTime    time.Time
	RequestCount int64
	ErrorCount   int64
}

// Version is the application version, following semantic versioning.
const Version = "0.1.0"

// maxDurationMinutes is the maximum allowed duration for a token (365 days)
const maxDurationMinutes = 525600

// New creates a new HTTP server with the provided configuration and store implementations.
// It initializes the server with appropriate timeouts and registers all necessary route handlers.
// The server is not started until the Start method is called.
func New(cfg *config.Config, tokenStore token.TokenStore, projectStore proxy.ProjectStore) (*Server, error) {
	return NewWithDatabase(cfg, tokenStore, projectStore, nil)
}

// NewWithDatabase creates a new HTTP server with database support for audit logging.
// This allows the server to store audit events in both file and database backends.
func NewWithDatabase(cfg *config.Config, tokenStore token.TokenStore, projectStore proxy.ProjectStore, db *database.DB) (*Server, error) {
	mux := http.NewServeMux()

	logger, err := logging.NewLogger(cfg.LogLevel, cfg.LogFormat, cfg.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// Initialize audit logger with optional database backend
	var auditLogger *audit.Logger
	if cfg.AuditEnabled && cfg.AuditLogFile != "" {
		auditConfig := audit.LoggerConfig{
			FilePath:       cfg.AuditLogFile,
			CreateDir:      cfg.AuditCreateDir,
			DatabaseStore:  db, // Database store for audit events
			EnableDatabase: cfg.AuditStoreInDB && db != nil,
		}
		auditLogger, err = audit.NewLogger(auditConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize audit logger: %w", err)
		}
		if cfg.AuditStoreInDB && db != nil {
			logger.Info("Audit logging enabled with database storage", zap.String("log_file", cfg.AuditLogFile))
		} else {
			logger.Info("Audit logging enabled", zap.String("log_file", cfg.AuditLogFile))
		}
	} else {
		auditLogger = audit.NewNullLogger()
		logger.Info("Audit logging disabled")
	}

	metrics := Metrics{StartTime: time.Now()}

	var bus eventbus.EventBus
	switch cfg.EventBusBackend {
	case "redis-streams":
		client := redis.NewClient(&redis.Options{
			Addr: cfg.RedisAddr,
			DB:   cfg.RedisDB,
		})
		// Redis diagnostics
		logger.Info("Connecting to Redis for Streams", zap.String("addr", cfg.RedisAddr), zap.Int("db", cfg.RedisDB))
		pong, err := client.Ping(context.Background()).Result()
		if err != nil {
			logger.Fatal("Failed to ping Redis",
				zap.String("addr", cfg.RedisAddr),
				zap.Int("db", cfg.RedisDB),
				zap.Error(err))
		}
		logger.Info("Successfully pinged Redis",
			zap.String("addr", cfg.RedisAddr),
			zap.Int("db", cfg.RedisDB),
			zap.String("response", pong))

		// Generate consumer name if not provided
		consumerName := cfg.RedisConsumerName
		if consumerName == "" {
			consumerName = fmt.Sprintf("proxy-%s", uuid.New().String()[:8])
		}

		streamsConfig := eventbus.RedisStreamsConfig{
			StreamKey:        cfg.RedisStreamKey,
			ConsumerGroup:    cfg.RedisConsumerGroup,
			ConsumerName:     consumerName,
			MaxLen:           cfg.RedisStreamMaxLen,
			BlockTimeout:     cfg.RedisStreamBlockTime,
			ClaimMinIdleTime: cfg.RedisStreamClaimTime,
			BatchSize:        cfg.RedisStreamBatchSize,
		}
		adapter := &eventbus.RedisStreamsClientAdapter{Client: client}
		bus = eventbus.NewRedisStreamsEventBus(adapter, streamsConfig)
		logger.Info("Using Redis Streams event bus",
			zap.String("addr", cfg.RedisAddr),
			zap.Int("db", cfg.RedisDB),
			zap.String("stream", cfg.RedisStreamKey),
			zap.String("consumer_group", cfg.RedisConsumerGroup),
			zap.String("consumer_name", consumerName))
	case "in-memory":
		logger.Info("Using in-memory event bus", zap.String("mode", "single-process"))
		bus = eventbus.NewInMemoryEventBus(cfg.ObservabilityBufferSize)
	default:
		return nil, fmt.Errorf("unknown event bus backend: %s", cfg.EventBusBackend)
	}

	s := &Server{
		config:       cfg,
		tokenStore:   tokenStore,
		projectStore: projectStore,
		logger:       logger,
		metrics:      metrics,
		eventBus:     bus,
		auditLogger:  auditLogger,
		db:           db,
		server: &http.Server{
			Addr:         cfg.ListenAddr,
			Handler:      mux,
			ReadTimeout:  cfg.RequestTimeout,
			WriteTimeout: cfg.RequestTimeout,
			IdleTimeout:  cfg.RequestTimeout * 2,
		},
	}

	// Register routes
	mux.HandleFunc("/health", s.logRequestMiddleware(s.handleHealth))
	mux.HandleFunc("/ready", s.logRequestMiddleware(s.handleReady))
	mux.HandleFunc("/live", s.logRequestMiddleware(s.handleLive))
	mux.HandleFunc("/manage/projects", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleProjects)))
	mux.HandleFunc("/manage/projects/", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleProjectByID)))
	mux.HandleFunc("/manage/tokens", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleTokens)))
	mux.HandleFunc("/manage/tokens/", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleTokenByID)))
	mux.HandleFunc("/manage/audit", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleAuditEvents)))
	mux.HandleFunc("/manage/audit/", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleAuditEventByID)))
	mux.HandleFunc("/manage/cache/purge", s.logRequestMiddleware(s.managementAuthMiddleware(s.handleCachePurge)))

	// Add catch-all handler for unmatched routes to ensure logging
	mux.HandleFunc("/", s.logRequestMiddleware(s.handleNotFound))

	if cfg.EnableMetrics {
		path := cfg.MetricsPath
		if path == "" {
			path = "/metrics"
		}
		mux.HandleFunc(path, s.logRequestMiddleware(s.handleMetrics))
		mux.HandleFunc(path+"/prometheus", s.logRequestMiddleware(s.handleMetricsPrometheus))
	}

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

	s.logger.Info("Server starting", zap.String("listen_addr", s.config.ListenAddr))

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
		s.logger.Warn("Failed to load API config, using default OpenAI configuration",
			zap.String("config_path", s.config.APIConfigPath),
			zap.Error(err))

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
		s.logger.Warn("Specified API provider not found, using default", zap.Error(err))
		proxyConfig, err = apiConfig.GetProxyConfigForAPI(apiConfig.DefaultAPI)
		if err != nil {
			return fmt.Errorf("failed to get proxy configuration: %w", err)
		}
	}

	// Apply HTTP cache env overrides (simple toggle + backend selection)
	if v := os.Getenv("HTTP_CACHE_ENABLED"); v != "" {
		// Parse bool; default to true on invalid for safety
		proxyConfig.HTTPCacheEnabled = strings.EqualFold(v, "true") || strings.EqualFold(v, "1") || strings.EqualFold(v, "yes")
	} else {
		// Default: enabled
		proxyConfig.HTTPCacheEnabled = true
	}
	backend := strings.ToLower(os.Getenv("HTTP_CACHE_BACKEND"))
	if backend == "redis" {
		// Use REDIS_CACHE_URL if explicitly set, otherwise construct from REDIS_ADDR
		url := os.Getenv("REDIS_CACHE_URL")
		if url == "" {
			// Construct URL from unified REDIS_ADDR config (same as event bus)
			addr := os.Getenv("REDIS_ADDR")
			if addr == "" {
				addr = "localhost:6379"
			}
			db := os.Getenv("REDIS_DB")
			if db == "" {
				db = "0"
			}
			url = fmt.Sprintf("redis://%s/%s", addr, db)
		}
		proxyConfig.RedisCacheURL = url
		if kp := os.Getenv("REDIS_CACHE_KEY_PREFIX"); kp != "" {
			proxyConfig.RedisCacheKeyPrefix = kp
		}
	}

	// Use the injected tokenStore and projectStore
	// (No more creation of mock stores or test data here)
	tokenValidator := token.NewValidator(s.tokenStore)
	cachedValidator := token.NewCachedValidator(tokenValidator)

	obsCfg := middleware.ObservabilityConfig{Enabled: s.config.ObservabilityEnabled, EventBus: s.eventBus}

	proxyHandler, err := proxy.NewTransparentProxyWithAudit(*proxyConfig, cachedValidator, s.projectStore, s.logger, s.auditLogger, obsCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize proxy: %w", err)
	}
	s.proxy = proxyHandler

	// Initialize cache stats aggregator for per-token cache hit tracking.
	// NOTE: Cache stats tracking is only enabled when HTTP caching is enabled (HTTPCacheEnabled=true).
	// When caching is disabled, no cache hits occur, so tracking is not needed.
	// The Admin UI will show CacheHitCount=0 for all tokens when caching is disabled.
	if s.db != nil && proxyConfig.HTTPCacheEnabled {
		aggConfig := proxy.CacheStatsAggregatorConfig{
			BufferSize:    s.config.CacheStatsBufferSize,
			FlushInterval: 5 * time.Second,
			BatchSize:     100,
		}
		s.cacheStatsAgg = proxy.NewCacheStatsAggregator(aggConfig, s.db, s.logger)
		s.cacheStatsAgg.Start()
		proxyHandler.SetCacheStatsAggregator(s.cacheStatsAgg)
		s.logger.Info("Cache stats aggregator started", zap.Int("buffer_size", aggConfig.BufferSize))
	}

	// Register proxy routes
	s.server.Handler.(*http.ServeMux).Handle("/v1/", proxyHandler.Handler())

	s.logger.Info("Initialized proxy",
		zap.String("target_base_url", proxyConfig.TargetBaseURL),
		zap.Int("allowed_endpoints", len(proxyConfig.AllowedEndpoints)))

	return nil
}

// Shutdown gracefully shuts down the server without interrupting
// active connections. It waits for all connections to complete
// or for the provided context to be canceled, whichever comes first.
//
// The context should typically include a timeout to prevent
// the shutdown from blocking indefinitely.
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop cache stats aggregator first to flush pending stats
	if s.cacheStatsAgg != nil {
		s.logger.Info("Stopping cache stats aggregator")
		if err := s.cacheStatsAgg.Stop(ctx); err != nil {
			s.logger.Error("failed to stop cache stats aggregator during shutdown", zap.Error(err))
		}
	}
	// Close audit logger to ensure all events are written
	if s.auditLogger != nil {
		if err := s.auditLogger.Close(); err != nil {
			s.logger.Error("failed to close audit logger during shutdown", zap.Error(err))
		}
	}
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
		s.logger.Error("Error encoding health response", zap.Error(err))
		return
	}
	// Status code 200 OK is set implicitly when the response is written successfully
}

// handleReady is used for readiness probes.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

// handleLive is used for liveness probes.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("alive"))
}

// handleMetrics returns basic runtime metrics in JSON format.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	m := struct {
		UptimeSeconds float64 `json:"uptime_seconds"`
		RequestCount  int64   `json:"request_count"`
		ErrorCount    int64   `json:"error_count"`
		// Cache metrics (provider-agnostic counters)
		CacheHits   int64 `json:"cache_hits"`
		CacheMisses int64 `json:"cache_misses"`
		CacheBypass int64 `json:"cache_bypass"`
		CacheStores int64 `json:"cache_stores"`
	}{
		UptimeSeconds: time.Since(s.metrics.StartTime).Seconds(),
	}
	if s.proxy != nil {
		pm := s.proxy.Metrics()
		m.RequestCount = pm.RequestCount
		m.ErrorCount = pm.ErrorCount
		m.CacheHits = pm.CacheHits
		m.CacheMisses = pm.CacheMisses
		m.CacheBypass = pm.CacheBypass
		m.CacheStores = pm.CacheStores
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(m); err != nil {
		s.logger.Error("Failed to encode metrics", zap.Error(err))
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
	}
}

// handleMetricsPrometheus returns metrics in Prometheus text exposition format.
func (s *Server) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	var buf strings.Builder

	// Uptime gauge
	uptimeSeconds := time.Since(s.metrics.StartTime).Seconds()
	buf.WriteString("# HELP llm_proxy_uptime_seconds Time since the server started\n")
	buf.WriteString("# TYPE llm_proxy_uptime_seconds gauge\n")
	fmt.Fprintf(&buf, "llm_proxy_uptime_seconds %g\n", uptimeSeconds)

	// Get proxy metrics or use zero values
	var requestCount, errorCount, cacheHits, cacheMisses, cacheBypass, cacheStores int64
	if s.proxy != nil {
		pm := s.proxy.Metrics()
		requestCount = pm.RequestCount
		errorCount = pm.ErrorCount
		cacheHits = pm.CacheHits
		cacheMisses = pm.CacheMisses
		cacheBypass = pm.CacheBypass
		cacheStores = pm.CacheStores
	}

	// Write metrics in Prometheus format
	buf.WriteString("# HELP llm_proxy_requests_total Total number of proxy requests\n")
	buf.WriteString("# TYPE llm_proxy_requests_total counter\n")
	fmt.Fprintf(&buf, "llm_proxy_requests_total %d\n", requestCount)

	buf.WriteString("# HELP llm_proxy_errors_total Total number of proxy errors\n")
	buf.WriteString("# TYPE llm_proxy_errors_total counter\n")
	fmt.Fprintf(&buf, "llm_proxy_errors_total %d\n", errorCount)

	buf.WriteString("# HELP llm_proxy_cache_hits_total Total number of cache hits\n")
	buf.WriteString("# TYPE llm_proxy_cache_hits_total counter\n")
	fmt.Fprintf(&buf, "llm_proxy_cache_hits_total %d\n", cacheHits)

	buf.WriteString("# HELP llm_proxy_cache_misses_total Total number of cache misses\n")
	buf.WriteString("# TYPE llm_proxy_cache_misses_total counter\n")
	fmt.Fprintf(&buf, "llm_proxy_cache_misses_total %d\n", cacheMisses)

	buf.WriteString("# HELP llm_proxy_cache_bypass_total Total number of cache bypasses\n")
	buf.WriteString("# TYPE llm_proxy_cache_bypass_total counter\n")
	fmt.Fprintf(&buf, "llm_proxy_cache_bypass_total %d\n", cacheBypass)

	buf.WriteString("# HELP llm_proxy_cache_stores_total Total number of cache stores\n")
	buf.WriteString("# TYPE llm_proxy_cache_stores_total counter\n")
	fmt.Fprintf(&buf, "llm_proxy_cache_stores_total %d\n", cacheStores)

	// Go runtime metrics
	s.writeGoRuntimeMetrics(&buf)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if _, err := w.Write([]byte(buf.String())); err != nil {
		s.logger.Error("Failed to write Prometheus metrics", zap.Error(err))
	}
}

// writeGoRuntimeMetrics writes Go runtime metrics to the buffer in Prometheus format.
func (s *Server) writeGoRuntimeMetrics(buf *strings.Builder) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Goroutines
	buf.WriteString("# HELP llm_proxy_goroutines Number of goroutines currently running\n")
	buf.WriteString("# TYPE llm_proxy_goroutines gauge\n")
	fmt.Fprintf(buf, "llm_proxy_goroutines %d\n", runtime.NumGoroutine())

	// Memory metrics
	buf.WriteString("# HELP llm_proxy_memory_heap_alloc_bytes Number of heap bytes allocated and currently in use\n")
	buf.WriteString("# TYPE llm_proxy_memory_heap_alloc_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_memory_heap_alloc_bytes %d\n", memStats.Alloc)

	buf.WriteString("# HELP llm_proxy_memory_heap_sys_bytes Number of heap bytes obtained from the OS\n")
	buf.WriteString("# TYPE llm_proxy_memory_heap_sys_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_memory_heap_sys_bytes %d\n", memStats.HeapSys)

	buf.WriteString("# HELP llm_proxy_memory_heap_idle_bytes Number of heap bytes waiting to be used\n")
	buf.WriteString("# TYPE llm_proxy_memory_heap_idle_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_memory_heap_idle_bytes %d\n", memStats.HeapIdle)

	buf.WriteString("# HELP llm_proxy_memory_heap_inuse_bytes Number of heap bytes that are in use\n")
	buf.WriteString("# TYPE llm_proxy_memory_heap_inuse_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_memory_heap_inuse_bytes %d\n", memStats.HeapInuse)

	buf.WriteString("# HELP llm_proxy_memory_heap_released_bytes Number of heap bytes released to the OS\n")
	buf.WriteString("# TYPE llm_proxy_memory_heap_released_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_memory_heap_released_bytes %d\n", memStats.HeapReleased)

	buf.WriteString("# HELP llm_proxy_memory_total_alloc_bytes Total number of bytes allocated (cumulative)\n")
	buf.WriteString("# TYPE llm_proxy_memory_total_alloc_bytes counter\n")
	fmt.Fprintf(buf, "llm_proxy_memory_total_alloc_bytes %d\n", memStats.TotalAlloc)

	buf.WriteString("# HELP llm_proxy_memory_sys_bytes Total number of bytes obtained from the OS\n")
	buf.WriteString("# TYPE llm_proxy_memory_sys_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_memory_sys_bytes %d\n", memStats.Sys)

	buf.WriteString("# HELP llm_proxy_memory_mallocs_total Total number of malloc operations\n")
	buf.WriteString("# TYPE llm_proxy_memory_mallocs_total counter\n")
	fmt.Fprintf(buf, "llm_proxy_memory_mallocs_total %d\n", memStats.Mallocs)

	buf.WriteString("# HELP llm_proxy_memory_frees_total Total number of free operations\n")
	buf.WriteString("# TYPE llm_proxy_memory_frees_total counter\n")
	fmt.Fprintf(buf, "llm_proxy_memory_frees_total %d\n", memStats.Frees)

	// GC metrics
	buf.WriteString("# HELP llm_proxy_gc_runs_total Total number of GC runs\n")
	buf.WriteString("# TYPE llm_proxy_gc_runs_total counter\n")
	fmt.Fprintf(buf, "llm_proxy_gc_runs_total %d\n", memStats.NumGC)

	buf.WriteString("# HELP llm_proxy_gc_pause_total_seconds Total GC pause time in seconds\n")
	buf.WriteString("# TYPE llm_proxy_gc_pause_total_seconds counter\n")
	fmt.Fprintf(buf, "llm_proxy_gc_pause_total_seconds %g\n", float64(memStats.PauseTotalNs)/1e9)

	buf.WriteString("# HELP llm_proxy_gc_next_bytes Target heap size for next GC cycle\n")
	buf.WriteString("# TYPE llm_proxy_gc_next_bytes gauge\n")
	fmt.Fprintf(buf, "llm_proxy_gc_next_bytes %d\n", memStats.NextGC)
}

// managementAuthMiddleware checks the management token in the Authorization header
func (s *Server) managementAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, prefix) || len(header) <= len(prefix) {
			http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}
		token := header[len(prefix):]
		if token != s.config.ManagementToken {
			http.Error(w, `{"error":"invalid management token"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// GET /manage/projects
// We only register /manage/projects (no trailing slash) for handleProjects. This ensures that both /manage/projects and /manage/projects/ are handled identically, and only /manage/projects/{id} is handled by handleProjectByID. This avoids ambiguity and double handling in Go's http.ServeMux.
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("handleProjects: START", zap.String("method", r.Method), zap.String("path", r.URL.Path))
	// Normalize path: treat /manage/projects/ as /manage/projects
	if r.URL.Path == "/manage/projects/" {
		r.URL.Path = "/manage/projects"
	}
	// DEBUG: Log method and headers
	for k, v := range r.Header {
		if strings.EqualFold(k, "Authorization") {
			s.logger.Debug("handleProjects: header", zap.String("key", k), zap.String("value", "******"))
		} else {
			s.logger.Debug("handleProjects: header", zap.String("key", k), zap.Any("value", v))
		}
	}
	// Mask management token in logs
	maskedToken := "******"
	if len(s.config.ManagementToken) > 4 {
		maskedToken = s.config.ManagementToken[:4] + "******"
	}
	s.logger.Debug("handleProjects: config.ManagementToken", zap.String("ManagementToken", maskedToken))
	if !s.checkManagementAuth(w, r) {
		s.logger.Debug("handleProjects: END (auth failed)")
		return
	}
	ctx := r.Context()
	requestID := getRequestID(ctx)

	switch r.Method {
	case http.MethodGet:
		s.logger.Info("listing projects", zap.String("request_id", requestID))
		s.handleListProjects(w, r.WithContext(ctx))
	case http.MethodPost:
		s.logger.Info("creating project", zap.String("request_id", requestID))
		s.handleCreateProject(w, r.WithContext(ctx))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
	s.logger.Debug("handleProjects: END", zap.String("method", r.Method), zap.String("path", r.URL.Path))
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug("handleListProjects: START")
	ctx := r.Context()
	requestID := getRequestID(ctx)
	projects, err := s.projectStore.ListProjects(ctx)
	if err != nil {
		s.logger.Error("failed to list projects", zap.Error(err))

		// Audit: project list failure
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectList, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithError(err))

		http.Error(w, `{"error":"failed to list projects"}`, http.StatusInternalServerError)
		s.logger.Debug("handleListProjects: END (error)")
		return
	}

	// Audit: project list success
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectList, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithDetail("project_count", len(projects)))

	// Create response with obfuscated API keys
	sanitizedProjects := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		sanitizedProjects[i] = ProjectResponse{
			ID:            p.ID,
			Name:          p.Name,
			OpenAIAPIKey:  obfuscate.ObfuscateTokenGeneric(p.OpenAIAPIKey),
			IsActive:      p.IsActive,
			DeactivatedAt: p.DeactivatedAt,
			CreatedAt:     p.CreatedAt,
			UpdatedAt:     p.UpdatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sanitizedProjects); err != nil {
		s.logger.Error("failed to encode projects response", zap.Error(err))
		s.logger.Debug("handleListProjects: END (encode error)")
	} else {
		s.logger.Debug("handleListProjects: END (success)")
	}
}

// POST /manage/projects
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	var req struct {
		Name         string `json:"name"`
		OpenAIAPIKey string `json:"openai_api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("invalid request body", zap.Error(err), zap.String("request_id", requestID))

		// Audit: project creation failure - invalid request
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithError(err).
			WithDetail("validation_error", "invalid request body"))

		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.OpenAIAPIKey == "" {
		s.logger.Error(
			"missing required fields",
			zap.String("name", req.Name),
			zap.Bool("api_key_provided", req.OpenAIAPIKey != ""),
			zap.String("request_id", requestID),
		)

		// Audit: project creation failure - missing fields
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithDetail("validation_error", "missing required fields").
			WithDetail("name_provided", req.Name != "").
			WithDetail("api_key_provided", req.OpenAIAPIKey != ""))

		http.Error(w, `{"error":"name and openai_api_key are required"}`, http.StatusBadRequest)
		return
	}

	// Reject obfuscated keys to prevent data corruption
	if strings.Contains(req.OpenAIAPIKey, "...") || strings.Contains(req.OpenAIAPIKey, "****") {
		s.logger.Error("attempted to create project with obfuscated API key", zap.String("request_id", requestID))

		// Audit: project creation failure - obfuscated key
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithDetail("validation_error", "cannot save obfuscated API key"))

		http.Error(w, `{"error":"cannot save obfuscated API key - please provide the full API key"}`, http.StatusBadRequest)
		return
	}
	id := uuid.NewString()
	now := time.Now().UTC()
	project := proxy.Project{
		ID:           id,
		Name:         req.Name,
		OpenAIAPIKey: req.OpenAIAPIKey,
		IsActive:     true, // Projects are active by default
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.projectStore.CreateProject(ctx, project); err != nil {
		s.logger.Error("failed to create project", zap.Error(err), zap.String("name", req.Name), zap.String("request_id", requestID))

		// Audit: project creation failure - store error
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithProjectID(id).
			WithError(err).
			WithDetail("project_name", req.Name))

		http.Error(w, `{"error":"failed to create project"}`, http.StatusInternalServerError)
		return
	}
	s.logger.Info("project created", zap.String("id", id), zap.String("name", req.Name), zap.String("request_id", requestID))

	// Audit: project creation success
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectCreate, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithProjectID(id).
		WithDetail("project_name", req.Name))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(project); err != nil {
		s.logger.Error("failed to encode project response", zap.Error(err))
	}
}

// GET /manage/projects/{id}
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := strings.TrimPrefix(r.URL.Path, "/manage/projects/")
	if id == "" || strings.Contains(id, "/") {
		s.logger.Error("invalid project id", zap.String("id", id))
		http.Error(w, `{"error":"invalid project id"}`, http.StatusBadRequest)
		return
	}
	project, err := s.projectStore.GetProjectByID(ctx, id)
	if err != nil {
		s.logger.Error("project not found", zap.String("id", id), zap.Error(err))
		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	// Create response with obfuscated API key
	response := ProjectResponse{
		ID:            project.ID,
		Name:          project.Name,
		OpenAIAPIKey:  obfuscate.ObfuscateTokenGeneric(project.OpenAIAPIKey),
		IsActive:      project.IsActive,
		DeactivatedAt: project.DeactivatedAt,
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.UpdatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode project response", zap.Error(err))
	}
}

// PATCH /manage/projects/{id}
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	id := strings.TrimPrefix(r.URL.Path, "/manage/projects/")
	if id == "" || strings.Contains(id, "/") {
		s.logger.Error("invalid project id for update", zap.String("id", id))

		// Audit: project update failure - invalid ID
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithDetail("validation_error", "invalid project id").
			WithDetail("provided_id", id))

		http.Error(w, `{"error":"invalid project id"}`, http.StatusBadRequest)
		return
	}
	var req struct {
		Name         *string `json:"name,omitempty"`
		OpenAIAPIKey *string `json:"openai_api_key,omitempty"`
		IsActive     *bool   `json:"is_active,omitempty"`
		RevokeTokens *bool   `json:"revoke_tokens,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("invalid request body for update", zap.Error(err))

		// Audit: project update failure - invalid request body
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithProjectID(id).
			WithError(err).
			WithDetail("validation_error", "invalid request body"))

		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate revoke_tokens usage: it is only valid when explicitly deactivating the project
	if req.RevokeTokens != nil {
		if req.IsActive == nil || (req.IsActive != nil && *req.IsActive) {
			// Audit: invalid field combination
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(id).
				WithDetail("validation_error", "revoke_tokens requires is_active=false"))

			http.Error(w, `{"error":"revoke_tokens requires is_active=false"}`, http.StatusBadRequest)
			return
		}
	}
	project, err := s.projectStore.GetProjectByID(ctx, id)
	if err != nil {
		s.logger.Error("project not found for update", zap.String("id", id), zap.Error(err))

		// Audit: project update failure - not found
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithProjectID(id).
			WithError(err).
			WithDetail("error_type", "project not found"))

		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	// Track what fields are being updated
	var updatedFields []string
	if req.Name != nil {
		project.Name = *req.Name
		updatedFields = append(updatedFields, "name")
	}
	if req.OpenAIAPIKey != nil && *req.OpenAIAPIKey != "" {
		// Reject obfuscated keys to prevent data corruption
		if strings.Contains(*req.OpenAIAPIKey, "...") || strings.Contains(*req.OpenAIAPIKey, "****") {
			s.logger.Error("attempted to save obfuscated API key", zap.String("project_id", id))

			// Audit: project update failure - obfuscated key
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(id).
				WithDetail("validation_error", "cannot save obfuscated API key"))

			http.Error(w, `{"error":"cannot save obfuscated API key - please provide the full API key"}`, http.StatusBadRequest)
			return
		}
		project.OpenAIAPIKey = *req.OpenAIAPIKey
		updatedFields = append(updatedFields, "openai_api_key")
	}

	// Handle project activation/deactivation
	var shouldRevokeTokens bool
	if req.IsActive != nil {
		if *req.IsActive != project.IsActive {
			project.IsActive = *req.IsActive
			updatedFields = append(updatedFields, "is_active")

			// If deactivating project, set deactivated timestamp
			if !*req.IsActive {
				now := time.Now().UTC()
				project.DeactivatedAt = &now
				updatedFields = append(updatedFields, "deactivated_at")

				// Check if tokens should be revoked when deactivating
				if req.RevokeTokens != nil && *req.RevokeTokens {
					shouldRevokeTokens = true
				}
			} else {
				// Reactivating project, clear deactivated timestamp
				project.DeactivatedAt = nil
			}
		}
	}

	project.UpdatedAt = time.Now().UTC()
	if err := s.projectStore.UpdateProject(ctx, project); err != nil {
		s.logger.Error("failed to update project", zap.String("id", id), zap.Error(err))

		// Audit: project update failure - store error
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithProjectID(id).
			WithError(err).
			WithDetail("updated_fields", updatedFields))

		http.Error(w, `{"error":"failed to update project"}`, http.StatusInternalServerError)
		return
	}

	// Revoke project tokens if requested
	var revokedTokensCount int
	if shouldRevokeTokens {
		tokens, err := s.tokenStore.GetTokensByProjectID(ctx, id)
		if err != nil {
			s.logger.Warn("failed to get project tokens for revocation", zap.String("project_id", id), zap.Error(err))
		} else {
			// Revoke all active tokens for this project
			for _, token := range tokens {
				if token.IsActive {
					token.IsActive = false
					token.DeactivatedAt = nowPtrUTC()
					if err := s.tokenStore.UpdateToken(ctx, token); err != nil {
						s.logger.Warn("failed to revoke token during project deactivation",
							zap.String("token_id", token.ID),
							zap.String("project_id", id),
							zap.Error(err))
					} else {
						revokedTokensCount++
					}
				}
			}
		}
	}

	s.logger.Info("project updated", zap.String("id", id), zap.Strings("updated_fields", updatedFields))

	// Audit: project update success
	auditEvent := s.auditEvent(audit.ActionProjectUpdate, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithProjectID(id).
		WithDetail("updated_fields", updatedFields).
		WithDetail("project_name", project.Name)

	if shouldRevokeTokens {
		auditEvent.WithDetail("tokens_revoked", revokedTokensCount)
	}
	_ = s.auditLogger.Log(auditEvent)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(project); err != nil {
		s.logger.Error("failed to encode project response", zap.Error(err))
	}
}

// DELETE /manage/projects/{id}
// DELETE /manage/projects/{id} - Returns 405 Method Not Allowed
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	id := strings.TrimPrefix(r.URL.Path, "/manage/projects/")

	// Audit: project delete attempt - method not allowed
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionProjectDelete, audit.ActorManagement, audit.ResultFailure, r, requestID).
		WithProjectID(id).
		WithDetail("error_type", "method not allowed").
		WithDetail("reason", "project deletion is not permitted"))

	w.Header().Set("Allow", "GET, PATCH")
	http.Error(w, `{"error":"method not allowed","message":"project deletion is not permitted"}`, http.StatusMethodNotAllowed)
}

// POST /manage/projects/{id}/tokens/revoke - Bulk revoke all tokens for a project
func (s *Server) handleBulkRevokeProjectTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	// Extract project ID from path
	pathSuffix := strings.TrimPrefix(r.URL.Path, "/manage/projects/")
	if !strings.HasSuffix(pathSuffix, "/tokens/revoke") {
		s.logger.Error("invalid bulk revoke path", zap.String("path", r.URL.Path), zap.String("request_id", requestID))
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}
	projectID := strings.TrimSuffix(pathSuffix, "/tokens/revoke")
	if projectID == "" {
		s.logger.Error("invalid project ID in bulk revoke path", zap.String("path", r.URL.Path), zap.String("request_id", requestID))
		http.Error(w, `{"error":"invalid path"}`, http.StatusBadRequest)
		return
	}

	// Verify project exists
	_, err := s.projectStore.GetProjectByID(ctx, projectID)
	if err != nil {
		s.logger.Error("project not found for bulk token revoke", zap.String("project_id", projectID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: bulk revoke failure - project not found
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevokeBatch, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithProjectID(projectID).
			WithError(err).
			WithDetail("error_type", "project not found"))

		http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
		return
	}

	// Get all tokens for the project
	tokens, err := s.tokenStore.GetTokensByProjectID(ctx, projectID)
	if err != nil {
		s.logger.Error("failed to get tokens for bulk revoke", zap.String("project_id", projectID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: bulk revoke failure - failed to get tokens
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevokeBatch, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithProjectID(projectID).
			WithError(err).
			WithDetail("error_type", "failed to get tokens"))

		http.Error(w, `{"error":"failed to get project tokens"}`, http.StatusInternalServerError)
		return
	}

	// Count and revoke active tokens
	var revokedCount, alreadyRevokedCount int
	var failedRevocations []string

	for _, token := range tokens {
		if !token.IsActive {
			alreadyRevokedCount++
			continue
		}

		// Revoke the token
		token.IsActive = false
		token.DeactivatedAt = nowPtrUTC()

		if err := s.tokenStore.UpdateToken(ctx, token); err != nil {
			s.logger.Warn("failed to revoke individual token during bulk revoke",
				zap.String("token_id", token.ID),
				zap.String("project_id", projectID),
				zap.Error(err))
			failedRevocations = append(failedRevocations, token.ID)
		} else {
			revokedCount++
		}
	}

	s.logger.Info("bulk token revocation completed",
		zap.String("project_id", projectID),
		zap.Int("revoked_count", revokedCount),
		zap.Int("already_revoked_count", alreadyRevokedCount),
		zap.Int("failed_count", len(failedRevocations)),
		zap.String("request_id", requestID),
	)

	// Audit: bulk revoke success
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevokeBatch, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithProjectID(projectID).
		WithRequestID(requestID).
		WithHTTPMethod(r.Method).
		WithEndpoint(r.URL.Path).
		WithDetail("total_tokens", len(tokens)).
		WithDetail("revoked_count", revokedCount).
		WithDetail("already_revoked_count", alreadyRevokedCount).
		WithDetail("failed_count", len(failedRevocations)))

	// Return summary response
	response := map[string]interface{}{
		"revoked_count":         revokedCount,
		"already_revoked_count": alreadyRevokedCount,
		"total_tokens":          len(tokens),
	}

	if len(failedRevocations) > 0 {
		response["failed_count"] = len(failedRevocations)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode bulk revoke response", zap.Error(err))
	}
}

// Add this helper to *Server
func (s *Server) checkManagementAuth(w http.ResponseWriter, r *http.Request) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	s.logger.Debug("checkManagementAuth: header",
		zap.Bool("present", header != ""),
		zap.Bool("has_bearer_prefix", strings.HasPrefix(header, prefix)),
		zap.Int("header_len", len(header)),
	)
	if !strings.HasPrefix(header, prefix) || len(header) <= len(prefix) {
		s.logger.Debug("checkManagementAuth: missing or invalid prefix")
		http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
		return false
	}
	token := header[len(prefix):]
	s.logger.Debug("checkManagementAuth: token compare",
		zap.Int("provided_len", len(token)),
		zap.Int("expected_len", len(s.config.ManagementToken)),
	)
	if token != s.config.ManagementToken {
		s.logger.Debug("checkManagementAuth: token mismatch")
		http.Error(w, `{"error":"invalid management token"}`, http.StatusUnauthorized)
		return false
	}
	s.logger.Debug("checkManagementAuth: token match")
	return true
}

// Add the handler function
func (s *Server) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	// Check if this is a bulk token revoke request
	if strings.HasSuffix(r.URL.Path, "/tokens/revoke") && r.Method == http.MethodPost {
		s.handleBulkRevokeProjectTokens(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetProject(w, r)
	case http.MethodPatch:
		s.handleUpdateProject(w, r)
	case http.MethodDelete:
		s.handleDeleteProject(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Handler for /manage/tokens (POST: create, GET: list)
func (s *Server) handleTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)
	switch r.Method {
	case http.MethodPost:
		var req struct {
			ProjectID       string `json:"project_id"`
			DurationMinutes int    `json:"duration_minutes"`
			MaxRequests     *int   `json:"max_requests"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.logger.Error("invalid token create request body", zap.Error(err), zap.String("request_id", requestID))

			// Audit: token creation failure - invalid request
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithError(err).
				WithDetail("validation_error", "invalid request body"))

			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		var duration time.Duration
		if req.DurationMinutes > 0 {
			if req.DurationMinutes > maxDurationMinutes {
				s.logger.Error("duration_minutes exceeds maximum allowed", zap.Int("duration_minutes", req.DurationMinutes), zap.String("request_id", requestID))

				// Audit: token creation failure - duration too long
				_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
					WithProjectID(req.ProjectID).
					WithDetail("validation_error", "duration exceeds maximum").
					WithDetail("requested_duration_minutes", req.DurationMinutes).
					WithDetail("max_duration_minutes", maxDurationMinutes))

				http.Error(w, `{"error":"duration_minutes exceeds maximum allowed"}`, http.StatusBadRequest)
				return
			}
			duration = time.Duration(req.DurationMinutes) * time.Minute
		} else {
			s.logger.Error("missing required fields for token create", zap.String("project_id", req.ProjectID), zap.Int("duration_minutes", req.DurationMinutes), zap.String("request_id", requestID))

			// Audit: token creation failure - missing duration
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(req.ProjectID).
				WithDetail("validation_error", "missing duration_minutes"))

			http.Error(w, `{"error":"project_id and duration_minutes are required"}`, http.StatusBadRequest)
			return
		}
		if req.ProjectID == "" {
			s.logger.Error("missing project_id for token create", zap.String("request_id", requestID))

			// Audit: token creation failure - missing project ID
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithDetail("validation_error", "missing project_id"))

			http.Error(w, `{"error":"project_id is required"}`, http.StatusBadRequest)
			return
		}
		if req.MaxRequests != nil {
			if *req.MaxRequests < 0 {
				s.logger.Error("max_requests must be >= 0", zap.Int("max_requests", *req.MaxRequests), zap.String("request_id", requestID))

				// Audit: token creation failure - invalid max_requests
				_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
					WithProjectID(req.ProjectID).
					WithDetail("validation_error", "max_requests must be >= 0").
					WithDetail("requested_max_requests", *req.MaxRequests))

				http.Error(w, `{"error":"max_requests must be >= 0"}`, http.StatusBadRequest)
				return
			}
			// 0 means unlimited.
			if *req.MaxRequests == 0 {
				req.MaxRequests = nil
			}
		}

		// Check project exists and is active
		project, err := s.projectStore.GetProjectByID(ctx, req.ProjectID)
		if err != nil {
			s.logger.Error("project not found for token create", zap.String("project_id", req.ProjectID), zap.Error(err), zap.String("request_id", requestID))

			// Audit: token creation failure - project not found
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(req.ProjectID).
				WithError(err).
				WithDetail("error_type", "project not found"))

			http.Error(w, `{"error":"project not found"}`, http.StatusNotFound)
			return
		}

		// Check if project is active
		if !project.IsActive {
			s.logger.Warn("token creation denied for inactive project", zap.String("project_id", req.ProjectID), zap.String("request_id", requestID))

			// Audit: token creation failure - project inactive
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(req.ProjectID).
				WithDetail("error_type", "project_inactive").
				WithDetail("reason", "cannot create tokens for inactive projects"))

			http.Error(w, `{"error":"cannot create tokens for inactive projects","code":"project_inactive"}`, http.StatusForbidden)
			return
		}
		// Generate token ID (UUID) and token string
		tokenID := uuid.New().String()
		tokenStr, expiresAt, _, err := token.NewTokenGenerator().GenerateWithOptions(duration, req.MaxRequests)
		if err != nil {
			s.logger.Error("failed to generate token", zap.Error(err), zap.String("request_id", requestID))

			// Audit: token creation failure - generation error
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(req.ProjectID).
				WithError(err).
				WithDetail("error_type", "token generation failed"))

			http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
			return
		}
		now := time.Now().UTC()
		dbToken := token.TokenData{
			ID:           tokenID,
			Token:        tokenStr,
			ProjectID:    req.ProjectID,
			ExpiresAt:    expiresAt,
			IsActive:     true,
			RequestCount: 0,
			MaxRequests:  req.MaxRequests,
			CreatedAt:    now,
		}
		if err := s.tokenStore.CreateToken(ctx, dbToken); err != nil {
			s.logger.Error("failed to store token", zap.Error(err), zap.String("request_id", requestID))

			// Audit: token creation failure - storage error
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithProjectID(req.ProjectID).
				WithTokenID(tokenID).
				WithError(err).
				WithDetail("error_type", "storage failed"))

			http.Error(w, `{"error":"failed to store token"}`, http.StatusInternalServerError)
			return
		}
		s.logger.Info("token created",
			zap.String("token", token.ObfuscateToken(tokenStr)),
			zap.String("project_id", req.ProjectID),
			zap.String("request_id", requestID),
		)

		// Audit: token creation success
		auditEvent := s.auditEvent(audit.ActionTokenCreate, audit.ActorManagement, audit.ResultSuccess, r, requestID).
			WithProjectID(req.ProjectID).
			WithRequestID(requestID).
			WithHTTPMethod(r.Method).
			WithEndpoint(r.URL.Path).
			WithTokenID(tokenID).
			WithDetail("duration_minutes", req.DurationMinutes).
			WithDetail("expires_at", expiresAt.Format(time.RFC3339))
		if req.MaxRequests != nil {
			auditEvent.WithDetail("max_requests", *req.MaxRequests)
		}
		_ = s.auditLogger.Log(auditEvent)

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"id":         tokenID,
			"token":      tokenStr,
			"expires_at": expiresAt,
		}
		if req.MaxRequests != nil {
			response["max_requests"] = *req.MaxRequests
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			s.logger.Error("failed to encode token response", zap.Error(err))
		}
	case http.MethodGet:
		projectID := r.URL.Query().Get("projectId")
		var tokens []token.TokenData
		var err error
		if projectID != "" {
			tokens, err = s.tokenStore.GetTokensByProjectID(ctx, projectID)
		} else {
			tokens, err = s.tokenStore.ListTokens(ctx)
		}
		if err != nil {
			s.logger.Error("failed to list tokens", zap.Error(err))

			// Audit: token list failure
			auditEvent := s.auditEvent(audit.ActionTokenList, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithRequestID(requestID).
				WithHTTPMethod(r.Method).
				WithEndpoint(r.URL.Path).
				WithError(err)
			if projectID != "" {
				auditEvent.WithProjectID(projectID)
			}
			_ = s.auditLogger.Log(auditEvent)

			http.Error(w, `{"error":"failed to list tokens"}`, http.StatusInternalServerError)
			return
		}
		s.logger.Info("tokens listed", zap.Int("count", len(tokens)))

		// Audit: token list success
		auditEvent := s.auditEvent(audit.ActionTokenList, audit.ActorManagement, audit.ResultSuccess, r, requestID).
			WithRequestID(requestID).
			WithHTTPMethod(r.Method).
			WithEndpoint(r.URL.Path).
			WithDetail("token_count", len(tokens))
		if projectID != "" {
			auditEvent.WithProjectID(projectID).WithDetail("filtered_by_project", true)
		}
		_ = s.auditLogger.Log(auditEvent)

		w.Header().Set("Content-Type", "application/json")

		// Create sanitized response with token IDs and obfuscated token strings
		sanitizedTokens := make([]TokenListResponse, len(tokens))
		for i, t := range tokens {
			sanitizedTokens[i] = TokenListResponse{
				ID:            t.ID,
				Token:         token.ObfuscateToken(t.Token),
				ProjectID:     t.ProjectID,
				ExpiresAt:     t.ExpiresAt,
				IsActive:      t.IsActive,
				RequestCount:  t.RequestCount,
				MaxRequests:   t.MaxRequests,
				CreatedAt:     t.CreatedAt,
				LastUsedAt:    t.LastUsedAt,
				CacheHitCount: t.CacheHitCount,
			}
		}

		if err := json.NewEncoder(w).Encode(sanitizedTokens); err != nil {
			s.logger.Error("failed to encode tokens response", zap.Error(err))
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Handler for /manage/tokens/{id} (GET: retrieve, PATCH: update, DELETE: revoke)
func (s *Server) handleTokenByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	// Extract token ID from path
	tokenID := strings.TrimPrefix(r.URL.Path, "/manage/tokens/")
	if tokenID == "" || tokenID == "/" {
		s.logger.Error("invalid token ID in path", zap.String("path", r.URL.Path), zap.String("request_id", requestID))
		http.Error(w, `{"error":"token ID is required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetToken(w, r, tokenID)
	case http.MethodPatch:
		s.handleUpdateToken(w, r, tokenID)
	case http.MethodDelete:
		s.handleRevokeToken(w, r, tokenID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// GET /manage/tokens/{id}
func (s *Server) handleGetToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	// Get token from store
	tokenData, err := s.tokenStore.GetTokenByID(ctx, tokenID)
	if err != nil {
		s.logger.Error("failed to get token", zap.String("token_id", tokenID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: token get failure
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRead, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithError(err).
			WithDetail("error_type", "token not found"))

		http.Error(w, `{"error":"token not found"}`, http.StatusNotFound)
		return
	}

	// Audit: token get success
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRead, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithTokenID(tokenID).
		WithProjectID(tokenData.ProjectID).
		WithRequestID(requestID).
		WithHTTPMethod(r.Method).
		WithEndpoint(r.URL.Path))

	// Create sanitized response with ID and obfuscated token string
	response := TokenListResponse{
		ID:           tokenData.ID,
		Token:        token.ObfuscateToken(tokenData.Token),
		ProjectID:    tokenData.ProjectID,
		ExpiresAt:    tokenData.ExpiresAt,
		IsActive:     tokenData.IsActive,
		RequestCount: tokenData.RequestCount,
		MaxRequests:  tokenData.MaxRequests,
		CreatedAt:    tokenData.CreatedAt,
		LastUsedAt:   tokenData.LastUsedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode token response", zap.Error(err))
	}
}

// PATCH /manage/tokens/{id}
func (s *Server) handleUpdateToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	// Parse request body
	var req struct {
		IsActive    *bool `json:"is_active,omitempty"`
		MaxRequests *int  `json:"max_requests,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("invalid token update request body", zap.Error(err), zap.String("request_id", requestID))

		// Audit: token update failure - invalid request
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithError(err).
			WithDetail("validation_error", "invalid request body"))

		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Get existing token
	tokenData, err := s.tokenStore.GetTokenByID(ctx, tokenID)
	if err != nil {
		s.logger.Error("failed to get token for update", zap.String("token_id", tokenID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: token update failure - token not found
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithError(err).
			WithDetail("error_type", "token not found"))

		http.Error(w, `{"error":"token not found"}`, http.StatusNotFound)
		return
	}

	// Normalize max_requests semantics:
	// - negative: invalid
	// - 0: unlimited (stored as nil)
	maxRequestsProvided := req.MaxRequests != nil
	normalizedMaxRequests := req.MaxRequests
	if maxRequestsProvided {
		if *req.MaxRequests < 0 {
			s.logger.Error("max_requests must be >= 0", zap.Int("max_requests", *req.MaxRequests), zap.String("request_id", requestID))

			_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithTokenID(tokenID).
				WithError(fmt.Errorf("max_requests must be >= 0")))
			http.Error(w, `{"error":"max_requests must be >= 0"}`, http.StatusBadRequest)
			return
		}
		if *req.MaxRequests == 0 {
			normalizedMaxRequests = nil
		}
	}

	// Update fields if provided
	updated := false
	if req.IsActive != nil {
		tokenData.IsActive = *req.IsActive
		updated = true
	}
	if maxRequestsProvided {
		tokenData.MaxRequests = normalizedMaxRequests
		updated = true
	}

	if !updated {
		s.logger.Error("no fields to update", zap.String("token_id", tokenID), zap.String("request_id", requestID))

		// Audit: token update failure - no fields
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithDetail("validation_error", "no fields to update"))

		http.Error(w, `{"error":"no fields to update"}`, http.StatusBadRequest)
		return
	}

	// Update token in store
	if err := s.tokenStore.UpdateToken(ctx, tokenData); err != nil {
		s.logger.Error("failed to update token", zap.String("token_id", tokenID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: token update failure - storage error
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenUpdate, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithProjectID(tokenData.ProjectID).
			WithError(err).
			WithDetail("error_type", "storage failed"))

		http.Error(w, `{"error":"failed to update token"}`, http.StatusInternalServerError)
		return
	}

	s.logger.Info("token updated",
		zap.String("token_id", tokenID),
		zap.String("project_id", tokenData.ProjectID),
		zap.String("request_id", requestID),
	)

	// Audit: token update success
	auditEvent := s.auditEvent(audit.ActionTokenUpdate, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithTokenID(tokenID).
		WithProjectID(tokenData.ProjectID).
		WithRequestID(requestID).
		WithHTTPMethod(r.Method).
		WithEndpoint(r.URL.Path)

	if req.IsActive != nil {
		auditEvent.WithDetail("updated_is_active", *req.IsActive)
	}
	if maxRequestsProvided && normalizedMaxRequests != nil {
		auditEvent.WithDetail("updated_max_requests", *normalizedMaxRequests)
	}
	if maxRequestsProvided && normalizedMaxRequests == nil {
		auditEvent.WithDetail("updated_max_requests", "unlimited")
	}
	_ = s.auditLogger.Log(auditEvent)

	// Return updated token (sanitized with ID and obfuscated token)
	response := TokenListResponse{
		ID:           tokenData.ID,
		Token:        token.ObfuscateToken(tokenData.Token),
		ProjectID:    tokenData.ProjectID,
		ExpiresAt:    tokenData.ExpiresAt,
		IsActive:     tokenData.IsActive,
		RequestCount: tokenData.RequestCount,
		MaxRequests:  tokenData.MaxRequests,
		CreatedAt:    tokenData.CreatedAt,
		LastUsedAt:   tokenData.LastUsedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode updated token response", zap.Error(err))
	}
}

// DELETE /manage/tokens/{id} (revoke token)
func (s *Server) handleRevokeToken(w http.ResponseWriter, r *http.Request, tokenID string) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	// Get existing token first to verify it exists and get project ID
	tokenData, err := s.tokenStore.GetTokenByID(ctx, tokenID)
	if err != nil {
		s.logger.Error("failed to get token for revocation", zap.String("token_id", tokenID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: token revoke failure - token not found
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevoke, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithError(err).
			WithDetail("error_type", "token not found"))

		http.Error(w, `{"error":"token not found"}`, http.StatusNotFound)
		return
	}

	// Check if already inactive
	if !tokenData.IsActive {
		s.logger.Warn("token already revoked", zap.String("token_id", tokenID), zap.String("request_id", requestID))

		// Audit: token revoke success (idempotent)
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevoke, audit.ActorManagement, audit.ResultSuccess, r, requestID).
			WithTokenID(tokenID).
			WithProjectID(tokenData.ProjectID).
			WithRequestID(requestID).
			WithHTTPMethod(r.Method).
			WithEndpoint(r.URL.Path).
			WithDetail("already_revoked", true))

		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Revoke token by setting is_active to false
	tokenData.IsActive = false
	tokenData.DeactivatedAt = nowPtrUTC()

	if err := s.tokenStore.UpdateToken(ctx, tokenData); err != nil {
		s.logger.Error("failed to revoke token", zap.String("token_id", tokenID), zap.Error(err), zap.String("request_id", requestID))

		// Audit: token revoke failure - storage error
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevoke, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithTokenID(tokenID).
			WithProjectID(tokenData.ProjectID).
			WithError(err).
			WithDetail("error_type", "storage failed"))

		http.Error(w, `{"error":"failed to revoke token"}`, http.StatusInternalServerError)
		return
	}

	s.logger.Info("token revoked",
		zap.String("token_id", tokenID),
		zap.String("project_id", tokenData.ProjectID),
		zap.String("request_id", requestID),
	)

	// Audit: token revoke success
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionTokenRevoke, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithTokenID(tokenID).
		WithProjectID(tokenData.ProjectID).
		WithRequestID(requestID).
		WithHTTPMethod(r.Method).
		WithEndpoint(r.URL.Path))

	w.WriteHeader(http.StatusNoContent)
}

func getRequestID(ctx context.Context) string {
	if requestID, ok := logging.GetRequestID(ctx); ok && requestID != "" {
		return requestID
	}
	return uuid.New().String()
}

// nowPtrUTC returns the current UTC time as a *time.Time convenience helper.
func nowPtrUTC() *time.Time {
	t := time.Now().UTC()
	return &t
}

// parseInt parses a string to an integer with a default value
func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultValue
	}
	return result
}

// logRequestMiddleware logs all incoming requests with timing information using structured logging
func (s *Server) logRequestMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		// Get or generate request ID from header
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Get or generate correlation ID from header
		correlationID := r.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		// Add to context using our new context helpers
		ctx := logging.WithRequestID(r.Context(), requestID)
		ctx = logging.WithCorrelationID(ctx, correlationID)

		// Set response headers
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Correlation-ID", correlationID)

		// Create a response writer that captures status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Get client IP
		clientIP := s.getClientIP(r)

		// Create logger with request context
		reqLogger := logging.WithRequestContext(ctx, s.logger)
		reqLogger = logging.WithCorrelationContext(ctx, reqLogger)

		reqLogger.Info("request started",
			logging.ClientIP(clientIP),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("user_agent", r.UserAgent()),
		)

		// Call the next handler
		next(rw, r.WithContext(ctx))

		duration := time.Since(startTime)
		durationMs := int(duration.Milliseconds())

		// Log completion with canonical fields
		if rw.statusCode >= 500 {
			reqLogger.Error("request completed with server error",
				logging.RequestFields(requestID, r.Method, r.URL.Path, rw.statusCode, durationMs)...,
			)
		} else {
			reqLogger.Info("request completed",
				logging.RequestFields(requestID, r.Method, r.URL.Path, rw.statusCode, durationMs)...,
			)
		}
	}
}

// getClientIP extracts the client IP address from the request
func (s *Server) getClientIP(r *http.Request) string {
	// Check for X-Forwarded-For header first (in case of proxy)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check for X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}
	return r.RemoteAddr
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Add Flush forwarding for streaming support
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// handleNotFound is a catch-all handler for unmatched routes
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("route not found",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.String("remote_addr", r.RemoteAddr),
	)
	http.NotFound(w, r)
}

// EventBus returns the event bus used by the server (may be nil if observability is disabled)
func (s *Server) EventBus() eventbus.EventBus {
	return s.eventBus
}

// auditEvent creates a new audit event with common fields filled from the HTTP request
func (s *Server) auditEvent(action string, actor string, result audit.ResultType, r *http.Request, requestID string) *audit.Event {
	clientIP := s.getClientIP(r)
	// Prefer forwarded UA and referer from Admin UI if present
	forwardedUA := r.Header.Get("X-Forwarded-User-Agent")
	userAgent := r.UserAgent()
	if forwardedUA != "" {
		userAgent = forwardedUA
	}
	forwardedRef := r.Header.Get("X-Forwarded-Referer")

	ev := audit.NewEvent(action, actor, result).
		WithRequestID(requestID).
		WithHTTPMethod(r.Method).
		WithEndpoint(r.URL.Path).
		WithClientIP(clientIP).
		WithUserAgent(userAgent)
	if forwardedRef != "" {
		ev = ev.WithDetail("referer", forwardedRef)
	}
	if r.Header.Get("X-Admin-Origin") == "1" {
		ev = ev.WithDetail("origin", "admin-ui")
	}
	return ev
}

// Handler for /manage/audit (GET: list)
func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only proceed if we have database access
	if s.db == nil {
		s.logger.Error("audit events requested but database not available", zap.String("request_id", requestID))
		http.Error(w, `{"error":"audit events not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for filtering
	query := r.URL.Query()
	filters := database.AuditEventFilters{
		Action:        query.Get("action"),
		ClientIP:      query.Get("client_ip"),
		ProjectID:     query.Get("project_id"),
		Outcome:       query.Get("outcome"),
		Actor:         query.Get("actor"),
		RequestID:     query.Get("request_id"),
		CorrelationID: query.Get("correlation_id"),
		Method:        query.Get("method"),
		Path:          query.Get("path"),
		Search:        query.Get("search"),
	}

	// Parse time filters
	if startTime := query.Get("start_time"); startTime != "" {
		filters.StartTime = &startTime
	}
	if endTime := query.Get("end_time"); endTime != "" {
		filters.EndTime = &endTime
	}

	// Parse pagination
	page := parseInt(query.Get("page"), 1)
	pageSize := parseInt(query.Get("page_size"), 20)
	if pageSize > 100 {
		pageSize = 100 // Limit page size
	}
	filters.Limit = pageSize
	filters.Offset = (page - 1) * pageSize

	// Get audit events
	events, err := s.db.ListAuditEvents(ctx, filters)
	if err != nil {
		s.logger.Error("failed to list audit events", zap.Error(err), zap.String("request_id", requestID))
		http.Error(w, `{"error":"failed to list audit events"}`, http.StatusInternalServerError)
		return
	}

	// Get total count for pagination
	totalCount, err := s.db.CountAuditEvents(ctx, filters)
	if err != nil {
		s.logger.Error("failed to count audit events", zap.Error(err), zap.String("request_id", requestID))
		http.Error(w, `{"error":"failed to count audit events"}`, http.StatusInternalServerError)
		return
	}

	// Calculate pagination info
	totalPages := (totalCount + pageSize - 1) / pageSize
	hasNext := page < totalPages
	hasPrev := page > 1

	response := map[string]interface{}{
		"events": events,
		"pagination": map[string]interface{}{
			"page":        page,
			"page_size":   pageSize,
			"total_count": totalCount,
			"total_pages": totalPages,
			"has_next":    hasNext,
			"has_prev":    hasPrev,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode audit events response", zap.Error(err), zap.String("request_id", requestID))
	}

	// Audit: successful audit events listing
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionAuditList, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithDetail("events_count", len(events)).
		WithDetail("page", page).
		WithDetail("page_size", pageSize).
		WithDetail("total_count", totalCount))
}

// Handler for /manage/audit/{id} (GET: show)
func (s *Server) handleAuditEventByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := getRequestID(ctx)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Only proceed if we have database access
	if s.db == nil {
		s.logger.Error("audit event requested but database not available", zap.String("request_id", requestID))
		http.Error(w, `{"error":"audit event not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Extract audit event ID from path
	id := strings.TrimPrefix(r.URL.Path, "/manage/audit/")
	if id == "" {
		http.Error(w, `{"error":"audit event ID is required"}`, http.StatusBadRequest)
		return
	}

	// Get specific audit event by ID
	event, err := s.db.GetAuditEventByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"audit event not found"}`, http.StatusNotFound)
		} else {
			s.logger.Error("failed to get audit event", zap.Error(err), zap.String("audit_id", id), zap.String("request_id", requestID))
			http.Error(w, `{"error":"failed to get audit event"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(event); err != nil {
		s.logger.Error("failed to encode audit event response", zap.Error(err), zap.String("audit_id", id), zap.String("request_id", requestID))
	}

	// Audit: successful audit event retrieval
	_ = s.auditLogger.Log(s.auditEvent(audit.ActionAuditShow, audit.ActorManagement, audit.ResultSuccess, r, requestID).
		WithDetail("audit_event_id", id))
}

// CachePurgeRequest represents the request body for cache purge operations
type CachePurgeRequest struct {
	Method string `json:"method" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Prefix string `json:"prefix,omitempty"`
}

// CachePurgeResponse represents the response body for cache purge operations
type CachePurgeResponse struct {
	Deleted interface{} `json:"deleted"` // bool for exact purge, int for prefix purge
}

// Handler for POST /manage/cache/purge
func (s *Server) handleCachePurge(w http.ResponseWriter, r *http.Request) {
	requestID := getRequestID(r.Context())

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Check if proxy and cache are available
	if s.proxy == nil {
		s.logger.Error("proxy not initialized", zap.String("request_id", requestID))
		http.Error(w, `{"error":"proxy not available"}`, http.StatusInternalServerError)
		return
	}

	cache := s.proxy.Cache()
	if cache == nil {
		s.logger.Warn("cache purge attempted but caching is disabled", zap.String("request_id", requestID))
		http.Error(w, `{"error":"caching is disabled"}`, http.StatusBadRequest)
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionCachePurge, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithDetail("reason", "caching_disabled"))
		return
	}

	var req CachePurgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Warn("invalid JSON in cache purge request", zap.Error(err), zap.String("request_id", requestID))
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionCachePurge, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithDetail("reason", "invalid_json"))
		return
	}

	// Validate required fields
	if req.Method == "" || req.URL == "" {
		s.logger.Warn("missing required fields in cache purge request",
			zap.String("method", req.Method), zap.String("url", req.URL), zap.String("request_id", requestID))
		http.Error(w, `{"error":"method and url are required"}`, http.StatusBadRequest)
		_ = s.auditLogger.Log(s.auditEvent(audit.ActionCachePurge, audit.ActorManagement, audit.ResultFailure, r, requestID).
			WithDetail("reason", "missing_fields"))
		return
	}

	var response CachePurgeResponse
	var auditDetails map[string]interface{}

	if req.Prefix != "" {
		// Prefix purge
		deleted := cache.PurgePrefix(req.Prefix)
		response.Deleted = deleted
		auditDetails = map[string]interface{}{
			"purge_type": "prefix",
			"prefix":     req.Prefix,
			"deleted":    deleted,
		}
		s.logger.Info("cache prefix purge completed",
			zap.String("prefix", req.Prefix), zap.Int("deleted", deleted), zap.String("request_id", requestID))
	} else {
		// Exact key purge - need to compute cache key from method and URL
		// Create a mock request to generate the cache key
		mockURL, err := url.Parse(req.URL)
		if err != nil {
			s.logger.Warn("invalid URL in cache purge request", zap.Error(err), zap.String("url", req.URL), zap.String("request_id", requestID))
			http.Error(w, `{"error":"invalid URL"}`, http.StatusBadRequest)
			_ = s.auditLogger.Log(s.auditEvent(audit.ActionCachePurge, audit.ActorManagement, audit.ResultFailure, r, requestID).
				WithDetail("reason", "invalid_url"))
			return
		}

		mockReq := &http.Request{
			Method: req.Method,
			URL:    mockURL,
			Header: make(http.Header),
		}

		// Generate cache key using existing helper
		cacheKey := proxy.CacheKeyFromRequest(mockReq)
		deleted := cache.Purge(cacheKey)
		response.Deleted = deleted
		auditDetails = map[string]interface{}{
			"purge_type": "exact",
			"method":     req.Method,
			"url":        req.URL,
			"cache_key":  cacheKey,
			"deleted":    deleted,
		}
		s.logger.Info("cache exact purge completed",
			zap.String("method", req.Method), zap.String("url", req.URL),
			zap.String("cache_key", cacheKey), zap.Bool("deleted", deleted), zap.String("request_id", requestID))
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode cache purge response", zap.Error(err), zap.String("request_id", requestID))
		return
	}

	// Audit: successful cache purge
	auditEvent := s.auditEvent(audit.ActionCachePurge, audit.ActorManagement, audit.ResultSuccess, r, requestID)
	for k, v := range auditDetails {
		auditEvent = auditEvent.WithDetail(k, v)
	}
	_ = s.auditLogger.Log(auditEvent)
}
