# CLI Reference

This document is a snapshot of the LLM Proxy CLI capabilities and flags intended for GitHub Pages consumption. Use it as a quick reference while exploring or contributing. For examples and full workflows, see the sections below.

## Table of Contents

- [Overview](#overview)
- [Commands](#commands)
  - [`llm-proxy server`](#llm-proxy-server)
  - [`llm-proxy setup`](#llm-proxy-setup)
  - [`llm-proxy manage`](#llm-proxy-manage)
  - [`llm-proxy dispatcher`](#llm-proxy-dispatcher)
  - [`llm-proxy benchmark`](#llm-proxy-benchmark)
  - [`llm-proxy openai chat`](#llm-proxy-openai-chat)
- [Configuration Files](#configuration-files)
- [Exit Codes](#exit-codes)
- [Tips and Best Practices](#tips-and-best-practices)
- [Examples and Workflows](#examples-and-workflows)

This document provides a comprehensive reference for all LLM Proxy command-line interface (CLI) commands.

## Overview

The LLM Proxy CLI provides commands for:
- Starting the proxy server
- Setting up the initial configuration
- Managing projects and tokens
- Running the event dispatcher
- Benchmarking proxy performance and testing cache behavior
- Interactive chat with OpenAI

## Global Options

Most commands support these global flags:
- `--help, -h`: Show help for the command
- `--version`: Show version information

## Commands

### `llm-proxy server`

Starts the LLM Proxy HTTP server.

**Usage:**
```bash
llm-proxy server [flags]
```

**Flags:**
- `--addr string`: Address to listen on (default: ":8080", overridden by LISTEN_ADDR)
- `--db string`: Path to SQLite database (default: "data/llm-proxy.db", overridden by DATABASE_PATH)
- `--config string`: Path to configuration file (default: ".env")

**Environment Variables:**
All server configuration can be set via environment variables. See [Configuration Guide](api-configuration.md) for complete list.

**Examples:**
```bash
# Start with default settings
MANAGEMENT_TOKEN=your-token llm-proxy server

# Start on custom port with custom database
llm-proxy server --addr :9000 --db /path/to/db.sqlite

# Start with custom config file
llm-proxy server --config /path/to/.env
```

---

### `llm-proxy setup`

Interactive or non-interactive setup for configuring the proxy.

**Usage:**
```bash
llm-proxy setup [flags]
```

**Flags:**
- `--config string`: Path to configuration file (default: ".env")
- `--interactive`: Run interactive setup
- `--openai-key string`: OpenAI API Key
- `--management-token string`: Management token for the proxy
- `--project string`: Name of the project to create (default: "DefaultProject")
- `--duration int`: Duration of the token in hours (default: 24)
- `--skip-project`: Skip project and token setup
- `--db string`: Path to SQLite database (default: "data/llm-proxy.db")
- `--addr string`: Address to listen on (default: "localhost:8080")

**Examples:**
```bash
# Interactive setup (recommended for first-time setup)
llm-proxy setup --interactive

# Non-interactive setup with all parameters
llm-proxy setup \
  --openai-key sk-your-openai-key \
  --management-token your-secure-token \
  --project "My Project" \
  --duration 168 \
  --config /path/to/.env

# Setup without creating a project
llm-proxy setup --openai-key sk-key --management-token token --skip-project
```

---

### `llm-proxy manage`

Management commands for projects and tokens.

**Usage:**
```bash
llm-proxy manage [command] [subcommand] [flags]
```

**Global Management Flags:**
- `--manage-api-base-url string`: Management API base URL (default: "http://localhost:8080")
- `--management-token string`: Management token (or set MANAGEMENT_TOKEN env variable)
- `--json`: Output results as JSON

#### Project Management

##### `llm-proxy manage project list`

List all projects in the system.

**Usage:**
```bash
llm-proxy manage project list [flags]
```

**Flags:**
- `--json`: Output as JSON
- `--management-token string`: Management token

**Examples:**
```bash
# List projects in table format
llm-proxy manage project list --management-token your-token

# List projects as JSON
llm-proxy manage project list --management-token your-token --json

# Use environment variable for token
export MANAGEMENT_TOKEN=your-token
llm-proxy manage project list
```

##### `llm-proxy manage project get`

Get details for a specific project.

**Usage:**
```bash
llm-proxy manage project get <project-id> [flags]
```

**Arguments:**
- `project-id`: UUID of the project to retrieve

**Examples:**
```bash
llm-proxy manage project get 123e4567-e89b-12d3-a456-426614174000 --management-token your-token
```

##### `llm-proxy manage project create`

Create a new project.

**Usage:**
```bash
llm-proxy manage project create [flags]
```

**Flags:**
- `--name string`: Project name (required)
- `--openai-key string`: OpenAI API key (required)

**Examples:**
```bash
llm-proxy manage project create \
  --name "My AI Project" \
  --openai-key sk-your-openai-api-key \
  --management-token your-token
```

##### `llm-proxy manage project update`

Update an existing project.

**Usage:**
```bash
llm-proxy manage project update <project-id> [flags]
```

**Arguments:**
- `project-id`: UUID of the project to update

**Flags:**
- `--name string`: New project name
- `--openai-key string`: New OpenAI API key

**Examples:**
```bash
# Update project name only
llm-proxy manage project update 123e4567-e89b-12d3-a456-426614174000 \
  --name "Updated Project Name" \
  --management-token your-token

# Update OpenAI key only
llm-proxy manage project update 123e4567-e89b-12d3-a456-426614174000 \
  --openai-key sk-new-api-key \
  --management-token your-token

# Update both name and key
llm-proxy manage project update 123e4567-e89b-12d3-a456-426614174000 \
  --name "New Name" \
  --openai-key sk-new-key \
  --management-token your-token
```

##### `llm-proxy manage project delete`

Delete a project and all associated tokens.

**Usage:**
```bash
llm-proxy manage project delete <project-id> [flags]
```

**Arguments:**
- `project-id`: UUID of the project to delete

**Examples:**
```bash
llm-proxy manage project delete 123e4567-e89b-12d3-a456-426614174000 --management-token your-token
```

#### Token Management

##### `llm-proxy manage token list`

List all tokens in the system.

**Usage:**
```bash
llm-proxy manage token list [flags]
```

**Flags:**
- `--project-id string`: Filter tokens by project ID
- `--active-only`: Show only active tokens
- `--json`: Output as JSON

**Examples:**
```bash
# List all tokens
llm-proxy manage token list --management-token your-token

# List tokens for specific project
llm-proxy manage token list --project-id 123e4567-e89b-12d3-a456-426614174000 --management-token your-token

# List only active tokens
llm-proxy manage token list --active-only --management-token your-token
```

##### `llm-proxy manage token get`

Get details for a specific token.

**Usage:**
```bash
llm-proxy manage token get <token-id> [flags]
```

**Arguments:**
- `token-id`: UUID of the token to retrieve

**Examples:**
```bash
llm-proxy manage token get sk-ABC123DEF456GHI789 --management-token your-token
```

##### `llm-proxy manage token generate`

Generate a new token for a project.

**Usage:**
```bash
llm-proxy manage token generate [flags]
```

**Flags:**
- `--project-id string`: Project ID to create token for (required)
- `--duration int`: Token duration in hours (default: 24)
- `--max-requests int`: Maximum number of requests (0 = unlimited, default: 0)

**Examples:**
```bash
# Generate token with default 24-hour expiration
llm-proxy manage token generate \
  --project-id 123e4567-e89b-12d3-a456-426614174000 \
  --management-token your-token

# Generate token with custom duration and request limit
llm-proxy manage token generate \
  --project-id 123e4567-e89b-12d3-a456-426614174000 \
  --duration 168 \
  --max-requests 1000 \
  --management-token your-token
```

##### `llm-proxy manage token revoke`

Revoke (deactivate) a token.

**Usage:**
```bash
llm-proxy manage token revoke <token-id> [flags]
```

**Arguments:**
- `token-id`: Token to revoke

**Examples:**
```bash
llm-proxy manage token revoke sk-ABC123DEF456GHI789 --management-token your-token
```

---

### `llm-proxy dispatcher`

Event dispatcher service for handling observability events.

**Usage:**
```bash
llm-proxy dispatcher [flags]
```

**Flags:**
- `--service string`: Dispatcher service type (currently supports "file")
- `--endpoint string`: Endpoint configuration (file path for file service)
- `--buffer int`: Event bus buffer size (default: 100)

**Examples:**
```bash
# Start file dispatcher
llm-proxy dispatcher --service file --endpoint ./events.jsonl

# Start with custom buffer size
llm-proxy dispatcher --service file --endpoint ./events.jsonl --buffer 1000
```

---

### `llm-proxy benchmark`

Benchmark latency, throughput, and error rates by sending concurrent requests to the proxy or target API directly. Includes cache testing capabilities to validate cache hit/miss behavior.

**Usage:**
```bash
llm-proxy benchmark [flags]
```

**Required Flags:**
- `--base-url string`: Base URL of the target (e.g., `http://localhost:8080` or `https://api.openai.com/v1`)
- `--endpoint string`: API path to hit (e.g., `/v1/chat/completions` or `/chat/completions` for OpenAI)
- `--token string`: Bearer token (proxy token or OpenAI API key)
- `--requests, -r int`: Total number of requests to send
- `--concurrency, -c int`: Number of concurrent workers

**Optional Flags:**
- `--json string`: JSON request body for POST requests
- `--method string`: HTTP method to use (GET, POST, PUT, PATCH) (default: "POST")
- `--cache`: Set `Cache-Control: public` with high TTL for benchmarking cache behavior
- `--cache-ttl int`: TTL seconds to use with `--cache` (default: 86400)
- `--debug`: Print sample responses and headers by status code

**Cache Testing Examples:**
```bash
# Test cache warming with POST requests
llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/chat/completions" \
  --token "$PROXY_TOKEN" \
  --requests 10 --concurrency 1 \
  --cache --cache-ttl 3600 \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"test cache"}]}'

# Test cache hits with GET requests
llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/models" \
  --token "$PROXY_TOKEN" \
  --method GET \
  --requests 20 --concurrency 5 \
  --debug

# Compare cache performance - first populate cache, then test hits
llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/chat/completions" \
  --token "$PROXY_TOKEN" \
  --requests 1 --concurrency 1 \
  --cache --cache-ttl 300 \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"warmup"}]}'

llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/chat/completions" \
  --token "$PROXY_TOKEN" \
  --requests 50 --concurrency 10 \
  --debug \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"warmup"}]}'
```

**Standard Benchmarking Examples:**
```bash
# Proxy performance test
llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/chat/completions" \
  --token "$PROXY_TOKEN" \
  --requests 100 --concurrency 4 \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'

# Direct OpenAI comparison
llm-proxy benchmark \
  --base-url "https://api.openai.com/v1" \
  --endpoint "/chat/completions" \
  --token "$OPENAI_API_KEY" \
  --requests 100 --concurrency 4 \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'

# Debug mode to inspect cache headers
llm-proxy benchmark \
  --base-url "http://localhost:8080" \
  --endpoint "/v1/chat/completions" \
  --token "$PROXY_TOKEN" \
  --requests 20 --concurrency 5 \
  --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}' \
  --debug
```

**Latency Breakdown:**
The benchmark tool provides detailed timing information:
- Request includes `X-REQUEST-START` (ns)
- Proxy returns `X-UPSTREAM-REQUEST-START` and `X-UPSTREAM-REQUEST-STOP` (ns)
- This allows precise separation of upstream vs proxy latency

**Cache Headers:**
When testing cache behavior, look for these response headers:
- `X-PROXY-CACHE`: `hit` or `miss`
- `X-PROXY-CACHE-KEY`: Cache key used for the request
- `Cache-Status`: `hit`, `miss`, `bypass`, `stored`, or `conditional-hit`

---

### `llm-proxy openai`

Commands for interacting with OpenAI API through the proxy.

#### `llm-proxy openai chat`

Interactive chat interface with OpenAI API.

**Usage:**
```bash
llm-proxy openai chat [flags]
```

**Flags:**
- `--token string`: Withering token for proxy authentication
- `--base-url string`: Base URL of the proxy (default: "http://localhost:8080")
- `--model string`: OpenAI model to use (default: "gpt-4")

**Examples:**
```bash
# Start interactive chat
llm-proxy openai chat --token sk-your-withering-token

# Use specific model
llm-proxy openai chat --token sk-your-withering-token --model gpt-4o

# Connect to proxy on different URL
llm-proxy openai chat --token sk-your-withering-token --base-url https://proxy.example.com
```

---

## Configuration Files

### `.env` Configuration File

The setup command creates a `.env` file with the following format:

```bash
OPENAI_API_KEY=sk-your-openai-api-key
MANAGEMENT_TOKEN=your-management-token
DATABASE_PATH=./data/llm-proxy.db
LISTEN_ADDR=:8080
```

### API Configuration File

See [API Configuration Guide](api-configuration.md) for details on configuring `api_providers.yaml`.

## Exit Codes

- `0`: Success
- `1`: General error (invalid arguments, configuration error, etc.)
- `2`: Network error (unable to connect to API)
- `3`: Authentication error (invalid token)

## Tips and Best Practices

### Security
- Never include tokens or API keys in command history
- Use environment variables for sensitive values
- Rotate management tokens regularly

### Performance
- Use `--json` output for programmatic processing
- Set appropriate buffer sizes for the dispatcher based on event volume

### Troubleshooting
- Use `--help` with any command for detailed usage information
- Check server logs when management commands fail
- Verify network connectivity to the management API

## Examples and Workflows

### Initial Setup Workflow
```bash
# 1. Interactive setup
llm-proxy setup --interactive

# 2. Start the server
llm-proxy server

# 3. Create a project (in another terminal)
llm-proxy manage project create --name "My Project" --openai-key sk-... --management-token your-token

# 4. Generate a token
llm-proxy manage token generate --project-id <project-id> --management-token your-token

# 5. Test with chat
llm-proxy openai chat --token <generated-token>
```

### Production Deployment Workflow
```bash
# 1. Non-interactive setup
llm-proxy setup --openai-key sk-... --management-token secure-token --skip-project

# 2. Start server with file event logging
FILE_EVENT_LOG=./events.jsonl llm-proxy server

# 3. Create projects and tokens via API or CLI as needed
```

### Token Management Workflow
```bash
# List all tokens and their status
llm-proxy manage token list --management-token your-token

# Check specific token details
llm-proxy manage token get sk-token-id --management-token your-token

# Revoke compromised token
llm-proxy manage token revoke sk-token-id --management-token your-token

# Generate replacement token
llm-proxy manage token generate --project-id <project-id> --management-token your-token
```