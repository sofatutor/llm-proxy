---
title: Configuration Reference
parent: Getting Started
nav_order: 3
---

# Configuration Reference

This document provides a comprehensive reference for all LLM Proxy configuration options.

## Configuration Methods

LLM Proxy supports configuration through:

1. **Environment Variables** (highest priority)
2. **Configuration Files** (`.env`, `api_providers.yaml`)
3. **Command-line Flags** (for CLI commands)

### Precedence

When the same setting is defined in multiple places:
```
Command-line flags > Environment variables > Configuration files > Defaults
```

## Core Configuration

### Server Settings

| Variable | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `MANAGEMENT_TOKEN` | string | - | **Yes** | Admin API access token. Generate with `openssl rand -base64 32` |
| `LISTEN_ADDR` | string | `:8080` | No | Server listen address and port |
| `REQUEST_TIMEOUT` | duration | `30s` | No | Timeout for upstream API requests |
| `MAX_REQUEST_SIZE` | string | `10MB` | No | Maximum size of incoming requests |
| `ENABLE_STREAMING` | bool | `true` | No | Enable support for streaming responses |

### Database Configuration

LLM Proxy supports SQLite (default) and PostgreSQL databases.

#### SQLite (Default)

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DB_DRIVER` | string | `sqlite` | Database driver |
| `DATABASE_PATH` | string | `./data/llm-proxy.db` | Path to SQLite database file |

#### PostgreSQL

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DB_DRIVER` | string | `sqlite` | Set to `postgres` for PostgreSQL |
| `DATABASE_URL` | string | - | PostgreSQL connection string |

**PostgreSQL Connection String Format:**
```
postgres://user:password@host:port/database?sslmode=require
```

**Example:**
```bash
DATABASE_URL=postgres://llmproxy:secret@localhost:5432/llmproxy?sslmode=require
```

#### Connection Pool (Both Drivers)

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DATABASE_POOL_SIZE` | int | `10` | Maximum open connections |
| `DATABASE_MAX_IDLE_CONNS` | int | `5` | Maximum idle connections |
| `DATABASE_CONN_MAX_LIFETIME` | duration | `1h` | Maximum connection lifetime |

## Caching Configuration

HTTP response caching improves performance by storing and reusing responses.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `HTTP_CACHE_ENABLED` | bool | `true` | Enable HTTP response caching |
| `HTTP_CACHE_BACKEND` | string | `in-memory` | Cache backend (`redis` or `in-memory`) |
| `REDIS_ADDR` | string | `localhost:6379` | Redis server address (shared with event bus) |
| `REDIS_DB` | int | `0` | Redis database number |
| `REDIS_CACHE_URL` | string | (auto) | Optional override; constructed from `REDIS_ADDR` + `REDIS_DB` if not set |
| `REDIS_CACHE_KEY_PREFIX` | string | `llmproxy:cache:` | Prefix for Redis cache keys |
| `HTTP_CACHE_MAX_OBJECT_BYTES` | int | `1048576` | Maximum cached object size (1MB) |
| `HTTP_CACHE_DEFAULT_TTL` | int | `300` | Default TTL in seconds (5 minutes) |
| `CACHE_STATS_BUFFER_SIZE` | int | `1000` | Buffer size for cache hit tracking |
| `USAGE_STATS_BUFFER_SIZE` | int | `1000` | Buffer size for async unlimited-token usage tracking (falls back to `CACHE_STATS_BUFFER_SIZE`) |

### Cache Behavior

- **GET/HEAD requests**: Cached by default when upstream permits
- **POST requests**: Only cached when client explicitly opts in via `Cache-Control` header
- **Authentication**: Cached responses for authenticated requests are only served if marked as publicly cacheable

See [Caching Strategy](caching-strategy.md) for detailed caching behavior.

## Logging Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LOG_LEVEL` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | string | `json` | Log format: `json` or `text` |
| `LOG_FILE` | string | - | Path to log file (stdout if empty) |
| `LOG_MAX_SIZE_MB` | int | `10` | Rotate log after this size in MB |
| `LOG_MAX_BACKUPS` | int | `5` | Number of rotated log files to keep |

## Audit Logging

Audit logging records security-sensitive operations for compliance.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `AUDIT_ENABLED` | bool | `true` | Enable audit logging |
| `AUDIT_LOG_FILE` | string | `./data/audit.log` | Audit log file path |
| `AUDIT_STORE_IN_DB` | bool | `true` | Store audit events in database |
| `AUDIT_CREATE_DIR` | bool | `true` | Create audit log directories |

### Audit Storage Options

```bash
# File-only audit logging
AUDIT_ENABLED=true
AUDIT_LOG_FILE=./data/audit.log
AUDIT_STORE_IN_DB=false

# Database-only audit logging
AUDIT_ENABLED=true
AUDIT_LOG_FILE=""
AUDIT_STORE_IN_DB=true

# Both file and database
AUDIT_ENABLED=true
AUDIT_LOG_FILE=./data/audit.log
AUDIT_STORE_IN_DB=true
```

