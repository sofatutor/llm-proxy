# Implementation Plan for Transparent API Proxy (Case Study: OpenAI)

> This file is the canonical source for the coverage policy. All other documentation should reference this file for coverage requirements and rationale.

> **Agent-Driven Test-Driven Development (TDD) Mandate**
>
> This project is implemented entirely by autonomous agents. All development MUST strictly follow test-driven development (TDD):
> - Every feature or change must first be implemented as a failing unit test.
> - Only after the test is written may the implementation be created or modified to make the test pass.
> - No code may be merged unless it is covered by tests.
> - A minimum of 90% code coverage is required at all times, enforced by GitHub Actions.
> - Pull requests must demonstrate that new/changed code is covered by tests and that overall coverage remains above 90%.
> - Coverage checks are mandatory in CI and must block merges if not met.

> **Coverage Policy Note:**
> - Code in `cmd/` (CLI entrypoints, flag parsing, glue code) is **not included in coverage checks**.
> - All core logic, business rules, and testable functionality **must reside in `internal/`** packages.
> - Only minimal CLI glue/flag parsing should be in `cmd/`.
> - This ensures high test coverage and maintainability.

> **Note:** While this project uses OpenAI as a case study, the architecture is intentionally generic and can be adapted to any API requiring secure, short-lived (withering) tokens and transparent proxying. The only required intervention is minimal (e.g., Authorization header replacement), ensuring maximum transparency. Future extensions may include custom request/response transformations.

> **Minimum Latency Mandate:** All design and implementation decisions must prioritize minimum added latency. The proxy should introduce as little overhead as possible, with all middleware, token validation, and logging optimized for speed. Performance testing and optimization for low latency are required at every stage.

## Overview
This document outlines the implementation plan for a transparent proxy for OpenAI's API. The proxy is designed to handle **withering tokens** (tokens with limited validity, revocation, and rate-limiting), log API calls with metadata (e.g., token counts), support streaming responses, and provide administrative capabilities. Built using Go for performance and concurrency, with SQLite for storage, the system includes a web-based admin UI, Docker deployment, and a CLI benchmark tool.

## Objectives
- **Transparent Proxying**: Forward requests to OpenAI's API with minimal overhead and lowest possible latency
- **Withering Token Management**: Generate tokens with expiration, revocation, and rate-limiting
- **Secure Authentication**: Restrict token management with `MANAGEMENT_TOKEN`
- **Logging**: Record API calls with metadata to local files and async backends
- **Streaming Support**: Handle Server-Sent Events for streaming responses
- **SQLite Database**: Store projects and tokens
- **Admin UI**: Web interface for managing projects and tokens
- **Docker Deployment**: Containerized proxy and benchmark tool
- **Benchmark Tool**: CLI for measuring latency, throughput, and errors
- **Unit Tests**: Comprehensive tests for all components

## Architecture

### Core Components
1. **HTTP Server**
   - Routes for `/manage/tokens`, `/v1/*`, and `/admin/*`
   - Authentication middleware for management and admin endpoints
   - Request validation and proxying

2. **Withering Token Management**
   - UUID-based tokens scoped to projects
   - Expiration logic (`expires_at` timestamp)
   - Revocation mechanism (`is_active` boolean)
   - Rate-limiting via `request_count`

3. **Database (SQLite)**
   - Schema for `projects` and `tokens` tables
   - CRUD operations for each entity
   - Indexes for fast lookups

4. **Logging System**
   - Local JSON Lines file (`api_logs.jsonl`)
   - Asynchronous worker for external JSON backends
   - Metadata extraction from OpenAI responses

5. **Proxy Logic**
   - Token validation
   - Request forwarding with header manipulation
   - Response parsing for metadata
   - Streaming support (SSE)
   - Generic design for multiple API providers
   - Minimal request/response transformation
   - High performance with connection pooling

6. **Admin UI**
   - HTML-based interface with basic auth
   - Project and token management
   - Simple CSS styling

