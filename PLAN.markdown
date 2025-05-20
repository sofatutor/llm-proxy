# Implementation Plan for Transparent LLM Proxy for OpenAI

> **Agent-Driven Test-Driven Development (TDD) Mandate**
>
> This project is implemented entirely by autonomous agents. All development MUST strictly follow test-driven development (TDD):
> - Every feature or change must first be implemented as a failing unit test.
> - Only after the test is written may the implementation be created or modified to make the test pass.
> - No code may be merged unless it is covered by tests.
> - A minimum of 90% code coverage is required at all times, enforced by GitHub Actions.
> - Pull requests must demonstrate that new/changed code is covered by tests and that overall coverage remains above 90%.
> - Coverage checks are mandatory in CI and must block merges if not met.

## Overview
This document outlines the implementation plan for a transparent proxy for OpenAI's API. The proxy is designed to handle **withering tokens** (tokens with limited validity, revocation, and rate-limiting), log API calls with metadata (e.g., token counts), support streaming responses, and provide administrative capabilities. Built using Go for performance and concurrency, with SQLite for storage, the system includes a web-based admin UI, Docker deployment, and a CLI benchmark tool.

## Objectives
- **Transparent Proxying**: Forward requests to OpenAI's API with minimal overhead
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
   - Streaming support

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
- Implement JSON Lines local logging
- Create asynchronous worker for external logging
- Extract metadata from responses

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

### Admin UI (`/admin/*`)
- **Authentication**: Basic auth (`ADMIN_USER`, `ADMIN_PASSWORD`)
- **Endpoints**:
  - `/admin/`: Serves HTML interface
  - `/admin/projects`: CRUD for projects
  - `/admin/tokens`: Revoke tokens

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

## Testing Strategy
- **Test-Driven Development (TDD) is mandatory for all code.**
- **90%+ code coverage is required and enforced by CI.**
- Unit tests for all components
- Integration tests for end-to-end flows
- Docker tests for container validation
- Benchmark tests for performance verification

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