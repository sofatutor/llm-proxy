# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Common Changelog](https://common-changelog.org/).

---

## December 14, 2025

### Added

- **Redis Streams dispatcher** ([#193](https://github.com/sofatutor/llm-proxy/pull/193)): Integrated the dispatcher with Redis Streams for durable at-least-once delivery plus automatic recovery and detailed lag/processing metrics so operator health checks can observe processing rate, lag count, stream length, and last processed time; added unit and integration tests that cover health, metric collection, exponential backoff, and recovery scenarios.
- **Localize admin timestamps** ([#192](https://github.com/sofatutor/llm-proxy/pull/192)): Admin UI timestamps now emit RFC3339 UTC metadata consumed by new client-side localization so viewers see times in their local timezone while UTC-only fields remain explicit, backed by template helper tests and browser/Playwright coverage.

### Changed

- **Documented package READMEs** ([#194](https://github.com/sofatutor/llm-proxy/pull/194)): Completed README coverage for all internal packages by adding comprehensive admin and config documentation, standardizing sections across the ten packages, and fixing docs links so every reference and configuration guide stays consistent and discoverable.
- **Redis Streams CLI defaults** ([#193](https://github.com/sofatutor/llm-proxy/pull/193)): Proxy CLI now uses the redis-streams bus by default, auto-generates unique dispatcher consumers, ensures the consumer group exists, exposes new stream tuning knobs, and adds exponential backoff with permanent-error bypass to prevent stalled delivery during Redis or Helicone issues.
- **Obfuscate secrets** ([#189](https://github.com/sofatutor/llm-proxy/pull/189)): Management API GETs and Admin UI templates now return pre-obfuscated token and project values, keep token secrets show-once, and only update API keys when a new value is supplied so secrets cannot be redisplayed, while token creation/display flows still surface max_requests alongside the protected values.
- **UUID-based token identifiers** ([#189](https://github.com/sofatutor/llm-proxy/pull/189)): Tokens now use stable UUID IDs for management operations and database relations while keeping the secret value separate, backed by updated lookups and secure token store hashing so identifiers never leak secrets.
- **Aligned DB bootstrap/migrations** ([#189](https://github.com/sofatutor/llm-proxy/pull/189)): SQLite now treats scripts/schema.sql as the authoritative schema, PostgreSQL adds a goose migration for the token UUID column, and migration/CLI wiring was updated so bootstrap flows stay consistent with the new token model.
- **Hardened dev/CI workflows** ([#189](https://github.com/sofatutor/llm-proxy/pull/189)): Docker images bundle scripts/schema.sql, a postgres-test compose profile/service on port 55432 supports isolated integration runs, the redundant migration validation step was removed from the unit-tests job, and docs now clarify migration and bootstrap guidance for contributors.

### Fixed

- **Refresh cache timing headers** ([#190](https://github.com/sofatutor/llm-proxy/pull/190)): Cache entries now strip upstream timing and Date headers plus Set-Cookie before storage, and cache hits reset proxy timing headers so benchmarks reflect fresh timestamps and no stale upstream latency is reported.

## December 03, 2025

### Added

- **Unified Redis Configuration** ([#187](https://github.com/sofatutor/llm-proxy/pull/187)): Builds REDIS_CACHE_URL from REDIS_ADDR/REDIS_DB when the explicit cache URL is missing so the HTTP cache and event bus share a single Redis configuration while keeping backward compatibility.
- **Cache Stats in Admin UI** ([#187](https://github.com/sofatutor/llm-proxy/pull/187)): Adds cache_hit_count to the token API response and shows request, cache hit, and upstream request badges in the Admin UI token list so operators can monitor cache effectiveness at a glance.
- **Redis Streams EventBus** ([#185](https://github.com/sofatutor/llm-proxy/pull/185)): Implements a Redis Streams backend with consumer groups to deliver durable, distributed events with at-least-once semantics, automatic recovery, and stream trimming, configurable through new environment variables for batching, timeouts, and stream limits.
- **Automated Changelog Generation** ([#184](https://github.com/sofatutor/llm-proxy/pull/184)): GitHub Actions workflow that generates changelog entries on PR approval using OpenAI API.

### Changed

- **Documented Redis Config** ([#187](https://github.com/sofatutor/llm-proxy/pull/187)): Updated Docker compose and twelve documentation files to refer to the unified REDIS_ADDR configuration so deployments and guides stay aligned with the new Redis setup.
- **Redis Streams Documentation** ([#185](https://github.com/sofatutor/llm-proxy/pull/185)): Expanded the instrumentation docs with comprehensive Redis Streams configuration guidance, covering environment variable defaults, usage examples, and deployment comparisons to simplify observability.
- **Enhanced CHANGELOG.md** ([#184](https://github.com/sofatutor/llm-proxy/pull/184)): Transformed 79 PR entries from basic titles to detailed entries with descriptions.
- **December 2025 Documentation Cleanup** ([#183](https://github.com/sofatutor/llm-proxy/pull/183)): Major documentation reorganization reducing ~30 top-level items to 8 collapsible sections (Getting Started, Architecture, Admin UI, Guides, Database, Observability, Deployment, Development). Updated technical debt docs marking PostgreSQL, migrations, distributed rate limiting, and cache invalidation as resolved. Bumped brownfield architecture to v2.0 with AWS ECS production deployment section. Added Jekyll front matter to all pages.

### Fixed

- **Lint os.Unsetenv** ([#187](https://github.com/sofatutor/llm-proxy/pull/187)): Handles os.Unsetenv return values in config tests to satisfy errcheck.

## December 02, 2025

### Added

- **Database Migration System Epic** ([#147](https://github.com/sofatutor/llm-proxy/pull/147)): Complete stories 1.1 (Goose library integration) and 1.2 (initial schema migrations). Implemented goose-based migration runner with embedded Go API, transaction support, and SQLite/PostgreSQL compatibility. Added SQL migrations for projects, tokens, audit_events tables with up/down support and version tracking via `goose_db_version` table.
- **Redis-Backed Distributed Rate Limiting** ([#151](https://github.com/sofatutor/llm-proxy/pull/151)): Horizontal scaling support with Redis-backed rate limiter using atomic Lua scripts for INCR+EXPIRE operations. Configurable via `DISTRIBUTED_RATE_LIMIT_*` environment variables with automatic fallback to in-memory limiter. Added health check integration and comprehensive Redis connection management.
- **PostgreSQL Driver Support** ([#153](https://github.com/sofatutor/llm-proxy/pull/153)): Production database support with pgx driver and driver selection factory pattern. Configurable via `DATABASE_DRIVER` (sqlite/postgres) and `DATABASE_URL` for PostgreSQL connection strings. All stores implement PostgreSQL-compatible queries with `$1` parameter placeholders.
- **PostgreSQL Docker Compose & Integration Tests** ([#156](https://github.com/sofatutor/llm-proxy/pull/156)): Production-like development environment with `docker-compose.postgres.yml`, health checks, automatic schema initialization. Added `scripts/run-postgres-integration.sh` for integration testing against real PostgreSQL, plus CI workflow for PostgreSQL validation.
- **Conditional PostgreSQL Compilation** ([#158](https://github.com/sofatutor/llm-proxy/pull/158)): Build tags for optional PostgreSQL support via `factory.go` (SQLite only), `factory_postgres.go` (postgres tag), and `factory_postgres_stub.go`. Default binary ~31MB without PostgreSQL, ~37MB with. Added `POSTGRES_SUPPORT` Dockerfile arg and CI testing for both variants.
- **Encryption for Sensitive Data at Rest** ([#160](https://github.com/sofatutor/llm-proxy/pull/160)): AES-256-GCM encryption for API keys with `enc:v1:` prefix, SHA-256 token hashing for lookup with bcrypt verification. Added `SecureProjectStore`, `SecureTokenStore`, `SecureRevocationStore`, `SecureRateLimitStore` wrappers. New `llm-proxy migrate encrypt` and `encrypt-status` CLI commands. Configurable via `ENCRYPTION_KEY` environment variable.
- **Per-Token Cache Hit Tracking** ([#161](https://github.com/sofatutor/llm-proxy/pull/161)): Async cache statistics aggregation with buffered channel, 5-second flush intervals or 100-event batches. Added `cache_hit_count` column via migration `00003_add_cache_hit_count.sql`. Admin UI displays CACHED | UPSTREAM / LIMIT badges. Configurable via `CACHE_STATS_BUFFER_SIZE` (default 1000).
- **Core Package README Documentation** ([#168](https://github.com/sofatutor/llm-proxy/pull/168)): Comprehensive READMEs for `internal/server`, `internal/proxy`, `internal/token`, `internal/database` with consistent structure: Purpose, Architecture (mermaid diagrams), Key Types, Configuration, Testing, Troubleshooting, Related Packages.
- **Infrastructure Package README Documentation** ([#169](https://github.com/sofatutor/llm-proxy/pull/169)): Comprehensive READMEs for `internal/eventbus`, `internal/dispatcher`, `internal/audit`, expanded `internal/logging`. Added mermaid diagrams for event flow and dispatcher architecture.
- **Log Search Guide & CLI Helper** ([#170](https://github.com/sofatutor/llm-proxy/pull/170)): New `docs/logging.md` with structured field conventions and reference table. Added `scripts/log-search.sh` CLI helper with documented jq queries, Loki/LogQL, Elasticsearch/KQL, and Datadog examples.
- **User Documentation Suite** ([#171](https://github.com/sofatutor/llm-proxy/pull/171)): New guides for `docs/installation.md`, `docs/configuration.md`, `docs/token-management.md`, `docs/troubleshooting.md`, `docs/performance.md`. Expanded Admin UI docs with `docs/admin/index.md`, projects, tokens, quickstart, and screens documentation.
- **AWS ECS Deployment Architecture RFC** ([#173](https://github.com/sofatutor/llm-proxy/pull/173)): New `docs/architecture/planned/aws-ecs-cdk.md` with ECS Fargate, Aurora PostgreSQL Serverless v2, ElastiCache Redis architecture. Cost-optimized defaults ~$130/month with path-based routing for dual-port architecture.

### Changed

- **PostgreSQL Documentation** ([#157](https://github.com/sofatutor/llm-proxy/pull/157)): New database selection guide comparing SQLite vs PostgreSQL use cases, troubleshooting guide for common PostgreSQL issues, and comprehensive configuration documentation for connection pooling, SSL, and performance tuning.
- **HMAC-SHA256 Token ID Hashing in Redis** ([#159](https://github.com/sofatutor/llm-proxy/pull/159)): Security fix replacing raw token IDs in Redis rate limit keys with HMAC-SHA256 hashes (first 16 hex characters). Added `KeyHashSecret` config and `DISTRIBUTED_RATE_LIMIT_KEY_SECRET` environment variable. Backward compatible—without secret uses plaintext.
- **Documentation Cleanup & Reorganization** ([#172](https://github.com/sofatutor/llm-proxy/pull/172)): Reorganized `docs/issues/` into `done/`, `planned/`, `backlog/` subdirectories. Deleted ~3,800 lines of redundant/stale documentation. Added GitHub Pages link to README.

## November 11, 2025

### Added

- **Technical Debt Documentation & Epic Breakdowns** ([#141](https://github.com/sofatutor/llm-proxy/pull/141)): Comprehensive documentation of technical debt with brownfield architecture, technical debt register (13 items across 4 priorities), and BMad epic breakdowns with 24 sub-issues. Created 8 parent issues and actionable stories with effort estimates, dependencies, and risk mitigation plans.
- **GitHub Copilot Custom Agents** ([#142](https://github.com/sofatutor/llm-proxy/pull/142)): Seven specialized AI agents for different development roles: dev (full stack developer with TDD), po (product owner), qa (test architect), architect (system architect), pm (product manager), sm (scrum master), and analyst (business analyst). Each agent includes persona, principles, and project-specific context.
- **Database Migration System with Goose** ([#143](https://github.com/sofatutor/llm-proxy/pull/143)): Implemented migration runner using goose with embedded Go API, transaction support, and SQLite/PostgreSQL compatibility. Includes 13 test cases with 90.2% coverage, SQL migration format with up/down support, and version tracking via goose_db_version table.
- **Schema Migration Implementation** ([#146](https://github.com/sofatutor/llm-proxy/pull/146)): Initial schema migration files for projects, tokens, and audit_events tables, plus soft deactivation columns. Replaced initDatabase() with MigrationRunner, added environment-aware path resolution, and comprehensive migration integration tests.

## September 12, 2025

### Added

- **Complete Phase 6 HTTP Caching: Metrics, Vary, Purge** ([#103](https://github.com/sofatutor/llm-proxy/pull/103)): Production-ready HTTP caching with provider-agnostic cache metrics (hits, misses, bypass, stores) exposed via `/metrics` endpoint. Implemented full Vary header handling with per-response driven cache key generation honoring upstream Vary headers. Added cache purge infrastructure with exact key and prefix-based purging interfaces.
- **Cache Purge Management Endpoint & CLI** ([#107](https://github.com/sofatutor/llm-proxy/pull/107)): New `POST /manage/cache/purge` endpoint protected by management token with exact and prefix purge support. Added `llm-proxy manage cache purge` CLI command with method, URL, and optional prefix flags. Includes `ActionCachePurge` audit action for tracking cache operations and exported cache key generation functions.

### Changed

- **Phase 6 Cache Documentation Polish** ([#108](https://github.com/sofatutor/llm-proxy/pull/108)): Updated CLI reference with cache purge command response formats and examples. Documented cache metrics counters in instrumentation docs with header clarifications. Enhanced architecture docs with per-response Vary cache key strategy and operations purge section.

## September 11, 2025

### Added

- **Project Active Guard Middleware** ([#94](https://github.com/sofatutor/llm-proxy/pull/94)): Optional proxy guard that denies requests for inactive projects with 403 Forbidden and `project_inactive` error code. Configurable via `LLM_PROXY_ENFORCE_PROJECT_ACTIVE` (default true), ready for caching with TTL settings.
- **Proxy Audit Events & OpenAPI Updates** ([#95](https://github.com/sofatutor/llm-proxy/pull/95)): Comprehensive audit logging for proxy lifecycle actions including project inactive denials (403) and database errors (503). Updated OpenAPI spec with 403/503 error schemas, `ErrorResponse` schema, and bulk token revoke endpoint documentation.
- **Phase 5 E2E Test Implementation** ([#98](https://github.com/sofatutor/llm-proxy/pull/98)): New Playwright specs for audit, workflows, and cross-feature testing. Projects default to active on create; token creation forbidden for inactive projects. Admin UI enhancements with conditional action buttons based on project/token state.

### Changed

- **Phase 5 Documentation Updates** ([#99](https://github.com/sofatutor/llm-proxy/pull/99)): Enhanced architecture docs with data flow diagrams, lifecycle management components. Updated README features section and CLI documentation to accurately distinguish CLI-available vs API-only operations. Marked Phase 5 as completed in PLAN.md.

## September 10, 2025

### Added

- **Management API Extensions** ([#85](https://github.com/sofatutor/llm-proxy/pull/85)): Individual token management (GET/PATCH/DELETE `/manage/tokens/{id}`), bulk token revoke for projects, and enhanced project controls (`is_active` toggle, `revoke_tokens` flag). Includes critical security fix for missing authentication middleware on `/manage/projects` endpoint.
- **Admin UI Enhancements with Playwright E2E** ([#93](https://github.com/sofatutor/llm-proxy/pull/93)): Token views (show/edit/update/revoke), project bulk revoke, status badges, and HTML form method override middleware. Complete Playwright E2E testing setup with CI workflow, fixtures for login/seed flows, and ephemeral DB per run.

## August 15, 2025

### Added

- **Phase 6 HTTP Caching** ([#87](https://github.com/sofatutor/llm-proxy/pull/87)): Redis-backed shared HTTP response cache with in-memory fallback. Opt-in via standard headers (public/s-maxage/max-age), RFC-aligned behavior. Streaming capture-and-store, optional POST caching via request Cache-Control, auth-aware public reuse, upstream conditional revalidation (If-None-Match/If-Modified-Since), and exact client-forced TTL window via key suffix.

### Changed

- **Phase 6 Documentation Consolidation** ([#91](https://github.com/sofatutor/llm-proxy/pull/91)): Updated README, architecture, features, CLI reference, and instrumentation docs with comprehensive HTTP caching coverage including Redis backend, streaming support, and benchmark tool cache flags.

### Fixed

- **HTTP Cache & Streaming Fixes** ([#92](https://github.com/sofatutor/llm-proxy/pull/92)): Fixed cache status header ordering so `modifyResponse` correctly sets "stored" status. Restored proper cache opt-in logic honoring client Cache-Control headers. Added early return for streaming responses to prevent cache/metrics processing. Fixed duplicate error count increments.

## August 14, 2025

### Added

- **Audit Events Admin UI** ([#66](https://github.com/sofatutor/llm-proxy/pull/66)): Comprehensive audit event management interface with paginated list, time range/action/outcome/actor filtering, full-text search, event detail view, and navigation integration for security investigations and compliance auditing.
- **Token & Project Soft Deactivation** ([#76](https://github.com/sofatutor/llm-proxy/pull/76)): Database foundation with `is_active`/`deactivated_at` columns, complete RevocationStore implementation (individual, batch, project-wide, expired token revocation), and idempotent soft deletion patterns.
- **Developer Documentation** ([#73](https://github.com/sofatutor/llm-proxy/pull/73)): New `code-organization.md` and `testing-guide.md` guides. Updated architecture docs for async event system, enhanced CONTRIBUTING.md with TDD workflow.
- **Cursor Rules** ([#74](https://github.com/sofatutor/llm-proxy/pull/74)): Repo-aligned Cursor rule documents for PRD generation, task breakdown, and task progress tracking with Go-specific workflows.
- **GitHub Pages Planning** ([#77](https://github.com/sofatutor/llm-proxy/pull/77)): PRD and task breakdown for marketing site, CI coverage publishing, and Admin UI screenshots.

## August 13, 2025

### Added

- **Async Observability Pipeline** ([#41](https://github.com/sofatutor/llm-proxy/pull/41)): Complete pluggable dispatcher architecture with File, Lunary, and Helicone backends. Enhanced CLI for dispatcher configuration, batching with exponential backoff retry, graceful shutdown, and background daemon mode.
- **Audit Logging with Client IP Tracking** ([#64](https://github.com/sofatutor/llm-proxy/pull/64)): Comprehensive audit logging for security-sensitive events with client IP capture (X-Forwarded-For, X-Real-IP fallback) and database storage enabling firewall rule derivation and security analytics.
- **Structured Logging Foundation** ([#56](https://github.com/sofatutor/llm-proxy/pull/56)): Context-aware structured logging with Request/Correlation ID propagation middleware. Helicone manual-logger cost tracking compatibility. Token counting for 4o/omni/o1 models using `o200k_base` encoding.

### Changed

- [docs] Update logging, context propagation, and audit logging documentation ([#72](https://github.com/sofatutor/llm-proxy/pull/72))
- [logging] Replace ad-hoc logging with structured zap logging across codebase ([#71](https://github.com/sofatutor/llm-proxy/pull/71))
- [docs] Add comprehensive audit logging documentation ([#69](https://github.com/sofatutor/llm-proxy/pull/69))
- **Docker Optimization** ([#58](https://github.com/sofatutor/llm-proxy/pull/58)): Lean runtime image with non-root user, healthcheck, volumes. Added GHCR build/push CI workflow, Makefile docker targets, and docker-compose security hardening with Postgres scaffold.
- **Phase 5/6 Documentation Refresh** ([#48](https://github.com/sofatutor/llm-proxy/pull/48)): Linked issue docs to GitHub issues, marked completed Phase 5 items as done, added AWS CDK/Helm deployment guidance, and structured agent instruction files.

## July 03, 2025

### Changed

- **Public API Documentation** ([#42](https://github.com/sofatutor/llm-proxy/pull/42)): Reorganized and expanded project documentation, consolidated agent guidance for improved clarity and efficiency.

## June 04, 2025

### Added

- **Async Event Bus Implementation** ([#34](https://github.com/sofatutor/llm-proxy/pull/34)): Full async event bus with fan-out subscriber support, Redis implementation for distributed delivery, metrics, batching, and retry logic. New `dispatcher` CLI command for JSONL event logging and `--file-event-log` flag. Fixed OpenAI token counting using `tiktoken-go` for accurate prompt/completion token separation.

### Changed

- **Copilot Agent Environment** ([#37](https://github.com/sofatutor/llm-proxy/pull/37)): Refactored workflow for robust, cache-enabled, docs-aware development. Same Go version and caching as main CI, multi-level caching for modules/build/tools, non-blocking validation steps, and comprehensive environment summary.

### Reverted

- **Test Coverage for PR34** ([#40](https://github.com/sofatutor/llm-proxy/pull/40)): Reverted ineffective coverage additions from PR#39.

## May 25, 2025

### Added

- **Benchmark Tool Core** ([#29](https://github.com/sofatutor/llm-proxy/pull/29)): Robust CLI for performance and load testing with concurrent request handling, flexible flag parsing, and detailed latency metrics (overall, upstream, proxy). Supports worker pool for concurrent requests and debug output.

- **Short-Lived Tokens** ([#30](https://github.com/sofatutor/llm-proxy/pull/30)): Token durations now support 1 minute to 365 days (previously 1 hour minimum). UI, backend, and API all support granular duration selection. Added CLI `--config` flag for API provider configuration.

- **Async Observability Middleware** ([#33](https://github.com/sofatutor/llm-proxy/pull/33)): Generic async observability middleware with in-memory event bus. Configurable via `OBSERVABILITY_ENABLED` and `OBSERVABILITY_BUFFER_SIZE` environment variables. Wired into server and proxy components.

### Changed

- **Issue-Based Workflow Migration** ([#28](https://github.com/sofatutor/llm-proxy/pull/28)): Migrated to fully issue-based workflow with all phases tracked in `docs/issues/`. WIP.md and PLAN.md now serve as high-level indices. Automated release workflow via Makefile/GitHub Actions and SecOps with daily security scans.

- **Async Event Bus Architecture** ([#31](https://github.com/sofatutor/llm-proxy/pull/31)): Replaced synchronous file logging with modern, extensible, non-blocking observability pipeline. Event bus supports in-memory/Redis backends, batching, retries, and multiple subscribers. Cleanly separates app logs (zap) from instrumentation/event logs.

## May 24, 2025

### Added

- **Metrics & Readiness Probes** ([#23](https://github.com/sofatutor/llm-proxy/pull/23)): Added `/ready`, `/live`, and `/metrics` endpoints for health checks and Kubernetes orchestration. Thread-safe proxy metrics with uptime, request counts, and error rates.

- **Rotating JSON Log System** ([#24](https://github.com/sofatutor/llm-proxy/pull/24)): Production-grade log rotation with configurable max size, backups, and age. Prevents unbounded log file growth.

- **Async External Logging Worker** ([#26](https://github.com/sofatutor/llm-proxy/pull/26)): Buffered asynchronous worker for external logging with retry logic, error handling, and fallback to local logging.

### Changed

- **Admin UI Improvements** ([#22](https://github.com/sofatutor/llm-proxy/pull/22)): Enhanced `/health` endpoint returning admin UI and backend status. Unified dashboard template with improved status cards, project names in token list, and auto-refreshing backend status.

- **Issue-Based Workflow** ([#21](https://github.com/sofatutor/llm-proxy/pull/21)): Migrated to fully issue-based workflow with `docs/issues/` for tracking all phases. WIP.md and PLAN.md now serve as high-level indices. Automated release workflow and SecOps with daily security scans.

## May 23, 2025

### Added

- **Admin UI Foundation** ([#19](https://github.com/sofatutor/llm-proxy/pull/19)): Complete web interface as separate optional server (port 8081) with zero impact on main proxy performance. Modern Bootstrap 5 dashboard with statistics overview, project management (CRUD), and token management using Gin framework. Includes CLI integration (`llm-proxy admin-server`), graceful shutdown, and security-focused token handling.

### Fixed

- **Template Inheritance** ([#20](https://github.com/sofatutor/llm-proxy/pull/20)): Resolved "template undefined" errors causing blank pages in admin UI. Implemented flexible glob patterns for automatic template discovery, unique template block names, and dynamic template resolution.

## May 22, 2025

### Added

- **Management API Foundation** ([#18](https://github.com/sofatutor/llm-proxy/pull/18)): Complete management API endpoints for `/manage/projects` (CRUD) and `/manage/tokens` (GET, POST) with security enhancements. Token list endpoints return sanitized responses without exposing actual token values. Parallel test infrastructure using GitHub Actions test matrix.

- **Retry Logic & Circuit Breaker** ([#17](https://github.com/sofatutor/llm-proxy/pull/17)): Minimal retry logic for transient upstream failures with conservative backoff, simple circuit breaker to protect proxy when upstream is down, and validation scope enforcement (token, path, method only). Zero impact on latency for healthy traffic.

- **Transparent Proxy Core** ([#16](https://github.com/sofatutor/llm-proxy/pull/16)): Created `internal/api` package centralizing shared types and utilities. Enhanced proxy with streaming, allowlist, error handling, and metrics. Added new API endpoints (`/v1/audio/speech`, `/v1/threads`, `/v1/messages`, `/v1/runs`) and OPTIONS method to config.

- **Robust CLI** ([#15](https://github.com/sofatutor/llm-proxy/pull/15)): Production-ready CLI with setup command (interactive/non-interactive config), server command (foreground/daemon mode, PID file management), and OpenAI chat command (streaming responses, verbose mode, readline support). Comprehensive help output and usage documentation.

### Changed

- **Documentation Update** ([#14](https://github.com/sofatutor/llm-proxy/pull/14)): Updated project plan and architecture documentation for new CLI, management API, logging/monitoring features, and proxy configuration. Added `/health` endpoint for container orchestration.

## May 21, 2025

### Added

- **SQLite Database Layer** ([#8](https://github.com/sofatutor/llm-proxy/pull/8)): Complete SQLite database layer for projects and tokens with comprehensive test suite (100% coverage), utility functions for database operations and maintenance.

- **Token Management System** ([#9](https://github.com/sofatutor/llm-proxy/pull/9)): Comprehensive token management with secure UUID v7 generation (`tkn_` prefix), validation system, expiration management, soft/hard revocation, rate limiting, and performance-optimized caching via unified TokenManager interface.

- **Transparent Proxy Architecture** ([#10](https://github.com/sofatutor/llm-proxy/pull/10)): Core transparent proxy using `httputil.ReverseProxy` with middleware-based request processing, token validation, authorization header replacement, streaming responses (SSE), configurable endpoint/method allowlists, connection pooling, and comprehensive error handling.

- **API Allowlists & Token Validation** ([#11](https://github.com/sofatutor/llm-proxy/pull/11)): YAML configuration for API provider endpoints/methods, allowlist-based transparent proxying, token validation with caching, header manipulation, metadata extraction, and streaming response pass-through.

- **JSON File Logger** ([#13](https://github.com/sofatutor/llm-proxy/pull/13)): JSON logger with optional file output, integrated with server and proxy components.

## May 20, 2025

### Added

- **Project Foundation** ([#1](https://github.com/sofatutor/llm-proxy/pull/1)): Initial project setup including Dockerfile, devcontainer configuration, Makefile for common tasks, GitHub Actions workflows for CI/CD (lint, test, build, Docker), issue templates, and SQLite schema for projects/tokens.

- **Directory Structure** ([#2](https://github.com/sofatutor/llm-proxy/pull/2)): Created internal directories with placeholder README files explaining each component's purpose.

- **Project Configuration** ([#3](https://github.com/sofatutor/llm-proxy/pull/3)): Added test coverage targets and API doc generation to Makefile, comprehensive README with usage instructions, and `.env.example` with configuration options.

- **Application Setup** ([#4](https://github.com/sofatutor/llm-proxy/pull/4)): Implemented application entry point with CLI flag parsing, configuration management via environment variables, basic HTTP server with health check endpoint, and tests for config/server packages.

- **CI/CD & Docker** ([#5](https://github.com/sofatutor/llm-proxy/pull/5)): Added golangci-lint configuration, enhanced GitHub Actions with dependency management and formatting checks, improved Docker workflow with caching, updated Dockerfile with non-root user and volumes, created docker-compose.yml, and comprehensive OpenAPI 3.0 specification.

- **Security & Documentation** ([#6](https://github.com/sofatutor/llm-proxy/pull/6)): Enhanced `.env.example` with security settings, improved `.gitignore` for sensitive files, hardened Dockerfile with non-root execution and health checks, added security best practices documentation (`docs/security.md`), architecture documentation with diagrams, and CONTRIBUTING.md with TDD focus.

### Changed

- **Architecture Documentation** ([#7](https://github.com/sofatutor/llm-proxy/pull/7)): Revised implementation plan to clarify generic API proxy architecture (not OpenAI-specific), added whitelist/allowlist approach for valid API URIs and HTTP methods.
