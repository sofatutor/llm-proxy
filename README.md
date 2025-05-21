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

See `docs/configuration.md` for all options.

## Main API Endpoints

### Token Management
- `POST /manage/tokens` — Create token
- `DELETE /manage/tokens` — Revoke token
  
Example:
```bash
curl -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_id": "<uuid>", "duration_hours": 24}'
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

### Admin UI
- `/admin/` — Web interface (requires admin credentials)

## Project Structure
- `/cmd` — Entrypoints (`proxy`, `benchmark`)
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