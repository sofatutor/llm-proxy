# LLM Proxy

A transparent proxy for OpenAI's API that enables secure token management and usage tracking.

## Features

- **API Compatibility**: Fully compatible with OpenAI's API
- **Token Management**: Create and manage access tokens with fine-grained control
- **Rate Limiting**: Control usage with configurable rate limits per token
- **Usage Tracking**: Monitor API usage across projects and tokens
- **Security**: Secure your OpenAI API keys behind the proxy
- **Admin UI**: Web-based interface for easy management
- **Streaming Support**: Full support for streaming API responses
- **SQLite Database**: Simple, file-based storage without external dependencies
- **Logging**: Comprehensive structured logging
- **Monitoring**: Prometheus metrics for operational visibility

## Quick Start

### Installation

#### Using Docker (Recommended)

```bash
# Pull the image
docker pull sofatutor/llm-proxy:latest

# Create a configuration directory
mkdir -p ./llm-proxy/data

# Create and start the container
docker run -d \
  --name llm-proxy \
  -p 8080:8080 \
  -v ./llm-proxy/data:/app/data \
  -e MANAGEMENT_TOKEN=your-secure-management-token \
  sofatutor/llm-proxy:latest
```

#### From Source

```bash
# Clone the repository
git clone https://github.com/sofatutor/llm-proxy.git
cd llm-proxy

# Build the binaries
make build

# Set up the database
make db-setup

# Run the proxy
MANAGEMENT_TOKEN=your-secure-management-token ./bin/llm-proxy
```

### Configuration

LLM Proxy can be configured using environment variables or a configuration file:

```bash
# Core settings
MANAGEMENT_TOKEN=your-secure-management-token  # Required for admin operations
LISTEN_ADDR=:8080                              # Default is :8080
DATABASE_PATH=./data/llm-proxy.db              # SQLite database path
LOG_LEVEL=info                                 # Log level (debug, info, warn, error)

# Advanced settings
REQUEST_TIMEOUT=30s                            # Timeout for upstream API requests
MAX_REQUEST_SIZE=10MB                          # Maximum size of incoming requests
ADMIN_UI_ENABLED=true                          # Enable/disable the admin UI
```

See [Configuration Documentation](docs/configuration.md) for complete details.

### Basic Usage

1. **Create a Project**

```bash
curl -X POST http://localhost:8080/manage/projects \
  -H "Authorization: Bearer your-management-token" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Project",
    "openai_api_key": "sk-your-openai-api-key"
  }'
```

2. **Generate a Token**

```bash
curl -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer your-management-token" \
  -H "Content-Type: application/json" \
  -d '{
    "project_id": "your-project-id",
    "expires_at": "2023-12-31T23:59:59Z",
    "max_requests": 1000
  }'
```

3. **Use the Proxy**

```bash
curl https://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer your-generated-token" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello, world!"}]
  }'
```

## Architecture

LLM Proxy consists of the following main components:

- **Proxy Server**: Handles OpenAI API requests, validation, and forwarding
- **Token Manager**: Manages creation, validation, and tracking of access tokens
- **Database Layer**: Stores project and token information
- **Admin Interface**: Web UI for managing projects and tokens
- **Logging System**: Structured logging with configurable outputs
- **Monitoring**: Runtime metrics for observability

For more details, see the [Architecture Documentation](docs/architecture.md).

## Development

### Prerequisites

- Go 1.23 or later
- SQLite 3
- Git

### Development Environment Setup

1. **Clone the repository**

```bash
git clone https://github.com/sofatutor/llm-proxy.git
cd llm-proxy
```

2. **Set up the Go environment**

The project uses a Makefile to simplify development tasks:

```bash
# Install required Go tools
make tools

# Download dependencies and tidy the go.mod file
make dev-setup

# Set up the SQLite database
make db-setup
```

3. **Using VSCode with Dev Containers (Optional)**

If you're using VSCode with the Dev Containers extension, you can open the project in a pre-configured development container:

1. Install the [Remote Development extension pack](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.vscode-remote-extensionpack)
2. Open the project folder in VSCode
3. When prompted, click "Reopen in Container"
4. The container will be built and configured with all necessary dependencies

### Common Development Tasks

```bash
# Build the project
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# View coverage report in browser
make test-coverage-html

# Run linting
make lint

# Format code
make fmt

# Build and run the proxy
make run

# Clean up build artifacts
make clean

# Generate API documentation
make swag

# Build Docker image
make docker
```

### Project Structure

- `/cmd` - Application entry points
  - `/proxy` - Main proxy server
  - `/benchmark` - Performance benchmarking tool
- `/internal` - Internal packages
  - `/database` - Database operations and models
  - `/token` - Token management and validation
  - `/proxy` - Proxy logic and OpenAI API client
  - `/admin` - Admin UI handlers
  - `/logging` - Logging system
- `/api` - OpenAPI specs and shared API types
- `/web` - Static assets for Admin UI
- `/config` - Configuration templates
- `/scripts` - Build and deployment scripts
- `/docs` - Documentation and architecture diagrams
- `/test` - Integration/E2E tests and fixtures

## Benchmarking

LLM Proxy includes a benchmarking tool to evaluate performance:

```bash
# Run benchmark with default settings
./bin/llm-benchmark -url http://localhost:8080

# Customize benchmark parameters
./bin/llm-benchmark -url http://localhost:8080 -requests 1000 -concurrent 10
```

For more options, see [Benchmarking Documentation](docs/benchmarking.md).

## Deployment

### Docker Compose

```yaml
version: '3'
services:
  llm-proxy:
    image: sofatutor/llm-proxy:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - MANAGEMENT_TOKEN=your-secure-management-token
      - LOG_LEVEL=info
    restart: unless-stopped
```

### Kubernetes

For Kubernetes deployment examples, see [Kubernetes Configuration](docs/kubernetes.md).

## Contributing

Contributions are welcome! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

This project follows strict test-driven development practices:
- All new code must be covered by tests
- Minimum 90% code coverage is enforced
- Pull requests must include tests for new functionality

## Security

For details on securing your LLM Proxy deployment, see [Security Documentation](docs/security.md).

To report security vulnerabilities, please email [security@example.com](mailto:security@example.com).

## License

[MIT License](LICENSE)