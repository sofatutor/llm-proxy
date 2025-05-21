# Transparent Proxy Design Decisions

This document explains the key design decisions for the transparent LLM proxy architecture.

## Key Design Principles

1. **Maximum Transparency** 
   - Proxy modifies only what's absolutely necessary (authorization header)
   - All other request/response data passes through unchanged
   - No API-specific client/SDK dependencies

2. **Universal Compatibility**
   - Works with any API, not just OpenAI
   - Configurable target URL and endpoint allowlist
   - Generic handling of all HTTP methods and content types

3. **High Performance**
   - Optimized for minimum latency overhead
   - Connection pooling for HTTP clients
   - Efficient buffer management
   - Stream processing optimized for SSE

4. **Strong Security**
   - Token validation and expiration
   - Rate limiting
   - Whitelist approach for allowed endpoints
   - Protection of API keys

## Architecture Design

### httputil.ReverseProxy as Foundation

We chose Go's built-in `httputil.ReverseProxy` as the foundation for our proxy for several reasons:

1. **Production-Tested Code**: Part of Go's standard library and widely used in production
2. **Full HTTP Support**: Handles all HTTP methods, headers, status codes
3. **Streaming Support**: Properly handles chunked transfer encoding and streaming responses
4. **Customization Points**: The Director, ModifyResponse, and ErrorHandler functions provide optimal customization points

### Middleware-Based Processing

Rather than building an API client, we implemented a middleware chain approach:

1. **Separation of Concerns**: Each middleware handles a specific aspect (logging, validation, etc.)
2. **Testability**: Each middleware can be tested independently
3. **Flexibility**: Middlewares can be added/removed based on configuration
4. **Consistent Error Handling**: Common error format across all processing stages

### Design Patterns Used

1. **Chain of Responsibility**: Middleware chain processes requests in sequence
2. **Decorator**: Each middleware decorates the HTTP handler with additional functionality
3. **Strategy**: Different validation and authentication strategies can be plugged in
4. **Adapter**: Converts external token validation to HTTP middleware

## Key Components

### TransparentProxy

The core proxy component that orchestrates request/response handling:

- Configures and initializes the `httputil.ReverseProxy`
- Registers middleware chain
- Handles token validation and API key substitution
- Manages connection pooling and timeout settings

### Middleware Stack

Middleware functions that process requests before they reach the proxy:

1. **LoggingMiddleware**: Logs request details with timing information
2. **ValidateRequestMiddleware**: Ensures requests target allowed endpoints with allowed methods
3. **TimeoutMiddleware**: Adds a context timeout to limit request duration
4. **MetricsMiddleware**: Collects performance metrics on requests

### Request Processing Flow

1. **Middleware Chain**: Request passes through middleware for logging, validation, etc.
2. **Director Function**: Updates request URL, performs token validation, replaces authorization header
3. **HTTP Transport**: Forwards request to target API
4. **ModifyResponse Function**: Processes response (adds headers, extracts metadata)
5. **Response Return**: Returns response to client

## Streaming Support

Special consideration was given to properly support Server-Sent Events (SSE):

1. **FlushInterval Configuration**: Short interval ensures chunks are sent promptly
2. **No Buffering**: Streaming responses bypass in-memory buffers
3. **Content-Type Detection**: Automatically detects text/event-stream
4. **Transfer-Encoding Support**: Properly handles chunked transfer encoding

## Error Handling

Centralized error handling with consistent response format:

1. **Validation Errors**: Token validation failures return appropriate status codes and error messages
2. **Proxy Errors**: Network issues, timeouts, etc. are mapped to appropriate HTTP status codes
3. **JSON Error Format**: Consistent error response structure with error code and description

## Why Not Use an API Client?

We explicitly chose not to build an API client library for several reasons:

1. **Transparency**: A client library would need to understand API-specific details and formats
2. **Flexibility**: Supporting all possible API parameters would be difficult and maintenance-heavy
3. **Performance**: Direct proxying avoids extra parsing/serialization overhead
4. **Future-Proofing**: APIs evolve; a transparent proxy automatically supports new endpoints/parameters
5. **Streaming Support**: Direct proxy streaming is more efficient than client library handling

## Configuration Flexibility

The proxy is designed to be highly configurable:

1. **Target API**: Change base URL in configuration to proxy to any API
2. **Allowed Endpoints/Methods**: Configure security restrictions
3. **Timeout Settings**: Control request timeouts, response header timeouts, and streaming flush intervals
4. **Connection Pooling**: Fine-tune connection settings for optimal performance

## Testing Strategy

The proxy is thoroughly tested with several types of tests:

1. **Unit Tests**: For individual components and middlewares
2. **Integration Tests**: Testing the full request/response flow
3. **Streaming Tests**: Special tests for SSE handling
4. **Performance Tests**: Benchmarks for latency and throughput
5. **Error Handling Tests**: Tests for various error conditions

## Conclusion

Our transparent proxy design prioritizes maximum transparency, minimal overhead, and generic applicability. By building directly on the `httputil.ReverseProxy` with a middleware chain, we achieve high performance, strong security, and API flexibility.