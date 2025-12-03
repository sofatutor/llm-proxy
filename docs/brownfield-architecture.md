# LLM Proxy - Brownfield Architecture Document

**Document Version**: 2.0  
**Date**: December 3, 2025  
**Purpose**: Capture the ACTUAL state of the LLM Proxy codebase for AI agents and developers

> **CRITICAL**: This document describes the CURRENT STATE of the system, including technical debt, workarounds, and real-world constraints. It is NOT an aspirational architecture document. For ideal architecture, see `docs/architecture.md`.

---

## Document Scope & Change Log

### Scope
This document captures the brownfield reality of the LLM Proxy project as it exists today, including:
- Actual implementation patterns (not theoretical best practices)
- Technical debt and known issues
- Workarounds and constraints that must be respected
- Real file locations and module organization
- Performance characteristics and bottlenecks
- What's implemented vs what's planned

### Change Log

| Date | Version | Description | Author |
|------|---------|-------------|--------|
| 2025-11-11 | 1.0 | Initial brownfield analysis | AI Documentation Agent |
| 2025-12-03 | 2.0 | Major update: PostgreSQL, migrations, rate limiting, cache invalidation completed; AWS ECS deployment approach documented | AI Documentation Agent |

---

## Quick Reference - Critical Files & Entry Points

### Main Entry Points
- **Primary CLI**: `cmd/proxy/main.go` - Main llm-proxy command with all subcommands
- **Event Dispatcher**: `cmd/eventdispatcher/main.go` - Standalone dispatcher service
- **Server Startup**: `internal/server/server.go:New()` - HTTP server initialization

### Critical Business Logic
- **Token Validation**: `internal/token/validate.go` + `internal/token/cache.go` (LRU cache)
- **Proxy Core**: `internal/proxy/proxy.go` - Transparent reverse proxy
- **Event Bus**: `internal/eventbus/eventbus.go` - In-memory and Redis implementations
- **Database Layer**: `internal/database/database.go` - SQLite (default) or PostgreSQL
- **Audit Logging**: `internal/audit/logger.go` - Dual storage (file + database)
- **Distributed Rate Limiting**: `internal/token/redis_ratelimit.go` - Redis-backed rate limiting

### Configuration Files
- **Environment**: `.env` (not in repo, created by setup)
- **API Providers**: `config/api_providers.yaml` - Endpoint whitelists and provider config
- **Database Migrations**: `internal/database/migrations/sql/` - Goose migrations

### Key Algorithms & Complex Logic
- **Token Cache Eviction**: `internal/token/cache.go:evictOldest()` - Min-heap based LRU
- **Cache Key Generation**: `internal/proxy/cache_helpers.go:generateCacheKey()` - Deterministic key with Vary support
- **Streaming Capture**: `internal/proxy/stream_capture.go` - Captures streaming responses for caching
- **Event Transformation**: `internal/eventtransformer/openai.go` - OpenAI-specific event transformation with tiktoken

---

## High-Level Architecture (Actual Implementation)

### Tech Stack Reality Check

| Category | Technology | Version | Notes & Constraints |
|----------|------------|---------|---------------------|
| Language | Go | 1.23.9 | Must use 1.23+ for latest features |
| Database (Dev) | SQLite | 3.x | Via `mattn/go-sqlite3`, default for local dev |
| Database (Prod) | PostgreSQL | 13+ | ✅ **Implemented** - Use `DB_DRIVER=postgres` |
| Cache Backend | Redis | 7.x | Required for distributed rate limiting |
| HTTP Framework | Gin | 1.10.1 | Used ONLY for admin UI, not proxy |
| Logging | Zap | 1.27.0 | Structured logging, app-level only |
| Testing | Testify | 1.10.0 | Mock generation and assertions |
| Migrations | Goose | 3.x | ✅ **Implemented** - SQL-based migrations |

**IMPORTANT CONSTRAINTS**:
- SQLite is for development; **PostgreSQL is recommended for production**
- Redis is required for distributed rate limiting and caching in production
- Gin is isolated to admin UI - proxy uses standard `net/http`

