# Logging Package

This package provides structured logging utilities for the LLM proxy, built on top of [Zap](https://github.com/uber-go/zap). It includes canonical log fields, context propagation, and flexible output formatting.

## Purpose & Responsibilities

The `logging` package handles all application logging concerns:

- **Structured Logging**: JSON and console output formats via Zap
- **Canonical Fields**: Standardized field names for consistent log analysis
- **Context Propagation**: Request ID and correlation ID tracking
- **Log Level Filtering**: Configurable verbosity (debug, info, warn, error)
- **Output Routing**: Stdout or file-based log output

## Configuration Options

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `LOG_LEVEL` | Minimum log level (`debug`, `info`, `warn`, `error`) | `info` |
| `LOG_FORMAT` | Output format (`json`, `console`) | `json` |
| `LOG_FILE` | File path for log output (empty = stdout) | - |

## Creating a Logger

### Basic Logger

```go
package main

import (
    "github.com/sofatutor/llm-proxy/internal/logging"
)

func main() {
    // Create JSON logger at info level, output to stdout
    logger, err := logging.NewLogger("info", "json", "")
    if err != nil {
        panic(err)
    }
    defer logger.Sync()
    
    logger.Info("Server started", zap.Int("port", 8080))
}
```

### Console Logger for Development

```go
logger, err := logging.NewLogger("debug", "console", "")
```

### File-based Logger

```go
logger, err := logging.NewLogger("info", "json", "/var/log/llm-proxy/app.log")
```

## Log Levels

| Level | Description | Use Case |
|-------|-------------|----------|
| `debug` | Verbose debugging information | Development, troubleshooting |
| `info` | Normal operational messages | Production default |
| `warn` | Potentially harmful situations | Configuration issues, deprecations |
| `error` | Error conditions | Failures requiring attention |

## Canonical Log Fields

The package provides helper functions for consistent field naming across all logs.

### Request Fields

```go
fields := logging.RequestFields(
    requestID,   // request_id
    method,      // method
    path,        // path
    statusCode,  // status_code
    durationMs,  // duration_ms
)
logger.Info("Request completed", fields...)
```

Output:
```json
{
    "msg": "Request completed",
    "request_id": "req-abc123",
    "method": "POST",
    "path": "/v1/chat/completions",
    "status_code": 200,
    "duration_ms": 150
}
```

### Individual Field Helpers

```go
// Correlation ID for distributed tracing
logger.Info("Processing", logging.CorrelationID("corr-xyz"))

// Project identifier
logger.Info("Project loaded", logging.ProjectID("proj-123"))

// Token ID (automatically obfuscated for security)
logger.Info("Token validated", logging.TokenID("sk-abc123..."))

// Client IP address
logger.Info("Request received", logging.ClientIP("192.168.1.100"))
```

### Security: Token Obfuscation

The `TokenID` helper automatically obfuscates tokens to prevent sensitive data leakage:

```go
logging.TokenID("sk-abc123xyz789...")
// Output: token_id: "sk-abc1...789"
```

## Context Propagation

### Adding IDs to Context

```go
import (
    "context"
    "github.com/sofatutor/llm-proxy/internal/logging"
)

func HandleRequest(ctx context.Context) {
    // Add request ID to context
    ctx = logging.WithRequestID(ctx, "req-abc123")
    
    // Add correlation ID for distributed tracing
    ctx = logging.WithCorrelationID(ctx, "corr-xyz789")
    
    // Pass context to downstream functions
    processRequest(ctx)
}
```

### Retrieving IDs from Context

```go
func processRequest(ctx context.Context) {
    if requestID, ok := logging.GetRequestID(ctx); ok {
        log.Printf("Processing request: %s", requestID)
    }
    
    if correlationID, ok := logging.GetCorrelationID(ctx); ok {
        log.Printf("Correlation: %s", correlationID)
    }
}
```

### Enriching Logger from Context

```go
func handleRequest(ctx context.Context, logger *zap.Logger) {
    // Add request ID if present in context
    logger = logging.WithRequestContext(ctx, logger)
    
    // Add correlation ID if present in context
    logger = logging.WithCorrelationContext(ctx, logger)
    
    logger.Info("Processing request")
    // Output includes request_id and correlation_id if present
}
```

## Child Loggers

Create component-specific loggers with additional context:

```go
func main() {
    rootLogger, _ := logging.NewLogger("info", "json", "")
    
    // Create child logger for specific component
    proxyLogger := logging.NewChildLogger(rootLogger, "proxy")
    proxyLogger.Info("Proxy initialized")
    // Output: {"component": "proxy", "msg": "Proxy initialized"}
    
    tokenLogger := logging.NewChildLogger(rootLogger, "token")
    tokenLogger.Info("Token validated")
    // Output: {"component": "token", "msg": "Token validated"}
}
```

## Integration Patterns

### With HTTP Middleware

```go
func LoggingMiddleware(logger *zap.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        requestID := c.GetHeader("X-Request-ID")
        
        // Add to context
        ctx := logging.WithRequestID(c.Request.Context(), requestID)
        c.Request = c.Request.WithContext(ctx)
        
        c.Next()
        
        // Log request completion
        duration := time.Since(start)
        logger.Info("Request completed",
            logging.RequestFields(
                requestID,
                c.Request.Method,
                c.Request.URL.Path,
                c.Writer.Status(),
                int(duration.Milliseconds()),
            )...,
        )
    }
}
```

### With Services

```go
type TokenService struct {
    logger *zap.Logger
}

func NewTokenService(rootLogger *zap.Logger) *TokenService {
    return &TokenService{
        logger: logging.NewChildLogger(rootLogger, "token-service"),
    }
}

func (s *TokenService) ValidateToken(ctx context.Context, token string) error {
    logger := logging.WithRequestContext(ctx, s.logger)
    logger.Info("Validating token", logging.TokenID(token))
    // ...
}
```

## JSON Output Format

When using `json` format, logs are structured for easy parsing:

```json
{
    "ts": "2024-01-15T10:30:45.123Z",
    "level": "info",
    "msg": "Request completed",
    "caller": "server/handler.go:42",
    "request_id": "req-abc123",
    "method": "POST",
    "path": "/v1/chat/completions",
    "status_code": 200,
    "duration_ms": 150,
    "component": "proxy"
}
```

## Console Output Format

When using `console` format, logs are human-readable:

```
2024-01-15T10:30:45.123Z	info	server/handler.go:42	Request completed	{"request_id": "req-abc123", "method": "POST", "path": "/v1/chat/completions", "status_code": 200, "duration_ms": 150}
```

## Testing Guidance

### Testing with Zap Test Logger

```go
import (
    "testing"
    "go.uber.org/zap/zaptest"
)

func TestMyFunction(t *testing.T) {
    logger := zaptest.NewLogger(t)
    
    // Use logger in tests - output captured by testing framework
    svc := NewService(logger)
    svc.DoSomething()
}
```

### Testing with Observable Logger

```go
import (
    "testing"
    "go.uber.org/zap"
    "go.uber.org/zap/zaptest/observer"
)

func TestLoggingOutput(t *testing.T) {
    core, logs := observer.New(zap.InfoLevel)
    logger := zap.New(core)
    
    // Use logger
    logger.Info("test message", zap.String("key", "value"))
    
    // Assert log output
    entries := logs.All()
    assert.Equal(t, 1, len(entries))
    assert.Equal(t, "test message", entries[0].Message)
}
```

## Related Documentation

- [Audit Package](../audit/README.md) - Security audit logging (separate from application logs)
- [Middleware Package](../middleware/README.md) - HTTP middleware including request logging
- [Issue #60](https://github.com/sofatutor/llm-proxy/issues/60) - Log search utilities (planned)

## Files

| File | Description |
|------|-------------|
| `logger.go` | Logger creation, canonical fields, and context propagation |
