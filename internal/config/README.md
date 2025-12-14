# Config Package

## Purpose & Responsibilities

The `config` package provides centralized configuration management for the LLM Proxy. It handles:

- **Environment Variable Loading**: Type-safe loading from environment variables
- **Default Values**: Sensible defaults for all configuration options
- **Validation**: Required field validation and type checking
- **Configuration Sources**: Environment variables with fallback to defaults
- **Type Safety**: Strongly-typed Config struct with proper types (durations, booleans, etc.)

## Architecture

```mermaid
graph LR
    subgraph Sources
        ENV[Environment Variables]
        DEF[Default Values]
    end
    
    subgraph Config["Config Package"]
        LOAD[New()]
        VAL[Validate]
        CFG[Config Struct]
    end
    
    subgraph Consumers
        SRV[Server]
        PRX[Proxy]
        DB[Database]
        ADM[Admin UI]
    end
    
    ENV --> LOAD
    DEF --> LOAD
    LOAD --> VAL
    VAL --> CFG
    CFG --> SRV
    CFG --> PRX
    CFG --> DB
    CFG --> ADM
```

## Key Types & Interfaces

| Type | Description |
|------|-------------|
| `Config` | Main configuration struct with all application settings |
| `AdminUIConfig` | Admin UI server-specific configuration |

### Constructor Functions

| Function | Description |
|----------|-------------|
| `New()` | Creates Config from environment variables with validation |

### Helper Functions

| Function | Description |
|----------|-------------|
| `EnvOrDefault(key, fallback)` | Get string from env or fallback |
| `EnvIntOrDefault(key, fallback)` | Get int from env or fallback |
| `EnvBoolOrDefault(key, fallback)` | Get bool from env or fallback |
| `EnvFloat64OrDefault(key, fallback)` | Get float64 from env or fallback |

## Configuration Structure

The `Config` struct is organized into logical sections:

### Server Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `ListenAddr` | `string` | Server listen address | `:8080` |
| `RequestTimeout` | `time.Duration` | Upstream API request timeout | `30s` |
| `MaxRequestSize` | `int64` | Max incoming request size (bytes) | `10485760` (10MB) |
| `MaxConcurrentReqs` | `int` | Max concurrent requests | `100` |

### Environment

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `APIEnv` | `string` | Environment: `production`, `development`, `test` | `development` |

### Database Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `DatabasePath` | `string` | SQLite database file path | `./data/llm-proxy.db` |
| `DatabasePoolSize` | `int` | Connection pool size | `10` |

### Authentication

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `ManagementToken` | `string` | Token for Management API access | **Required** |

### API Provider Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `APIConfigPath` | `string` | Path to API providers YAML config | `./config/api_providers.yaml` |
| `DefaultAPIProvider` | `string` | Default API provider name | `openai` |
| `OpenAIAPIURL` | `string` | OpenAI API base URL (legacy) | `https://api.openai.com` |
| `EnableStreaming` | `bool` | Enable SSE streaming responses | `true` |

### Admin UI Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `AdminUIPath` | `string` | Base path for admin UI | `/admin` |
| `AdminUI` | `AdminUIConfig` | Admin UI server config | See below |

#### AdminUIConfig Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `ListenAddr` | `string` | Admin UI listen address | `:8081` |
| `APIBaseURL` | `string` | Management API base URL | `http://localhost:8080` |
| `ManagementToken` | `string` | Management API token | Same as `ManagementToken` |
| `Enabled` | `bool` | Enable admin UI server | `true` |
| `TemplateDir` | `string` | HTML template directory | `web/templates` |

### Logging Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `LogLevel` | `string` | Log level: `debug`, `info`, `warn`, `error` | `info` |
| `LogFormat` | `string` | Log format: `json`, `console` | `json` |
| `LogFile` | `string` | Log file path (empty = stdout) | `` |

### Audit Logging

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `AuditEnabled` | `bool` | Enable audit logging | `true` |
| `AuditLogFile` | `string` | Audit log file path | `./data/audit.log` |
| `AuditCreateDir` | `bool` | Create parent directories | `true` |
| `AuditStoreInDB` | `bool` | Store audit events in database | `true` |

### Observability Middleware

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `ObservabilityEnabled` | `bool` | Enable async observability | `true` |
| `ObservabilityBufferSize` | `int` | Event buffer size | `1000` |

