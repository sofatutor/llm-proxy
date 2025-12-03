---
title: Code Organization
parent: Architecture
nav_order: 5
---

# Code Organization Guide

This document provides a comprehensive overview of the LLM Proxy codebase organization, including package structure, layering principles, and architectural boundaries.

## Directory Structure Overview

```
llm-proxy/
├── cmd/                    # Command-line applications
│   ├── proxy/             # Main CLI application (llm-proxy)
│   └── eventdispatcher/   # Standalone event dispatcher CLI
├── internal/              # Internal packages (not for external import)
│   ├── server/            # HTTP server and lifecycle management
│   ├── proxy/             # Transparent reverse proxy implementation
│   ├── token/             # Token management and validation
│   ├── database/          # Data storage abstraction and implementations
│   ├── eventbus/          # Async event system
│   ├── dispatcher/        # Event dispatcher service
│   ├── middleware/        # HTTP middleware components
│   ├── admin/             # Admin UI handlers and logic
│   ├── api/               # Management API handlers
│   ├── config/            # Configuration management
│   ├── logging/           # Structured logging utilities
│   ├── audit/             # Audit logging system
│   ├── utils/             # Shared utilities and helpers
│   └── ...                # Other internal packages
├── api/                   # API specifications and shared types
├── web/                   # Static web assets (admin UI)
├── docs/                  # Documentation
├── test/                  # Integration tests
└── testdata/              # Test fixtures and data
```

## Package Layering Architecture

The codebase follows a clean architecture with well-defined layers and dependencies:

### Layer 1: Core Domain Logic

**Location**: `internal/token/`, `internal/database/`, `internal/audit/`

**Purpose**: Core business logic and domain models
- Token generation, validation, and lifecycle management
- Database models and repository patterns
- Audit event structures and rules

**Dependencies**: Only standard library and minimal external dependencies
**Testing**: High coverage with unit tests, table-driven tests

### Layer 2: Infrastructure Services

**Location**: `internal/eventbus/`, `internal/dispatcher/`, `internal/logging/`

**Purpose**: Infrastructure concerns and external integrations
- Event bus implementations (in-memory, Redis)
- Event dispatcher with pluggable backends
- Structured logging and observability

**Dependencies**: External libraries for specific implementations (Redis, Zap)
**Testing**: Integration tests with mocks for external dependencies

### Layer 3: HTTP Layer

**Location**: `internal/server/`, `internal/proxy/`, `internal/middleware/`, `internal/api/`, `internal/admin/`

**Purpose**: HTTP-specific concerns and request handling
- Server lifecycle management
- Reverse proxy implementation
- Middleware chain composition
- API handlers and routing

**Dependencies**: Layers 1-2, HTTP libraries (Gin, net/http)
**Testing**: HTTP tests with test servers and mocked backends

### Layer 4: Application Layer

**Location**: `cmd/proxy/`, `cmd/eventdispatcher/`

**Purpose**: CLI applications and configuration
- Command-line interface definition
- Configuration parsing and validation
- Application bootstrapping

**Dependencies**: All other layers
**Testing**: CLI integration tests, configuration validation tests

## Package Responsibilities

### Core Packages

#### `internal/token/`
**Purpose**: Token lifecycle management
**Key Components**:
- `manager.go`: High-level token operations
- `generator.go`: Secure token generation
- `validator.go`: Token validation with caching
- `cache.go`: LRU cache for validation performance
- `ratelimit.go`: Token-based rate limiting

**Design Principles**:
- Immutable token structures
- Thread-safe operations
- Configurable caching and rate limiting

#### `internal/database/`
**Purpose**: Data persistence abstraction
**Key Components**:
- `store.go`: Repository interface definitions
- `sqlite.go`: SQLite implementation
- `postgres.go`: PostgreSQL implementation (planned)
- `migrations/`: Database schema management

**Design Principles**:
- Database-agnostic interfaces
- Transaction support
- Connection pooling and health checks

#### `internal/proxy/`
**Purpose**: Transparent HTTP proxying
**Key Components**:
- `proxy.go`: Main reverse proxy implementation
- `director.go`: Request routing and transformation
- `transport.go`: HTTP transport configuration
- `streaming.go`: SSE and streaming response handling

**Design Principles**:
- Minimal request/response transformation
- High performance and low latency
- Support for all HTTP methods and content types

### Infrastructure Packages

#### `internal/eventbus/`
**Purpose**: Async event publishing and subscription
**Key Components**:
- `eventbus.go`: Core event bus interface and in-memory implementation
- `redis.go`: Redis-based event bus for distributed deployments
- Event structures and serialization

**Design Principles**:
- Non-blocking event publishing
- Pluggable backend implementations
- Fan-out broadcasting to multiple subscribers

#### `internal/dispatcher/`
**Purpose**: Event processing and backend integration
**Key Components**:
- `service.go`: Main dispatcher service
- `plugins/`: Backend plugin implementations (File, Lunary, Helicone)
- `transformer.go`: Event transformation and batching

