# LLM Proxy Architecture

This document describes the architecture of the LLM Proxy, explaining the main components, their interactions, and design decisions.

## Overview

The LLM Proxy is a transparent proxy server for OpenAI API requests, providing token management, authentication, and usage tracking. It acts as an intermediary between client applications and the OpenAI API.

## System Architecture

```
┌─────────────┐     ┌─────────────────────────────────────┐     ┌──────────┐
│             │     │             LLM Proxy               │     │          │
│   Clients   │────▶│                                     │────▶│  OpenAI  │
│             │     │ ┌─────────┐ ┌─────────┐ ┌────────┐  │     │   API    │
└─────────────┘     │ │  Auth   │ │ Token   │ │ OpenAI │  │     │          │
                    │ │ System  │ │ Manager │ │ Client │  │     └──────────┘
                    │ └─────────┘ └─────────┘ └────────┘  │
┌─────────────┐     │ ┌─────────┐ ┌─────────┐ ┌────────┐  │
│             │     │ │ Admin UI │ │ Logging │ │   DB   │  │
│    Admin    │────▶│ │         │ │ System  │ │        │  │
│             │     │ └─────────┘ └─────────┘ └────────┘  │
└─────────────┘     └─────────────────────────────────────┘
```

## Core Components

### Proxy Server

- **Purpose**: Accept and forward API requests, handle request/response transformation
- **Key Functions**:
  - Route registration
  - Request validation
  - Authentication
  - Header management
  - Response handling
  - Streaming support
  - Error handling
- **Implementation**: `internal/server/server.go`

### Configuration System

- **Purpose**: Manage application settings from environment variables
- **Key Features**:
  - Environment variable parsing with defaults
  - Configuration validation
  - Type-safe access to settings
- **Implementation**: `internal/config/config.go`

### Database Layer

- **Purpose**: Store projects, tokens, and usage data
- **Schema**:
  - Projects table: Stores project metadata and API keys
  - Tokens table: Stores tokens with expiration and usage limits
- **Implementation**: `internal/database/*`
- **Technology**: SQLite for simplicity and zero-dependency deployment

### Token Management

- **Purpose**: Generate, validate, and track tokens
- **Key Features**:
  - Secure token generation
  - Token validation with expiration checks
  - Rate limiting
  - Usage tracking
- **Implementation**: `internal/token/*`

### Administration UI

- **Purpose**: Provide a simple interface for managing projects and tokens
- **Key Features**:
  - Project management
  - Token generation
  - Token revocation
  - Usage statistics
- **Implementation**: `internal/admin/*` and `web/*`

### Logging System

- **Purpose**: Record application events and request details
- **Key Features**:
  - Structured logging
  - Log levels
  - Request/response logging
  - Error tracking
- **Implementation**: `internal/logging/*`

## API Structure

### Proxy API (`/v1/*`)

Provides a transparent proxy to OpenAI endpoints:

- `/v1/chat/completions`: Chat completion requests
- `/v1/completions`: Text completion requests
- `/v1/embeddings`: Embedding generation
- `/v1/models`: Model listing

### Management API (`/manage/*`)

Endpoints for project and token management:

- `/manage/tokens`: Token CRUD operations
- `/manage/projects`: Project CRUD operations

### Admin UI (`/admin/*`)

Web interface for system administration:

- `/admin/projects`: Project management
- `/admin/tokens`: Token management
- `/admin/dashboard`: Usage statistics

## Data Flow

1. **Client Request**:
   - Client sends API request with proxy token
   - Proxy receives and authenticates the request

2. **Token Validation**:
   - Token manager validates the token
   - Checks expiration and rate limits
   - Updates usage statistics

3. **Request Forwarding**:
   - Proxy retrieves the OpenAI API key associated with the token
   - Transforms the request (replaces authorization header)
   - Forwards the request to OpenAI

4. **Response Handling**:
   - Proxy receives response from OpenAI
   - Collects metadata (tokens used, model, etc.)
   - Returns response to client

## Security Considerations

- **Token Security**:
  - Tokens are stored securely in the database
  - Token revocation mechanism
  - Expiration controls
  - Rate limiting

- **API Key Protection**:
  - API keys are never exposed to clients
  - API keys are stored securely

- **Request Validation**:
  - Input validation
  - Rate limiting
  - Size limits

## Deployment Architecture

The application is designed for flexible deployment:

### Single Container Deployment

```
┌─────────────────────────────┐
│ Docker Container            │
│                             │
│ ┌─────────┐     ┌─────────┐ │
│ │ LLM     │     │         │ │
│ │ Proxy   │━━━━━│ SQLite  │ │
│ │         │     │         │ │
│ └─────────┘     └─────────┘ │
│                             │
└─────────────────────────────┘
```

### Docker Compose Deployment

```
┌─────────────────┐  ┌─────────────────┐
│ LLM Proxy       │  │ Monitoring      │
│ Container       │  │ Container       │
│                 │  │                 │
│ ┌─────────────┐ │  │ ┌─────────────┐ │
│ │ Application │ │  │ │ Prometheus  │ │
│ └─────────────┘ │  │ └─────────────┘ │
└─────────────────┘  └─────────────────┘
        │                    │
        ▼                    ▼
┌─────────────────┐  ┌─────────────────┐
│ Volume:         │  │ Volume:         │
│ Data            │  │ Metrics         │
└─────────────────┘  └─────────────────┘
```

## Performance Considerations

- **Connection Pooling**: Database connections are pooled for performance
- **Concurrent Request Handling**: Go's goroutines enable efficient concurrent processing
- **Streaming Support**: Efficient handling of streaming responses
- **Rate Limiting**: Protects both the proxy and upstream API from overload

## Future Extensions

- **Multiple LLM Provider Support**: Expand beyond OpenAI to other providers
- **Advanced Analytics**: More detailed usage analytics and reporting
- **Custom Rate Limiting Policies**: Per-project and per-endpoint rate limiting
- **Caching**: Response caching for frequently used queries
- **Load Balancing**: Support for multiple OpenAI API keys with load balancing