7. **Benchmark Tool**
   - CLI with flag parsing
   - Concurrent request handling
   - Metrics calculation and reporting

### Database Schema
- **projects**
  - `id`: TEXT (UUID, primary key)
  - `name`: TEXT (project name)
  - `openai_api_key`: TEXT (OpenAI API key)
- **tokens**
  - `token`: TEXT (UUID, primary key)
  - `project_id`: TEXT (foreign key to projects)
  - `expires_at`: DATETIME (expiration timestamp)
  - `is_active`: BOOLEAN (true/false, default true)
  - `request_count`: INTEGER (rate-limiting counter, default 0)

## Database Architecture

- **SQLite** is the default database for MVP, local development, and small-scale/self-hosted deployments. It offers simplicity, zero-config, and fast prototyping.
- **PostgreSQL** is recommended for production deployments requiring high concurrency, advanced features, or distributed/cloud-native scaling. 
- The codebase and schema/migrations should be designed to support both SQLite and PostgreSQL, enabling a smooth migration path as needed.

## Implementation Steps

### 1. Project Setup
- Initialize Go module with dependencies (Go 1.23)
- Create directory structure
- Document project in README

### 2. Database Implementation
- Define schema for `projects` and `tokens` tables
- Implement database initialization and CRUD operations
- Add indexes for performance

### 3. Withering Token Management
- Implement token endpoints for generation and revocation
- Add management token authentication
- Create validation and rate-limiting logic

### 4. Proxy Logic
- Create handlers for OpenAI API endpoints
- Implement token validation and header manipulation
- Add metadata parsing for non-streaming responses
- Support streaming with Server-Sent Events

### 5. Logging System
- The proxy implements a canonical local logging system (JSON Lines, local file) for compliance, debugging, and fallback.
- **Helicone** is integrated as an optional, asynchronous observability middleware. When enabled, it forwards request/response data to Helicone for advanced analytics and dashboards.
- All Helicone operations are performed asynchronously (in a goroutine or via a worker) to ensure minimum added latency.
- Prometheus metrics are not implemented; Helicone serves as the main observability/analytics platform.
- Helicone integration is opt-in and can be enabled/disabled via configuration. If disabled, only custom logging is used.
- All core logging and metrics remain in the proxy's control; Helicone is additive, not a critical dependency.

### 6. Admin UI
- Design HTML interface with basic CSS
- Implement admin routes with basic auth
- Add JavaScript for form submissions and actions

### 7. Unit Testing
- **Test-Driven Development (TDD) Required**: All code must be written using TDD. Write failing tests before implementation.
- **Coverage Requirement**: Maintain at least 90% code coverage, enforced by CI.
- Write tests for all components
- Create mocks for external services
- Verify test coverage in every PR

### 8. Benchmark Tool
- Implement CLI with flag parsing
- Add concurrent request handling
- Calculate and report performance metrics

### 9. Containerization
- Create multi-stage Dockerfile
- Configure volumes for data persistence
- Set up environment variables

### 10. Performance Optimization
- Use goroutines for concurrency
- Implement connection pooling
- Optimize database queries
- **Aggressively profile and minimize latency at every layer of the stack**

## API Endpoints

### Token Management (`/manage/tokens`)
- **Authentication**: `Authorization: Bearer <MANAGEMENT_TOKEN>`
- **POST**: Generate a token
  - Request: `{"project_id": "<uuid>", "duration_hours": <int>}`
  - Response: `{"token": "<uuid>", "expires_at": "<iso8601>"}`
- **DELETE**: Revoke a token
  - Request: `{"token": "<uuid>"}`
  - Response: 204 No Content

### Proxy (`/v1/*`)
- **Authentication**: `Authorization: Bearer <withering-token>`
- Forwards requests to `https://api.openai.com/v1/*`
- Supports streaming (`stream=true`)
- **Documentation Note:** The proxy API is not documented with Swagger/OpenAPI except for authentication, allowed paths/methods, and transparency. Request/response schemas are not defined here; refer to the backend provider's documentation for those details. See rationale below.

