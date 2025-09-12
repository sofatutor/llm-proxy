# Go Package Documentation

This document provides documentation for using LLM Proxy packages in your own Go applications.

## Import Path

```go
import "github.com/sofatutor/llm-proxy/internal/..."
```

> **Note:** The packages are currently in the `internal/` directory, which means they are not intended for external use according to Go conventions. If you need to use these packages externally, consider moving them to a public location or vendoring them.

## Core Packages

### Configuration (`internal/config`)

The config package provides application configuration management with environment variable support.

#### Basic Usage

```go
package main

import (
    "log"
    "github.com/sofatutor/llm-proxy/internal/config"
)

func main() {
    // Load configuration from environment variables
    cfg, err := config.New()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Access configuration values
    fmt.Printf("Listen address: %s\n", cfg.ListenAddr)
    fmt.Printf("Database path: %s\n", cfg.DatabasePath)
    fmt.Printf("Log level: %s\n", cfg.LogLevel)
}
```

#### Configuration Structure

```go
type Config struct {
    // Server configuration
    ListenAddr        string        // Address to listen on
    RequestTimeout    time.Duration // Timeout for upstream API requests
    MaxRequestSize    int64         // Maximum size of incoming requests
    MaxConcurrentReqs int           // Maximum concurrent requests

    // Database configuration
    DatabasePath     string // Path to SQLite database file
    DatabasePoolSize int    // Connection pool size

    // Authentication
    ManagementToken string // Token for admin operations

    // API Provider configuration
    APIConfigPath      string // Path to API providers config file
    DefaultAPIProvider string // Default API provider
    OpenAIAPIURL       string // Base URL for OpenAI API

    // Logging
    LogLevel  string // Log level (debug, info, warn, error)
    LogFormat string // Log format (json, text)
    LogFile   string // Path to log file

    // Observability
    ObservabilityEnabled    bool // Enable observability middleware
    ObservabilityBufferSize int  // Buffer size for event bus

    // Rate limiting
    GlobalRateLimit int // Maximum requests per minute globally
    IPRateLimit     int // Maximum requests per minute per IP

    // Monitoring
    EnableMetrics bool   // Enable lightweight metrics endpoint (provider-agnostic; Prometheus optional)
    MetricsPath   string // Path for metrics endpoint
}
```

---

### Token Management (`internal/token`)

The token package provides secure token generation, validation, and management.

#### Token Generation

```go
package main

import (
    "log"
    "github.com/sofatutor/llm-proxy/internal/token"
)

func main() {
    // Generate a new token
    newToken, err := token.GenerateToken()
    if err != nil {
        log.Fatalf("Failed to generate token: %v", err)
    }
    
    fmt.Printf("Generated token: %s\n", newToken)
    // Output: Generated token: sk-ABC123DEF456GHI789JKL012
}
```

#### Token Validation

```go
package main

import (
    "context"
    "log"
    "github.com/sofatutor/llm-proxy/internal/token"
    "github.com/sofatutor/llm-proxy/internal/database"
)

func main() {
    // Initialize database and token store
    db, err := database.NewDB("./data/proxy.db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    tokenStore := database.NewTokenStore(db)
    
    // Create validator
    validator := token.NewValidator(tokenStore)
    
    // Validate a token
    ctx := context.Background()
    projectID, err := validator.ValidateToken(ctx, "sk-some-token")
    if err != nil {
        log.Printf("Token validation failed: %v", err)
        return
    }
    
    fmt.Printf("Token is valid for project: %s\n", projectID)
}
```

#### Token Utilities

```go
package main

import (
    "net/http"
    "github.com/sofatutor/llm-proxy/internal/token"
)

func handleRequest(w http.ResponseWriter, r *http.Request) {
    // Extract token from Authorization header
    tokenValue, found := token.ExtractTokenFromRequest(r)
    if !found {
        http.Error(w, "Missing authorization token", http.StatusUnauthorized)
        return
    }
    
    // Validate token format
    if err := token.ValidateTokenFormat(tokenValue); err != nil {
        http.Error(w, "Invalid token format", http.StatusBadRequest)
        return
    }
    
    // Obfuscate token for logging
    fmt.Printf("Processing request with token: %s\n", token.ObfuscateToken(tokenValue))
    
    // Continue processing...
}
```

