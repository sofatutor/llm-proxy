# LLM Proxy

A transparent, secure proxy for OpenAI's API with token management, rate limiting, logging, and admin UI.

## Features
- **OpenAI API Compatibility**
- **Withering Tokens**: Expiration, revocation, and rate-limiting
- **Project-based Access Control**
- **HTTP Response Caching**: Redis-backed cache with configurable TTL, auth-aware shared caching, and streaming response support. Enable with `HTTP_CACHE_ENABLED=true`.
- **Admin UI**: Web interface for management
- **Comprehensive Logging**
- **Async Instrumentation Middleware**: Non-blocking, streaming-capable instrumentation for all API calls. See [docs/instrumentation.md](docs/instrumentation.md) for advanced usage and extension.
- **Async Event Bus & Dispatcher**: All API instrumentation events are handled via an always-on, fully asynchronous event bus (in-memory or Redis) with support for multiple subscribers, batching, retry logic, and graceful shutdown. Persistent event logging is handled by a dispatcher CLI or the `--file-event-log` flag.
- **OpenAI Token Counting**: Accurate prompt and completion token counting using tiktoken-go.
- **Prometheus Monitoring**
- **SQLite Storage**
- **Docker Deployment**

## Quick Start

### Docker (Recommended)
```bash
docker pull ghcr.io/sofatutor/llm-proxy:latest
mkdir -p ./llm-proxy/data
docker run -d \
  --name llm-proxy \
  -p 8080:8080 \
  -v ./llm-proxy/data:/app/data \
  -e MANAGEMENT_TOKEN=your-secure-management-token \
  ghcr.io/sofatutor/llm-proxy:latest
```

#### With Redis Caching
```bash
# Start Redis
docker run -d --name redis -p 6379:6379 redis:alpine

# Start proxy with caching enabled
docker run -d \
  --name llm-proxy \
  -p 8080:8080 \
  -v ./llm-proxy/data:/app/data \
  -e MANAGEMENT_TOKEN=your-secure-management-token \
  -e HTTP_CACHE_ENABLED=true \
  -e HTTP_CACHE_BACKEND=redis \
  -e REDIS_CACHE_URL=redis://redis:6379/0 \
  --link redis \
  ghcr.io/sofatutor/llm-proxy:latest
```

### From Source
```bash
git clone https://github.com/sofatutor/llm-proxy.git
cd llm-proxy
make build
MANAGEMENT_TOKEN=your-secure-management-token ./bin/llm-proxy
```

## Configuration (Essentials)
- `MANAGEMENT_TOKEN` (required): Admin API access
- `LISTEN_ADDR`: Default `:8080`
- `DATABASE_PATH`: Default `./data/llm-proxy.db`
- `LOG_LEVEL`: Default `info`
- `LOG_FILE`: Path to log file (stdout if empty)
- `LOG_MAX_SIZE_MB`: Rotate log after this size in MB (default 10)
- `LOG_MAX_BACKUPS`: Number of rotated log files to keep (default 5)
- `AUDIT_ENABLED`: Enable audit logging (default `true`)
- `AUDIT_LOG_FILE`: Audit log file path (default `./data/audit.log`)
- `AUDIT_STORE_IN_DB`: Store audit events in database (default `true`)
- `AUDIT_CREATE_DIR`: Create audit log directories (default `true`)
- `OBSERVABILITY_ENABLED`: Deprecated; the async event bus is now always enabled
- `OBSERVABILITY_BUFFER_SIZE`: Event buffer size for instrumentation events (default 1000)
- `FILE_EVENT_LOG`: Path to persistent event log file (enables file event logging via dispatcher)

### Caching Configuration
- `HTTP_CACHE_ENABLED`: Enable HTTP response caching (default `true`)
- `HTTP_CACHE_BACKEND`: Cache backend (`redis` or `in-memory`, default `in-memory`)
- `REDIS_CACHE_URL`: Redis connection URL (default `redis://localhost:6379/0` when backend=redis)
- `REDIS_CACHE_KEY_PREFIX`: Cache key prefix (default `llmproxy:cache:`)
- `HTTP_CACHE_MAX_OBJECT_BYTES`: Maximum cached object size in bytes (default 1048576)
- `HTTP_CACHE_DEFAULT_TTL`: Default TTL in seconds when upstream doesn't specify (default 300)

