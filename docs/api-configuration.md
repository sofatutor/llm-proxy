# API Configuration Guide

The LLM Proxy uses a configuration-driven approach to define which API providers and endpoints are allowed. This document explains how to configure the API providers and customize the proxy for different AI providers.

## Configuration File

The API configuration is defined in a YAML file, typically located at `./config/api_providers.yaml`. You can specify a different location using the `API_CONFIG_PATH` environment variable.

### Basic Structure

The configuration file has the following structure:

```yaml
# Default API provider to use if not specified
default_api: openai

# Configuration for each API provider
apis:
  provider1:
    base_url: https://api.provider1.com
    allowed_endpoints:
      - /v1/endpoint1
      - /v1/endpoint2
    allowed_methods:
      - GET
      - POST
    timeouts:
      request: 60s
      response_header: 30s
      idle_connection: 90s
      flush_interval: 100ms
    connection:
      max_idle_conns: 100
      max_idle_conns_per_host: 20
  
  provider2:
    # ... similar configuration
```

### Configuration Fields

#### Top-Level Fields

- `default_api`: The default API provider to use if not specified in requests
- `apis`: A map of API provider configurations

#### API Provider Configuration

Each API provider has the following configuration options:

- `base_url`: The base URL of the API provider (required)
- `allowed_endpoints`: A list of endpoint paths that are allowed to be accessed (required)
- `allowed_methods`: A list of HTTP methods that are allowed (required)
- `timeouts`: Timeout settings for various operations
  - `request`: Overall request timeout
  - `response_header`: Timeout for receiving response headers
  - `idle_connection`: How long to keep idle connections alive
  - `flush_interval`: How often to flush streaming responses
- `connection`: Connection pool settings
  - `max_idle_conns`: Maximum number of idle connections
  - `max_idle_conns_per_host`: Maximum number of idle connections per host
- `param_whitelist`: (optional) Restrict allowed values for specific request parameters (e.g., model). Supports glob patterns (e.g., `gpt-4.1-*`).
- `allowed_origins`: (optional) Restrict allowed CORS origins for API requests. Only requests from these origins will be accepted.
- `required_headers`: (optional) Require specific headers (e.g., `Origin`) for requests to be accepted.

##### Example with Advanced Options

```yaml
apis:
  openai:
    base_url: https://api.openai.com
    allowed_endpoints:
      - /v1/chat/completions
      - /v1/completions
    allowed_methods:
      - POST
    param_whitelist:
      model:
        - gpt-4o
        - gpt-4.1-*
    allowed_origins:
      - https://www.sofatutor.com
      - http://localhost:4000
    required_headers:
      - origin
    timeouts:
      request: 60s
      response_header: 30s
      idle_connection: 90s
      flush_interval: 100ms
    connection:
      max_idle_conns: 100
      max_idle_conns_per_host: 20
```

**param_whitelist**: Use this to restrict which models or other parameters can be used in requests. If a request specifies a value not in the whitelist, it will be rejected with a 400 error.

**allowed_origins**: Use this to enforce CORS policies. Only requests from these origins will be accepted. If not set, all origins are allowed by default.

**required_headers**: Use this to require headers like `Origin` for all requests. If a required header is missing, the request will be rejected with a 400 error.
- **If `origin` is listed in `required_headers`, the proxy will also check `allowed_origins` and block requests with an Origin header not in the allowed list.**

## Security Considerations

The allowlist-based configuration provides several security benefits:

1. **Restricted Access**: Only explicitly allowed endpoints and methods can be accessed, reducing the attack surface.
2. **API Isolation**: Each API provider has its own separate configuration.
3. **Transparent Validation**: All requests are validated against the allowlist before being proxied.

## Adding a New API Provider

To add a new API provider, follow these steps:

1. Add a new entry to the `apis` map in the configuration file
2. Define the `base_url` for the provider
3. List all `allowed_endpoints` that should be accessible
4. Define the `allowed_methods` for those endpoints
5. Configure appropriate timeouts and connection settings

## Environment Variables

The proxy uses the following environment variables related to API configuration:

- `API_CONFIG_PATH`: Path to the API providers configuration file (default: `./config/api_providers.yaml`)
- `DEFAULT_API_PROVIDER`: Default API provider to use (overrides the `default_api` in the config file)
- `OPENAI_API_URL`: Base URL for OpenAI API (legacy support, default: `https://api.openai.com`)

### HTTP Caching Configuration

The proxy supports HTTP response caching with the following environment variables:

- `HTTP_CACHE_ENABLED`: Enable or disable HTTP response caching (default: `true`)
- `HTTP_CACHE_BACKEND`: Cache backend to use, either `redis` or `in-memory` (default: `in-memory`)
- `REDIS_CACHE_URL`: Redis connection URL for cache storage (default: `redis://localhost:6379/0` when backend is `redis`)
- `REDIS_CACHE_KEY_PREFIX`: Prefix for Redis cache keys (default: `llmproxy:cache:`)
- `HTTP_CACHE_MAX_OBJECT_BYTES`: Maximum size in bytes for cached objects (default: `1048576` - 1MB)
- `HTTP_CACHE_DEFAULT_TTL`: Default TTL in seconds when upstream response doesn't specify caching directives (default: `300` - 5 minutes)

#### Cache Behavior

The caching system follows HTTP standards:

- **GET/HEAD requests**: Cached by default when upstream permits
- **POST requests**: Only cached when client explicitly opts in via request `Cache-Control` header
- **Authentication**: Cached responses for authenticated requests are only served if marked as publicly cacheable (`Cache-Control: public` or `s-maxage` present)
- **Streaming responses**: Captured during streaming and stored after completion
- **TTL precedence**: `s-maxage` (shared cache) takes precedence over `max-age`
- **Headers**: Responses include `X-PROXY-CACHE`, `X-PROXY-CACHE-KEY`, and `Cache-Status` for observability

## Example Configuration

See [api_providers_example.yaml](../config/api_providers_example.yaml) for a comprehensive example configuration with multiple API providers.

## Fallback Behavior

If the configuration file cannot be loaded or contains errors, the proxy will fall back to a default OpenAI configuration with common endpoints. This ensures that the proxy can still function even without a valid configuration file.

## Endpoint Matching

Endpoints are matched by prefix. For example, if `/v1/chat/completions` is in the allowed endpoints, then both `/v1/chat/completions` and `/v1/chat/completions?temperature=0.7` will match.

## Method Validation

HTTP methods (GET, POST, DELETE, etc.) are validated against the `allowed_methods` list. If a request uses a method that is not in the list, it will be rejected with a 405 Method Not Allowed error.

## Testing Your Configuration

You can test your configuration by sending requests to the proxy endpoints and verifying that only allowed endpoints and methods are accepted. Disallowed endpoints will return a 404 Not Found error, and disallowed methods will return a 405 Method Not Allowed error.