### CORS Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `CORSAllowedOrigins` | `[]string` | Allowed CORS origins | `["*"]` |
| `CORSAllowedMethods` | `[]string` | Allowed HTTP methods | `["GET", "POST", "PUT", "DELETE", "OPTIONS"]` |
| `CORSAllowedHeaders` | `[]string` | Allowed request headers | `["Authorization", "Content-Type"]` |
| `CORSMaxAge` | `time.Duration` | Preflight cache duration | `24h` |

### Rate Limiting

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `GlobalRateLimit` | `int` | Global requests per minute | `100` |
| `IPRateLimit` | `int` | Requests per minute per IP | `30` |

### Distributed Rate Limiting

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `DistributedRateLimitEnabled` | `bool` | Enable Redis-based rate limiting | `false` |
| `DistributedRateLimitPrefix` | `string` | Redis key prefix | `ratelimit:` |
| `DistributedRateLimitKeySecret` | `string` | HMAC secret for key hashing | `` |
| `DistributedRateLimitWindow` | `time.Duration` | Rate limit window | `1m` |
| `DistributedRateLimitMax` | `int` | Max requests per window | `60` |
| `DistributedRateLimitFallback` | `bool` | Fallback to in-memory on Redis error | `true` |

### Monitoring

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `EnableMetrics` | `bool` | Enable metrics endpoint | `true` |
| `MetricsPath` | `string` | Metrics endpoint path | `/metrics` |

### Cleanup

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `TokenCleanupInterval` | `time.Duration` | Expired token cleanup interval | `1h` |

### Project Active Guard

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `EnforceProjectActive` | `bool` | Enforce project active status | `true` |
| `ActiveCacheTTL` | `time.Duration` | Project active cache TTL | `5s` |
| `ActiveCacheMax` | `int` | Max cache entries | `10000` |

### Event Bus Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `EventBusBackend` | `string` | Event bus type: `redis`, `redis-streams`, `in-memory` | `redis` |
| `RedisAddr` | `string` | Redis server address | `localhost:6379` |
| `RedisDB` | `int` | Redis database number | `0` |

### Redis Streams Configuration

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `RedisStreamKey` | `string` | Stream key name | `llm-proxy-events` |
| `RedisConsumerGroup` | `string` | Consumer group name | `llm-proxy-dispatchers` |
| `RedisConsumerName` | `string` | Consumer name (auto-generated if empty) | `` |
| `RedisStreamMaxLen` | `int64` | Max stream length (0 = unlimited) | `10000` |
| `RedisStreamBlockTime` | `time.Duration` | Block timeout for XREADGROUP | `5s` |
| `RedisStreamClaimTime` | `time.Duration` | Min idle time before claiming pending | `30s` |
| `RedisStreamBatchSize` | `int64` | Batch size for reading | `100` |

### Cache Stats Aggregation

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `CacheStatsBufferSize` | `int` | Buffer size for cache stats | `1000` |

## Environment Variable Reference

Complete mapping of environment variables to configuration fields:

### Server & Core

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `LISTEN_ADDR` | string | `ListenAddr` | `:8080` |
| `REQUEST_TIMEOUT` | duration | `RequestTimeout` | `30s` |
| `MAX_REQUEST_SIZE` | int64 | `MaxRequestSize` | `10485760` |
| `MAX_CONCURRENT_REQUESTS` | int | `MaxConcurrentReqs` | `100` |
| `API_ENV` | string | `APIEnv` | `development` |
| `MANAGEMENT_TOKEN` | string | `ManagementToken` | **Required** |

### Database

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `DATABASE_PATH` | string | `DatabasePath` | `./data/llm-proxy.db` |
| `DATABASE_POOL_SIZE` | int | `DatabasePoolSize` | `10` |

### API Provider

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `API_CONFIG_PATH` | string | `APIConfigPath` | `./config/api_providers.yaml` |
| `DEFAULT_API_PROVIDER` | string | `DefaultAPIProvider` | `openai` |
| `OPENAI_API_URL` | string | `OpenAIAPIURL` | `https://api.openai.com` |
| `ENABLE_STREAMING` | bool | `EnableStreaming` | `true` |

### Admin UI

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `ADMIN_UI_PATH` | string | `AdminUIPath` | `/admin` |
| `ADMIN_UI_LISTEN_ADDR` | string | `AdminUI.ListenAddr` | `:8081` |
| `ADMIN_UI_API_BASE_URL` | string | `AdminUI.APIBaseURL` | `http://localhost:8080` |
| `ADMIN_UI_ENABLED` | bool | `AdminUI.Enabled` | `true` |
| `ADMIN_UI_TEMPLATE_DIR` | string | `AdminUI.TemplateDir` | `web/templates` |

