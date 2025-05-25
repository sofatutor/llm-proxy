# LLM Proxy

A transparent, secure proxy for OpenAI's API with token management, rate limiting, logging, and admin UI.

## Features
- **OpenAI API Compatibility**
- **Withering Tokens**: Expiration, revocation, and rate-limiting
- **Project-based Access Control**
- **Admin UI**: Web interface for management
- **Comprehensive Logging**
- **Prometheus Monitoring**
- **SQLite Storage**
- **Docker Deployment**

## Quick Start

### Docker (Recommended)
```bash
docker pull sofatutor/llm-proxy:latest
mkdir -p ./llm-proxy/data
docker run -d \
  --name llm-proxy \
  -p 8080:8080 \
  -v ./llm-proxy/data:/app/data \
  -e MANAGEMENT_TOKEN=your-secure-management-token \
  sofatutor/llm-proxy:latest
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

See `docs/configuration.md` for all options.

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

## Project Structure
- `/cmd` — Entrypoints (`proxy`)
- `/internal` — Core logic (token, database, proxy, admin, logging)
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

## License
MIT License

---
For advanced usage, architecture, contributing, and benchmarking, see the `/docs` directory.