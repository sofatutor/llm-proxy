# Proxy Package

## Purpose & Responsibilities

The `proxy` package implements a transparent HTTP reverse proxy for LLM APIs. It handles:

- Transparent request forwarding to upstream APIs (OpenAI, Anthropic, etc.)
- Token-based authentication and authorization
- Request validation and endpoint whitelisting
- HTTP response caching (in-memory and Redis backends)
- Server-Sent Events (SSE) streaming support
- Metadata extraction from responses
- Observability event publishing

## Key Types & Interfaces

| Type | Description |
|------|-------------|
| `TransparentProxy` | Main proxy implementation with caching and metrics |
| `ProxyConfig` | Configuration for proxy behavior |
| `ProxyMetrics` | Runtime metrics (requests, errors, cache stats) |
| `TokenValidator` | Interface for token validation |
| `ProjectStore` | Interface for project data access |
| `AuditLogger` | Interface for audit event logging |

### Core Interfaces

```go
// TokenValidator validates tokens and returns project IDs
type TokenValidator interface {
    ValidateToken(ctx context.Context, token string) (string, error)
    ValidateTokenWithTracking(ctx context.Context, token string) (string, error)
}

// ProjectStore retrieves project information
type ProjectStore interface {
    GetAPIKeyForProject(ctx context.Context, projectID string) (string, error)
    GetProjectActive(ctx context.Context, projectID string) (bool, error)
    ListProjects(ctx context.Context) ([]Project, error)
    CreateProject(ctx context.Context, project Project) error
    GetProjectByID(ctx context.Context, projectID string) (Project, error)
    UpdateProject(ctx context.Context, project Project) error
    DeleteProject(ctx context.Context, projectID string) error
}
```

### ProxyConfig Structure

```go
type ProxyConfig struct {
    TargetBaseURL         string            // Upstream API base URL
    AllowedEndpoints      []string          // Whitelisted endpoint paths
    AllowedMethods        []string          // Whitelisted HTTP methods
    RequestTimeout        time.Duration     // Max request duration
    ResponseHeaderTimeout time.Duration     // Time to wait for headers
    FlushInterval         time.Duration     // Streaming flush interval
    MaxIdleConns          int               // Connection pool size
    HTTPCacheEnabled      bool              // Enable response caching
    RedisCacheURL         string            // Redis cache connection URL
    EnforceProjectActive  bool              // Require active project status
}
```

## Usage Examples

### Basic Proxy Setup

```go
package main

import (
    "net/http"

    "github.com/sofatutor/llm-proxy/internal/proxy"
    "github.com/sofatutor/llm-proxy/internal/token"
)

func main() {
    config := proxy.ProxyConfig{
        TargetBaseURL: "https://api.openai.com",
        AllowedEndpoints: []string{
            "/v1/chat/completions",
            "/v1/embeddings",
            "/v1/models",
        },
        AllowedMethods:   []string{"GET", "POST"},
        RequestTimeout:   120 * time.Second,
        HTTPCacheEnabled: true,
    }

    // Create validator and store (use your implementations)
    validator := token.NewValidator(tokenStore)
    projectStore := database.NewProjectStore(db)

    // Create proxy
    p, err := proxy.NewTransparentProxy(config, validator, projectStore)
    if err != nil {
        log.Fatal(err)
    }

    // Use as HTTP handler
    http.Handle("/", p.Handler())
    http.ListenAndServe(":8080", nil)
}
```

### With Custom Logger

```go
import "go.uber.org/zap"

logger, _ := zap.NewProduction()

p, err := proxy.NewTransparentProxyWithLogger(
    config,
    validator,
    projectStore,
    logger,
)
```

### With Observability

```go
import "github.com/sofatutor/llm-proxy/internal/middleware"

obsCfg := middleware.ObservabilityConfig{
    Enabled:  true,
    EventBus: eventBus,
}

p, err := proxy.NewTransparentProxyWithObservability(
    config,
    validator,
    projectStore,
    obsCfg,
)
```

## Streaming Support (SSE)

The proxy fully supports Server-Sent Events for streaming responses:

- Automatic detection of streaming requests via `Accept: text/event-stream`
- Proper header handling for SSE (`Cache-Control: no-cache`)
- Configurable flush interval for real-time streaming
- Response capture for streaming with size limits

```go
// Streaming requests are automatically detected and handled
// The proxy sets appropriate headers and flushes incrementally
config := proxy.ProxyConfig{
    FlushInterval: 100 * time.Millisecond, // Flush every 100ms
}
```

## HTTP Caching

The proxy includes built-in HTTP caching with two backends:

### In-Memory Cache (Default)

```go
config := proxy.ProxyConfig{
    HTTPCacheEnabled: true,
    // In-memory cache is used by default
}
```

### Redis Cache

```go
config := proxy.ProxyConfig{
    HTTPCacheEnabled:    true,
    RedisCacheURL:       "redis://localhost:6379/0",
    RedisCacheKeyPrefix: "llmproxy:cache:",
}
```

### Cache Key Generation

Cache keys incorporate:
- Request method and path
- Query parameters
- Request body hash (for POST requests)
- Vary header values from upstream responses

```go
// Generate cache key for a request
key := proxy.CacheKeyFromRequest(req)

// With Vary header support
key := proxy.CacheKeyFromRequestWithVary(req, "Authorization, Accept")
```

### Cache Metrics

