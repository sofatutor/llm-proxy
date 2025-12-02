# Server Package

## Purpose & Responsibilities

The `server` package provides HTTP server lifecycle management for the LLM Proxy. It handles:

- HTTP server creation, configuration, and startup
- Graceful shutdown with resource cleanup
- Route registration for health, management, and proxy endpoints
- Management API endpoints for projects, tokens, and audit
- Request logging and middleware composition
- Integration with event bus, audit logging, and proxy components

## Key Types & Interfaces

| Type | Description |
|------|-------------|
| `Server` | Main HTTP server struct managing lifecycle, routes, and dependencies |
| `HealthResponse` | JSON response for health check endpoint |
| `Metrics` | Runtime metrics including request/error counts and start time |

### Server Struct

```go
type Server struct {
    config       *config.Config
    server       *http.Server
    tokenStore   token.TokenStore
    projectStore proxy.ProjectStore
    logger       *zap.Logger
    proxy        *proxy.TransparentProxy
    metrics      Metrics
    eventBus     eventbus.EventBus
    auditLogger  *audit.Logger
    db           *database.DB
}
```

### Constructor Functions

| Function | Description |
|----------|-------------|
| `New(cfg, tokenStore, projectStore)` | Creates server without database |
| `NewWithDatabase(cfg, tokenStore, projectStore, db)` | Creates server with database for audit logging |

## Usage Examples

### Basic Server Setup

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/sofatutor/llm-proxy/internal/config"
    "github.com/sofatutor/llm-proxy/internal/database"
    "github.com/sofatutor/llm-proxy/internal/server"
)

func main() {
    // Load configuration
    cfg := config.DefaultConfig()
    cfg.ListenAddr = ":8080"
    cfg.ManagementToken = "secret-token"

    // Initialize database
    db, err := database.New(database.DefaultConfig())
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    // Create token and project stores from database
    tokenStore := database.NewTokenStore(db)
    projectStore := database.NewProjectStore(db)

    // Create server with database support
    srv, err := server.NewWithDatabase(&cfg, tokenStore, projectStore, db)
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }

    // Start server in goroutine
    go func() {
        if err := srv.Start(); err != nil {
            log.Printf("Server error: %v", err)
        }
    }()

    // Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

### Graceful Shutdown

The server supports graceful shutdown that:
1. Stops the cache stats aggregator and flushes pending stats
2. Closes the audit logger to ensure all events are written
3. Gracefully stops accepting new connections
4. Waits for existing requests to complete

```go
// Shutdown with 30-second timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    log.Printf("Forced shutdown: %v", err)
}
```

## Configuration

The server accepts configuration via `config.Config`:

| Field | Description | Default |
|-------|-------------|---------|
| `ListenAddr` | Address to listen on | `:8080` |
| `RequestTimeout` | Maximum request duration | `120s` |
| `ManagementToken` | Token for management API auth | Required |
| `EnableMetrics` | Enable metrics endpoint | `false` |
| `MetricsPath` | Path for metrics endpoint | `/metrics` |
| `AuditEnabled` | Enable audit logging | `false` |
| `AuditLogFile` | Path to audit log file | - |
| `EventBusBackend` | Event bus type (`redis`, `in-memory`) | `in-memory` |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `MANAGEMENT_TOKEN` | Token for management API authentication |
| `LISTEN_ADDR` | Server listen address |
| `HTTP_CACHE_ENABLED` | Enable HTTP cache (`true`/`false`) |
| `HTTP_CACHE_BACKEND` | Cache backend (`redis` or in-memory) |
| `REDIS_CACHE_URL` | Redis URL for cache |

## API Routes

### Health Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Full health check with status and version |
| `/ready` | GET | Readiness probe for k8s |
| `/live` | GET | Liveness probe for k8s |

### Management API (requires `MANAGEMENT_TOKEN`)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/manage/projects` | GET, POST | List or create projects |
| `/manage/projects/{id}` | GET, PUT, DELETE | Project CRUD by ID |
| `/manage/tokens` | GET, POST | List or create tokens |
| `/manage/tokens/{id}` | GET, PUT, DELETE | Token CRUD by ID |
| `/manage/audit` | GET | Query audit events |
| `/manage/audit/{id}` | GET | Get audit event by ID |
| `/manage/cache/purge` | POST | Purge HTTP cache |

