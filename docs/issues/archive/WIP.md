# WIP: Admin UI Foundation Implementation (PR #19)

# WIP: OpenAI Token Counting Bugfix (2025-05-27)

# WIP: Async Event Bus, Dispatcher, and Persistent Event Logging (PR #34)

## Summary
- Fixed a bug in OpenAI event transformation where `completion_tokens` and `prompt_tokens` were incorrectly calculated.
- Previously, token counts were computed over the entire response/request JSON, not just the relevant content.
- Now, `completion_tokens` are counted only from the assistant's reply content (`choices[0].message.content`), and `prompt_tokens` from the request's `messages` array (if present).
- Added a helper (`extractAssistantReplyContent`) and comprehensive unit tests for edge/error cases.
- All changes are covered by tests and maintain 90%+ code coverage.
- See PLAN.md for rationale and architecture.

## Details
- Bug: Error responses and non-completion responses resulted in nonzero or inflated completion token counts.
- Fix: Only count tokens in the actual assistant reply content. If not present, set completion tokens to zero.
- Tests: Added table-driven tests for the new helper, covering normal, error, empty, and malformed cases.
- All tests pass (`go test -v ./internal/eventtransformer/...`).

# WIP: Async Event Bus, Dispatcher, and Persistent Event Logging (PR #34)

## Summary
- Implemented a fully asynchronous event bus architecture with both InMemoryEventBus and RedisEventBus backends, supporting multiple subscribers (fan-out), batching, retry logic, and graceful shutdown.
- Added a dispatcher CLI and `--file-event-log` flag for persistent event logging to JSONL files.
- Middleware now captures and restores the request body for all events, and the event context is richer for diagnostics and debugging.
- The event bus is always enabled by default, with a larger buffer for high-throughput scenarios.
- OpenAI token counting bug fixed: `completion_tokens` are now counted only from the assistant's reply, and `prompt_tokens` from the request's `messages` array, using tiktoken-go for accuracy.
- All changes are covered by comprehensive unit tests and maintain 90%+ code coverage.
- See PLAN.md for architectural details and rationale.

## Details
- Event bus and dispatcher are now the foundation for all observability and analytics in the proxy.
- Persistent event logging is handled by the dispatcher CLI or the `--file-event-log` flag.
- All event delivery is async, batched, and non-blocking, with robust error handling and retry logic.
- Migration note: The event bus is now always enabled; configuration options have changed. See PLAN.md and README.md for updated usage and configuration.

> **Note:** All detailed development tasks for Phases 4–7 are now tracked in individual issue files in `docs/issues/`. This WIP.md will serve as a high-level status and index. All future development, tracking, and progress updates will be managed via the issue files. Please refer to the linked issues for detailed tasks, rationale, and acceptance criteria.

## Status - Admin UI Foundation  
- [x] Admin UI server integrated as `admin` subcommand (moved to dedicated admin.go file)
- [x] Complete Bootstrap 5 responsive UI with custom styling  
- [x] API client for Management API communication
- [x] Dashboard with statistics cards and quick actions
- [x] Project management (list, create, show, edit, delete) with full CRUD
- [x] Token management (list, generate, success page) with security focus
- [x] Template system with custom functions (pagination, dates, comparisons, arithmetic, etc.)
- [x] Configuration integration (AdminUIConfig, CLI flags, env support)
- [x] Error handling and user-friendly error pages
- [x] JavaScript enhancements and interactivity (admin.js)
- [x] Template loading fix: switched to LoadHTMLFiles with explicit template listing
- [x] Graceful shutdown handling for admin server
- [x] Security-focused session and token handling (browser-based login, session cookie, token never exposed after creation)
- [x] Command refactoring: renamed from `admin-server` to `admin` for better UX
- [x] Code organization: moved admin command to separate `cmd/proxy/admin.go` file
- [x] Test coverage improvements: Added comprehensive tests for extracted functionality
- [ ] Consolidate AdminUIEnabled/Enabled config fields (see review feedback)
- [ ] Standardize template arithmetic helpers (sub/add vs inc/dec) across all templates (see review feedback)
- [ ] Centralize common JS functions (togglePassword, copyToClipboard, confirmDelete) in admin.js (see review feedback)

## Next Steps
- [x] PR #19 created and ready for review
- [ ] Template inheritance refactoring (optional improvement)
- [ ] Real-time dashboard updates (future enhancement)
- [ ] Create help/documentation pages (future enhancement)

