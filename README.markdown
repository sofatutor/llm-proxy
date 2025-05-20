# Transparent LLM Proxy for OpenAI

## Overview
This project implements a transparent proxy for OpenAI's API, enabling secure and controlled access through **withering tokens** (tokens with expiration, revocation, and rate-limiting). It logs API calls with metadata (e.g., token counts), supports streaming responses, and provides a web-based admin UI for management. The proxy is built in Go, uses SQLite for storage, and is deployed via Docker. A CLI benchmark tool measures performance of OpenAI endpoints, with or without the proxy.

## Features
- **Transparent Proxying**: Forwards requests to `https://api.openai.com` with minimal overhead.
- **Withering Tokens**: Scoped to projects, with expiration, revocation, and rate-limiting.
- **Secure Authentication**: `/manage/tokens` restricted by `MANAGEMENT_TOKEN`.
- **Logging**: Local JSON Lines (`api_logs.jsonl`) and async JSON backend (e.g., CloudWatch).
- **Metadata Logging**: Captures OpenAI response metadata (`usage`, `model`, `created`).
- **Streaming Support**: Handles Server-Sent Events for `stream=true`.
- **SQLite Database**: Stores projects and tokens.
- **Admin UI**: Web interface for managing projects and tokens.
- **Docker Deployment**: Containerized proxy and benchmark tool.
- **Unit Tests**: Comprehensive tests for all components.
- **Benchmark Tool**: CLI for measuring latency, throughput, and errors.

## Prerequisites
- Go 1.21+ (for development).
- Docker (for deployment and testing).
- OpenAI API key (for testing/benchmarking).
- Optional: AWS CloudWatch Logs for async logging.

## Setup
1. **Clone Repository**:
   ```bash
   git clone <repository-url>
   cd llm-proxy
   ```
2. **Install Dependencies**:
   ```bash
   go mod tidy
   ```
3. **Build Locally**:
   ```bash
   go build -o llm-proxy main.go
   go build -o llm-benchmark benchmark.go
   ```

## Database Schema
- **projects**:
  - `id`: TEXT (UUID, primary key).
  - `name`: TEXT (project name).
  - `openai_api_key`: TEXT (OpenAI API key).
- **tokens**:
  - `token`: TEXT (UUID, primary key).
  - `project_id`: TEXT (foreign key to projects).
  - `expires_at`: DATETIME (expiration timestamp).
  - `is_active`: BOOLEAN (true/false, default true).
  - `request_count`: INTEGER (rate-limiting counter, default 0).

## API Endpoints
### Token Management (`/manage/tokens`)
- **Authentication**: `Authorization: Bearer <MANAGEMENT_TOKEN>`
- **POST**: Generate a token.
  - Request: `{"project_id": "<uuid>", "duration_hours": <int>}`
  - Response: `{"token": "<uuid>", "expires_at": "<iso8601>"}`
  - Example:
    ```bash
    curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
         -H "Content-Type: application/json" \
         -X POST -d '{"project_id": "<uuid>", "duration_hours": 24}' \
         http://localhost:8080/manage/tokens
    ```
- **DELETE**: Revoke a token.
  - Request: `{"token": "<uuid>"}`
  - Response: 204 No Content.
  - Example:
    ```bash
    curl -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
         -H "Content-Type: application/json" \
         -X DELETE -d '{"token": "<uuid>"}' \
         http://localhost:8080/manage/tokens
    ```

### Proxy (`/v1/*`)
- **Authentication**: `Authorization: Bearer <withering-token>`
- Forwards requests to `https://api.openai.com/v1/*`.
- Supports streaming (`stream=true`).
- Example:
  ```bash
  curl -H "Authorization: Bearer <withering-token>" \
       -H "Content-Type: application/json" \
       -d '{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
       http://localhost:8080/v1/chat/completions
  ```

### Admin UI (`/admin/*`)
- **Authentication**: Basic auth (`ADMIN_USER`, `ADMIN_PASSWORD`).
- **Endpoints**:
  - `/admin/`: Serves HTML interface.
  - `/admin/projects`: CRUD for projects (POST, DELETE).
  - `/admin/tokens`: Revoke tokens (DELETE).
- Access: `http://localhost:8080/admin/` in browser.

## Logging
- **Local**: `api_logs.jsonl` in `/app/data`.
- **Format**:
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
- **Async**: Sends logs to `LOGGING_URL` (e.g., CloudWatch).

## Benchmark Tool
- **Binary**: `llm-benchmark`
- **Usage**:
  - Proxied:
    ```bash
    ./llm-benchmark \
      --base-url=http://localhost:8080 \
      --endpoint=/v1/chat/completions \
      --token=<withering-token> \
      --requests=100 \
      --concurrency=10 \
      --payload='{"model":"gpt-4","messages":[{"role":"user","content":"Hello"}]}' \
      --output=results.json
    ```
  - Direct:
    ```bash
    ./llm-benchmark \
      --base-url=https://api.openai.com \
      --endpoint=/v1/chat/completions \
      --token=<openai-api-key> \
      --requests=100 \
      --concurrency=10
    ```
- **Output** (`results.json`):
  ```json
  {
    "base_url": "http://localhost:8080",
    "endpoint": "/v1/chat/completions",
    "requests": 100,
    "concurrency": 10,
    "total_duration_ms": 1500,
    "average_latency_ms": 15.0,
    "p50_latency_ms": 14.5,
    "p90_latency_ms": 20.0,
    "p99_latency_ms": 25.0,
    "throughput_rps": 66.67,
    "error_rate": 0.02,
    "errors": 2
  }
  ```

## Docker Deployment
1. **Build Image**:
   ```bash
   docker build -t llm-proxy .
   ```
2. **Create Data Directory**:
   ```bash
   mkdir data
   ```
3. **Run Proxy**:
   ```bash
   docker run -d \
     -p 8080:8080 \
     -v $(pwd)/data:/app/data \
     -e MANAGEMENT_TOKEN=$(uuidgen) \
     -e LOGGING_URL=http://logs.example.com/logs \
     -e ADMIN_USER=admin \
     -e ADMIN_PASSWORD=secret \
     llm-proxy
   ```
4. **Run Benchmark**:
   ```bash
   docker run --rm llm-proxy llm-benchmark \
     --base-url=http://host.docker.internal:8080 \
     --endpoint=/v1/chat/completions \
     --token=<withering-token> \
     --requests=100 \
     --concurrency=10
   ```

## Testing
- **Unit Tests**:
  ```bash
  go test -v ./...
  ```
- **Docker Tests**:
  ```bash
  docker build --target test -t llm-proxy-test . && docker run llm-proxy-test
  ```
- **Coverage**:
  ```bash
  go test -cover ./...
  ```

## Security
- **Tokens**: Expiration, revocation, rate-limiting (1000 requests/hour).
- **Management Token**: Stored in `MANAGEMENT_TOKEN`, validated for `/manage/tokens`.
- **API Keys**: Encrypt `openai_api_key` in database (future enhancement).
- **Admin UI**: Basic auth with `ADMIN_USER`, `ADMIN_PASSWORD`.
- **Docker**: Minimal `alpine` image, non-root user.

## Production Considerations
- Use PostgreSQL for scalability.
- Encrypt API keys.
- Add HTTPS via reverse proxy.
- Monitor with Prometheus/Grafana.
- Secure `MANAGEMENT_TOKEN` in secrets manager.
- Clean up expired tokens periodically.

## Contributing
- Add features via pull requests.
- Ensure tests pass and documentation is updated.
- Follow Go coding standards.

## License
MIT License.