See [Security Best Practices](security.md#audit-logging) for detailed audit configuration.

## Event Bus and Instrumentation

The async event bus handles observability events for all API calls.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `OBSERVABILITY_ENABLED` | bool | - | **Deprecated**: Event bus is always enabled |
| `OBSERVABILITY_BUFFER_SIZE` | int | `1000` | Event buffer size (for in-memory backend) |
| `FILE_EVENT_LOG` | string | - | Path to persistent event log file |
| `LLM_PROXY_EVENT_BUS` | string | `redis-streams` | Event bus backend: `redis-streams` or `in-memory` |
| `REDIS_ADDR` | string | `localhost:6379` | Redis address for event bus |
| `REDIS_DB` | int | `0` | Redis database number |

### Redis Streams Configuration (Recommended for Production)

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `REDIS_STREAM_KEY` | string | `llm-proxy-events` | Stream key name |
| `REDIS_CONSUMER_GROUP` | string | `llm-proxy-dispatchers` | Consumer group name |
| `REDIS_CONSUMER_NAME` | string | auto-generated | Consumer name (unique per instance) |
| `REDIS_STREAM_MAX_LEN` | int | `10000` | Max stream length (0 = unlimited) |
| `REDIS_STREAM_BLOCK_TIME` | duration | `5s` | Block timeout for reading |
| `REDIS_STREAM_CLAIM_TIME` | duration | `30s` | Min idle time before claiming pending messages |
| `REDIS_STREAM_BATCH_SIZE` | int | `100` | Batch size for reading messages |

### Event Dispatcher

For external observability platforms:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `LLM_PROXY_API_KEY` | string | - | API key for external service |
| `LLM_PROXY_ENDPOINT` | string | - | Default endpoint URL |

See [Instrumentation Guide](instrumentation.md) for event system details.

## Admin UI Configuration

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ADMIN_UI_ENABLED` | bool | `true` | Enable/disable the Admin UI |
| `ADMIN_UI_PATH` | string | `/admin` | Base path for Admin UI |
| `ADMIN_UI_API_BASE_URL` | string | - | API base URL for Admin UI service |

## Security Configuration

### CORS Settings

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `CORS_ALLOWED_ORIGINS` | string | `*` | Allowed origins (comma-separated) |
| `CORS_ALLOWED_METHODS` | string | `GET,POST,PUT,DELETE,OPTIONS` | Allowed HTTP methods |
| `CORS_ALLOWED_HEADERS` | string | `Authorization,Content-Type` | Allowed request headers |
| `CORS_MAX_AGE` | int | `86400` | Preflight cache duration (seconds) |

> **Security Note**: For production, specify exact origins instead of wildcard (`*`).

### TLS/HTTPS

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ENABLE_TLS` | bool | `false` | Enable TLS |
| `TLS_CERT_FILE` | string | `./certs/server.crt` | Path to TLS certificate |
| `TLS_KEY_FILE` | string | `./certs/server.key` | Path to TLS private key |
| `TLS_MIN_VERSION` | string | `1.2` | Minimum TLS version |

### Rate Limiting

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `GLOBAL_RATE_LIMIT` | int | `100` | Max requests per minute globally |
| `IP_RATE_LIMIT` | int | `30` | Max requests per minute per IP |

#### Distributed Rate Limiting (Redis)

For multi-instance deployments:

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DISTRIBUTED_RATE_LIMIT_ENABLED` | bool | `false` | Enable Redis-backed rate limiting |
| `DISTRIBUTED_RATE_LIMIT_PREFIX` | string | `ratelimit:` | Redis key prefix |
| `DISTRIBUTED_RATE_LIMIT_WINDOW` | duration | `1m` | Sliding window duration |
| `DISTRIBUTED_RATE_LIMIT_MAX` | int | `60` | Max requests per window |
| `DISTRIBUTED_RATE_LIMIT_FALLBACK` | bool | `true` | Fallback to in-memory when Redis unavailable |
| `DISTRIBUTED_RATE_LIMIT_KEY_SECRET` | string | - | HMAC secret for hashing token IDs |

### Encryption

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ENCRYPTION_KEY` | string | - | 32-byte key for encrypting API keys at rest |

Generate encryption key:
```bash
export ENCRYPTION_KEY=$(openssl rand -base64 32)
```

### Token Security

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `DEFAULT_TOKEN_LIFETIME` | duration | `30d` | Default token lifetime |
| `DEFAULT_TOKEN_REQUEST_LIMIT` | int | `5000` | Default max requests per token |
| `TOKEN_CLEANUP_INTERVAL` | duration | `1h` | Interval for cleaning up expired tokens |

### API Key Security

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `MASK_API_KEYS` | bool | `true` | Mask API keys in logs |
| `VALIDATE_API_KEY_FORMAT` | bool | `true` | Validate API key format |

## API Provider Configuration

Advanced API provider settings are configured in `api_providers.yaml`.

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `API_CONFIG_PATH` | string | `./config/api_providers.yaml` | Path to API providers config |
| `DEFAULT_API_PROVIDER` | string | `openai` | Default API provider |
| `OPENAI_API_URL` | string | `https://api.openai.com` | Base URL for OpenAI API |

See [API Configuration Guide](api-configuration.md) for detailed provider configuration.

## Performance Tuning

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `MAX_CONCURRENT_REQUESTS` | int | `100` | Max concurrent requests |
| `WORKER_POOL_SIZE` | int | `10` | Worker goroutines for background tasks |

## Metrics and Monitoring

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `ENABLE_METRICS` | bool | `true` | Enable metrics endpoints |
| `METRICS_PATH` | string | `/metrics` | Base path for metrics endpoints (JSON at base, Prometheus at `<base>/prometheus`) |

### Available Metrics Endpoints

- **`/metrics`**: Provider-agnostic JSON format (default)
- **`/metrics/prometheus`**: Prometheus text exposition format

Both endpoints expose the same core metrics (uptime, requests, errors, cache statistics) in different formats. The Prometheus endpoint is registered at `METRICS_PATH + "/prometheus"`. See [Instrumentation Guide](../observability/instrumentation.md#prometheus-metrics-endpoint) for Prometheus scraping configuration.

## Example Configuration Files

### Minimal `.env`

```bash
# Required
MANAGEMENT_TOKEN=your-secure-random-token

# Recommended
LOG_LEVEL=info
```

### Development `.env`

```bash
# Core
MANAGEMENT_TOKEN=dev-management-token
LISTEN_ADDR=:8080

# Database (SQLite)
DB_DRIVER=sqlite
DATABASE_PATH=./data/llm-proxy.db

# Logging
LOG_LEVEL=debug
LOG_FORMAT=text

# Caching (in-memory for development)
HTTP_CACHE_ENABLED=true
HTTP_CACHE_BACKEND=in-memory

# Admin UI
ADMIN_UI_ENABLED=true
```

### Production `.env`

```bash
# Core
MANAGEMENT_TOKEN=<secure-random-token>
LISTEN_ADDR=:8080

# Database (PostgreSQL)
DB_DRIVER=postgres
DATABASE_URL=postgres://llmproxy:${POSTGRES_PASSWORD}@postgres:5432/llmproxy?sslmode=require
DATABASE_POOL_SIZE=20
DATABASE_MAX_IDLE_CONNS=10
DATABASE_CONN_MAX_LIFETIME=30m

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=/var/log/llm-proxy/app.log

# Audit
AUDIT_ENABLED=true
AUDIT_LOG_FILE=/var/log/llm-proxy/audit.log
AUDIT_STORE_IN_DB=true

# Caching (Redis)
HTTP_CACHE_ENABLED=true
HTTP_CACHE_BACKEND=redis

# Redis (shared by cache and event bus)
LLM_PROXY_EVENT_BUS=redis-streams
REDIS_ADDR=redis:6379
REDIS_DB=0
REDIS_STREAM_KEY=llm-proxy-events
REDIS_CONSUMER_GROUP=llm-proxy-dispatchers
REDIS_STREAM_MAX_LEN=50000
OBSERVABILITY_BUFFER_SIZE=2000

# Security
ENCRYPTION_KEY=<32-byte-base64-key>
CORS_ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com
ENABLE_TLS=false  # Use reverse proxy for TLS termination

# Rate Limiting
DISTRIBUTED_RATE_LIMIT_ENABLED=true
DISTRIBUTED_RATE_LIMIT_WINDOW=1m
DISTRIBUTED_RATE_LIMIT_MAX=100

# Performance
MAX_CONCURRENT_REQUESTS=200
WORKER_POOL_SIZE=20
```

### `api_providers.yaml` Example

```yaml
default_api: openai

apis:
  openai:
    base_url: https://api.openai.com
    allowed_endpoints:
      - /v1/chat/completions
      - /v1/completions
      - /v1/models
      - /v1/embeddings
    allowed_methods:
      - GET
      - POST
    param_whitelist:
      model:
        - gpt-4o
        - gpt-4o-mini
        - gpt-4
        - gpt-3.5-turbo
    timeouts:
      request: 60s
      response_header: 30s
      idle_connection: 90s
      flush_interval: 100ms
    connection:
      max_idle_conns: 100
      max_idle_conns_per_host: 20
```

## Configuration Validation

The proxy validates configuration at startup and will fail if:

- `MANAGEMENT_TOKEN` is not set
- `DATABASE_URL` is invalid (when using PostgreSQL)
- `ENCRYPTION_KEY` is not 32 bytes (when provided)
- API provider configuration is malformed

Check logs for configuration errors:
```bash
# Docker
docker logs llm-proxy | grep -i "config\|error"

# From source
./bin/llm-proxy server 2>&1 | grep -i "config\|error"
```

## Related Documentation

- [Installation Guide](installation.md)
- [API Configuration Guide](../guides/api-configuration.md)
- [Security Best Practices](../deployment/security.md)
- [Performance Tuning Guide](../deployment/performance.md)
- [PostgreSQL Troubleshooting](../database/postgresql-troubleshooting.md)