### Admin UI (`/admin/*`)
- **Authentication**: Basic auth (`ADMIN_USER`, `ADMIN_PASSWORD`)
- **Endpoints**:
  - `/admin/`: Serves HTML interface
  - `/admin/projects`: CRUD for projects
  - `/admin/tokens`: Revoke tokens

### Management API
- `/manage/projects` (CRUD): POST, GET, PATCH, DELETE
  - Auth: `Authorization: Bearer <MANAGEMENT_TOKEN>`
  - Request/response formats:
    - **POST**: Create a project
      - Request: `{"name": "<string>", "description": "<string>", "metadata": {"key": "value"}}`
      - Response: `{"project_id": "<uuid>", "name": "<string>", "description": "<string>", "metadata": {"key": "value"}, "created_at": "<iso8601>"}`
    - **GET**: Retrieve projects
      - Request: None
      - Response: `[{"project_id": "<uuid>", "name": "<string>", "description": "<string>", "metadata": {"key": "value"}, "created_at": "<iso8601>"}]`
    - **PATCH**: Update a project
      - Request: `{"project_id": "<uuid>", "name": "<string>", "description": "<string>", "metadata": {"key": "value"}}`
      - Response: `{"project_id": "<uuid>", "name": "<string>", "description": "<string>", "metadata": {"key": "value"}, "updated_at": "<iso8601>"}`
    - **DELETE**: Delete a project
      - Request: `{"project_id": "<uuid>"}`
      - Response: 204 No Content
- `/manage/tokens` (CRUD): POST, GET, DELETE
  - Auth: `Authorization: Bearer <MANAGEMENT_TOKEN>`
  - Request/response formats:
    - **POST**: Generate a token
      - Request: `{"project_id": "<uuid>", "duration_hours": <int>}`
      - Response: `{"token": "<uuid>", "expires_at": "<iso8601>"}`
    - **GET**: Retrieve tokens
      - Request: None
      - Response: `[{"token": "<uuid>", "project_id": "<uuid>", "expires_at": "<iso8601>", "is_active": true, "request_count": 0}]`
    - **DELETE**: Revoke a token
      - Request: `{"token": "<uuid>"}`
      - Response: 204 No Content
- **CLI is now fully configurable via --manage-api-base-url; 'token get' subcommand is implemented.**
- **Planned:** Add more integration specs for management API flows.

#### CLI Usage Example
```sh
llm-proxy manage project list --manage-api-base-url http://localhost:8080 --management-token <token>
llm-proxy manage token generate --project-id <project-id> --management-token <token> --manage-api-base-url http://localhost:8080
llm-proxy manage token get <token> --management-token <token> --manage-api-base-url http://localhost:8080 --json
```

### Health Check
- `/health`: Returns status, timestamp, version
  - Used for readiness/liveness probes, monitoring, and orchestration.

## Logging Format
```json
{
  "timestamp": "2025-05-20T00:03:00Z",
  "token": "<uuid>",
  "project_id": "<uuid>",
  "endpoint": "/v1/chat/completions",
  "method": "POST",
  "status_code": 200,
  "duration_ms": 150,
  "metadata": {
    "prompt_tokens": 9,
    "completion_tokens": 12,
    "total_tokens": 21,
    "model": "gpt-4",
    "created": 1677652288
  }
}
```

## Deployment

### Docker Setup
```bash
docker build -t llm-proxy .
mkdir data
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e MANAGEMENT_TOKEN=$(uuidgen) \
  -e LOGGING_URL=http://logs.example.com/logs \
  -e ADMIN_USER=admin \
  -e ADMIN_PASSWORD=secret \
  llm-proxy
```

### Container Orchestration
Container orchestration is now split into two supported tracks:
- [AWS ECS Deployment](docs/issues/phase-6-aws-ecs.md)
- [Kubernetes Deployment with HELM](docs/issues/phase-6-kubernetes-helm.md)