See `docs/configuration.md` and [docs/instrumentation.md](docs/instrumentation.md) for all options and advanced usage.

### Advanced Example
```yaml
apis:
  openai:
    param_whitelist:
      model:
        - gpt-4o
        - gpt-4.1-*
    allowed_origins:
      - https://www.sofatutor.com
      - http://localhost:4000
    required_headers:
      - origin
```

See `docs/issues/phase-7-param-cors-whitelist.md` for advanced configuration and rationale.

## Main API Endpoints

### Management API
- `/manage/projects` — Project CRUD
  - `GET /manage/projects` — List all projects
  - `POST /manage/projects` — Create a new project
- `/manage/projects/{projectId}`
  - `GET` — Get project details
  - `PATCH` — Update a project (partial update)
  - `DELETE` — Delete a project
- `/manage/tokens` — Token CRUD
  - `GET /manage/tokens` — List all tokens
  - `POST /manage/tokens` — Generate a new token
- `/manage/tokens/{token}`
  - `GET` — Get token details
  - `DELETE` — Revoke a token

All management endpoints require:
```
Authorization: Bearer <MANAGEMENT_TOKEN>
```

#### Example (curl):
```bash
curl -X POST http://localhost:8080/manage/projects \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "My Project", "openai_api_key": "sk-..."}'

curl -X PATCH http://localhost:8080/manage/projects/<project-id> \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "New Name"}'
```

### Proxy
- `POST /v1/*` — Forwarded to OpenAI, requires withering token

Example:
```bash
curl -H "Authorization: Bearer <withering-token>" \
     -H "Content-Type: application/json" \
     -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
     http://localhost:8080/v1/chat/completions
```

> **Note:** The proxy API is not documented with Swagger/OpenAPI except for authentication and allowed paths/methods. For backend schemas, refer to the provider's documentation.

### Admin UI
- `/admin/` — Web interface (requires admin credentials)

## CLI Management Tool