## Architecture
- **Separate Port**: Admin UI runs on :8081 (zero impact on proxy :8080)
- **Optional Component**: Can be completely disabled in production
- **CLI Integration**: `llm-proxy admin --management-token TOKEN`
- **Security**: Tokens only exposed once, sanitized API responses, session-based login
- **Modern UI**: Bootstrap 5, responsive design, custom CSS/JS
- **Graceful Shutdown**: Admin server supports graceful shutdown on SIGINT/SIGTERM
- **Explicit Template Loading**: Uses LoadHTMLFiles with explicit template listing to avoid directory conflicts

---

# LLM Proxy Implementation Checklist

> **Agent-Driven Test-Driven Development (TDD) Mandate**
>
> This project is implemented entirely by autonomous agents. All development MUST strictly follow test-driven development (TDD):
> - Every feature or change must first be implemented as a failing unit test.
> - Only after the test is written may the implementation be created or modified to make the test pass.
> - No code may be merged unless it is covered by tests.
> - A minimum of 90% code coverage is required at all times, enforced by GitHub Actions.
> - Pull requests must demonstrate that new/changed code is covered by tests and that overall coverage remains above 90%.
> - Coverage checks are mandatory in CI and must block merges if not met.

> See PLAN.md for the canonical coverage policy and rationale.

This document provides a detailed sequential implementation checklist for the Transparent LLM Proxy for OpenAI. Tasks are organized into phases with dependencies clearly marked. Each task has a status indicator:

- [ ] TODO: Task not yet started
- [ ] IN PROGRESS: Task currently being implemented
- [ ] SKIPPED: Task temporarily skipped
- [x] DONE: Task completed

## Outstanding Review Comments

See `docs/issues/archive/DONE.md` for all completed review comments. Only unresolved or in-progress comments are tracked here.

## Pull Request Strategy

All completed PR strategy tasks have been moved to `docs/issues/archive/DONE.md`. Only new or in-progress strategy changes will be tracked here.

## Phase 0: Pre-Development Setup

**Abstract:**
Initial repository setup, GitHub configuration, development environment, and CI/CD were fully established. See `docs/issues/archive/DONE.md` for all completed setup and project management tasks.

## Phase 1: Project Setup

**Abstract:**
Repository structure, configuration, Docker, security, documentation, and foundational testing were implemented. All core project scaffolding and best practices are in place. See `docs/issues/archive/DONE.md` for details.

## Phase 2: Core Components

**Abstract:**
Core database implementation, token management system, and proxy logic completed. All fundamental functionality implemented with comprehensive testing. See `docs/issues/archive/DONE.md` for detailed implementation checklist.

### CLI Tool (Setup & OpenAI Chat)
- [x] Implement CLI tool (`llm-proxy setup` and `llm-proxy openai chat`) **in a separate PR** (`feature/llm-proxy-cli`)
  - CLI tool structure and commands design
  - Basic CLI framework with flag parsing
  - 'llm-proxy setup' command for configuration with these improvements:
    - Automatic project creation and token generation
    - Secure random token generation for MANAGEMENT_TOKEN
    - Ability to skip through existing settings in interactive mode
    - Preserving existing configuration values
  - 'llm-proxy openai chat' command with advanced features:
    - Streaming mode for real-time responses
    - Verbose mode for displaying timing information
    - Shows proxy overhead compared to remote call duration
  - 'llm-proxy server' command with daemon mode (-d option) and PID file support
  - 'llm-proxy admin' command for Admin UI server (renamed from admin-server, moved to separate admin.go file)
  - Advanced CLI flag parsing and configuration overrides
  - Comprehensive end-to-end usage documentation and advanced examples
  - Test cases for CLI tool verification (expanded with extracted functionality tests) **[COMPLETED]**
  - Documentation for CLI usage (needs update for new features) **[IN PROGRESS]**
  - **Management API CLI is now fully configurable via --manage-api-base-url; 'token get' subcommand is implemented.**
  - **Planned:** Add more integration specs for management API flows.

#### CLI Usage Example
```sh
llm-proxy manage project list --manage-api-base-url http://localhost:8080 --management-token <token>
llm-proxy manage token generate --project-id <project-id> --management-token <token> --manage-api-base-url http://localhost:8080
llm-proxy manage token get <token> --management-token <token> --manage-api-base-url http://localhost:8080 --json
```

## IN PROGRESS: CLI Management Command for Projects and Tokens