Refer to the linked issue files for detailed tasks, rationale, and acceptance criteria for each orchestration platform.

### Benchmark Tool
```bash
docker run --rm llm-proxy llm-benchmark \
  --base-url=http://host.docker.internal:8080 \
  --endpoint=/v1/chat/completions \
  --token=<withering-token> \
  --requests=100 \
  --concurrency=10
```

## Security Considerations
- **Tokens**: Expiration, revocation, rate-limiting (1000 requests/hour)
- **Management Token**: Secure storage, validation
- **API Keys**: Encrypt in database
- **Admin UI**: Basic auth protection
- **Docker**: Minimal image, non-root user

## Production Enhancements
- Use PostgreSQL for scalability
- Add HTTPS via reverse proxy
- Monitor with Prometheus/Grafana
- Clean up expired tokens periodically
- Store secrets in a secure manager
- Implement Redis-backed distributed rate limiting
- Set up horizontal scaling with load balancing
- Add Redis-backed request caching for improved performance
- Implement cache invalidation and consistency mechanisms

## Testing Strategy
- **Test-Driven Development (TDD) is mandatory for all code.**
- **90%+ code coverage is required and enforced by CI.**
- Unit tests for all components
- Integration tests for end-to-end flows
- Docker tests for container validation
- Benchmark tests for performance verification, with a focus on measuring and minimizing added latency

## Timeline
- **Day 1-2**: Project setup, database, token management
- **Day 3-4**: Proxy logic, streaming, metadata parsing
- **Day 5-6**: Logging, admin UI, testing, benchmarking
- **Day 7-8**: Docker, optimization, documentation, deployment

## Deliverables
- Source code and tests
- Docker container
- SQLite database
- Logging configuration
- Benchmark tool
- Documentation

- Docker builds are now only triggered on main branch and tags (not on PRs)
- CI linting is now fully aligned with local linting and Go best practices

## Whitelist Approach for URIs and Methods

To maximize security and minimize attack surface, the proxy implements a whitelist (allowlist) for valid API URIs and HTTP methods. For the MVP, this list is hardcoded for OpenAI endpoints (e.g., `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings`, `/v1/models`) and methods (`POST`, `GET`).

- **Purpose:** Prevents abuse and accidental exposure by restricting access to only known, safe endpoints and methods.
- **Design:** The whitelist logic is implemented so it can be easily extended or made configurable for other APIs in the future.
- **Transparency:** All other request/response data is passed through unchanged, except for necessary header replacements (e.g., Authorization).
- **Extensibility:** Future versions may support dynamic or config-driven whitelists, and on-the-fly request/response transformations via middleware.

> **Minimum Latency Principle:** Every architectural component, from HTTP server to middleware and database access, must be designed for minimal latency. Avoid unnecessary processing, blocking operations, or synchronous I/O in the request path. Use concurrency and asynchronous operations where possible to keep proxy response times as close to direct API calls as possible.

## CLI Tool (Setup, Server, Chat)
- Add support for daemon mode (`llm-proxy server -d`), PID file management, and advanced CLI flags
- Expanded documentation and end-to-end usage examples
- Improved flag parsing and configuration overrides
- Planned: `llm-proxy server` will support subcommands such as `start` (with `-d` for daemon mode), `stop`, and `health` for operational control in the final version.

## Logging System
- [x] Research logging best practices
- [x] Define comprehensive log format:
  - Standard fields (timestamp, level, message)
  - Request-specific fields (endpoint, method, status)
  - Performance metrics (duration, token counts)
  - Error details when applicable
- [x] Implement JSON Lines local logging:
  - Set up log file creation
  - Implement log rotation (configurable size/backups)
  - Configure log levels and file path
