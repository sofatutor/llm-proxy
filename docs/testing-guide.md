# Testing Guide

This guide covers testing practices, workflows, and requirements for the LLM Proxy project. The project follows strict Test-Driven Development (TDD) principles with a minimum coverage requirement of 90%.

## Testing Philosophy

### Test-Driven Development (TDD)

**Mandatory TDD Workflow:**
1. **Red**: Write a failing test first
2. **Green**: Implement minimal code to make the test pass
3. **Refactor**: Improve code while maintaining passing tests

**No exceptions**: All features and bug fixes must start with a failing test.

### Coverage Requirements

- **Minimum Coverage**: 90% for all packages under `internal/`
- **Aggregate Coverage**: Currently at 75.4%, target is 90%+
- **Coverage Enforcement**: CI fails if coverage drops below threshold
- **New Code**: Must maintain or improve coverage percentage

## Testing Levels

### 1. Unit Tests

**Purpose**: Test individual functions, methods, and components in isolation

**Location**: `*_test.go` files in the same package as the implementation

**Characteristics**:
- Fast execution (< 1ms per test)
- No external dependencies (use mocks/stubs)
- Test pure functions and business logic
- Use table-driven tests for multiple scenarios

**Example Structure**:
```go
func TestTokenValidator_ValidateToken(t *testing.T) {
    tests := []struct {
        name           string
        token          string
        expectedResult ValidationResult
        expectedError  error
        setup          func(*testing.T) *mockStore
    }{
        {
            name:  "valid_token",
            token: "valid-token-123",
            expectedResult: ValidationResult{Valid: true},
            setup: func(t *testing.T) *mockStore {
                store := &mockStore{}
                store.On("GetToken", mock.Anything).Return(validToken, nil)
                return store
            },
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            store := tt.setup(t)
            validator := NewValidator(store)
            
            result, err := validator.ValidateToken(context.Background(), tt.token)
            
            assert.Equal(t, tt.expectedResult, result)
            assert.Equal(t, tt.expectedError, err)
            store.AssertExpectations(t)
        })
    }
}
```

### 2. Integration Tests

**Purpose**: Test package interactions and external integrations

**Location**: `*_integration_test.go` files with build tag `//go:build integration`

**Characteristics**:
- Test real interactions between components
- Use real database connections (SQLite for tests)
- Test HTTP endpoints with test servers
- Longer execution time acceptable

**Example Structure**:
```go
//go:build integration

func TestTokenManager_Integration(t *testing.T) {
    // Setup real database
    db := setupTestDB(t)
    defer db.Close()
    
    manager := token.NewManager(db)
    
    // Test complete token lifecycle
    token, err := manager.CreateToken(ctx, projectID, duration)
    require.NoError(t, err)
    
    result, err := manager.ValidateToken(ctx, token.ID)
    require.NoError(t, err)
    assert.True(t, result.Valid)
    
    err = manager.RevokeToken(ctx, token.ID)
    require.NoError(t, err)
}
```

### 3. End-to-End (E2E) Tests

**Purpose**: Test complete user flows and system behavior

**Location**: `test/` directory with CLI and HTTP tests

**Characteristics**:
- Test complete user workflows
- Use real server instances
- Test CLI commands and outputs
- Validate system behavior under load

**Example Structure**:
```go
func TestE2E_CompleteWorkflow(t *testing.T) {
    // Start real server
    server := startTestServer(t)
    defer server.Stop()
    
    // Test project creation via CLI
    output := runCLI(t, "manage", "project", "create", "--name", "test-project")
    projectID := extractProjectID(output)
    
    // Test token generation
    output = runCLI(t, "manage", "token", "generate", "--project-id", projectID)
    token := extractToken(output)
    
    // Test proxy request with token
    resp := makeProxyRequest(t, server.URL, token, "/v1/models")
    assert.Equal(t, http.StatusOK, resp.StatusCode)
}
```

## Test Organization

### File Naming Conventions

- `component_test.go`: Unit tests for `component.go`
- `component_integration_test.go`: Integration tests
- `component_bench_test.go`: Benchmark tests
- `testdata/`: Static test data and fixtures

### Test Function Naming

- `TestFunctionName`: Basic functionality test
- `TestFunctionName_Scenario`: Specific scenario test
- `TestFunctionName_EdgeCase`: Edge case handling
- `BenchmarkFunctionName`: Performance benchmarks

### Test Utilities