```go
metrics := p.Metrics()
fmt.Printf("Cache Hits: %d\n", metrics.CacheHits)
fmt.Printf("Cache Misses: %d\n", metrics.CacheMisses)
fmt.Printf("Cache Bypass: %d\n", metrics.CacheBypass)
fmt.Printf("Cache Stores: %d\n", metrics.CacheStores)
```

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `HTTP_CACHE_ENABLED` | Enable HTTP cache (`true`/`false`) |
| `HTTP_CACHE_BACKEND` | Cache backend (`redis` or in-memory) |
| `REDIS_CACHE_URL` | Redis URL for cache backend |
| `REDIS_CACHE_KEY_PREFIX` | Key prefix for Redis cache |

### API Configuration File

The proxy supports YAML configuration for multiple API providers:

```yaml
default_api: openai
apis:
  openai:
    base_url: https://api.openai.com
    allowed_endpoints:
      - /v1/chat/completions
      - /v1/embeddings
    allowed_methods:
      - GET
      - POST
    timeouts:
      request: 120s
      response_header: 30s
    connection:
      max_idle_conns: 100
      max_idle_conns_per_host: 20
```

## Testing Guidance

### Unit Testing with Mocks

```go
package proxy_test

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/sofatutor/llm-proxy/internal/proxy"
)

// MockValidator for testing
type MockValidator struct {
    projectID string
    err       error
}

func (m *MockValidator) ValidateToken(ctx context.Context, token string) (string, error) {
    return m.projectID, m.err
}

func (m *MockValidator) ValidateTokenWithTracking(ctx context.Context, token string) (string, error) {
    return m.projectID, m.err
}

// MockProjectStore for testing
type MockProjectStore struct {
    apiKey   string
    isActive bool
}

func (m *MockProjectStore) GetAPIKeyForProject(ctx context.Context, id string) (string, error) {
    return m.apiKey, nil
}

func (m *MockProjectStore) GetProjectActive(ctx context.Context, id string) (bool, error) {
    return m.isActive, nil
}

func TestProxyRequest(t *testing.T) {
    // Create mock upstream server
    upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"response": "ok"}`))
    }))
    defer upstream.Close()

    config := proxy.ProxyConfig{
        TargetBaseURL:    upstream.URL,
        AllowedEndpoints: []string{"/v1/chat/completions"},
        AllowedMethods:   []string{"POST"},
    }

    validator := &MockValidator{projectID: "test-project"}
    store := &MockProjectStore{apiKey: "sk-test", isActive: true}

    p, err := proxy.NewTransparentProxy(config, validator, store)
    if err != nil {
        t.Fatal(err)
    }

    req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
    req.Header.Set("Authorization", "Bearer sk-valid-token")
    rec := httptest.NewRecorder()

    p.Handler().ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rec.Code)
    }
}
```

### Integration Testing

```go
func TestProxyIntegration(t *testing.T) {
    // Setup test database and stores
    db, _ := database.New(database.Config{Path: ":memory:"})
    defer db.Close()

    tokenStore := database.NewTokenStore(db)
    projectStore := database.NewProjectStore(db)

    // Create test project and token
    project := database.Project{ID: "proj-1", Name: "Test", IsActive: true}
    projectStore.CreateProject(context.Background(), project)

    // Test proxy with real stores
    config := proxy.ProxyConfig{
        TargetBaseURL:        "https://api.openai.com",
        AllowedEndpoints:     []string{"/v1/models"},
        AllowedMethods:       []string{"GET"},
        EnforceProjectActive: true,
    }

    validator := token.NewCachedValidator(token.NewValidator(tokenStore))
    p, _ := proxy.NewTransparentProxy(config, validator, projectStore)

    // Make test request...
}
```

## Troubleshooting

### Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| 401 Unauthorized | Invalid or expired token | Check token format and validity |
| 403 Forbidden | Project inactive or endpoint not allowed | Verify project status and AllowedEndpoints |
| 502 Bad Gateway | Upstream API unreachable | Check network and TargetBaseURL |
| 504 Gateway Timeout | Request exceeded timeout | Increase RequestTimeout |
| Cache not working | Cache disabled or misconfigured | Verify HTTPCacheEnabled and backend config |

### Debug Headers

The proxy adds debug headers to responses:

| Header | Description |
|--------|-------------|
| `X-Request-ID` | Unique request identifier |
| `X-Cache-Status` | Cache hit/miss/bypass status |
| `X-Cache-Debug` | Additional cache debug info |
| `X-Upstream-Response-Time` | Upstream response time in ms |

### Logging

Enable debug logging for troubleshooting:

```go
config := proxy.ProxyConfig{
    LogLevel:  "debug",
    LogFormat: "console", // or "json"
}
```

## Related Packages

| Package | Relationship |
|---------|--------------|
| [`server`](../server/README.md) | Uses proxy as handler for API requests |
| [`token`](../token/README.md) | Token validation interface |
| [`database`](../database/README.md) | Project and token storage |
| [`middleware`](../middleware/) | Observability middleware integration |
| [`eventbus`](../eventbus/) | Event publishing for observability |
| [`audit`](../audit/) | Audit logging integration |

## Files

| File | Description |
|------|-------------|
| `proxy.go` | Main TransparentProxy implementation |
| `interfaces.go` | Interface definitions and ProxyConfig |
| `cache.go` | In-memory cache implementation |
| `cache_redis.go` | Redis cache implementation |
| `cache_helpers.go` | Cache key generation and helpers |
| `cache_stats.go` | Cache statistics aggregation |
| `stream_capture.go` | Streaming response capture |
| `config_schema.go` | API configuration loading |
| `project_guard.go` | Project active status enforcement |
| `circuitbreaker.go` | Circuit breaker for upstream failures |