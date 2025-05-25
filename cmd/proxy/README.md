# LLM Proxy CLI Tool

This CLI tool provides a command-line interface for interacting with the LLM Proxy, including setup, OpenAI chat, and benchmarking capabilities.

## Installation

To build the CLI tool:

```bash
go build -o llm-proxy ./cmd/proxy
```

## Commands

### Setup

Configure the LLM Proxy with your API keys and settings:

```bash
# Interactive setup
llm-proxy setup --interactive

# Non-interactive setup
llm-proxy setup --openai-key YOUR_OPENAI_API_KEY
```

Options:
- `--config`: Path to the configuration file (default: `.env`)
- `--openai-key`: OpenAI API Key
- `--management-token`: Management token for the proxy (generated if not provided)
- `--db`: Path to SQLite database (default: `data/proxy.db`)
- `--addr`: Address to listen on (default: `localhost:8080`)
- `--interactive`: Run interactive setup
- `--project`: Name of the project to create (default: `DefaultProject`)
- `--duration`: Duration of the token in hours (default: `24`)
- `--skip-project`: Skip project and token setup

### OpenAI Chat

Start an interactive chat session with OpenAI models:

```bash
llm-proxy openai chat --token YOUR_PROXY_TOKEN
```

Options:
- `--proxy`: LLM Proxy URL (default: `http://localhost:8080`)
- `--token`: LLM Proxy token (required)
- `--model`: Model to use (default: `gpt-4.1-mini`)
- `--temperature`: Temperature for generation (default: `0.7`)
- `--max-tokens`: Maximum tokens to generate (default: `0` - no limit)
- `--system`: System prompt (default: `You are a helpful assistant.`)
- `-v, --verbose`: Show detailed timing information, including proxy overhead
- `--stream`: Use streaming for responses (default: `true`, set to `false` to disable)

### Server

Start the LLM Proxy server:

```bash
# Start server in foreground
llm-proxy server

# Start server in daemon mode (background)
llm-proxy server -d
```

Options:
- `-d, --daemon`: Run server in daemon mode (background)
- `--env`: Path to .env file (default: `.env`)
- `--addr`: Address to listen on (overrides env var)
- `--db`: Path to SQLite database (overrides env var)
- `--log-level`: Log level: debug, info, warn, error (overrides env var)
- `--pid-file`: PID file for daemon mode (default: `/tmp/llm-proxy.pid`)

When running in daemon mode, the server will detach from the terminal and run in the background. The PID will be saved to the specified PID file, which can be used to stop the server later:

```bash
# Stop the daemon
kill $(cat /tmp/llm-proxy.pid)
```

### Benchmark

Run benchmarks against the LLM Proxy:

```bash
llm-proxy benchmark
```

Note: The benchmark command is a placeholder for future implementation.

## Examples

### Basic Setup

```bash
# Interactive setup with project and token creation
llm-proxy setup --interactive

# Non-interactive setup with custom configuration and automatic project/token creation
llm-proxy setup --openai-key sk-your-api-key --db ./mydata/proxy.db --addr 0.0.0.0:9000 --project "ProductionProject" --duration 72

# Setup with existing configuration values (will preserve existing settings)
llm-proxy setup --interactive
# Just press Enter to keep existing values

# Setup without project/token creation
llm-proxy setup --openai-key sk-your-api-key --skip-project
```

### Server Examples

```bash
# Start server with default configuration
llm-proxy server

# Start server with custom configuration
llm-proxy server --addr 0.0.0.0:9000 --db ./mydata/proxy.db --log-level debug

# Start server as a daemon
llm-proxy server -d --pid-file /var/run/llm-proxy.pid
```

### Chat Example

```bash
# Use the automatically generated token from setup
source proxy-token.env
llm-proxy openai chat --token $PROXY_TOKEN

# Basic chat with directly specified token
llm-proxy openai chat --token your-proxy-token

# Chat with customized parameters
llm-proxy openai chat --token your-proxy-token --model gpt-4 --temperature 0.9 --system "You are a helpful coding assistant."

# Show timing information with verbose mode
llm-proxy openai chat --token $PROXY_TOKEN --verbose

# Disable streaming (not recommended for interactive use)
llm-proxy openai chat --token $PROXY_TOKEN --stream=false
```

