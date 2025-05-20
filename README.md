# LLM Proxy

A transparent proxy for OpenAI's API that enables secure token management and usage tracking.

## Features

- Proxy for OpenAI API with transparent forwarding
- Token-based authentication system with expiration
- Request tracking and rate limiting
- Web-based admin interface
- SQLite database for storage

## Development

### Prerequisites

- Go 1.21 or later
- SQLite 3
- Git

### Development Environment Setup

1. **Clone the repository**

```bash
git clone https://github.com/manuelfittko/llm-proxy.git
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

### Common Tasks

```bash
# Build the project
make build

# Run tests
make test

# Run linting
make lint

# Build and run the proxy
make run

# Clean up build artifacts
make clean

# Build Docker image
make docker
```

## License

[MIT License](LICENSE) 