The CLI provides full management of projects and tokens via the `llm-proxy manage` command. All subcommands support the `--manage-api-base-url` flag (default: http://localhost:8080) and require a management token (via `--management-token` or `MANAGEMENT_TOKEN` env).

### Project Management
```sh
llm-proxy manage project list --manage-api-base-url http://localhost:8080 --management-token <token>
llm-proxy manage project get <project-id> --manage-api-base-url http://localhost:8080 --management-token <token>
llm-proxy manage project create --name "My Project" --openai-key sk-... --manage-api-base-url http://localhost:8080 --management-token <token>
llm-proxy manage project update <project-id> --name "New Name" --manage-api-base-url http://localhost:8080 --management-token <token>
llm-proxy manage project delete <project-id> --manage-api-base-url http://localhost:8080 --management-token <token>
```

### Token Management
```sh
llm-proxy manage token generate --project-id <project-id> --duration 24 --manage-api-base-url http://localhost:8080 --management-token <token>
```

### Flags
- `--manage-api-base-url` — Set the management API base URL (default: http://localhost:8080)
- `--management-token` — Provide the management token (or set `MANAGEMENT_TOKEN` env)
- `--json` — Output results as JSON (optional)

## Event Dispatcher CLI

The LLM Proxy includes a powerful, pluggable dispatcher system for sending observability events to external services. The dispatcher supports multiple backends and can be run as a separate service.

### Supported Backends
- **file**: Write events to JSONL file
- **lunary**: Send events to Lunary.ai platform
- **helicone**: Send events to Helicone platform

### Basic Usage
```bash
# File output  
llm-proxy dispatcher --service file --endpoint events.jsonl

# Lunary integration
export LLM_PROXY_API_KEY="your-lunary-api-key"
llm-proxy dispatcher --service lunary

# Helicone integration
llm-proxy dispatcher --service helicone --api-key your-helicone-key

# Custom batch size and buffer
llm-proxy dispatcher --service lunary --api-key $API_KEY --batch-size 50 --buffer 2000
```

### Deployment Options
The dispatcher can be deployed in multiple ways:
- **Standalone Process**: Run as a separate service for production
- **Sidecar Container**: Deploy alongside the main proxy in Kubernetes
- **Background Mode**: Use `--detach` flag for daemon-like operation

See [docs/instrumentation.md](docs/instrumentation.md) for detailed configuration and architecture.

> Warning: Event loss can occur if the Redis event log is configured with TTL/max length values that are too low for your dispatcher lag and throughput. In production, increase Redis TTL and list length to cover worst-case backlogs and keep the dispatcher running with sufficient batch size/throughput. For strict guarantees, use a durable queue (e.g., Redis Streams with consumer groups or Kafka). See the Production Reliability section in `docs/instrumentation.md`.

## Using Redis for Distributed Event Bus (Local Development)

> **Note:** The in-memory event bus only works within a single process. For multi-process setups (e.g., running the proxy and dispatcher as separate processes or containers), you must use Redis as the event bus backend.

### Local Setup with Docker Compose

A `redis` service is included in the `docker-compose.yml` for local development:

```yaml
db:
  image: redis:7
  container_name: llm-proxy-redis
  ports:
    - "6379:6379"
  restart: unless-stopped
```

### Configuring the Proxy and Dispatcher to Use Redis

Set the event bus backend to Redis by using the appropriate environment variable or CLI flag (see documentation for exact flag):

```bash
LLM_PROXY_EVENT_BUS=redis llm-proxy ...
LLM_PROXY_EVENT_BUS=redis llm-proxy dispatcher ...
```

This ensures both the proxy and dispatcher share events via Redis, enabling full async pipeline testing and production-like operation.

## Project Structure
- `/cmd` — Entrypoints (`proxy`, `eventdispatcher`)
- `/internal` — Core logic (token, database, proxy, admin, logging, eventbus, dispatcher)
- `/api` — OpenAPI specs
- `/web` — Admin UI static assets
- `/docs` — Full documentation

## Security & Production Notes
- Tokens support expiration, revocation, and rate limits
- Management API protected by `MANAGEMENT_TOKEN`
- Admin UI uses basic auth (`ADMIN_USER`, `ADMIN_PASSWORD`)
- Logs stored locally and/or sent to external backends
- Use HTTPS in production (via reverse proxy)
- See `docs/security.md` and `docs/production.md` for best practices

### Containerization Notes
- Multi-stage Dockerfile builds a static binary and ships a minimal Alpine runtime
- Runs as non-root user `appuser` with read-only filesystem by default
- Healthcheck hits `/health`; see `docker-compose.yml` or Dockerfile `HEALTHCHECK`
- Volumes: `/app/data`, `/app/logs`, `/app/config`, `/app/certs`
- Example local build/test:
```bash
make docker-build
make docker-run
make docker-smoke
```

### Publishing
Images are built and published to GitHub Container Registry on pushes to `main` and tags `v*`.

Registry: `ghcr.io/sofatutor/llm-proxy`

Workflow: `.github/workflows/docker.yml` builds for `linux/amd64` and `linux/arm64` and pushes labels/tags.

## Documentation

This README provides a quick overview and getting started guide. For comprehensive documentation, see the `/docs` directory:

### 📚 **[Complete Documentation Index](docs/README.md)**

**Key Documentation:**
- **[CLI Reference](docs/cli-reference.md)** - Complete command-line interface documentation
- **[Go Package Documentation](docs/go-packages.md)** - Using LLM Proxy packages in your applications
- **[Architecture Guide](docs/architecture.md)** - System architecture and design
- **[API Configuration](docs/api-configuration.md)** - Advanced API provider configuration
- **[Security Best Practices](docs/security.md)** - Production security guidelines
- **[Instrumentation Guide](docs/instrumentation.md)** - Event system and observability

**For Developers:**
- [OpenAPI Specification](api/openapi.yaml) - Machine-readable API definitions
- [Contributing Guidelines](CONTRIBUTING.md) - How to contribute to the project

## License
MIT License