#### Token Expiration Management

```go
package main

import (
    "time"
    "github.com/sofatutor/llm-proxy/internal/token"
)

func main() {
    // Calculate expiration time
    expiry := token.CalculateExpiration(24 * time.Hour)
    
    // Check if token would be expired
    if token.IsExpired(expiry) {
        fmt.Println("Token has expired")
    } else {
        remaining := token.TimeUntilExpiration(expiry)
        fmt.Printf("Token expires in: %v\n", remaining)
    }
    
    // Format expiration time for display
    formatted := token.FormatExpirationTime(expiry)
    fmt.Printf("Expires at: %s\n", formatted)
}
```

---

### Database Layer (`internal/database`)

The database package provides storage interfaces and implementations.

#### Database Connection

```go
package main

import (
    "log"
    "github.com/sofatutor/llm-proxy/internal/database"
)

func main() {
    // Initialize database
    db, err := database.NewDB("./data/proxy.db")
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()
    
    // Create stores
    tokenStore := database.NewTokenStore(db)
    projectStore := database.NewProjectStore(db)
    
    // Use stores...
}
```

#### Working with Projects

```go
package main

import (
    "context"
    "time"
    "github.com/sofatutor/llm-proxy/internal/database"
    "github.com/sofatutor/llm-proxy/internal/proxy"
)

func main() {
    db, _ := database.NewDB("./data/proxy.db")
    defer db.Close()
    
    projectStore := database.NewProjectStore(db)
    ctx := context.Background()
    
    // Create a new project
    project := proxy.Project{
        ID:           "123e4567-e89b-12d3-a456-426614174000",
        Name:         "My AI Project",
        OpenAIAPIKey: "sk-your-api-key",
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }
    
    err := projectStore.CreateProject(ctx, project)
    if err != nil {
        log.Printf("Failed to create project: %v", err)
        return
    }
    
    // Get project by ID
    retrievedProject, err := projectStore.GetProjectByID(ctx, project.ID)
    if err != nil {
        log.Printf("Failed to get project: %v", err)
        return
    }
    
    fmt.Printf("Project: %+v\n", retrievedProject)
}
```

---

### Event System (`internal/eventbus`)

The eventbus package provides asynchronous event publishing and subscription.

#### In-Memory Event Bus

```go
package main

import (
    "context"
    "time"
    "github.com/sofatutor/llm-proxy/internal/eventbus"
)

func main() {
    // Create event bus
    bus := eventbus.NewInMemoryEventBus(1000)
    defer bus.Stop()
    
    // Subscribe to events
    sub := bus.Subscribe()
    go func() {
        for event := range sub {
            fmt.Printf("Received event: %+v\n", event)
        }
    }()
    
    // Publish an event
    event := eventbus.Event{
        RequestID: "req-123",
        Method:    "POST",
        Path:      "/v1/chat/completions",
        Status:    200,
        Duration:  100 * time.Millisecond,
    }
    
    bus.Publish(context.Background(), event)
    
    // Wait a bit for processing
    time.Sleep(100 * time.Millisecond)
}
```

#### Custom Event Bus Implementation

```go
package main

import (
    "context"
    "github.com/sofatutor/llm-proxy/internal/eventbus"
)

// Custom event bus that logs to a file
type FileEventBus struct {
    file *os.File
}

func (b *FileEventBus) Publish(ctx context.Context, evt eventbus.Event) {
    data, _ := json.Marshal(evt)
    b.file.Write(append(data, '\n'))
}

func (b *FileEventBus) Subscribe() <-chan eventbus.Event {
    // Implementation for reading from file...
    return make(<-chan eventbus.Event)
}

func (b *FileEventBus) Stop() {
    b.file.Close()
}
```

---