**Common Test Helpers** (`internal/testing/`):
```go
// Database helpers
func SetupTestDB(t *testing.T) *sql.DB
func CleanupTestDB(t *testing.T, db *sql.DB)

// HTTP helpers  
func NewTestServer(t *testing.T, handler http.Handler) *httptest.Server
func MakeRequest(t *testing.T, server *httptest.Server, method, path string) *http.Response

// CLI helpers
func RunCLI(t *testing.T, args ...string) string
func ExpectCLIError(t *testing.T, expectedError string, args ...string)
```

## Running Tests

### Local Development

```bash
# Run all tests with race detection
make test

# Run with coverage reporting
make test-coverage

# Run only unit tests (fast)
go test -short ./...

# Run integration tests
go test -tags=integration ./...

# Run specific package tests
go test -v ./internal/token/

# Run specific test function
go test -v -run TestTokenValidator_ValidateToken ./internal/token/

# Run benchmarks
go test -bench=. ./internal/token/

# Generate coverage HTML report
make test-coverage-html
```

### CI/CD Testing

**GitHub Actions Workflow** (`.github/workflows/test.yml`):
- **Unit Tests**: Fast tests on multiple Go versions
- **Integration Tests**: Tests with real database
- **Race Detection**: All tests run with `-race` flag
- **Coverage Reporting**: Upload coverage to artifacts
- **Benchmark Testing**: Performance regression detection

## Test Data Management

### Test Database Setup

**SQLite for Tests**:
```go
func setupTestDB(t *testing.T) *sql.DB {
    db, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    
    // Run migrations
    err = runMigrations(db)
    require.NoError(t, err)
    
    t.Cleanup(func() {
        db.Close()
    })
    
    return db
}
```

### Test Fixtures

**Using testdata/**:
```
testdata/
├── valid_token.json       # Valid token test data
├── expired_token.json     # Expired token scenarios
├── api_responses/         # Mock API responses
│   ├── openai_models.json
│   └── openai_chat.json
└── sql/                   # Test database fixtures
    └── sample_data.sql
```

## Mocking and Test Doubles

### Interface-Based Mocking

**Example Interface**:
```go
type TokenStore interface {
    GetToken(ctx context.Context, id string) (*Token, error)
    CreateToken(ctx context.Context, token *Token) error
    UpdateToken(ctx context.Context, token *Token) error
    DeleteToken(ctx context.Context, id string) error
}
```

**Mock Implementation**:
```go
type MockTokenStore struct {
    mock.Mock
}

func (m *MockTokenStore) GetToken(ctx context.Context, id string) (*Token, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*Token), args.Error(1)
}

// Use testify/mock for behavior verification
func TestWithMock(t *testing.T) {
    mockStore := &MockTokenStore{}
    mockStore.On("GetToken", mock.Anything, "token-123").Return(validToken, nil)
    
    // Test code using mockStore
    result, err := service.ProcessToken(mockStore, "token-123")
    
    assert.NoError(t, err)
    mockStore.AssertExpectations(t)
}
```

### HTTP Test Doubles

**Test Server Setup**:
```go
func setupMockOpenAI(t *testing.T) *httptest.Server {
    mux := http.NewServeMux()
    
    mux.HandleFunc("/v1/models", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write(loadTestData(t, "openai_models.json"))
    })
    
    server := httptest.NewServer(mux)
    t.Cleanup(server.Close)
    
    return server
}
```

## Coverage Measurement

### Current Coverage Status

**Package Coverage** (as of latest run):
- `internal/token`: 95.2% ✅
- `internal/database`: 88.7% ❌ (needs improvement)
- `internal/proxy`: 92.1% ✅
- `internal/eventbus`: 89.3% ❌ (needs improvement)
- **Overall**: 75.4% ❌ (target: 90%+)

### Coverage Commands

```bash
# Generate coverage profile
go test -coverprofile=coverage.out ./...

# View coverage by function
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html

# Coverage for specific packages only
go test -coverprofile=coverage.out ./internal/...
```

### Coverage Analysis

**Exclude from Coverage**:
- Generated code (protobuf, mock files)
- Main functions and CLI entry points
- Error handling for "impossible" conditions
- Deprecated code scheduled for removal

**Focus Areas for Coverage**:
- Business logic in `internal/token/`, `internal/database/`
- HTTP handlers and middleware
- Error handling and edge cases
- Configuration validation

## Performance Testing

### Benchmark Tests

**CPU Benchmarks**:
```go
func BenchmarkTokenValidation(b *testing.B) {
    validator := setupValidator(b)
    token := "valid-token-123"
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := validator.ValidateToken(context.Background(), token)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

**Memory Benchmarks**:
```go
func BenchmarkEventPublishing(b *testing.B) {
    bus := setupEventBus(b)
    event := &Event{/* ... */}
    
    b.ResetTimer()
    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        bus.Publish(context.Background(), *event)
    }
}
```

### Load Testing

**Using built-in benchmark tool**:
```bash
# Build benchmark tool
make build