## End-to-End Testing Instructions

These instructions guide you through the complete proxy workflow, from initial setup to using the OpenAI chat interface. This end-to-end process demonstrates how the different components of the LLM Proxy work together:

1. Setting up the proxy environment with your OpenAI API key
2. Starting the proxy server (foreground or daemon mode)
3. Creating a project and generating a proxy token
4. Using the OpenAI chat with the proxy token
5. Testing the API directly with curl
6. Stopping the proxy server

Follow these steps in order to test the entire workflow:

### 1. Setup the Proxy Environment

```bash
# Build the CLI tool
go build -o llm-proxy ./cmd/proxy

# Setup with your OpenAI API key (interactive mode)
./llm-proxy setup --interactive
# Enter your OpenAI API key when prompted
# The setup will generate a secure management token for you
# You'll also be guided through creating a project and token automatically

# Alternatively, use non-interactive mode with automatic project and token creation
./llm-proxy setup --openai-key YOUR_OPENAI_API_KEY --project "MyProject" --duration 48

# Skip project and token creation if you want to do it manually later
./llm-proxy setup --openai-key YOUR_OPENAI_API_KEY --skip-project
```

The setup process will:
1. Create a configuration file (default: `.env`)
2. Generate a secure random management token if not provided
3. Start the server temporarily in the background
4. Create a project with your OpenAI API key
5. Generate a token for that project
6. Save the token to `proxy-token.env` for easy access
7. Stop the temporary server

### 2. Start the Proxy Server

```bash
# Start in foreground mode
./llm-proxy server

# Or, start as a daemon
./llm-proxy server -d
# The server will print the PID file location
```

The server should now be running on http://localhost:8080 (or the address you configured).

### 3. Use the OpenAI Chat with the Proxy Token Generated During Setup

The setup process automatically creates a project and token, which are saved to `proxy-token.env`. You can use this token directly:

```bash
# Source the token file to set the PROXY_TOKEN environment variable
source proxy-token.env

# Use the token with the chat command
./llm-proxy openai chat --token $PROXY_TOKEN
```

Alternatively, if you need to manually create a project and token (if you used `--skip-project`):

```bash
# Using curl to create a project (replace MANAGEMENT_TOKEN with your token)
curl -X POST http://localhost:8080/manage/projects \
  -H "Authorization: Bearer MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "TestProject", "openai_api_key": "YOUR_OPENAI_API_KEY"}'
# Note the project_id from the response

# Generate a token for the project
curl -X POST http://localhost:8080/manage/tokens \
  -H "Authorization: Bearer MANAGEMENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"project_id": "PROJECT_ID", "duration_minutes": 1440}'
# Note the token from the response
```

### 4. Chat with OpenAI Models

```bash
# Start a chat session using the proxy token
./llm-proxy openai chat --token PROXY_TOKEN --model gpt-4.1-mini

# For a more customized chat experience
./llm-proxy openai chat --token PROXY_TOKEN \
  --model gpt-4 \
  --temperature 0.8 \
  --system "You are a helpful assistant specializing in coding."
```

### 5. Testing the Proxy API Directly

You can also test the proxy by sending requests directly to the API:

```bash
# Test a chat completion request
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer PROXY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4.1-mini",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Hello, how are you?"}
    ]
  }'
```

### 6. Stop the Proxy Server

```bash
# If running in foreground mode, use Ctrl+C

# If running as a daemon
kill $(cat /tmp/llm-proxy.pid)
# Or use the custom PID file location if specified
```

## Development

To run tests:

```bash
go test ./cmd/proxy
```

## Notes

- This CLI tool is a verification tool for the LLM Proxy implementation.
- The OpenAI chat command currently simulates responses rather than making actual API calls.
- Feature implementations will be expanded in future versions.
- For production use, ensure you have properly configured your environment variables
  and security settings.

## Benchmark Tool

To run the benchmark tool, use the CLI command:

```bash
llm-proxy benchmark --base-url ... --endpoint ... --token ... --requests ... --concurrency ...
```