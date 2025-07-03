# LLM Proxy - Comprehensive API Documentation

## Table of Contents

1. [Overview](#overview)
2. [Command Line Interface (CLI)](#command-line-interface-cli)
3. [HTTP REST API](#http-rest-api)
4. [Core Libraries and Packages](#core-libraries-and-packages)
5. [Configuration](#configuration)
6. [Event System](#event-system)
7. [Token Management](#token-management)
8. [Proxy System](#proxy-system)
9. [Database Models](#database-models)
10. [Utilities](#utilities)

## Overview

The LLM Proxy is a transparent, secure proxy for OpenAI's API with token management, rate limiting, logging, and admin UI. This documentation covers all public APIs, functions, and components available for integration and usage.

**Project Module**: `github.com/sofatutor/llm-proxy`

## Command Line Interface (CLI)

### Main Commands

The LLM Proxy provides several CLI commands through the main executable:

#### `llm-proxy server`
Starts the HTTP proxy server.

**Usage:**
```bash
llm-proxy server [flags]
```

**Flags:**
- `--addr string`: Address to listen on (default: ":8080")
- `--db string`: Path to SQLite database (default: "data/llm-proxy.db")
- `--config string`: Path to configuration file (default: ".env")

**Example:**
```bash
MANAGEMENT_TOKEN=your-token llm-proxy server --addr :8080 --db ./data/proxy.db
```

#### `llm-proxy setup`
Interactive or non-interactive setup for configuring the proxy.

**Usage:**
```bash
llm-proxy setup [flags]
```

**Flags:**
- `--config string`: Path to configuration file (default: ".env")
- `--interactive`: Run interactive setup
- `--openai-key string`: OpenAI API Key
- `--management-token string`: Management token for the proxy
- `--project string`: Name of the project to create (default: "DefaultProject")
- `--duration int`: Duration of the token in hours (default: 24)
- `--skip-project`: Skip project and token setup

**Examples:**
```bash
# Interactive setup
llm-proxy setup --interactive

# Non-interactive setup
llm-proxy setup --openai-key sk-... --management-token your-token --project "My Project"
```

#### `llm-proxy manage`
Management commands for projects and tokens.

**Usage:**
```bash
llm-proxy manage [command] [subcommand] [flags]
```

**Available Commands:**
- `project`: Project management (list, get, create, update, delete)
- `token`: Token management (list, get, create, revoke)

**Global Flags:**
- `--manage-api-base-url string`: Management API base URL (default: "http://localhost:8080")
- `--management-token string`: Management token (or set MANAGEMENT_TOKEN env)
- `--json`: Output results as JSON

**Examples:**
```bash
# List all projects
llm-proxy manage project list --management-token your-token

# Create a project
llm-proxy manage project create --name "My Project" --openai-key sk-... --management-token your-token

# Generate a token
llm-proxy manage token generate --project-id project-uuid --duration 24 --management-token your-token
```

#### `llm-proxy dispatcher`
Event dispatcher service for handling observability events.

**Usage:**
```bash
llm-proxy dispatcher --service file --endpoint /path/to/file.jsonl [flags]
```

**Flags:**
- `--service string`: Dispatcher service type (currently supports "file")
- `--endpoint string`: Endpoint configuration (file path for file service)
- `--buffer int`: Event bus buffer size (default: 100)

**Example:**
```bash
llm-proxy dispatcher --service file --endpoint ./events.jsonl --buffer 1000
```

#### `llm-proxy openai chat`
Interactive chat interface with OpenAI API through the proxy.

**Usage:**
```bash
llm-proxy openai chat [flags]
```

**Flags:**
- `--token string`: Authentication token for the proxy
- `--base-url string`: Base URL of the proxy (default: "http://localhost:8080")
- `--model string`: OpenAI model to use (default: "gpt-4")

**Example:**
```bash
llm-proxy openai chat --token your-withering-token --model gpt-4o
```

## HTTP REST API

### Health and Status Endpoints

#### `GET /health`
Health check endpoint for monitoring and load balancers.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2023-09-15T14:30:45Z",
  "version": "0.1.0"
}
```

#### `GET /ready`
Readiness probe endpoint.

**Response:** `200 OK` with body `ready`

#### `GET /live`
Liveness probe endpoint.

**Response:** `200 OK` with body `alive`

#### `GET /metrics`
Prometheus-style metrics endpoint (when enabled).

**Response:**
```json
{
  "uptime_seconds": 3600.5,
  "request_count": 1234,
  "error_count": 5
}
```

### Management API

All management endpoints require the `Authorization: Bearer <MANAGEMENT_TOKEN>` header.

#### Projects

##### `GET /manage/projects`
List all projects in the system.

**Response:**
```json
[
  {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "My AI Project",
    "openai_api_key": "sk-abcd***EFGH",
    "created_at": "2023-09-15T14:30:45Z",
    "updated_at": "2023-09-15T14:30:45Z"
  }
]
```

##### `POST /manage/projects`
Create a new project.

**Request:**
```json
{
  "name": "My AI Project",
  "openai_api_key": "sk-abcdefghijklmnopqrstuvwxyz1234567890ABCDEFG"
}
```

**Response:** `201 Created` with project object

##### `GET /manage/projects/{projectId}`
Get details for a specific project.

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "My AI Project",
  "openai_api_key": "sk-abcd***EFGH",
  "created_at": "2023-09-15T14:30:45Z",
  "updated_at": "2023-09-15T14:30:45Z"
}
```

##### `PATCH /manage/projects/{projectId}`
Update a project (partial update).

**Request:**
```json
{
  "name": "Updated Project Name",
  "openai_api_key": "sk-new-api-key"
}
```

**Response:** `200 OK` with updated project object

##### `DELETE /manage/projects/{projectId}`
Delete a project and all associated tokens.

**Response:** `204 No Content`

#### Tokens

##### `GET /manage/tokens`
List all tokens in the system.

**Query Parameters:**
- `projectId` (optional): Filter tokens by project ID
- `activeOnly` (optional): Filter to only return active tokens

**Response:**
```json
[
  {
    "token": "sk-abc123def456ghi789",
    "project_id": "123e4567-e89b-12d3-a456-426614174000",
    "expires_at": "2023-12-31T23:59:59Z",
    "is_active": true,
    "request_count": 42,
    "max_requests": 1000,
    "created_at": "2023-09-15T14:30:45Z",
    "last_used_at": "2023-09-15T14:30:45Z"
  }
]
```

##### `POST /manage/tokens`
Create a new token for a project.

**Request:**
```json
{
  "project_id": "123e4567-e89b-12d3-a456-426614174000",
  "expires_in_days": 30,
  "max_requests": 1000
}
```

**Response:** `201 Created` with token object

##### `GET /manage/tokens/{tokenId}`
Get details for a specific token.

**Response:**
```json
{
  "token": "sk-abc123def456ghi789",
  "project_id": "123e4567-e89b-12d3-a456-426614174000",
  "expires_at": "2023-12-31T23:59:59Z",
  "is_active": true,
  "request_count": 42,
  "max_requests": 1000,
  "created_at": "2023-09-15T14:30:45Z",
  "last_used_at": "2023-09-15T14:30:45Z"
}
```

##### `DELETE /manage/tokens/{tokenId}`
Revoke (deactivate) a token.

**Response:** `204 No Content`

### Proxy API

#### `GET|POST /v1/{path}`
Proxy requests to OpenAI API with token authentication.

**Headers:**
- `Authorization: Bearer <token>` (required): Withering token for authentication

**Supported OpenAI Endpoints:**
- `/v1/chat/completions`
- `/v1/completions`
- `/v1/embeddings`
- `/v1/models`
- `/v1/edits`
- `/v1/fine-tunes`
- `/v1/files`
- `/v1/images/generations`
- `/v1/audio/transcriptions`
- `/v1/moderations`

**Example:**
```bash
curl -H "Authorization: Bearer sk-your-withering-token" \
     -H "Content-Type: application/json" \
     -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
     http://localhost:8080/v1/chat/completions
```

## Core Libraries and Packages

### Server Package (`internal/server`)

#### `func New(cfg *config.Config, tokenStore token.TokenStore, projectStore proxy.ProjectStore) (*Server, error)`
Creates a new HTTP server instance with the provided configuration and stores.

**Parameters:**
- `cfg`: Application configuration
- `tokenStore`: Token storage implementation
- `projectStore`: Project storage implementation

**Returns:** `*Server` instance and error

**Example:**
```go
import (
    "github.com/sofatutor/llm-proxy/internal/config"
    "github.com/sofatutor/llm-proxy/internal/server"
    "github.com/sofatutor/llm-proxy/internal/database"
)

cfg, err := config.New()
if err != nil {
    log.Fatal(err)
}

db, err := database.NewDB(cfg.DatabasePath)
if err != nil {
    log.Fatal(err)
}

tokenStore := database.NewTokenStore(db)
projectStore := database.NewProjectStore(db)

srv, err := server.New(cfg, tokenStore, projectStore)
if err != nil {
    log.Fatal(err)
}
```

#### `func (s *Server) Start() error`
Starts the HTTP server. This method blocks until the server shuts down.

#### `func (s *Server) Shutdown(ctx context.Context) error`
Gracefully shuts down the server.

**Example:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    log.Printf("Server shutdown error: %v", err)
}
```

### Token Package (`internal/token`)

#### `func GenerateToken() (string, error)`
Generates a new withering token with UUID v7 and base64 encoding.

**Returns:** Token string in format `sk-{base64-encoded-uuid}` and error

**Example:**
```go
import "github.com/sofatutor/llm-proxy/internal/token"

token, err := token.GenerateToken()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Generated token: %s\n", token)
// Output: Generated token: sk-ABC123DEF456GHI789JKL012
```

#### `func ValidateTokenFormat(token string) error`
Validates that a token follows the correct format and can be decoded.

**Parameters:**
- `token`: Token string to validate

**Returns:** Error if token format is invalid

**Example:**
```go
err := token.ValidateTokenFormat("sk-ABC123DEF456GHI789JKL012")
if err != nil {
    log.Printf("Invalid token format: %v", err)
}
```

#### `func DecodeToken(token string) (uuid.UUID, error)`
Extracts the UUID from a token string.

**Parameters:**
- `token`: Token string to decode

**Returns:** UUID and error

**Example:**
```go
uuid, err := token.DecodeToken("sk-ABC123DEF456GHI789JKL012")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Token UUID: %s\n", uuid)
```

#### Token Validator Interface

```go
type TokenValidator interface {
    ValidateToken(ctx context.Context, token string) (string, error)
    ValidateTokenWithTracking(ctx context.Context, token string) (string, error)
}
```

#### `func NewValidator(store TokenStore) *StandardValidator`
Creates a new token validator with the provided token store.

**Example:**
```go
validator := token.NewValidator(tokenStore)
projectID, err := validator.ValidateToken(ctx, "sk-token")
```

#### Token Generator

#### `func NewTokenGenerator() *TokenGenerator`
Creates a new token generator instance.

**Example:**
```go
generator := token.NewTokenGenerator()
newToken, err := generator.Generate()
```

#### Token Utilities

#### `func ExtractTokenFromRequest(r *http.Request) (string, bool)`
Extracts the bearer token from an HTTP request's Authorization header.

**Parameters:**
- `r`: HTTP request

**Returns:** Token string and boolean indicating if found

**Example:**
```go
token, found := token.ExtractTokenFromRequest(req)
if !found {
    http.Error(w, "Missing token", http.StatusUnauthorized)
    return
}
```

#### `func ObfuscateToken(token string) string`
Obfuscates a token for logging purposes, showing only the first 4 characters.

**Example:**
```go
obfuscated := token.ObfuscateToken("sk-ABC123DEF456GHI789JKL012")
fmt.Println(obfuscated) // Output: sk-A***
```

#### Token Expiration Functions

#### `func CalculateExpiration(duration time.Duration) *time.Time`
Calculates expiration time from now plus the given duration.

#### `func ValidateExpiration(expiresAt *time.Time) error`
Validates that an expiration time is in the future.

#### `func IsExpired(expiresAt *time.Time) bool`
Checks if a token has expired.

**Example:**
```go
expiry := token.CalculateExpiration(24 * time.Hour)
if token.IsExpired(expiry) {
    log.Println("Token has expired")
}
```

#### Rate Limiting

#### `type RateLimiter interface`
```go
type RateLimiter interface {
    AllowRequest(ctx context.Context, tokenID string) (bool, error)
    GetRemainingRequests(ctx context.Context, tokenID string) (int, error)
    ResetUsage(ctx context.Context, tokenID string) error
    UpdateLimit(ctx context.Context, tokenID string, newLimit int) error
}
```

#### Token Manager

#### `func NewManager(store ManagerStore, useCaching bool) (*Manager, error)`
Creates a new token manager with optional caching.

**Example:**
```go
manager, err := token.NewManager(store, true) // Enable caching
if err != nil {
    log.Fatal(err)
}
```

## Configuration

### Config Package (`internal/config`)

#### `func New() (*Config, error)`
Creates a new configuration loaded from environment variables.

**Example:**
```go
import "github.com/sofatutor/llm-proxy/internal/config"

cfg, err := config.New()
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Listen address: %s\n", cfg.ListenAddr)
fmt.Printf("Database path: %s\n", cfg.DatabasePath)
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
    EnableMetrics bool   // Enable Prometheus metrics
    MetricsPath   string // Path for metrics endpoint
}
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LISTEN_ADDR` | Server listen address | `:8080` |
| `DATABASE_PATH` | SQLite database path | `./data/llm-proxy.db` |
| `MANAGEMENT_TOKEN` | Management API token | **Required** |
| `LOG_LEVEL` | Logging level | `info` |
| `LOG_FORMAT` | Log format | `json` |
| `OPENAI_API_URL` | OpenAI API base URL | `https://api.openai.com` |
| `OBSERVABILITY_ENABLED` | Enable observability | `true` |
| `OBSERVABILITY_BUFFER_SIZE` | Event bus buffer size | `1000` |

## Event System

### EventBus Package (`internal/eventbus`)

#### Event Structure

```go
type Event struct {
    RequestID       string
    Method          string
    Path            string
    Status          int
    Duration        time.Duration
    ResponseHeaders http.Header
    ResponseBody    []byte
    RequestBody     []byte
}
```

#### EventBus Interface

```go
type EventBus interface {
    Publish(ctx context.Context, evt Event)
    Subscribe() <-chan Event
    Stop()
}
```

#### `func NewInMemoryEventBus(bufferSize int) *InMemoryEventBus`
Creates a new in-memory event bus with the specified buffer size.

**Example:**
```go
import "github.com/sofatutor/llm-proxy/internal/eventbus"

bus := eventbus.NewInMemoryEventBus(1000)
defer bus.Stop()

// Publish an event
event := eventbus.Event{
    RequestID: "req-123",
    Method:    "POST",
    Path:      "/v1/chat/completions",
    Status:    200,
    Duration:  100 * time.Millisecond,
}
bus.Publish(context.Background(), event)

// Subscribe to events
sub := bus.Subscribe()
go func() {
    for event := range sub {
        fmt.Printf("Received event: %+v\n", event)
    }
}()
```

#### `func NewRedisEventBus(client RedisClient, key string) *RedisEventBus`
Creates a Redis-backed event bus for distributed deployments.

**Example:**
```go
// Assuming you have a Redis client implementation
bus := eventbus.NewRedisEventBus(redisClient, "llm-proxy-events")
defer bus.Stop()
```

## Proxy System

### Proxy Package (`internal/proxy`)

#### Proxy Interface

```go
type Proxy interface {
    Handler() http.Handler
    Shutdown(ctx context.Context) error
}
```

#### `func NewTransparentProxy(config ProxyConfig, validator TokenValidator, projectStore ProjectStore) (*TransparentProxy, error)`
Creates a new transparent proxy with the specified configuration.

**Example:**
```go
import "github.com/sofatutor/llm-proxy/internal/proxy"

config := proxy.ProxyConfig{
    TargetBaseURL: "https://api.openai.com",
    AllowedEndpoints: []string{"/v1/chat/completions", "/v1/models"},
    AllowedMethods: []string{"GET", "POST"},
    RequestTimeout: 30 * time.Second,
}

p, err := proxy.NewTransparentProxy(config, validator, projectStore)
if err != nil {
    log.Fatal(err)
}

// Use as HTTP handler
http.Handle("/v1/", p.Handler())
```

#### ProxyConfig Structure

```go
type ProxyConfig struct {
    TargetBaseURL         string
    AllowedEndpoints      []string
    AllowedMethods        []string
    RequestTimeout        time.Duration
    ResponseHeaderTimeout time.Duration
    FlushInterval         time.Duration
    MaxIdleConns          int
    MaxIdleConnsPerHost   int
    IdleConnTimeout       time.Duration
    LogLevel              string
    LogFormat             string
    LogFile               string
    SetXForwardedFor      bool
    ParamWhitelist        map[string][]string
    AllowedOrigins        []string
    RequiredHeaders       []string
}
```

#### ProjectStore Interface

```go
type ProjectStore interface {
    GetAPIKeyForProject(ctx context.Context, projectID string) (string, error)
    ListProjects(ctx context.Context) ([]Project, error)
    CreateProject(ctx context.Context, project Project) error
    GetProjectByID(ctx context.Context, projectID string) (Project, error)
    UpdateProject(ctx context.Context, project Project) error
    DeleteProject(ctx context.Context, projectID string) error
}
```

## Database Models

### Models Package (`internal/database/models`)

#### Project Model

```go
type Project struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    OpenAIAPIKey string    `json:"openai_api_key"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

#### Token Model

```go
type Token struct {
    Token       string     `json:"token"`
    ProjectID   string     `json:"project_id"`
    ExpiresAt   *time.Time `json:"expires_at"`
    IsActive    bool       `json:"is_active"`
    RequestCount int       `json:"request_count"`
    MaxRequests  int       `json:"max_requests"`
    CreatedAt    time.Time `json:"created_at"`
    LastUsedAt   *time.Time `json:"last_used_at"`
}
```

## Utilities

### Crypto Utils (`internal/utils/crypto`)

#### `func GenerateSecureToken(length int) (string, error)`
Generates a cryptographically secure random token.

**Example:**
```go
import "github.com/sofatutor/llm-proxy/internal/utils"

token, err := utils.GenerateSecureToken(32)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Secure token: %s\n", token)
```

#### `func GenerateSecureTokenMustSucceed(length int) string`
Generates a secure token, panicking on error (for cases where failure is unacceptable).

## Error Handling

### Common Error Types

#### Token Errors

- `ErrInvalidTokenFormat`: Token doesn't match expected format
- `ErrTokenDecodingFailed`: Token cannot be decoded
- `ErrTokenExpired`: Token has expired
- `ErrTokenRevoked`: Token has been revoked
- `ErrTokenNotFound`: Token doesn't exist in the database

#### API Errors

Standard HTTP error responses follow this format:

```json
{
  "error": "error_code",
  "description": "Human-readable description",
  "code": "specific_error_code"
}
```

### Example Error Handling

```go
// Token validation example
projectID, err := validator.ValidateToken(ctx, token)
if err != nil {
    switch {
    case errors.Is(err, token.ErrInvalidTokenFormat):
        http.Error(w, "Invalid token format", http.StatusBadRequest)
    case errors.Is(err, token.ErrTokenExpired):
        http.Error(w, "Token expired", http.StatusUnauthorized)
    default:
        http.Error(w, "Token validation failed", http.StatusUnauthorized)
    }
    return
}
```

## Best Practices

### Security

1. **Always use HTTPS in production** - The proxy handles sensitive API keys
2. **Rotate management tokens regularly** - Use strong, unique tokens
3. **Monitor token usage** - Set appropriate expiration and rate limits
4. **Use project isolation** - Separate different use cases into different projects

### Performance

1. **Enable caching** - Use cached token validation for better performance
2. **Configure appropriate timeouts** - Set reasonable request timeouts
3. **Monitor buffer sizes** - Adjust event bus buffer sizes based on load
4. **Use connection pooling** - Configure database connection pools appropriately

### Monitoring

1. **Use the metrics endpoint** - Monitor `/metrics` for observability
2. **Configure structured logging** - Use JSON format for production logs
3. **Set up event monitoring** - Use the event bus for comprehensive monitoring
4. **Monitor token lifecycle** - Track token creation, usage, and expiration

## Examples

### Complete Integration Example

```go
package main

import (
    "context"
    "log"
    "net/http"
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

For additional information, see the main [README.md](../README.md) and other documentation in the `/docs` directory.