### Repository Structure (Actual)

```
llm-proxy/
├── cmd/
│   ├── proxy/              # Main CLI (all user commands)
│   │   ├── main.go         # Entry point
│   │   ├── server.go       # Server command
│   │   ├── admin.go        # Admin UI command
│   │   └── chat.go         # OpenAI chat command
│   └── eventdispatcher/    # Standalone dispatcher CLI
│       └── main.go
├── internal/               # All core logic (90%+ coverage required)
│   ├── server/             # HTTP server lifecycle
│   ├── proxy/              # Transparent reverse proxy (31 files!)
│   ├── token/              # Token management (21 files, includes redis_ratelimit)
│   ├── database/           # Data persistence (39 files, includes migrations)
│   │   └── migrations/     # Goose migrations (SQLite + PostgreSQL)
│   ├── eventbus/           # Async event system (4 files)
│   ├── dispatcher/         # Event dispatcher service (17 files)
│   ├── middleware/         # HTTP middleware (4 files)
│   ├── admin/              # Admin UI handlers (8 files)
│   ├── audit/              # Audit logging (4 files)
│   ├── config/             # Configuration (4 files)
│   ├── logging/            # Structured logging (4 files)
│   ├── eventtransformer/   # Event transformation (11 files)
│   ├── obfuscate/          # Token obfuscation (2 files)
│   ├── client/             # OpenAI client (2 files)
│   ├── setup/              # Setup wizard (2 files)
│   ├── api/                # Management API types (3 files)
│   └── utils/              # Crypto utilities (2 files)
├── web/                    # Admin UI static assets
│   ├── static/             # CSS, JS
│   └── templates/          # HTML templates (17 files)
├── e2e/                    # Playwright E2E tests
├── test/                   # Integration tests
├── config/                 # Configuration files
├── docs/                   # Documentation (this file!)
│   └── architecture/planned/  # AWS ECS CDK architecture
└── api/                    # OpenAPI specs
```

**CRITICAL NOTES**:
- `internal/proxy/` has 31 files - this is the most complex package
- `internal/token/` has 21 files - second most complex (includes rate limiting)
- `internal/database/` now has 39 files including migrations
- `cmd/` has minimal logic (coverage not required per PLAN.md)
- All testable logic MUST be in `internal/` packages

---

## Production Deployment: AWS ECS (Recommended)