**Design Principles**:
- Pluggable backend architecture
- Configurable batching and retry logic
- Graceful degradation on backend failures

#### `internal/middleware/`
**Purpose**: HTTP middleware components
**Key Components**:
- `instrumentation.go`: Request/response instrumentation
- `requestid.go`: Request ID generation and propagation
- `timeout.go`: Request timeout handling
- `recovery.go`: Panic recovery and error handling

**Design Principles**:
- Composable middleware chain
- Minimal performance overhead
- Consistent error handling

### HTTP Layer Packages

#### `internal/server/`
**Purpose**: HTTP server lifecycle and configuration
**Key Components**:
- `server.go`: HTTP server setup and graceful shutdown
- `routes.go`: Route registration and middleware composition
- `config.go`: Server configuration validation

#### `internal/api/`
**Purpose**: Management API handlers
**Key Components**:
- `projects.go`: Project CRUD operations
- `tokens.go`: Token management endpoints
- `health.go`: Health check endpoints

#### `internal/admin/`
**Purpose**: Admin UI handlers and logic
**Key Components**:
- `handlers.go`: UI route handlers
- `client.go`: Management API client
- `session.go`: Session management

## Dependency Management

### External Dependencies

The project minimizes external dependencies and carefully manages them:

**Core Dependencies**:
- `github.com/gin-gonic/gin`: HTTP framework for admin UI
- `github.com/redis/go-redis/v9`: Redis client for distributed event bus
- `go.uber.org/zap`: Structured logging
- `modernc.org/sqlite`: SQLite database driver

**Development Dependencies**:
- `github.com/stretchr/testify`: Testing utilities
- `github.com/golang/mock`: Mock generation for testing

### Internal Dependency Rules

1. **No Circular Dependencies**: Enforced by Go's module system
2. **Layer Isolation**: Higher layers can depend on lower layers, not vice versa
3. **Interface Segregation**: Use small, focused interfaces for decoupling
4. **Dependency Injection**: Pass dependencies explicitly, avoid global state

## Testing Strategy

### Package-Level Testing

Each package includes comprehensive tests following these patterns:

**Unit Tests**: `*_test.go` files in the same package
- Test individual functions and methods
- Use table-driven tests for multiple scenarios
- Mock external dependencies

**Integration Tests**: `*_integration_test.go` files
- Test package interactions
- Use real database connections
- Test HTTP endpoints with test servers

**Benchmark Tests**: `*_bench_test.go` files
- Performance testing for critical paths
- Memory allocation profiling
- Concurrency testing

### Test Organization

```
internal/package/
├── component.go           # Implementation
├── component_test.go      # Unit tests
├── component_bench_test.go # Benchmarks
├── integration_test.go    # Integration tests
└── testdata/             # Test fixtures
```

## Configuration Management

### Configuration Sources

Configuration is loaded from multiple sources in order of precedence:
1. Command-line flags
2. Environment variables
3. Configuration files (`.env`, YAML)
4. Default values

### Configuration Packages

- `internal/config/`: Core configuration structures and validation
- `cmd/proxy/config.go`: CLI-specific configuration
- Environment variable mapping follows `SNAKE_CASE` convention

## Error Handling Patterns

### Error Types

1. **Domain Errors**: Business logic validation errors
2. **Infrastructure Errors**: Database, network, or external service errors
3. **HTTP Errors**: Request validation and HTTP-specific errors

### Error Handling Strategy

- Use wrapped errors with context: `fmt.Errorf("operation failed: %w", err)`
- Define custom error types for domain errors
- Log errors at appropriate levels (ERROR for failures, WARN for retries)
- Return structured errors in API responses

## Performance Considerations

### Critical Performance Paths

1. **Token Validation**: Cached to minimize database queries
2. **Proxy Request Path**: Minimal middleware overhead
3. **Event Publishing**: Non-blocking async operations
4. **Database Queries**: Connection pooling and prepared statements

### Memory Management

- Reuse HTTP request/response objects where possible
- Use object pools for frequently allocated structures
- Limit concurrent connections and buffer sizes
- Implement circuit breakers for external services

## Development Workflow

### Adding New Features

1. **Design Phase**: Update relevant documentation first
2. **Test-Driven Development**: Write failing tests before implementation
3. **Implementation**: Minimal changes to achieve test passing
4. **Integration**: Ensure compatibility with existing components
5. **Documentation**: Update package documentation and examples

### Code Review Guidelines

- Focus on interface design and architectural boundaries
- Verify test coverage meets 90% minimum requirement
- Check for proper error handling and logging
- Validate performance impact on critical paths
- Ensure documentation is updated

This code organization guide serves as the foundation for understanding and contributing to the LLM Proxy codebase. For specific implementation details, refer to the package-level documentation and tests.