### Logging

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `LOG_LEVEL` | string | `LogLevel` | `info` |
| `LOG_FORMAT` | string | `LogFormat` | `json` |
| `LOG_FILE` | string | `LogFile` | `` |

### Audit

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `AUDIT_ENABLED` | bool | `AuditEnabled` | `true` |
| `AUDIT_LOG_FILE` | string | `AuditLogFile` | `./data/audit.log` |
| `AUDIT_CREATE_DIR` | bool | `AuditCreateDir` | `true` |
| `AUDIT_STORE_IN_DB` | bool | `AuditStoreInDB` | `true` |

### Observability

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `OBSERVABILITY_ENABLED` | bool | `ObservabilityEnabled` | `true` |
| `OBSERVABILITY_BUFFER_SIZE` | int | `ObservabilityBufferSize` | `1000` |

### CORS

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `CORS_ALLOWED_ORIGINS` | string (comma-separated) | `CORSAllowedOrigins` | `*` |
| `CORS_ALLOWED_METHODS` | string (comma-separated) | `CORSAllowedMethods` | `GET,POST,PUT,DELETE,OPTIONS` |
| `CORS_ALLOWED_HEADERS` | string (comma-separated) | `CORSAllowedHeaders` | `Authorization,Content-Type` |
| `CORS_MAX_AGE` | duration | `CORSMaxAge` | `24h` |

### Rate Limiting

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `GLOBAL_RATE_LIMIT` | int | `GlobalRateLimit` | `100` |
| `IP_RATE_LIMIT` | int | `IPRateLimit` | `30` |
| `DISTRIBUTED_RATE_LIMIT_ENABLED` | bool | `DistributedRateLimitEnabled` | `false` |
| `DISTRIBUTED_RATE_LIMIT_PREFIX` | string | `DistributedRateLimitPrefix` | `ratelimit:` |
| `DISTRIBUTED_RATE_LIMIT_KEY_SECRET` | string | `DistributedRateLimitKeySecret` | `` |
| `DISTRIBUTED_RATE_LIMIT_WINDOW` | duration | `DistributedRateLimitWindow` | `1m` |
| `DISTRIBUTED_RATE_LIMIT_MAX` | int | `DistributedRateLimitMax` | `60` |
| `DISTRIBUTED_RATE_LIMIT_FALLBACK` | bool | `DistributedRateLimitFallback` | `true` |

### Monitoring

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `ENABLE_METRICS` | bool | `EnableMetrics` | `true` |
| `METRICS_PATH` | string | `MetricsPath` | `/metrics` |

### Cleanup

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `TOKEN_CLEANUP_INTERVAL` | duration | `TokenCleanupInterval` | `1h` |

### Project Active Guard

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `LLM_PROXY_ENFORCE_PROJECT_ACTIVE` | bool | `EnforceProjectActive` | `true` |
| `LLM_PROXY_ACTIVE_CACHE_TTL` | duration | `ActiveCacheTTL` | `5s` |
| `LLM_PROXY_ACTIVE_CACHE_MAX` | int | `ActiveCacheMax` | `10000` |

### Event Bus

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `LLM_PROXY_EVENT_BUS` | string | `EventBusBackend` | `redis` |
| `REDIS_ADDR` | string | `RedisAddr` | `localhost:6379` |
| `REDIS_DB` | int | `RedisDB` | `0` |

### Redis Streams

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `REDIS_STREAM_KEY` | string | `RedisStreamKey` | `llm-proxy-events` |
| `REDIS_CONSUMER_GROUP` | string | `RedisConsumerGroup` | `llm-proxy-dispatchers` |
| `REDIS_CONSUMER_NAME` | string | `RedisConsumerName` | `` (auto-generated) |
| `REDIS_STREAM_MAX_LEN` | int64 | `RedisStreamMaxLen` | `10000` |
| `REDIS_STREAM_BLOCK_TIME` | duration | `RedisStreamBlockTime` | `5s` |
| `REDIS_STREAM_CLAIM_TIME` | duration | `RedisStreamClaimTime` | `30s` |
| `REDIS_STREAM_BATCH_SIZE` | int64 | `RedisStreamBatchSize` | `100` |

### Cache Stats

| Variable | Type | Field | Default |
|----------|------|-------|---------|
| `CACHE_STATS_BUFFER_SIZE` | int | `CacheStatsBufferSize` | `1000` |

## Configuration Sources Precedence