### Proxy System (`internal/proxy`)

The proxy package provides transparent HTTP proxying with authentication and caching capabilities.

#### HTTP Response Caching

The proxy includes a built-in HTTP response caching system with support for both Redis and an in-memory fallback backend (used when Redis is not configured):

```go
package main

import (
    "context"
    "github.com/sofatutor/llm-proxy/internal/proxy"
    "github.com/redis/go-redis/v9"
)

func main() {
    // Create Redis client for cache backend
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Configure proxy with caching
    config := &proxy.Config{
        HTTPCacheEnabled:       true,
        HTTPCacheBackend:       "redis",
        RedisClient:           redisClient,
        HTTPCacheKeyPrefix:    "myapp:cache:",
        HTTPCacheMaxObjectBytes: 1048576, // 1MB
        HTTPCacheDefaultTTL:   300,       // 5 minutes
    }
    
    // The proxy will automatically handle:
    // - Cache key generation based on method, path, query, headers
    // - TTL derivation from Cache-Control headers (s-maxage > max-age)
    // - Streaming response capture and storage
    // - Auth-aware caching (Authorization header not in cache key)
    // - Response headers: X-PROXY-CACHE, X-PROXY-CACHE-KEY, Cache-Status
}
```

#### Cache Integration Features

- **Redis Backend**: Primary cache store with in-memory fallback
- **HTTP Standards Compliance**: Respects Cache-Control, ETag, Last-Modified
- **Streaming Support**: Captures streaming responses during transmission
- **Conservative Vary**: Includes Accept, Accept-Encoding, Accept-Language in cache key
- **Event Bus Integration**: Cache hits bypass event publishing for performance
- **Authentication Aware**: Only serves public cached responses to authenticated requests

#### Creating a Transparent Proxy

```go
package main

import (
    "log"
    "net/http"
    "time"
    "github.com/sofatutor/llm-proxy/internal/proxy"
    "github.com/sofatutor/llm-proxy/internal/token"
    "github.com/sofatutor/llm-proxy/internal/database"
)

func main() {
    // Setup dependencies
    db, _ := database.NewDB("./data/proxy.db")
    defer db.Close()
    
    tokenStore := database.NewTokenStore(db)
    projectStore := database.NewProjectStore(db)
    validator := token.NewValidator(tokenStore)
    
    // Configure proxy
    config := proxy.ProxyConfig{
        TargetBaseURL: "https://api.openai.com",
        AllowedEndpoints: []string{
            "/v1/chat/completions",
            "/v1/completions",
            "/v1/models",
        },
        AllowedMethods: []string{"GET", "POST"},
        RequestTimeout: 30 * time.Second,
        MaxIdleConns:   100,
    }
    
    // Create proxy
    p, err := proxy.NewTransparentProxy(config, validator, projectStore)
    if err != nil {
        log.Fatalf("Failed to create proxy: %v", err)
    }
    defer p.Shutdown(context.Background())
    
    // Use as HTTP handler
    http.Handle("/v1/", p.Handler())
    
    log.Println("Proxy server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

---

### Server (`internal/server`)

The server package provides the complete HTTP server implementation.

#### Creating a Complete Server

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
    cfg, err := config.New()
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }
    
    // Initialize database
    db, err := database.NewDB(cfg.DatabasePath)
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()
    
    // Create stores
    tokenStore := database.NewTokenStore(db)
    projectStore := database.NewProjectStore(db)
    
    // Create server
    srv, err := server.New(cfg, tokenStore, projectStore)
    if err != nil {
        log.Fatalf("Failed to create server: %v", err)
    }
    
    // Start server in goroutine
    go func() {
        log.Printf("Starting server on %s", cfg.ListenAddr)
        if err := srv.Start(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()
    
    // Wait for interrupt signal
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop
    
    // Graceful shutdown
    log.Println("Shutting down server...")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("Server shutdown error: %v", err)
    } else {
        log.Println("Server stopped gracefully")
    }
}
```

---

### Utilities (`internal/utils`)