- [x] Create log format with detailed metadata
- [ ] **Integrate Helicone as optional, asynchronous observability middleware**
- [ ] Add configuration for enabling/disabling Helicone
- [ ] Add tests for Helicone integration (mocked, async)
- [ ] Document Helicone integration and fallback behavior
 - [x] Implement asynchronous worker for external logging:
  - Buffered sending
  - Batch processing
- [ ] Add structured logging throughout the application
- [ ] Implement log context propagation
- [ ] Create log search and filtering utilities
- [ ] Set up log aggregation for distributed deployments
- [ ] Implement audit logging for security events
- [ ] Create log visualization recommendations
- [ ] Add log sampling for high-volume deployments
- [ ] Add proxy metrics/logging/timing improvements 

## Monitoring
- The `/health` endpoint (see API Endpoints) is used for readiness/liveness probes.

## API Provider Config
- Expanded YAML config for API providers, endpoints, and methods

## Benchmark Tool
- Refactor and expand benchmark tool, add setup logic, restore tests

## Project Directory Structure (Updated)

- `cmd/proxy/`: Main CLI for the LLM Proxy. Contains all user/server commands (setup, server, openai chat, etc.), tests, and documentation for the main CLI.
- `cmd/benchmark/`: Reserved for benchmarking tools only. Contains only benchmark-specific code (or a placeholder if not yet implemented).
- `internal/`: Shared logic, server, config, token, database, etc.

**Rationale:**
- Follows Go best practices and Single Responsibility Principle (SRP).
- Avoids code duplication and confusion about command ownership.
- Ensures all user/server/management logic is in one place (`cmd/proxy/`), while benchmarks are isolated.

- In-memory DB is only used for tests

- Stage core proxy logic (streaming, allowlist, error handling, metrics/logging) from internal/proxy in PR: Feature: Transparent Proxy Core

## Clarification: The proxy only validates the token, the allowed path, and the allowed HTTP method. All other request validation or transformation is out of scope and must be handled by the upstream API or via YAML config if needed. This is to ensure minimum latency and maximum transparency.

## Proxy Robustness Features (PR17)

### Architecture
- Minimal retry logic for transient upstream failures (conservative, low retry limit)
- Simple circuit breaker (opens on repeated failures, closes after cooldown)
- Validation scope strictly limited to token, path, and method
- All API-specific logic must be config-driven, not in core

### Implementation Steps
- [x] Add failing tests for retry, circuit breaker, and validation scope
- [x] Implement retry middleware and wire into proxy
- [x] Implement circuit breaker middleware and wire into proxy
- [x] Enforce validation scope in middleware
- [x] Achieve >90% test coverage for all new logic
- [x] All tests passing (`make test-coverage`)
- [x] Update WIP.md and PLAN.md

### References
- See WIP.md for process and status

## Rationale
- Helicone provides advanced LLM observability and analytics with minimal integration effort.
- All Helicone operations are async to ensure minimum latency, in line with the project's minimum latency mandate.
- Custom logging remains canonical for compliance and fallback; Helicone is additive and optional.
- No vendor lock-in: disabling Helicone reverts to local logging only.

## Release Plan

- Releases are managed via **GitHub Releases**. Each release is tagged using [Semantic Versioning](https://semver.org/) (e.g., v1.2.3).
- Docker images are built and published to **GitHub Container Registry (GHCR)** on every tagged release (see `.github/workflows/docker.yml`).
- The release workflow is automated: pushing a new tag (e.g., `v1.2.3`) triggers the build and publish process for both binaries and Docker images.
- A dedicated CLI command (e.g., `llm-proxy release draft`) will be provided to help draft new releases, generate changelogs, and automate operational chores (e.g., version bumping, tagging, and pushing tags).
- Release notes are generated from merged PRs and issue files, ensuring traceability and transparency.
- All release artifacts (binaries, Docker images, changelogs) are attached to the GitHub Release.
- The release process is documented in `/docs/release.md` (to be created).
- All major release, versioning, and operational automation issues are tracked in `docs/issues/` (see: SecOps, Docker, operational, and CLI automation issues).
