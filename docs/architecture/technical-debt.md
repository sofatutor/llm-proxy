---
title: Technical Debt Register
parent: Architecture
nav_order: 4
---

# LLM Proxy - Technical Debt Register

**Last Updated**: December 3, 2025  
**Purpose**: Consolidated technical debt tracking for the LLM Proxy project

> **Note**: This document consolidates technical debt from WIP.md, PLAN.md, code comments, and review feedback into a single source of truth.

---

## Overview

This register tracks all known technical debt, workarounds, and future improvements for the LLM Proxy project. Items are categorized by priority and include effort estimates and mitigation strategies.

---

## Priority 1: Critical (Must Fix Before Production)

> ✅ **All Priority 1 items have been resolved!** See [Resolved Technical Debt](#resolved-technical-debt) section.

---

## Priority 2: Important (Should Fix Soon)

### 1. Event Loss Risk on Redis Event Bus

**Status**: 🟡 Documented warning, no mitigation  
**Impact**: MEDIUM - Events can be lost if dispatcher lags  
**Location**: `internal/eventbus/eventbus.go`  
**GitHub Issue**: [#112](https://github.com/sofatutor/llm-proxy/issues/112)

**Description**:
- Redis event bus uses list with TTL and max-length
- If dispatcher is down or lagging, events can expire before being read
- No offset tracking or guaranteed delivery
- Warning documented in `docs/instrumentation.md`

**Workaround**:
- Size Redis retention generously (high TTL, large max-length)
- Monitor dispatcher lag and alert on warnings
- Keep dispatcher running with sufficient throughput
- Use file dispatcher for critical event logging

**Effort Estimate**: 1-2 weeks
- Implement durable queue (Redis Streams with consumer groups or Kafka)
- Add offset tracking and acknowledgment
- Implement at-least-once delivery semantics
- Add monitoring and alerting for lag
- Update tests and documentation

**References**:
- `docs/instrumentation.md` lines 363-375
- Brownfield architecture: "Event Loss Risk" section

---

### 2. No Built-in HTTPS (Non-Issue for AWS Deployment)

**Status**: 🟢 Resolved via AWS infrastructure  
**Impact**: LOW - AWS ALB handles TLS termination  
**Location**: `internal/server/server.go`  
**GitHub Issue**: [#114](https://github.com/sofatutor/llm-proxy/issues/114)

**Description**:
- Server only supports HTTP natively
- **With AWS deployment**: ALB handles TLS termination with ACM certificates
- **For non-AWS**: Reverse proxy (nginx, Caddy, Traefik) recommended

**AWS Solution** (Recommended):
- ALB with ACM certificate handles HTTPS
- Auto-renewal of certificates
- No application changes needed
- See [#174](https://github.com/sofatutor/llm-proxy/issues/174) for AWS ECS architecture

**Non-AWS Workaround**:
- Use reverse proxy (Caddy recommended for automatic HTTPS)
- Document reverse proxy setup in deployment guides

**References**:
- AWS Architecture: `docs/architecture/planned/aws-ecs-cdk.md`
- Epic: [#174](https://github.com/sofatutor/llm-proxy/issues/174)

---

## Priority 3: Minor (Nice to Have)

### 3. Package READMEs Are Minimal

**Status**: 🟢 Low priority, quality of life  
**Impact**: LOW - Hard to understand package purpose quickly  
**Location**: `internal/*/README.md`  
**GitHub Issue**: [#115](https://github.com/sofatutor/llm-proxy/issues/115)

**Description**:
- Most package READMEs are 4-9 lines of bullet points
- No architecture explanations or usage examples
- No testing guidance or troubleshooting tips
- Developers must read code to understand packages

**Workaround**:
- Read code and main documentation
- Check `docs/code-organization.md` for package overview

**Effort Estimate**: 1-2 hours per package (8-10 packages = 8-20 hours total)
- Expand each README with:
  - Package purpose and responsibilities
  - Key types and interfaces
  - Usage examples
  - Testing guidance
  - Troubleshooting tips

**References**:
- Documentation gap analysis: "Package-Level READMEs" section
- Examples: `internal/proxy/README.md`, `internal/token/README.md`

---

### 4. Token Timestamp Not Extracted from UUIDv7

**Status**: 🟢 Low priority, optimization opportunity  
**Impact**: LOW - Could use for debugging and cache keys  
**Location**: `internal/token/token.go`  
**GitHub Issue**: [#116](https://github.com/sofatutor/llm-proxy/issues/116) (included in optimizations)

**Description**:
- Tokens use UUIDv7 which includes timestamp
- Timestamp is not extracted or used anywhere
- Could be useful for cache key generation, debugging, or sorting
- Currently use `created_at` from database instead

**Workaround**:
- Use `created_at` field from database
- UUIDv7 timestamp is redundant but harmless

**Effort Estimate**: 2-3 days
- Implement UUIDv7 timestamp extraction
- Add helper functions for timestamp access
- Update tests
- Document usage and limitations

**References**:
- Brownfield architecture: "Token Timestamp Not Used" section
- Code comment in `internal/token/token.go`

---

### 5. Vary Header Parsing Is Conservative

**Status**: 🟢 Low priority, optimization opportunity  
**Impact**: LOW - Lower cache hit rate, higher memory usage  
**Location**: `internal/proxy/cache_helpers.go`  
**GitHub Issue**: [#116](https://github.com/sofatutor/llm-proxy/issues/116) (included in optimizations)

**Description**:
- Cache key generation uses conservative Vary subset
- Uses Accept, Accept-Encoding, Accept-Language by default
- Doesn't parse per-response Vary header from upstream
- May cache separately when upstream doesn't vary on those headers

**Workaround**:
- Accept lower cache hit rate
- This is a safe default (over-caching is better than under-caching)

**Effort Estimate**: 3-5 days
- Implement per-response Vary header parsing
- Generate cache key based on actual Vary header
- Fall back to conservative subset if no Vary header
- Update tests and documentation

**References**:
- Brownfield architecture: "Vary Header Parsing" section
- `docs/caching-strategy.md` lines 230-233

---

## Priority 4: Deferred (Future Considerations)

### 6. Admin UI on Separate Port (Non-Issue for AWS Deployment)

**Status**: 🟢 Resolved via AWS infrastructure  
**Impact**: LOW - AWS ALB handles path-based routing  
**Location**: `cmd/proxy/admin.go`, `internal/admin/`  
**GitHub Issue**: [#113](https://github.com/sofatutor/llm-proxy/issues/113) (closed)

**Description**:
- Admin UI runs on :8081, proxy runs on :8080
- **With AWS deployment**: ALB path-based routing unifies to single HTTPS endpoint
- **For non-AWS**: Reverse proxy configuration recommended

**AWS Solution** (Recommended):
- ALB routes `/admin/*` to :8081, `/v1/*` and `/manage/*` to :8080
- Users see single HTTPS endpoint
- No firewall complexity
- See [#174](https://github.com/sofatutor/llm-proxy/issues/174) for AWS ECS architecture

**Non-AWS Workaround**:
- Use reverse proxy to unify under single domain
- Example: nginx/Caddy with path-based routing

**References**:
- AWS Architecture: `docs/architecture/planned/aws-ecs-cdk.md`
- Epic: [#174](https://github.com/sofatutor/llm-proxy/issues/174)

---

### 7. No Request/Response Transformation Pipeline

**Status**: 🔵 Future feature  
**Impact**: LOW - Currently not needed  
**Location**: `internal/proxy/proxy.go`  
**GitHub Issue**: [#116](https://github.com/sofatutor/llm-proxy/issues/116) (included in optimizations)

**Description**:
- Proxy is intentionally minimal (authorization header replacement only)
- No pluggable transformation pipeline
- Future providers may need custom transformations
- Would require middleware-style architecture

**Workaround**:
- Keep proxy minimal and transparent
- Add provider-specific logic only when needed

**Effort Estimate**: 2-3 weeks
- Design transformation pipeline architecture
- Implement plugin system for transformations
- Add provider-specific transformations as needed
- Update tests and documentation

**References**:
- PLAN.md line 21
- `docs/architecture.md` lines 13-17

---

### 8. No Multi-Provider Load Balancing

**Status**: 🔵 Future feature  
**Impact**: LOW - Currently not needed  
**Location**: `internal/proxy/proxy.go`  
**GitHub Issue**: [#116](https://github.com/sofatutor/llm-proxy/issues/116) (included in optimizations)

**Description**:
- Each project has single API key
- No load balancing across multiple API keys
- No failover to backup keys
- Could improve reliability and throughput

**Workaround**:
- Use single API key per project
- Rely on upstream provider's reliability

**Effort Estimate**: 1-2 weeks
- Support multiple API keys per project
- Implement round-robin or weighted load balancing
- Add failover logic for failed keys
- Update tests and documentation

**References**:
- PLAN.md line 672

---

### 9. No Real-Time Metrics Dashboard

**Status**: 🔵 Future feature  
**Impact**: LOW - Currently not needed  
**Location**: N/A (not implemented)  
**GitHub Issue**: [#116](https://github.com/sofatutor/llm-proxy/issues/116) (included in optimizations)

**Description**:
- Metrics endpoint exists but no dashboard
- No real-time visualization of proxy performance
- No alerting on anomalies or errors
- Monitoring requires external tools

**Workaround**:
- Use external monitoring tools (Grafana, Prometheus)
- Check logs and audit events manually

**Effort Estimate**: 2-3 weeks
- Design real-time metrics dashboard
- Implement WebSocket streaming for live updates
- Add visualization for key metrics
- Integrate with alerting system
- Update tests and documentation

**References**:
- PLAN.md line 673

---

## Resolved Technical Debt

### ✅ PostgreSQL Support (Completed December 2, 2025)

**Status**: ✅ Fixed  
**GitHub Issue**: [#57](https://github.com/sofatutor/llm-proxy/issues/57) (closed)

**Description**:
- PostgreSQL is now fully supported as an alternative to SQLite
- Configuration via `DB_DRIVER=postgres` and `DATABASE_URL`
- Full migration support for both databases
- Docker Compose configuration included

**Implementation**:
- `internal/database/factory_postgres.go` - PostgreSQL driver
- `internal/database/migrations/sql/postgres/` - PostgreSQL migrations
- `docker-compose.yml` - PostgreSQL service configuration

---

### ✅ Database Migration System (Completed December 2, 2025)

**Status**: ✅ Fixed  
**GitHub Issue**: [#109](https://github.com/sofatutor/llm-proxy/issues/109) (closed)

**Description**:
- Full migration system implemented using goose
- Migration tracking and rollback capability
- Support for both SQLite and PostgreSQL

**Implementation**:
- `internal/database/migrations/` - Migration runner and SQL files
- Automatic migrations on server startup
- CLI integration for migration management

---

### ✅ Distributed Rate Limiting (Completed December 2, 2025)

**Status**: ✅ Fixed  
**GitHub Issue**: [#110](https://github.com/sofatutor/llm-proxy/issues/110) (closed)

**Implementation Details**:
- Redis-backed rate limit counters using atomic INCR operations
- Sliding window algorithm for accurate rate limiting
- Graceful fallback to in-memory when Redis is unavailable
- Per-token configurable rate limits
- Configuration via environment variables

**Configuration**:
```bash
DISTRIBUTED_RATE_LIMIT_ENABLED=true    # Enable Redis-backed rate limiting
DISTRIBUTED_RATE_LIMIT_PREFIX=ratelimit:  # Redis key prefix
DISTRIBUTED_RATE_LIMIT_WINDOW=1m       # Window duration
DISTRIBUTED_RATE_LIMIT_MAX=60          # Max requests per window
DISTRIBUTED_RATE_LIMIT_FALLBACK=true   # Fallback to in-memory
```

**Files**:
- `internal/token/redis_ratelimit.go` - Main implementation
- `internal/token/redis_adapter.go` - Redis client adapter
- `internal/config/config.go` - Configuration options

---

### ✅ Cache Invalidation API (Completed December 2, 2025)

**Status**: ✅ Fixed  
**GitHub Issue**: [#111](https://github.com/sofatutor/llm-proxy/issues/111) (closed)

**Description**:
- Manual cache invalidation and purge capability implemented
- Support for purging by key, prefix, or all entries

**Implementation**:
- `internal/proxy/cache.go` - Cache invalidation methods
- `internal/proxy/cache_redis.go` - Redis cache invalidation
- Management API endpoint for cache purge
- CLI command for cache operations

---

### ✅ Cache Eviction Optimization (Completed)

**Status**: ✅ Fixed in PR (review feedback)  
**Impact**: Performance improvement  
**Location**: `internal/token/cache.go`

**Description**:
- Original implementation used linear scan for eviction (O(n))
- Optimized to use min-heap for O(log n) eviction
- Thoroughly tested for correctness and efficiency

**Resolution**:
- Implemented min-heap based eviction
- Added comprehensive tests
- Verified 90%+ code coverage

**References**:
- DONE.md lines 7-9
- WIP.md review comments

---

### ✅ Named Constant for Max Duration (Completed)

**Status**: ✅ Fixed in PR (review feedback)  
**Impact**: Code clarity  
**Location**: `internal/token/expiration.go`

**Description**:
- Used magic number `1<<63 - 1` for max duration
- Replaced with named constant `MaxDuration`

**Resolution**:
- Added `MaxDuration` constant
- Updated all usages
- Tests pass and coverage confirmed

**References**:
- DONE.md lines 11-13

---

### ✅ Composite Interface for Manager Store (Completed)

**Status**: ✅ Fixed in PR (review feedback)  
**Impact**: Type safety improvement  
**Location**: `internal/token/manager.go`

**Description**:
- Manager used separate store interfaces
- Refactored to use composite ManagerStore interface
- Enforces type safety at compile time

**Resolution**:
- Created ManagerStore composite interface
- Updated all usages and tests
- Type safety now enforced by compiler

**References**:
- DONE.md lines 14-16

---

## Tracking & Maintenance

### How to Use This Register

**Adding New Debt**:
1. Identify technical debt during development or review
2. Add entry to appropriate priority section
3. Include: status, impact, location, description, workaround, effort estimate
4. Reference related docs and code locations

**Updating Existing Debt**:
1. Change status as work progresses (🔴 → 🟡 → 🟢 → ✅)
2. Update workarounds as they're discovered
3. Adjust effort estimates based on new information
4. Move to "Resolved" section when completed

**Prioritization**:
- Priority 1 (Critical): Blocks production deployment or causes data loss
- Priority 2 (Important): Impacts performance, reliability, or user experience
- Priority 3 (Minor): Quality of life improvements, optimizations
- Priority 4 (Deferred): Future features, not currently needed

### Status Legend

- 🔴 **Critical** - Must fix before production
- 🟡 **Important** - Should fix soon
- 🟢 **Minor** - Nice to have
- 🔵 **Deferred** - Future consideration
- ✅ **Resolved** - Completed and tested

### Review Schedule

- **Weekly**: Review Priority 1 items, update status
- **Monthly**: Review Priority 2-3 items, adjust priorities
- **Quarterly**: Full register review, archive resolved items

---

## Summary Statistics

### By Priority

- **Priority 1 (Critical)**: 0 items ✅ All resolved!
- **Priority 2 (Important)**: 1 item (Event Loss Risk)
- **Priority 3 (Minor)**: 3 items
- **Priority 4 (Deferred)**: 2 items (plus 2 non-issues via AWS)
- **Resolved**: 7 items
- **Non-Issues (AWS handles)**: 2 items (HTTPS, Multi-port)

### By Effort

- **< 1 week**: 2 items
- **1-2 weeks**: 3 items
- **2-3 weeks**: 2 items

### Total Estimated Effort (Remaining)

- **Priority 2**: ~1-2 weeks
- **Priority 3**: ~1-2 weeks
- **Priority 4**: ~5-8 weeks (if implemented)
- **Total Active**: ~2-4 weeks

---

## Related Documentation

- **Brownfield Architecture**: `docs/brownfield-architecture.md` - Actual system state
- **PLAN.md**: Project architecture and objectives
- **WIP.md**: Current work in progress
- **DONE.md**: Completed work archive
- **Architecture**: `docs/architecture.md` - Ideal architecture

---

**Maintenance**: This register should be updated whenever:
- New technical debt is identified
- Existing debt is resolved or changes status
- Priorities shift based on business needs
- Effort estimates are refined based on experience

**Last Updated**: December 3, 2025

---

## GitHub Issues Summary

All technical debt items have been tracked as GitHub issues:

### Active Issues

#### Priority 2 (Important)
- [#112](https://github.com/sofatutor/llm-proxy/issues/112) - Durable Event Queue with Guaranteed Delivery

#### Priority 3-4 (Minor/Deferred)
- [#115](https://github.com/sofatutor/llm-proxy/issues/115) - Comprehensive Package Documentation
- [#116](https://github.com/sofatutor/llm-proxy/issues/116) - Optimizations and Future Enhancements (collection)

### Non-Issues (Resolved by AWS Infrastructure)
- [#114](https://github.com/sofatutor/llm-proxy/issues/114) - HTTPS Support → ALB + ACM handles TLS
- [#113](https://github.com/sofatutor/llm-proxy/issues/113) - Unified HTTP Server → ALB path-based routing

See [#174](https://github.com/sofatutor/llm-proxy/issues/174) (AWS ECS Epic) for the infrastructure approach.

### Resolved Issues

- [#57](https://github.com/sofatutor/llm-proxy/issues/57) - PostgreSQL Support ✅
- [#109](https://github.com/sofatutor/llm-proxy/issues/109) - Database Migration System ✅
- [#110](https://github.com/sofatutor/llm-proxy/issues/110) - Distributed Rate Limiting ✅
- [#111](https://github.com/sofatutor/llm-proxy/issues/111) - Cache Invalidation API ✅
