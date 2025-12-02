# Installation Guide

This guide covers all methods to install and run the LLM Proxy.

## Prerequisites

- **Management Token**: Generate a secure token for administrative access
  ```bash
  openssl rand -base64 32
  ```
- **OpenAI API Key** (optional): Required only if you want to proxy OpenAI requests

## Quick Start

The fastest way to get started is with Docker:

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

Verify the installation:
```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

## Installation Methods

### Docker (Recommended)

Docker provides the simplest deployment with automatic configuration.

#### Single Container

Pull and run the official image:

```bash
# Pull the latest image
docker pull ghcr.io/sofatutor/llm-proxy:latest

# Create data directory for persistence
mkdir -p ./llm-proxy/data

# Run the container
docker run -d \
  --name llm-proxy \
  -p 8080:8080 \
  -v ./llm-proxy/data:/app/data \
  -e MANAGEMENT_TOKEN=your-secure-management-token \
  ghcr.io/sofatutor/llm-proxy:latest
```

#### With Redis Caching

For production deployments, enable Redis-backed caching for better performance:

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

#### Docker Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release |
| `vX.Y.Z` | Specific version (e.g., `v1.0.0`) |
| `main` | Latest build from main branch (unstable) |

### Docker Compose

Docker Compose is recommended for multi-service deployments.

#### Basic Setup (SQLite)

Create a `docker-compose.yml`:

```yaml

services:
  llm-proxy:
    image: ghcr.io/sofatutor/llm-proxy:latest
    container_name: llm-proxy
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    environment:
      - MANAGEMENT_TOKEN=${MANAGEMENT_TOKEN}
      - LOG_LEVEL=info
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  data:
```

Start the services:
```bash
export MANAGEMENT_TOKEN=$(openssl rand -base64 32)
docker compose up -d
```

#### With Redis and Event Dispatcher

For production with caching and event logging:

```yaml

services:
  llm-proxy:
    image: ghcr.io/sofatutor/llm-proxy:latest
    container_name: llm-proxy
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./logs:/app/logs
      - ./config:/app/config
    environment:
      - MANAGEMENT_TOKEN=${MANAGEMENT_TOKEN}
      - LOG_LEVEL=info
      - HTTP_CACHE_ENABLED=true
      - HTTP_CACHE_BACKEND=redis
      - REDIS_CACHE_URL=redis://redis:6379/0
      - LLM_PROXY_EVENT_BUS=redis
      - REDIS_ADDR=redis:6379
    depends_on:
      redis:
        condition: service_started
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

  redis:
    image: redis:8
    container_name: llm-proxy-redis
    ports:
      - "6379:6379"
    restart: unless-stopped

  logger:
    image: ghcr.io/sofatutor/llm-proxy:latest
    container_name: llm-proxy-logger
    command: ["dispatcher", "--service", "file", "--endpoint", "/app/logs/events.jsonl"]
    volumes:
      - ./logs:/app/logs
    environment:
      - LLM_PROXY_EVENT_BUS=redis
      - REDIS_ADDR=redis:6379
    depends_on:
      - redis

volumes:
  data:
  logs:
```

#### With PostgreSQL

For production deployments requiring a robust database:

```yaml

services:
  postgres:
    image: postgres:15
    container_name: llm-proxy-postgres
    environment:
      POSTGRES_USER: llmproxy
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: llmproxy
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U llmproxy -d llmproxy"]
      interval: 5s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  llm-proxy:
    image: ghcr.io/sofatutor/llm-proxy:latest
    container_name: llm-proxy
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config
    environment:
      - MANAGEMENT_TOKEN=${MANAGEMENT_TOKEN}
      - DB_DRIVER=postgres
      - DATABASE_URL=postgres://llmproxy:${POSTGRES_PASSWORD}@postgres:5432/llmproxy?sslmode=disable
      - LOG_LEVEL=info
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  postgres_data:
```

Start with:
```bash
export MANAGEMENT_TOKEN=$(openssl rand -base64 32)
export POSTGRES_PASSWORD=$(openssl rand -base64 16)
docker compose up -d
```

See [Docker Compose PostgreSQL Setup](docker-compose-postgres.md) for detailed PostgreSQL configuration.

### Build from Source

Building from source requires Go 1.23 or later.

#### Requirements

- Go 1.23+
- Make
- Git

#### Build Steps

```bash
# Clone the repository
git clone https://github.com/sofatutor/llm-proxy.git
cd llm-proxy

# Install dependencies
make dev-setup

# Build the binary
make build

# The binary is created at ./bin/llm-proxy
```

#### Running

```bash
# Set required environment variables
export MANAGEMENT_TOKEN=$(openssl rand -base64 32)