Configuration values are loaded in the following order (later sources override earlier ones):

1. **Default Values**: Hard-coded defaults in `New()` function
2. **Environment Variables**: Values from OS environment

**Note**: There is no configuration file support. All configuration is done via environment variables or defaults.

## Validation Logic

The `New()` function performs validation:

### Required Fields

| Field | Validation |
|-------|------------|
| `ManagementToken` | Must be non-empty |

### Automatic Adjustments

- Empty strings remain empty (not replaced with defaults)
- Invalid durations fall back to defaults
- Invalid integers fall back to defaults
- Invalid booleans fall back to defaults

## Usage Examples

### Basic Configuration Loading

```go
package main

import (
    "fmt"
    "github.com/sofatutor/llm-proxy/internal/config"
)

func main() {
    cfg, err := config.New()
    if err != nil {
        panic(fmt.Sprintf("failed to load config: %v", err))
    }

    fmt.Printf("Server listening on: %s\n", cfg.ListenAddr)
    fmt.Printf("Database path: %s\n", cfg.DatabasePath)
    fmt.Printf("Log level: %s\n", cfg.LogLevel)
}
```

### Accessing Admin UI Config

```go
cfg, err := config.New()
if err != nil {
    panic(err)
}

if cfg.AdminUI.Enabled {
    fmt.Printf("Admin UI enabled on: %s\n", cfg.AdminUI.ListenAddr)
    fmt.Printf("Management API: %s\n", cfg.AdminUI.APIBaseURL)
}
```

### Using Helper Functions

```go
import "github.com/sofatutor/llm-proxy/internal/config"

// Get string with fallback
redisAddr := config.EnvOrDefault("REDIS_ADDR", "localhost:6379")

// Get int with fallback
poolSize := config.EnvIntOrDefault("DATABASE_POOL_SIZE", 10)

// Get bool with fallback
enabled := config.EnvBoolOrDefault("AUDIT_ENABLED", true)

// Get float64 with fallback
timeout := config.EnvFloat64OrDefault("TIMEOUT_SECONDS", 30.0)
```

### Custom Configuration in Tests

```go
func TestWithCustomConfig(t *testing.T) {
    // Set environment variables
    os.Setenv("MANAGEMENT_TOKEN", "test-token")
    os.Setenv("LISTEN_ADDR", ":9999")
    os.Setenv("LOG_LEVEL", "debug")
    
    cfg, err := config.New()
    if err != nil {
        t.Fatal(err)
    }
    
    assert.Equal(t, ":9999", cfg.ListenAddr)
    assert.Equal(t, "debug", cfg.LogLevel)
}
```

## Testing Guidance

- Set `MANAGEMENT_TOKEN` in test environment to avoid validation errors
- Use `os.Setenv()` to override defaults in tests
- Clean up environment variables with `os.Unsetenv()` after tests
- See `config_test.go` for comprehensive test examples

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `MANAGEMENT_TOKEN environment variable is required` | Token not set | Set `MANAGEMENT_TOKEN` env var |
| Config uses wrong defaults | Environment var typo | Check variable names (case-sensitive) |
| Duration parse error | Invalid duration format | Use Go duration format (e.g., `30s`, `1h`) |
| Boolean parse error | Invalid bool value | Use `true`, `false`, `1`, `0` |

### Duration Format

Go duration strings use these units:

| Unit | Example |
|------|---------|
| Nanoseconds | `100ns` |
| Microseconds | `100us` |
| Milliseconds | `100ms` |
| Seconds | `30s` |
| Minutes | `5m` |
| Hours | `24h` |

Can be combined: `1h30m`, `2h15m30s`

## Related Packages

| Package | Relationship |
|---------|--------------|
| [`server`](../server/README.md) | Uses Config for server setup |
| [`proxy`](../proxy/README.md) | Uses Config for proxy behavior |
| [`database`](../database/README.md) | Uses Config for database connection |
| [`admin`](../admin/README.md) | Uses AdminUIConfig for admin server |
| [`logging`](../logging/README.md) | Uses Config for log settings |
| [`audit`](../audit/README.md) | Uses Config for audit settings |
| [`eventbus`](../eventbus/README.md) | Uses Config for event bus backend |

## Files

| File | Description |
|------|-------------|
| `config.go` | Main Config struct, New() function, and defaults |
| `env.go` | Helper functions for environment variable parsing |
| `config_test.go` | Comprehensive configuration tests |
| `env_test.go` | Environment helper function tests |