> **NEW in v2.0**: AWS ECS with CDK is now the recommended production deployment approach. See [Issue #174](https://github.com/sofatutor/llm-proxy/issues/174) and `docs/architecture/planned/aws-ecs-cdk.md`.

### Architecture Overview

```
Internet → ALB (TLS via ACM) → ECS Fargate (1-4 tasks)
                                      ↓
                  ┌───────────────────┼───────────────────┐
                  ↓                   ↓                   ↓
             Aurora PG          ElastiCache          CloudWatch
            Serverless v2          Redis
```

### What AWS Handles (Not Application Concerns)

| Concern | AWS Solution | Status |
|---------|--------------|--------|
| **HTTPS/TLS** | ALB + ACM certificates | ✅ Auto-renewal |
| **Multi-port routing** | ALB path-based routing | ✅ Single entry point |
| **Secrets management** | Secrets Manager + SSM | ✅ Native injection |
| **Database** | Aurora PostgreSQL Serverless v2 | ✅ Auto-scaling |
| **Caching/Events** | ElastiCache Redis | ✅ TLS enabled |
| **Observability** | CloudWatch Logs/Metrics | ✅ Native integration |
| **Auto-scaling** | ECS Service Auto Scaling | ✅ CPU/request-based |

### Path-Based Routing (ALB)

| Path Pattern | Target | Port |
|--------------|--------|------|
| `/v1/*` | Proxy | 8080 |
| `/manage/*` | Proxy | 8080 |
| `/health`, `/ready`, `/live` | Proxy | 8080 |
| `/admin/*` | Admin UI | 8081 |

**Result**: Users see single HTTPS endpoint; ALB handles routing internally.

### Deployment Stories

| Story | Description | Status |
|-------|-------------|--------|
| [#176](https://github.com/sofatutor/llm-proxy/issues/176) | CDK Foundation & Project Setup | 🔲 Planned |
| [#177](https://github.com/sofatutor/llm-proxy/issues/177) | Data Layer (Aurora + Redis) | 🔲 Planned |
| [#178](https://github.com/sofatutor/llm-proxy/issues/178) | Compute Layer (ECS Fargate) | 🔲 Planned |
| [#179](https://github.com/sofatutor/llm-proxy/issues/179) | Networking (ALB + ACM) | 🔲 Planned |
| [#180](https://github.com/sofatutor/llm-proxy/issues/180) | Observability (CloudWatch) | 🔲 Planned |
| [#181](https://github.com/sofatutor/llm-proxy/issues/181) | CI/CD Pipeline | 🔲 Planned |
| [#182](https://github.com/sofatutor/llm-proxy/issues/182) | Production Readiness | 🔲 Planned |

---

## Core Components (Reality Check)

### 1. HTTP Server & Routing

**Implementation**: `internal/server/server.go`

**Actual Architecture**:
- Uses standard `net/http.Server` for proxy endpoints
- Gin framework ONLY for admin UI (separate port :8081)
- Middleware chain: RequestID → Instrumentation → Cache → Validation → Timeout

**Current State**:
- Admin UI and proxy run on different ports (design decision)
- **HTTPS**: Not built-in, but **not needed** with AWS ALB handling TLS termination
- **Multi-port**: Local concern only; AWS ALB unifies via path-based routing
- Graceful shutdown implemented and tested

**Performance Characteristics**:
- Request latency overhead: ~1-5ms (mostly token validation)
- Cache hit latency: <1ms
- Streaming responses: minimal buffering, true pass-through

### 2. Token Management System

**Implementation**: `internal/token/` (21 files)

**Actual Components**:
- `manager.go`: High-level token operations
- `validate.go`: Token validation logic
- `cache.go`: LRU cache with min-heap eviction (90%+ coverage)
- `ratelimit.go`: Per-token rate limiting (in-memory)
- `redis_ratelimit.go`: ✅ **NEW** Distributed rate limiting (Redis-backed)
- `revoke.go`: Soft deletion (sets `is_active = false`)

**Critical Implementation Details**:
- Tokens are UUIDv7 (time-ordered) but we DON'T extract timestamps
- Cache uses min-heap for O(log n) eviction (optimized after review)
- ✅ **Rate limiting is NOW distributed** via Redis (configurable)
- Revocation is soft delete - tokens never truly deleted from database

**Configuration (Distributed Rate Limiting)**:
```bash
DISTRIBUTED_RATE_LIMIT_ENABLED=true    # Enable Redis-backed rate limiting
DISTRIBUTED_RATE_LIMIT_PREFIX=ratelimit:  # Redis key prefix
DISTRIBUTED_RATE_LIMIT_WINDOW=1m       # Window duration
DISTRIBUTED_RATE_LIMIT_MAX=60          # Max requests per window
DISTRIBUTED_RATE_LIMIT_FALLBACK=true   # Fallback to in-memory if Redis unavailable
```

**Performance Characteristics**:
- Token validation (cache hit): ~100µs
- Token validation (cache miss): ~5-10ms (database query)
- Cache size: Configurable, default 1000 tokens
- Eviction: O(log n) with min-heap
- Distributed rate check: ~1-2ms (Redis)

### 3. Transparent Reverse Proxy

**Implementation**: `internal/proxy/` (31 files - largest package!)

**Actual Architecture**:
- Based on `httputil.ReverseProxy` with custom Director and ModifyResponse
- Minimal transformation: Authorization header replacement ONLY
- Streaming support: True pass-through with optional capture for caching
- Allowlist-based: Endpoints and methods validated against YAML config

**Critical Files**:
- `proxy.go`: Main proxy implementation
- `cache.go`: HTTP response caching middleware
- `cache_redis.go`: Redis backend for cache (with invalidation)
- `cache_purge_test.go`: Cache purge functionality tests
- `stream_capture.go`: Streaming response capture for caching
- `project_guard.go`: Blocks requests for inactive projects (403)

**Cache Invalidation** (✅ **NEW**):
- Manual cache purge via API endpoint
- Support for purge by key, prefix, or all entries
- Audit logging for purge operations

**Performance Characteristics**:
- Proxy overhead (no cache): ~2-5ms
- Proxy overhead (cache hit): <1ms
- Streaming latency: ~0ms (true pass-through)
- Connection pool: 100 max idle, 20 per host

**Caching Behavior** (IMPORTANT):
- GET/HEAD: Cached by default when upstream permits
- POST: Only cached when client sends `Cache-Control: public` (opt-in)
- Streaming: Captured during stream, cached after completion
- TTL: `s-maxage` > `max-age` > default (300s)
- Size limit: 1MB default (configurable)

### 4. Database Layer

**Implementation**: `internal/database/` (39 files)

**Actual State**:
- ✅ **SQLite** is the default for development
- ✅ **PostgreSQL** is fully supported for production
- ✅ **Goose migration system** implemented
- Configuration via `DB_DRIVER=sqlite|postgres` and `DATABASE_URL`

**Critical Tables**:
- `projects`: id (UUID), name, openai_api_key, is_active, created_at, updated_at, deactivated_at
- `tokens`: token (UUID), project_id, expires_at, is_active, request_count, created_at, deactivated_at, cache_hit_count
- `audit_events`: id, timestamp, action, actor, project_id, token_id, request_id, client_ip, result, details (JSON)

**Migration Files**:
```
internal/database/migrations/sql/
├── 00001_initial_schema.sql
├── 00002_add_deactivation_columns.sql
├── 00003_add_cache_hit_count.sql
└── postgres/
    ├── 00001_initial_schema.sql
    ├── 00002_add_deactivation_columns.sql
    └── 00003_add_cache_hit_count.sql
```

**PostgreSQL Configuration**:
```bash
DB_DRIVER=postgres
DATABASE_URL=postgres://user:pass@host:5432/llm_proxy?sslmode=require
```

**Performance Characteristics**:
- Token lookup: ~1-5ms (indexed on token column)
- Project lookup: ~1-5ms (indexed on id column)
- Audit log write: ~2-10ms (async, non-blocking)
- PostgreSQL: Full connection pooling support

### 5. Async Event System

**Implementation**: `internal/eventbus/` (4 files) + `internal/dispatcher/` (17 files)

**Actual Architecture**:
- Event bus: In-memory (default) or Redis (recommended for production)
- Dispatcher: Standalone service or embedded
- Backends: File (JSONL), Lunary, Helicone

**Critical Implementation Details**:
- **In-Memory Bus**: Buffered channel, fan-out to multiple subscribers
  - **Limitation**: Single-process only, events lost on restart
  - **Use Case**: Development, single-instance deployments
- **Redis Bus**: Redis list with TTL and max-length
  - **Limitation**: Events can be lost if dispatcher lags significantly
  - **Use Case**: Multi-process, distributed deployments

**Known Issues & Workarounds**:
- **Event Loss on Redis**: If dispatcher is down and Redis list expires, events are lost
  - **Workaround**: Increase Redis TTL and max-length, monitor dispatcher lag
  - **Future**: Redis Streams with consumer groups (Issue #112)

**Performance Characteristics**:
- Event publish: ~10-50µs (in-memory), ~1-2ms (Redis)
- Event delivery: Batched, configurable batch size
- Buffer size: 1000 events default (configurable)
- Throughput: ~10k events/sec (in-memory), ~1k events/sec (Redis)

### 6. HTTP Response Caching

**Implementation**: `internal/proxy/cache*.go` (multiple files)

**Actual State**:
- ✅ **Implemented and Working**: Redis backend + in-memory fallback
- ✅ **HTTP Standards Compliant**: Respects Cache-Control, ETag, Vary
- ✅ **Streaming Support**: Captures streaming responses during transmission
- ✅ **Cache Invalidation**: Manual purge via API endpoint

**Critical Implementation Details**:
- Cache key: `{prefix}:{project_id}:{method}:{path}:{sorted_query}:{vary_headers}:{body_hash}`
- TTL precedence: `s-maxage` > `max-age` > default (300s)
- Size limit: 1MB default (larger responses not cached)
- Vary handling: Conservative subset (Accept, Accept-Encoding, Accept-Language)

**Performance Characteristics**:
- Cache lookup: ~100-500µs (Redis), ~10-50µs (in-memory)
- Cache store: ~1-5ms (Redis), ~100µs (in-memory)
- Hit rate: Varies by workload, typically 20-50% for GET requests
- Memory usage: ~1KB per cached response (compressed)

---

## Technical Debt & Known Issues

### ✅ Resolved Technical Debt (December 2025)

| Issue | Status | Details |
|-------|--------|---------|
| PostgreSQL Support | ✅ Resolved | [#57](https://github.com/sofatutor/llm-proxy/issues/57) - Full support with migrations |
| Database Migrations | ✅ Resolved | [#109](https://github.com/sofatutor/llm-proxy/issues/109) - Goose migration system |
| Distributed Rate Limiting | ✅ Resolved | [#110](https://github.com/sofatutor/llm-proxy/issues/110) - Redis-backed |
| Cache Invalidation | ✅ Resolved | [#111](https://github.com/sofatutor/llm-proxy/issues/111) - Manual purge API |

### Non-Issues (Handled by AWS Infrastructure)

With the AWS ECS deployment approach ([#174](https://github.com/sofatutor/llm-proxy/issues/174)), these are no longer application concerns:

| Former Issue | AWS Solution | Status |
|--------------|--------------|--------|
| No Automatic HTTPS | ALB + ACM handles TLS | ✅ Non-issue |
| Admin UI on Separate Port | ALB path-based routing | ✅ Non-issue |
| Secrets in .env file | Secrets Manager + SSM | ✅ Non-issue |
| Database scaling | Aurora Serverless v2 | ✅ Non-issue |

### Remaining Technical Debt

#### 1. Event Loss Risk on Redis (Priority 2)
- **Status**: Documented warning, mitigation planned
- **Impact**: Events can be lost if dispatcher lags and Redis expires
- **GitHub Issue**: [#112](https://github.com/sofatutor/llm-proxy/issues/112)
- **Workaround**: Size Redis retention generously, monitor dispatcher lag
- **Future**: Redis Streams with consumer groups for guaranteed delivery

#### 2. Package READMEs Are Minimal (Priority 3)
- **Status**: Most are 4-9 lines, just bullet points
- **Impact**: Hard to understand package purpose and usage
- **GitHub Issue**: [#115](https://github.com/sofatutor/llm-proxy/issues/115)
- **Workaround**: Read code, check main docs

#### 3. Token Timestamp Not Used (Priority 4)
- **Status**: UUIDv7 has timestamp but we don't extract it
- **Impact**: Could use for cache key generation, debugging
- **Workaround**: Use `created_at` from database

---

## Workarounds & Gotchas (MUST KNOW)

### 1. Environment Variables Are Critical
- **Issue**: Many settings have no defaults, server won't start without them
- **Required**: `MANAGEMENT_TOKEN` (no default, must be set)
- **Workaround**: Use `llm-proxy setup --interactive` to generate `.env`
- **AWS Solution**: ECS injects from Secrets Manager/SSM automatically

### 2. Database Driver Selection
- **Default**: SQLite (`DB_DRIVER=sqlite` or unset)
- **Production**: PostgreSQL (`DB_DRIVER=postgres` + `DATABASE_URL`)
- **Build Tag**: PostgreSQL requires `go build -tags postgres`
- **AWS Solution**: Aurora PostgreSQL via DATABASE_URL from Secrets Manager

### 3. Redis Event Bus Requires Careful Tuning
- **Issue**: Redis list can expire before dispatcher reads events
- **Impact**: Event loss if dispatcher is down or lagging
- **Workaround**: Set high TTL and max-length, monitor dispatcher lag
- **AWS Solution**: ElastiCache Redis with proper sizing

### 4. Cache Hits Bypass Event Bus
- **Issue**: Cache hits don't publish events (performance optimization)
- **Impact**: Event logs are incomplete (missing cache hits)
- **Workaround**: This is by design - cache hits are logged separately
- **Gotcha**: Don't expect event bus to capture all requests

### 5. POST Caching Requires Client Opt-In
- **Issue**: POST requests only cached if client sends `Cache-Control: public`
- **Impact**: Most POST requests are not cached
- **Workaround**: Use `--cache` flag in benchmark tool for testing

### 6. Project Guard Queries Database on Every Request
- **Issue**: `is_active` check requires database query per request
- **Impact**: ~1-2ms latency overhead per request
- **Workaround**: This is a security vs performance tradeoff (no workaround)
- **Gotcha**: Can't disable this check (security requirement)

---

## Integration Points & External Dependencies

### External Services

| Service | Purpose | Integration Type | Key Files | Status |
|---------|---------|------------------|-----------|--------|
| OpenAI API | Primary API provider | HTTP REST | `internal/proxy/proxy.go` | ✅ Implemented |
| Redis (Cache) | HTTP response cache | Redis client | `internal/proxy/cache_redis.go` | ✅ Implemented |
| Redis (Events) | Event bus backend | Redis list | `internal/eventbus/eventbus.go` | ✅ Implemented |
| Redis (Rate Limit) | Distributed rate limiting | Redis INCR | `internal/token/redis_ratelimit.go` | ✅ Implemented |
| Lunary | Observability backend | HTTP REST | `internal/dispatcher/plugins/lunary.go` | ✅ Implemented |
| Helicone | Observability backend | HTTP REST | `internal/dispatcher/plugins/helicone.go` | ✅ Implemented |
| PostgreSQL | Production database | pgx driver | `internal/database/factory_postgres.go` | ✅ Implemented |

### Internal Integration Points

#### 1. Proxy → Token Validation
- **Flow**: Every request → Token validation → Project lookup → API key retrieval
- **Files**: `internal/proxy/proxy.go` → `internal/token/validate.go` → `internal/database/token.go`
- **Performance**: ~5-10ms (cache miss), ~100µs (cache hit)
- **Failure Mode**: 401 Unauthorized if token invalid, 403 Forbidden if project inactive

#### 2. Proxy → Distributed Rate Limiting
- **Flow**: Token validated → Rate limit check (Redis) → Allow/Deny
- **Files**: `internal/token/redis_ratelimit.go`
- **Performance**: ~1-2ms (Redis lookup)
- **Failure Mode**: 429 Too Many Requests if rate exceeded

#### 3. Proxy → Event Bus
- **Flow**: Request/response → Instrumentation middleware → Event bus → Dispatcher
- **Files**: `internal/middleware/instrumentation.go` → `internal/eventbus/eventbus.go` → `internal/dispatcher/service.go`
- **Performance**: ~10-50µs (in-memory), ~1-2ms (Redis)
- **Failure Mode**: Events dropped if buffer full, logged as warning

#### 4. Admin UI → Management API
- **Flow**: Admin UI (Gin) → Management API handlers → Database
- **Files**: `internal/admin/server.go` → `internal/server/management_api.go` → `internal/database/*.go`
- **Performance**: ~5-20ms per operation
- **Failure Mode**: 500 Internal Server Error if database unavailable

---

## Development & Deployment (Reality)

### Local Development Setup (Actual Steps)

1. **Clone and Install**:
   ```bash
   git clone https://github.com/sofatutor/llm-proxy.git
   cd llm-proxy
   make deps  # or go mod download
   ```

2. **Setup Configuration** (CRITICAL):
   ```bash
   # Interactive setup (recommended)
   go run cmd/proxy/main.go setup --interactive
   
   # This creates .env with MANAGEMENT_TOKEN and other settings
   ```

3. **Start Server**:
   ```bash
   # Start proxy server (SQLite by default)
   go run cmd/proxy/main.go server
   
   # Or with PostgreSQL
   DB_DRIVER=postgres DATABASE_URL=postgres://... go run -tags postgres cmd/proxy/main.go server
   ```

4. **Start Admin UI** (Optional):
   ```bash
   # In separate terminal
   go run cmd/proxy/main.go admin --management-token $MANAGEMENT_TOKEN
   ```

### Build & Deployment Process (Actual)

**Build Commands**:
```bash
# Build binaries (SQLite only)
make build  # Creates bin/llm-proxy

# Build with PostgreSQL support
go build -tags postgres -o bin/llm-proxy ./cmd/proxy

# Build Docker image
make docker-build  # Creates llm-proxy:latest
```

**Deployment Options**:

1. **Docker Compose** (Development/Testing):
   ```bash
   docker-compose up -d
   ```

2. **AWS ECS** (Production - Recommended):
   - See [#174](https://github.com/sofatutor/llm-proxy/issues/174) for full details
   - CDK-based infrastructure in `infra/` directory (planned)
   - Uses Aurora PostgreSQL + ElastiCache Redis
   - ALB handles HTTPS and path-based routing

---

## Testing Reality

### Test Coverage (Actual Numbers)

**Current Status** (as of latest run):
- **Overall**: ~90%+ (target met!)
- **internal/token**: 95%+ ✅
- **internal/proxy**: 92%+ ✅
- **internal/database**: 90%+ ✅

**Coverage Policy** (from PLAN.md):
- `cmd/` packages: NOT included in coverage (CLI glue code)
- `internal/` packages: 90%+ required, enforced by CI
- New code: Must maintain or improve coverage

### Running Tests (Actual Commands)

```bash
# Quick test run (unit tests only)
make test

# Full test suite with coverage
make test-coverage

# CI-style coverage (matches CI exactly)
make test-coverage-ci

# Integration tests (requires build tag)
go test -tags=integration ./...

# PostgreSQL integration tests
go test -tags=postgres,integration ./internal/database/...

# E2E tests (requires npm)
npm run e2e

# Specific package
go test -v ./internal/token/
```

---

## Performance Characteristics (Real-World)

### Latency Breakdown (Typical Request)

| Component | Latency | Notes |
|-----------|---------|-------|
| Token Validation (cache hit) | ~100µs | LRU cache lookup |
| Token Validation (cache miss) | ~5-10ms | Database query |
| Distributed Rate Limit Check | ~1-2ms | Redis INCR |
| Project Active Check | ~1-2ms | Database query (every request) |
| Cache Lookup | ~100-500µs | Redis or in-memory |
| Upstream API Call | ~500-2000ms | OpenAI API latency (dominant) |
| Event Bus Publish | ~10-50µs | In-memory, non-blocking |
| **Total Proxy Overhead** | **~3-7ms** | Without cache hit |
| **Total Proxy Overhead (cached)** | **<2ms** | With cache hit |

### Throughput Characteristics

| Scenario | Throughput | Bottleneck |
|----------|------------|------------|
| Cached Responses | ~5000 req/s | CPU (serialization) |
| Uncached Responses | ~100-200 req/s | Upstream API |
| Token Generation | ~500 req/s | Database write |
| Event Publishing | ~10k events/s | In-memory buffer |

---

## Security Considerations (Actual Implementation)

### Token Security (Implemented)

- **Generation**: UUIDv7 (cryptographically random)
- **Storage**: Database (encryption at rest via AWS/Aurora)
- **Transmission**: HTTPS via ALB (AWS handles TLS)
- **Obfuscation**: Tokens obfuscated in logs (first 4 + last 4 chars)
- **Revocation**: Soft delete (sets `is_active = false`)
- **Expiration**: Time-based, checked on every validation
- **Rate Limiting**: Distributed via Redis (prevents abuse)

### API Key Protection (Implemented)

- **Storage**: Database (encrypted at rest in production via Aurora)
- **AWS**: Secrets Manager for database credentials
- **Transmission**: Never exposed to clients (replaced in proxy)
- **Logging**: Never logged (obfuscated)

---

## What's Next? (Planned vs Implemented)

### Phase 6: AWS Production Deployment (In Progress)

**Completed** ✅:
- PostgreSQL support with migrations
- Distributed rate limiting (Redis)
- Cache invalidation API
- Core features (proxy, tokens, admin UI, event bus)
- HTTP response caching
- Audit logging
- E2E tests for admin UI

**In Progress** 🔄:
- AWS ECS CDK infrastructure ([#174](https://github.com/sofatutor/llm-proxy/issues/174))
  - CDK Foundation & Setup ([#176](https://github.com/sofatutor/llm-proxy/issues/176))
  - Data Layer - Aurora + Redis ([#177](https://github.com/sofatutor/llm-proxy/issues/177))
  - Compute Layer - ECS Fargate ([#178](https://github.com/sofatutor/llm-proxy/issues/178))
  - Networking - ALB + ACM ([#179](https://github.com/sofatutor/llm-proxy/issues/179))
  - Observability - CloudWatch ([#180](https://github.com/sofatutor/llm-proxy/issues/180))
  - CI/CD Pipeline ([#181](https://github.com/sofatutor/llm-proxy/issues/181))
  - Production Readiness ([#182](https://github.com/sofatutor/llm-proxy/issues/182))

**Remaining** ❌:
- Durable event queue with guaranteed delivery ([#112](https://github.com/sofatutor/llm-proxy/issues/112))
- Comprehensive package documentation ([#115](https://github.com/sofatutor/llm-proxy/issues/115))

---

## Appendix: Useful Commands & Scripts

### Frequently Used Commands

```bash
# Development
make test          # Run all tests
make lint          # Run linters
make fmt           # Format code
make build         # Build binaries
make run           # Run server

# Docker
make docker-build  # Build Docker image
make docker-run    # Run Docker container

# Coverage
make test-coverage    # Generate coverage report
make test-coverage-ci # CI-style coverage (matches CI)
```

### Common Troubleshooting

**"MANAGEMENT_TOKEN required" Error**:
```bash
# Solution: Create .env file
llm-proxy setup --interactive
# Or manually:
echo "MANAGEMENT_TOKEN=$(uuidgen)" > .env
```

**"Database is locked" Error (SQLite)**:
```bash
# Solution: Switch to PostgreSQL for production
DB_DRIVER=postgres DATABASE_URL=postgres://... ./bin/llm-proxy server
```

**Admin UI Not Accessible**:
```bash
# Check if port 8081 is open
curl http://localhost:8081/admin/

# In AWS: Use ALB URL with /admin/* path
```

---

## Conclusion & Recommendations

### For AI Agents

**Start Here**:
1. Read this document first (brownfield reality)
2. Then read `docs/architecture.md` (ideal architecture)
3. Check `PLAN.md` for current phase and objectives
4. Review [#174](https://github.com/sofatutor/llm-proxy/issues/174) for AWS deployment status

**Key Facts**:
- PostgreSQL is now fully supported (use for production)
- Distributed rate limiting works via Redis
- AWS ECS is the recommended deployment approach
- HTTPS and multi-port concerns are handled by AWS ALB

**Before Making Changes**:
- Check technical debt section for known issues
- Review workarounds and gotchas
- Ensure tests exist and pass (90%+ coverage required)
- Update documentation (this file, PLAN.md, relevant docs)

### For Human Developers

**Quick Start**:
1. Run `llm-proxy setup --interactive`
2. Run `make test` to verify setup
3. Run `make run` to start server
4. Read `docs/README.md` for documentation index

**Production Deployment**:
- Use AWS ECS approach ([#174](https://github.com/sofatutor/llm-proxy/issues/174))
- PostgreSQL via Aurora Serverless v2
- Redis via ElastiCache
- ALB handles HTTPS and routing
- Secrets via AWS Secrets Manager

---

**Document Maintenance**: This document should be updated whenever:
- New technical debt is identified
- Workarounds are added or removed
- Major architectural changes are made
- Performance characteristics change significantly
- New constraints or limitations are discovered

**Last Updated**: December 3, 2025