# Start the server
./bin/llm-proxy server

# Or start with custom options
./bin/llm-proxy server --addr :9000 --db ./custom/path/db.sqlite
```

#### Available Make Commands

| Command | Description |
|---------|-------------|
| `make build` | Build the binary |
| `make test` | Run all tests |
| `make lint` | Run linters |
| `make fmt` | Format code |
| `make docker-build` | Build Docker image locally |
| `make dev-setup` | Install development tools |

### Pre-built Binaries

Pre-built binaries are available from [GitHub Releases](https://github.com/sofatutor/llm-proxy/releases).

#### Download and Install

```bash
# Download the latest release (adjust version and platform)
VERSION=v1.0.0
PLATFORM=linux-amd64

curl -L -o llm-proxy.tar.gz \
  "https://github.com/sofatutor/llm-proxy/releases/download/${VERSION}/llm-proxy-${PLATFORM}.tar.gz"

# Extract
tar -xzf llm-proxy.tar.gz

# Move to a location in PATH
sudo mv llm-proxy /usr/local/bin/

# Verify
llm-proxy --version
```

#### Available Platforms

| Platform | Architecture | File |
|----------|--------------|------|
| Linux | amd64 | `llm-proxy-linux-amd64.tar.gz` |
| Linux | arm64 | `llm-proxy-linux-arm64.tar.gz` |
| macOS | amd64 | `llm-proxy-darwin-amd64.tar.gz` |
| macOS | arm64 (M1/M2) | `llm-proxy-darwin-arm64.tar.gz` |
| Windows | amd64 | `llm-proxy-windows-amd64.zip` |

## Verification Steps

After installation, verify your setup:

### 1. Health Check

```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

### 2. Create a Test Project

```bash
curl -X POST http://localhost:8080/manage/projects \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Test Project", "openai_api_key": "sk-test-key"}'
```

### 3. Generate a Token

```bash
PROJECT_ID=<project-id-from-above>
curl -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer $MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"project_id\": \"$PROJECT_ID\", \"duration_hours\": 24}"
```

### 4. Access the Admin UI

Open http://localhost:8080/admin/ in your browser.

## Platform-Specific Notes

### Linux

- **File Permissions**: Ensure the data directory is writable
  ```bash
  mkdir -p ./data && chmod 755 ./data
  ```
- **Systemd Service**: See [Production Deployment](security.md#container-security) for systemd configuration
- **SELinux**: If using SELinux, configure appropriate contexts for mounted volumes

### macOS

- **Docker Desktop**: Ensure Docker Desktop is running before using Docker commands
- **Apple Silicon (M1/M2)**: Use `darwin-arm64` binaries or Docker images (multi-arch supported)
- **Network Access**: Allow incoming connections when prompted by the firewall

### Windows

- **Docker Desktop**: Use WSL 2 backend for best performance
- **PowerShell**: Use PowerShell for environment variable commands
  ```powershell
  $env:MANAGEMENT_TOKEN = "your-token"
  .\llm-proxy.exe server
  ```
- **Paths**: Use forward slashes in Docker volume paths
  ```powershell
  docker run -v "${PWD}/data:/app/data" ...
  ```

## Troubleshooting Installation

### Docker Issues

**Container won't start:**
```bash
# Check container logs
docker logs llm-proxy

# Common causes:
# - Port 8080 already in use
# - Missing MANAGEMENT_TOKEN environment variable
# - Volume mount permission issues
```

**Permission denied on volumes:**
```bash
# Fix ownership (Linux)
sudo chown -R $(id -u):$(id -g) ./data
```

### Build Issues

**Go version mismatch:**
```bash
# Check Go version
go version
# Required: go1.23 or later

# Update Go from https://go.dev/dl/
```

**Missing dependencies:**
```bash
# Download dependencies
go mod download
go mod tidy
```

### Connection Issues

**Cannot connect to proxy:**
```bash
# Check if proxy is running
curl -v http://localhost:8080/health

# Check if port is in use
lsof -i :8080  # macOS/Linux
netstat -an | findstr :8080  # Windows
```

## Next Steps

After installation:

1. **[Configuration Guide](configuration.md)** - Configure environment variables and settings
2. **[Admin UI Quickstart](admin/quickstart.md)** - Set up projects and tokens via the web interface
3. **[Token Management](token-management.md)** - Understand token lifecycle and management
4. **[Security Best Practices](security.md)** - Secure your deployment

## Related Documentation

- [Configuration Reference](configuration.md)
- [Docker Compose PostgreSQL Setup](docker-compose-postgres.md)
- [CLI Reference](cli-reference.md)
- [Architecture Guide](architecture.md)