The utils package provides cryptographic utilities and helper functions.

#### Secure Token Generation

```go
package main

import (
    "log"
    "github.com/sofatutor/llm-proxy/internal/utils"
)

func main() {
    // Generate a secure random token
    token, err := utils.GenerateSecureToken(32)
    if err != nil {
        log.Fatalf("Failed to generate token: %v", err)
    }
    
    fmt.Printf("Secure token: %s\n", token)
    
    // Generate token that must succeed (panics on error)
    mustToken := utils.GenerateSecureTokenMustSucceed(32)
    fmt.Printf("Must-succeed token: %s\n", mustToken)
}
```

---

## Error Handling

### Common Error Types

The LLM Proxy packages define several error types for specific conditions:

```go
package main

import (
    "errors"
    "github.com/sofatutor/llm-proxy/internal/token"
)

func handleTokenErrors(err error) {
    switch {
    case errors.Is(err, token.ErrInvalidTokenFormat):
        // Handle invalid token format
        fmt.Println("Token format is invalid")
    case errors.Is(err, token.ErrTokenDecodingFailed):
        // Handle token decoding failure
        fmt.Println("Could not decode token")
    default:
        // Handle other errors
        fmt.Printf("Unknown error: %v\n", err)
    }
}
```

### Error Wrapping

The packages use Go's error wrapping conventions:

```go
projectID, err := validator.ValidateToken(ctx, token)
if err != nil {
    return fmt.Errorf("token validation failed: %w", err)
}
```

---

## Best Practices

### Security

1. **Never log sensitive data**:
   ```go
   // Good
   log.Printf("Processing request with token: %s", token.ObfuscateToken(tokenValue))
   
   // Bad
   log.Printf("Processing request with token: %s", tokenValue)
   ```

2. **Use context for cancellation**:
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   defer cancel()
   
   projectID, err := validator.ValidateToken(ctx, tokenValue)
   ```

3. **Handle errors appropriately**:
   ```go
   if err != nil {
       // Log the error with context
       log.Printf("Token validation failed for request %s: %v", requestID, err)
       
       // Return appropriate HTTP status
       http.Error(w, "Unauthorized", http.StatusUnauthorized)
       return
   }
   ```

### Performance

1. **Use connection pooling**:
   ```go
   config := proxy.ProxyConfig{
       MaxIdleConns:        100,
       MaxIdleConnsPerHost: 20,
       IdleConnTimeout:     90 * time.Second,
   }
   ```

2. **Set appropriate timeouts**:
   ```go
   config := proxy.ProxyConfig{
       RequestTimeout:        30 * time.Second,
       ResponseHeaderTimeout: 10 * time.Second,
   }
   ```

3. **Use caching when appropriate**:
   ```go
   // Enable caching for token validation
   validator := token.NewCachedValidator(baseValidator)
   ```

### Resource Management

1. **Always close resources**:
   ```go
   db, err := database.NewDB(path)
   if err != nil {
       return err
   }
   defer db.Close() // Important!
   ```

2. **Stop services gracefully**:
   ```go
   bus := eventbus.NewInMemoryEventBus(1000)
   defer bus.Stop() // Important!
   ```

---

## Testing

### Mock Implementations

The packages provide mock implementations for testing:

```go
package main

import (
    "testing"
    "github.com/sofatutor/llm-proxy/internal/database"
)

func TestTokenValidation(t *testing.T) {
    // Use mock token store for testing
    mockStore := database.NewMockTokenStore()
    
    // Add test data
    mockStore.AddToken("test-token", "project-123", time.Now().Add(time.Hour))
    
    // Test validation logic
    validator := token.NewValidator(mockStore)
    projectID, err := validator.ValidateToken(context.Background(), "test-token")
    
    if err != nil {
        t.Fatalf("Expected token to be valid: %v", err)
    }
    
    if projectID != "project-123" {
        t.Fatalf("Expected project-123, got %s", projectID)
    }
}
```

---

For more examples and detailed API documentation, see the individual package files and their test suites.