# Run load test
./bin/llm-proxy benchmark \
  --url http://localhost:8080 \
  --token <test-token> \
  --concurrent 10 \
  --requests 1000 \
  --duration 60s
```

## Test Environment Setup

### Local Development Setup

```bash
# Install test dependencies
go mod download

# Install testing tools
make tools

# Setup test database
make db-setup

# Run all tests to verify setup
make test
```

### CI Environment

**Required Environment Variables**:
- `MANAGEMENT_TOKEN`: Test management token
- `OPENAI_API_KEY`: Test API key (optional, can use mock)
- `DATABASE_PATH`: Test database path
- `LOG_LEVEL`: debug (for test visibility)

### Docker Testing

**Test in Container**:
```bash
# Build test image
docker build -f Dockerfile.test .

# Run tests in container
docker run --rm -v $(pwd):/app test-image make test

# Run with coverage
docker run --rm -v $(pwd):/app test-image make test-coverage
```

## Debugging Tests

### Test Debugging Techniques

**Verbose Output**:
```bash
# Verbose test output
go test -v ./internal/token/

# Show test coverage per function
go test -v -coverprofile=coverage.out ./internal/token/
go tool cover -func=coverage.out | grep -v "100.0%"
```

**Debug Logging in Tests**:
```go
func TestWithLogging(t *testing.T) {
    // Enable debug logging for tests
    logger := zap.NewDevelopment()
    
    // Use logger in component
    component := NewComponent(logger)
    
    // Test with detailed logging
    result, err := component.DoSomething()
    logger.Info("Test result", zap.Any("result", result))
}
```

### Common Test Failures

**Race Conditions**:
```bash
# Run tests with race detector
go test -race ./...

# Run specific test with race detection
go test -race -run TestConcurrentAccess ./internal/token/
```

**Timing Issues**:
```go
// Use eventually for async operations
func TestAsyncOperation(t *testing.T) {
    result := performAsyncOperation()
    
    // Wait for async completion
    assert.Eventually(t, func() bool {
        return result.IsComplete()
    }, 5*time.Second, 100*time.Millisecond)
}
```

## Continuous Integration

### Pre-Push Checks

**Required Checks** (run via Git hooks or manually):
```bash
# Format code
make fmt

# Run linter
make lint

# Run all tests
make test

# Check coverage
make test-coverage

# Integration tests
make integration-test
```

### CI Pipeline

**GitHub Actions Matrix**:
- **Go Versions**: 1.23, 1.24
- **Platforms**: ubuntu-latest, macos-latest
- **Test Types**: unit, integration, race
- **Coverage**: Generate and upload reports

## Best Practices

### Writing Effective Tests

1. **Test Behavior, Not Implementation**
   - Focus on public interface behavior
   - Avoid testing internal implementation details
   - Test outcomes and side effects

2. **Clear Test Names**
   - Use descriptive test names: `TestTokenValidator_ShouldReturnErrorForExpiredToken`
   - Group related tests with subtests
   - Document complex test scenarios

3. **Independent Tests**
   - Each test should be able to run in isolation
   - Use setup/teardown for test data
   - Avoid test order dependencies

4. **Minimal Test Data**
   - Use minimal data required for the test
   - Create focused test fixtures
   - Use factory functions for complex objects

### Test Maintenance

1. **Keep Tests DRY**
   - Extract common setup into helper functions
   - Use table-driven tests for multiple scenarios
   - Share test utilities across packages

2. **Update Tests with Code Changes**
   - Update tests when changing interfaces
   - Add new tests for new functionality
   - Remove tests for deprecated code

3. **Review Test Coverage Regularly**
   - Monitor coverage trends in CI
   - Identify and test uncovered edge cases
   - Remove redundant tests

This testing guide ensures high code quality and maintainability through comprehensive testing practices. All contributors must follow these guidelines to maintain the project's quality standards.