### Proxy Endpoint

All unmatched routes are handled by the transparent proxy, forwarding requests to the configured upstream API (e.g., OpenAI).

## Testing Guidance

### Unit Testing with httptest

```go
package server_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/sofatutor/llm-proxy/internal/config"
    "github.com/sofatutor/llm-proxy/internal/database"
    "github.com/sofatutor/llm-proxy/internal/server"
)

func TestHealthEndpoint(t *testing.T) {
    // Setup test database
    db, _ := database.New(database.Config{Path: ":memory:"})
    defer db.Close()

    tokenStore := database.NewTokenStore(db)
    projectStore := database.NewProjectStore(db)

    cfg := config.DefaultConfig()
    cfg.ManagementToken = "test-token"

    srv, err := server.NewWithDatabase(&cfg, tokenStore, projectStore, db)
    if err != nil {
        t.Fatalf("Failed to create server: %v", err)
    }

    // Create test request
    req := httptest.NewRequest(http.MethodGet, "/health", nil)
    rec := httptest.NewRecorder()

    // Get handler and execute
    // Note: For direct handler testing, you may need to expose handlers
    // or test via the full server using httptest.Server
}
```

### Integration Testing

```go
func TestServerIntegration(t *testing.T) {
    // Create in-memory database
    db, _ := database.New(database.Config{Path: ":memory:"})
    defer db.Close()

    cfg := config.DefaultConfig()
    cfg.ListenAddr = "127.0.0.1:0" // Random port
    cfg.ManagementToken = "test-token"

    srv, _ := server.NewWithDatabase(&cfg, 
        database.NewTokenStore(db),
        database.NewProjectStore(db),
        db,
    )

    // Start server
    go srv.Start()
    defer srv.Shutdown(context.Background())

    // Wait for server to be ready
    time.Sleep(100 * time.Millisecond)

    // Make test requests
    resp, err := http.Get("http://" + cfg.ListenAddr + "/health")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

### Mocking Dependencies

The server accepts interfaces for token and project stores, allowing easy mocking:

```go
type MockTokenStore struct {
    tokens map[string]token.TokenData
}

func (m *MockTokenStore) GetTokenByID(ctx context.Context, id string) (token.TokenData, error) {
    if t, ok := m.tokens[id]; ok {
        return t, nil
    }
    return token.TokenData{}, token.ErrTokenNotFound
}
// ... implement other methods
```

## Troubleshooting

### Server Won't Start

| Symptom | Cause | Solution |
|---------|-------|----------|
| "address already in use" | Port conflict | Change `ListenAddr` or stop conflicting process |
| "failed to initialize logger" | Invalid log config | Check `LogLevel` and `LogFile` path |
| "failed to initialize audit logger" | Invalid audit path | Ensure `AuditLogFile` directory exists |
| "unknown event bus backend" | Invalid config | Use `redis` or `in-memory` |

### Management API Returns 401

- Ensure `MANAGEMENT_TOKEN` environment variable is set
- Include `Authorization: Bearer <token>` header in requests
- Token must match exactly (case-sensitive)

### Health Check Fails

- Check if the server started successfully (logs)
- Verify the listen address is accessible
- Check for database connectivity issues

### Slow Shutdown

- Increase shutdown timeout if requests need more time
- Check for stuck connections or long-running requests
- Review audit logger flush behavior

## Related Packages

| Package | Relationship |
|---------|--------------|
| [`proxy`](../proxy/README.md) | Transparent proxy handler for API requests |
| [`token`](../token/README.md) | Token validation for authentication |
| [`database`](../database/README.md) | Token and project storage |
| [`middleware`](../middleware/) | Request instrumentation and observability |
| [`eventbus`](../eventbus/) | Async event publishing |
| [`audit`](../audit/) | Audit event logging |
| [`config`](../config/) | Server configuration |