A new `llm-proxy manage` command will be introduced to provide a clear, user-friendly CLI for project and token management. This command will support CRUD operations for projects and token generation/validation, separated from setup and server commands for clarity and best practices.

### Command Structure

- `llm-proxy manage project <subcommand>`
  - `list` — List all projects
  - `get <project-id>` — Get details for a project
  - `create --name <name> --openai-key <key>` — Create a new project
  - `update <project-id> [--name ...] [--openai-key ...]` — Update a project
  - `delete <project-id>` — Delete a project

- `llm-proxy manage token <subcommand>`
  - `generate --project-id <id> --duration <hours>` — Generate a new token for a project
  - `get <token>` — Get validity/status for a token

### Output Format
- By default, results are shown in a human-friendly table.
- Use `--json` flag for machine-readable JSON output.

### Rationale
- Keeps setup and management concerns separate for clarity and maintainability.
- Follows CLI and Go best practices (SRP, discoverability, UX).
- Makes it easy for both humans and scripts to use the CLI.

### Implementation Checklist
- [x] Scaffold `manage` command and subcommands in CLI
- [x] Implement project CRUD subcommands (list, get, create, update, delete)
- [x] Implement token subcommands (generate, get)
- [x] Wire subcommands to management API endpoints (`/manage/projects`, `/manage/tokens`)
- [x] Require management token for all manage commands (flag or env)
- [x] Print results as table by default, with `--json` for machine output
- [x] Add tests for CLI manage commands
- [x] Document usage in CLI README

## Phase 3: API and Interfaces

