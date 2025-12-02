# LLM Proxy - Technical Debt Register

**Last Updated**: November 11, 2025  
**Purpose**: Consolidated technical debt tracking for the LLM Proxy project

> **Note**: This document consolidates technical debt from WIP.md, PLAN.md, code comments, and review feedback into a single source of truth.

---

## Overview

This register tracks all known technical debt, workarounds, and future improvements for the LLM Proxy project. Items are categorized by priority and include effort estimates and mitigation strategies.

---

## Priority 1: Critical (Must Fix Before Production)

### 1. PostgreSQL Support Not Implemented

**Status**: 🔴 Planned but not implemented  
**Impact**: HIGH - SQLite has concurrency limitations for high-write workloads  
**Location**: `internal/database/`  
**GitHub Issue**: [#57](https://github.com/sofatutor/llm-proxy/issues/57) (existing, now with sub-issues #138-#140)

**Description**:
- PLAN.md mentions PostgreSQL support for production deployments
- Only SQLite is currently implemented and tested
- SQLite write lock limits throughput to ~500 writes/sec
- High-concurrency deployments will hit bottlenecks

**Workaround**:
- Use SQLite for MVP and low-traffic deployments
- Plan migration path to PostgreSQL before scaling

**Effort Estimate**: 2-3 weeks
- Implement PostgreSQL adapter
- Create migration system
- Update tests for both databases
- Document migration procedure

**References**:
- PLAN.md lines 119-122
- `docs/architecture.md` lines 359-362

---

### 2. No Database Migration System

**Status**: 🔴 Critical gap  
**Impact**: HIGH - Schema changes are error-prone and hard to track  
**Location**: `scripts/schema.sql`  
**GitHub Issue**: [#109](https://github.com/sofatutor/llm-proxy/issues/109)

**Description**:
- Schema is defined in `scripts/schema.sql` and applied manually
- No migration tracking or rollback capability
- Schema changes require manual SQL updates
- Risk of schema drift between environments

**Workaround**:
- Document all schema changes in PLAN.md
- Apply changes manually via SQL scripts
- Test schema changes in dev before production

**Effort Estimate**: 1 week
- Choose migration tool (golang-migrate, goose, or custom)
- Create initial migration from current schema
- Implement migration runner in setup command
- Document migration workflow

**References**:
- Brownfield architecture doc: "No Migrations" section
- WIP.md line 395

---

### 3. Distributed Rate Limiting ✅ IMPLEMENTED

**Status**: 🟢 Implemented  
**Impact**: MEDIUM-HIGH - Rate limiting is now global across all instances  
**Location**: `internal/token/redis_ratelimit.go`  
**GitHub Issue**: [#110](https://github.com/sofatutor/llm-proxy/issues/110)

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

## Priority 2: Important (Should Fix Soon)

### 4. Cache Invalidation Not Implemented

**Status**: 🟡 Planned but not implemented  
**Impact**: MEDIUM - Can't purge cache on demand  
**Location**: `internal/proxy/cache.go`  
**GitHub Issue**: [#111](https://github.com/sofatutor/llm-proxy/issues/111)

**Description**:
- Only time-based expiration (TTL) is implemented
- No manual cache invalidation or purge capability
- Can't clear cache after data updates or configuration changes
- Stale data may be served until TTL expires

**Workaround**:
- Set short TTLs for frequently changing data
- Wait for TTL expiration
- Restart service to clear cache (drastic)

**Effort Estimate**: 3-5 days
- Implement purge endpoint (`POST /manage/cache/purge`)
- Add CLI command (`llm-proxy manage cache purge`)
- Support purge by key, prefix, or pattern
- Add audit logging for purge operations
- Update tests and documentation

**References**:
- PLAN.md line 349
- `docs/caching-strategy.md` lines 87-94

---

### 5. Event Loss Risk on Redis Event Bus

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

### 6. Admin UI on Separate Port

**Status**: 🟡 Design decision, but confusing  
**Impact**: MEDIUM - Complicates deployment and firewall config  
**Location**: `cmd/proxy/admin.go`, `internal/admin/`  
**GitHub Issue**: [#113](https://github.com/sofatutor/llm-proxy/issues/113)

**Description**:
- Admin UI runs on :8081, proxy runs on :8080
- Requires two ports open in firewall
- Complicates reverse proxy configuration
- Users often forget to open :8081

**Workaround**:
- Document clearly in README and deployment guides
- Use reverse proxy to unify under single domain
- Example: `/admin/*` → :8081, `/*` → :8080

**Effort Estimate**: 1 week
- Refactor to single HTTP server with route prefixes
- Move admin routes to `/admin/*` on main server
- Update tests for unified server
- Ensure backward compatibility or document breaking change

**References**:
- Brownfield architecture: "Admin UI on Separate Port" section
- WIP.md lines 67-75

---

### 7. No Automatic HTTPS

**Status**: 🟡 Requires external reverse proxy  
**Impact**: MEDIUM - Extra deployment step, potential misconfiguration  
**Location**: `internal/server/server.go`  
**GitHub Issue**: [#114](https://github.com/sofatutor/llm-proxy/issues/114)

**Description**:
- Server only supports HTTP
- HTTPS requires external reverse proxy (nginx, Caddy, Traefik)
- No built-in Let's Encrypt integration
- Potential for misconfiguration or forgotten HTTPS setup

**Workaround**:
- Document reverse proxy setup in deployment guides
- Provide example nginx/Caddy configurations
- Recommend Caddy for automatic HTTPS

**Effort Estimate**: 1 week
- Add TLS configuration options
- Implement Let's Encrypt ACME integration (optional)
- Support both HTTP and HTTPS modes
- Update tests and documentation

**References**:
- PLAN.md line 322
- `docs/security.md` lines 73-79

---

## Priority 3: Minor (Nice to Have)

### 8. Package READMEs Are Minimal

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

### 9. Token Timestamp Not Extracted from UUIDv7

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

### 10. Vary Header Parsing Is Conservative

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

### 11. No Request/Response Transformation Pipeline

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

### 12. No Multi-Provider Load Balancing

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

### 13. No Real-Time Metrics Dashboard

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

- **Priority 1 (Critical)**: 3 items
- **Priority 2 (Important)**: 4 items
- **Priority 3 (Minor)**: 3 items
- **Priority 4 (Deferred)**: 3 items
- **Resolved**: 3 items

### By Effort

- **< 1 week**: 5 items
- **1-2 weeks**: 5 items
- **2-3 weeks**: 3 items
- **> 3 weeks**: 3 items

### Total Estimated Effort

- **Priority 1**: ~4-5 weeks
- **Priority 2**: ~4-5 weeks
- **Priority 3**: ~2-3 weeks
- **Priority 4**: ~5-8 weeks
- **Total**: ~15-21 weeks

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

**Last Updated**: November 11, 2025 by AI Documentation Agent

---

## GitHub Issues Summary

All technical debt items have been tracked as GitHub issues:

### Priority 1 (Critical)
- [#57](https://github.com/sofatutor/llm-proxy/issues/57) - PostgreSQL Support (existing, now with sub-issues #138-#140)
  - Depends on #109 (must complete migration system first)
- [#109](https://github.com/sofatutor/llm-proxy/issues/109) - Database Migration System (blocks #57)
- [#110](https://github.com/sofatutor/llm-proxy/issues/110) - Distributed Rate Limiting

### Priority 2 (Important)
- [#111](https://github.com/sofatutor/llm-proxy/issues/111) - Cache Invalidation API
- [#112](https://github.com/sofatutor/llm-proxy/issues/112) - Durable Event Queue with Guaranteed Delivery
- [#113](https://github.com/sofatutor/llm-proxy/issues/113) - Unified HTTP Server (Single Port)
- [#114](https://github.com/sofatutor/llm-proxy/issues/114) - Built-in HTTPS Support

### Priority 3-4 (Minor/Deferred)
- [#115](https://github.com/sofatutor/llm-proxy/issues/115) - Comprehensive Package Documentation
- [#116](https://github.com/sofatutor/llm-proxy/issues/116) - Optimizations and Future Enhancements (collection)