### Management API Endpoints
- [x] Design Management API with OpenAPI/Swagger:
  - Define endpoints (only for /manage/*, not /v1/*)
  - Document request/response formats for management endpoints
  - Specify authentication requirements
  - Detail error responses
  - **Note:** The proxy API (/v1/*) is not documented with Swagger/OpenAPI except for authentication and allowed paths/methods; refer to backend provider docs for schemas. See PLAN.md for rationale.
- [x] Implement authentication middleware with MANAGEMENT_TOKEN
- [x] Create /manage/tokens POST endpoint:
  - Validate request body
  - Generate token based on parameters
  - Return token details
- [ ] Implement /manage/tokens DELETE endpoint:
  - ~~Validate token format~~
  - ~~Revoke specified token~~
  - ~~Return success response~~
  - **Note:** Individual token deletion not implemented for security (prevents token enumeration). See PR18 security decisions.
- [x] Add /manage/tokens GET endpoint (list active tokens with sanitized responses)
- [x] Create /manage/projects endpoints:
  - POST for creation
  - GET for listing/retrieval
  - PATCH for updates
  - DELETE for removal
  - GET by ID for individual project details
- [x] Add health check endpoint `/health` for monitoring
- [ ] Implement rate limiting for management API
- [x] Add comprehensive error handling
- [x] Create API documentation with examples (expand for new endpoints)

### Proxy API Endpoints
- [x] Implement /v1/* forwarding to OpenAI
- [x] Create token validation middleware
- [x] Set up proper error responses:
  - Invalid token errors
  - Expired token errors
  - Rate limit errors
  - Upstream API errors
- [x] Implement request validation
- [ ] Add response caching (optional)
- [x] Create usage tracking
- [x] Implement proper handling for all OpenAI endpoints:
  - Different content types
  - Binary responses
  - Large payload handling
- [ ] Add telemetry collection
- [ ] Implement feature flags for gradual rollout
- [ ] Create API versioning strategy
- [ ] Add proxy metrics/logging/timing improvements

### Admin UI
- [x] Design HTML interface wireframes
- [x] Create basic CSS styling:
  - Responsive layout (Bootstrap 5)
  - Modern theme with custom CSS
  - Consistent styling across components
- [x] Implement admin routes with basic authentication
- [x] Set up static file serving
- [x] Create base HTML templates
- [x] Add JavaScript for interactive elements:
  - Form submissions
  - Interactive dashboard elements
  - Token generation workflow
  - Copy-to-clipboard functionality
  - Project management interfaces
- [x] Implement project management UI:
  - List view with pagination
  - Create form with validation
  - Edit form with security features
  - Delete confirmation with warnings
  - Individual project view
- [x] Create token management UI:
  - List view with status indicators
  - Generation form with duration options
  - Token creation success page
  - Usage statistics display
  - Security-focused design (no token exposure)
- [ ] Add real-time updates with WebSockets (optional)
- [x] Implement dashboard with usage statistics
- [x] Create user-friendly error handling
- [x] Add confirmation dialogs for destructive actions
- [x] Implement client-side validation
- [ ] Create help/documentation pages

### Pull Requests for Phase 3

1. **Management API Design** (`feature/phase-3-management-api-design`) ✅ **Completed in PR18**
   - Create OpenAPI/Swagger specification
   - Document request/response formats
   - Define authentication requirements
   - Detail error responses

2. **Token Management API** (`feature/phase-3-token-api`) ✅ **Completed in PR18**
   - Implement authentication middleware
   - Create POST endpoint for token generation
   - ~~Add DELETE endpoint for token revocation~~ (Not implemented for security)
   - Implement GET endpoint for listing tokens

3. **Project Management API** (`feature/phase-3-project-api`) ✅ **Completed in PR18**
   - Implement project CRUD endpoints
   - Add validation and error handling
   - ~~Create rate limiting~~ (Future PR)
   - Document API with examples

4. **Proxy API Implementation** (`feature/phase-3-proxy-api`)
   - Implement /v1/* forwarding
   - Create token validation middleware
   - Set up proper error responses
   - Add request validation

5. **Admin UI Foundation** (`feature/phase-3-admin-ui-foundation`) ✅ **Completed in PR19**
   - Design UI wireframes
   - Create base HTML templates
   - Implement basic CSS styling
   - Set up static file serving
   - Add admin routes with authentication
   - **Includes:** Separate admin server, CLI integration, complete Bootstrap UI

6. **Project Management UI** (`feature/phase-3-project-ui`) ✅ **Completed in PR19**
   - Implement project list view
   - Create project creation/edit forms
   - Add project deletion with confirmation
   - Implement error handling
   - **Includes:** Full CRUD operations, responsive design, security features

7. **Token Management UI** (`feature/phase-3-token-ui`) ✅ **Completed in PR19**
   - Create token list view with filtering
   - Implement token generation form
   - ~~Add token revocation functionality~~ (Security: no individual token operations)
   - Display usage statistics
   - **Includes:** Secure token workflow, creation success page, statistics

8. **Admin Dashboard** (`feature/phase-3-admin-dashboard`) ✅ **Completed in PR19**
   - Create dashboard with usage statistics
   - ~~Implement real-time updates~~ (Future enhancement)
   - ~~Add help/documentation pages~~ (Future enhancement)
   - Enhance UI with client-side validation
   - **Includes:** Statistics cards, quick actions, system status

## Phase 4: Logging and Monitoring
- [Logging System Core](docs/issues/done/phase-4-logging-core.md)
- [External Logging](docs/issues/archive/WIP.md#phase-4-logging-and-monitoring)
- [Log Integration](docs/issues/done/phase-5-log-integration.md)
- [Monitoring Core](docs/issues/done/phase-4-monitoring-core.md)
- [Helicone Middleware Integration](docs/issues/archive/WIP.md#phase-4-logging-and-monitoring)
- [Log Sampling and Visualization](docs/issues/archive/WIP.md#phase-4-logging-and-monitoring)
- [Alerting, Tracing, and Performance Benchmarks](docs/issues/backlog/optional/phase-4-alerting-tracing-benchmarks.md)
- [Resource Usage Monitoring and Grafana Dashboards](docs/issues/planned/phase-6-resource-usage-grafana.md)

## Phase 5: Testing and Performance
- [Testing Framework](docs/issues/archive/WIP.md#phase-5-testing-and-performance)
- [Database Tests](docs/issues/archive/WIP.md#phase-5-testing-and-performance)
- [Token Management Tests](docs/issues/archive/WIP.md#phase-5-testing-and-performance)
- [Proxy and API Tests](docs/issues/archive/WIP.md#phase-5-testing-and-performance)
- [UI and Integration Tests](docs/issues/planned/phase-5-ui-tests.md)
- [Benchmark Tool Core](docs/issues/done/phase-4-benchmark-core.md)
- [Benchmark Metrics](docs/issues/backlog/phase-7-benchmark-metrics.md)
- [Benchmark Scenarios](docs/issues/archive/WIP.md#phase-5-testing-and-performance)

## Phase 6: Deployment and Documentation
- [Docker Optimization](docs/issues/done/phase-6-docker-optimization.md)
- [Container Orchestration](docs/issues/backlog/phase-6-container-orchestration.md)
- [User Documentation](docs/issues/planned/phase-6-user-docs.md)
- [Developer Documentation](docs/issues/done/phase-6-dev-docs.md)
- [Security Documentation](docs/issues/planned/phase-6-security-docs.md)

## Phase 7: Optimization and Production Readiness
- [Performance Profiling](docs/issues/backlog/phase-7-performance-profiling.md)
- [Concurrency Optimization](docs/issues/backlog/phase-7-concurrency.md)
- [Database Optimization](docs/issues/backlog/phase-7-db-optimization.md)
- [Memory and CPU Optimization](docs/issues/backlog/phase-7-memory-cpu.md)
- [HTTPS and Security](docs/issues/backlog/phase-7-https.md)
- [Operational Features](docs/issues/backlog/phase-7-operational.md)
- [Scaling Support](docs/issues/backlog/phase-7-scaling.md)

---

**All future development, tracking, and progress updates will be managed via the above issue files.**

For any new features, enhancements, or bug fixes, please create a new issue file in `docs/issues/` and link it here if needed.

## Dependencies

- Phase 0 must be completed before Phase 1
- Phase 1 must be completed before Phase 2
- Database Implementation must be completed before Token Management System
- Token Management System must be completed before Management API Endpoints
- Proxy Logic must be completed before Proxy API Endpoints
- Core Components (Phase 2) must be completed before API and Interfaces (Phase 3)
- Logging System should be implemented early but can be enhanced incrementally
- Testing can begin after individual components are implemented
- Benchmark Tool requires Proxy API to be functional
- Containerization can start after core functionality is implemented
- Optimization should be done after basic functionality is working

## Timeline

- Phase 0-1: Day 1
- Phase 2: Days 1-2
- Phase 3-4: Days 3-4
- Phase 5-6: Days 5-6
- Phase 7: Days 7-8

- [x] Fix unchecked error returns in tests flagged by golangci-lint
- [x] Update GitHub Actions workflow to use correct golangci-lint version and options
- [x] Restrict Docker builds to main branch and tags only

**Note:** SQLite is used for MVP, local, and development deployments. PostgreSQL will be evaluated and tested for production use before launch. The codebase and schema should remain portable between both database engines.

## Current Focus
- The proxy architecture is now explicitly generic, designed to support any API requiring secure, short-lived (withering) tokens and transparent proxying. OpenAI is used as a case study for the MVP.
- ✅ Implementation of a YAML-based configuration system for API provider endpoints and methods. Configuration supports multiple providers and is extensible.
- ✅ The proxy performs minimal, necessary request/response transformations (e.g., Authorization header replacement) while extracting useful metadata (token counts, model information).
- ✅ Streaming responses are properly handled with transparent pass-through, maintaining the streaming nature of the API.
- ✅ **PHASE 3 COMPLETE:** Management API endpoints and Admin UI Foundation implemented
  - Management API with full CRUD operations (PR18)
  - Complete Admin UI with integrated `admin` command (PR19)
  - Security-focused design with modern Bootstrap interface
  - CLI integration and configuration system
- ✅ **CODE ORGANIZATION IMPROVEMENTS:** Enhanced testability and maintainability
  - Extracted business logic from cmd/ to internal/ packages for better testability
  - Improved test coverage from 67.0% → 68.9% with comprehensive unit tests
  - Fixed race conditions and test failures
  - Organized admin command in dedicated file with proper graceful shutdown
- The next focus areas are:
  - **Phase 4:** Logging and Monitoring system implementation
  - **Phase 5:** Comprehensive testing and performance optimization
  - **Optional enhancements:** Real-time updates, advanced UI features

## Remaining Configuration Tasks
- [ ] Expand provider config and YAML changes (document and test) - *moved to Phase 4/5*

// Note: Linter/staticcheck/errcheck issues for proxy and server resolved. Test race conditions and failures fixed. Coverage improved with extracted functionality testing.

## 2025-05-24: Benchmark Tool Core Implemented
- Implemented the core architecture for the benchmark tool in `cmd/proxy`.
- CLI now supports required flags: `--base-url`, `--endpoint`, `--token`, `--requests`, `--concurrency`.
- Added concurrent request handling with worker pool logic.
- Implemented initial request generator for supported endpoints.
- Added and updated tests for CLI, concurrency, and request generation (TDD followed).
- Integrated `make test-benchmark` for targeted test runs.
- All tests passing (`make test-benchmark`).
- Updated process and rationale per working agreement.