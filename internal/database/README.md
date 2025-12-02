# Database Package

## Purpose & Responsibilities

The `database` package provides data persistence for the LLM Proxy. It handles:

- Database connection management and configuration
- Token and project CRUD operations
- Audit event storage and retrieval
- Database migrations using goose
- Support for SQLite (default) and PostgreSQL
- Connection pooling and transaction management

## Key Types & Interfaces

| Type | Description |
|------|-------------|
| `DB` | Main database connection wrapper |
| `Config` | SQLite-specific configuration |
| `FullConfig` | Multi-driver configuration (SQLite + PostgreSQL) |
| `Project` | Project entity model |
| `Token` | Token entity model |
| `AuditEvent` | Audit log entry model |
| `DriverType` | Database driver enum (SQLite, Postgres) |

### DB Methods

The `DB` struct provides methods for all data operations:

```go
// Connection management
func New(config Config) (*DB, error)
func NewFromFullConfig(config FullConfig) (*DB, error)
func (d *DB) Close() error
func (d *DB) HealthCheck(ctx context.Context) error
func (d *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error

// Token operations
func (d *DB) CreateToken(ctx context.Context, token Token) error
func (d *DB) GetTokenByID(ctx context.Context, tokenID string) (Token, error)
func (d *DB) UpdateToken(ctx context.Context, token Token) error
func (d *DB) DeleteToken(ctx context.Context, tokenID string) error
func (d *DB) ListTokens(ctx context.Context) ([]Token, error)
func (d *DB) GetTokensByProjectID(ctx context.Context, projectID string) ([]Token, error)
func (d *DB) IncrementTokenUsage(ctx context.Context, tokenID string) error

// Project operations
func (d *DB) CreateProject(ctx context.Context, project Project) error
func (d *DB) GetProjectByID(ctx context.Context, projectID string) (Project, error)
func (d *DB) UpdateProject(ctx context.Context, project Project) error
func (d *DB) DeleteProject(ctx context.Context, projectID string) error
func (d *DB) ListProjects(ctx context.Context) ([]Project, error)
func (d *DB) GetAPIKeyForProject(ctx context.Context, projectID string) (string, error)
func (d *DB) GetProjectActive(ctx context.Context, projectID string) (bool, error)

// Audit operations
func (d *DB) CreateAuditEvent(ctx context.Context, event AuditEvent) error
func (d *DB) GetAuditEventByID(ctx context.Context, id string) (AuditEvent, error)
func (d *DB) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, error)
```

### Data Models

```go
type Project struct {
    ID            string     `json:"id"`
    Name          string     `json:"name"`
    OpenAIAPIKey  string     `json:"-"` // Not exposed in JSON
    IsActive      bool       `json:"is_active"`
    DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}

type Token struct {
    Token         string     `json:"token"`
    ProjectID     string     `json:"project_id"`
    ExpiresAt     *time.Time `json:"expires_at,omitempty"`
    IsActive      bool       `json:"is_active"`
    DeactivatedAt *time.Time `json:"deactivated_at,omitempty"`
    RequestCount  int        `json:"request_count"`
    MaxRequests   *int       `json:"max_requests,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
    CacheHitCount int        `json:"cache_hit_count"`
}
```

## Usage Examples

### Basic SQLite Setup

```go
package main

import (
    "context"
    "log"

    "github.com/sofatutor/llm-proxy/internal/database"
)

func main() {
    // Use default configuration
    config := database.DefaultConfig()
    // config.Path = "data/llm-proxy.db"

    db, err := database.New(config)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Health check
    if err := db.HealthCheck(context.Background()); err != nil {
        log.Fatal(err)
    }

    log.Println("Database connected successfully")
}
```

### Custom Configuration

```go
config := database.Config{
    Path:            "/var/data/llm-proxy.db",
    MaxOpenConns:    20,
    MaxIdleConns:    10,
    ConnMaxLifetime: 2 * time.Hour,
}

db, err := database.New(config)
```

### Multi-Driver Configuration

```go
// Configuration from environment
config := database.ConfigFromEnv()

// Or explicit multi-driver config
config := database.FullConfig{
    Driver:          database.DriverPostgres,
    DatabaseURL:     "postgres://user:pass@host:5432/llmproxy",
    MaxOpenConns:    25,
    MaxIdleConns:    10,
    ConnMaxLifetime: time.Hour,
}

db, err := database.NewFromFullConfig(config)
```

### CRUD Operations

```go
ctx := context.Background()

// Create project
project := database.Project{
    ID:           "proj-123",
    Name:         "My Project",
    OpenAIAPIKey: "sk-...",
    IsActive:     true,
    CreatedAt:    time.Now(),
    UpdatedAt:    time.Now(),
}
err := db.CreateProject(ctx, project)

// Read project
proj, err := db.GetProjectByID(ctx, "proj-123")

// Update project
proj.Name = "Updated Name"
proj.UpdatedAt = time.Now()
err = db.UpdateProject(ctx, proj)

// Delete project
err = db.DeleteProject(ctx, "proj-123")

// List all projects
projects, err := db.ListProjects(ctx)
```

### Token Operations

```go
ctx := context.Background()

// Create token
token := database.Token{
    Token:        "sk-abc123...",
    ProjectID:    "proj-123",
    IsActive:     true,
    RequestCount: 0,
    CreatedAt:    time.Now(),
}
err := db.CreateToken(ctx, token)

// Increment usage
err = db.IncrementTokenUsage(ctx, "sk-abc123...")

// Get tokens by project
tokens, err := db.GetTokensByProjectID(ctx, "proj-123")
```

### Transactions

```go
err := db.Transaction(ctx, func(tx *sql.Tx) error {
    // Multiple operations in single transaction
    _, err := tx.ExecContext(ctx, "UPDATE tokens SET is_active = ? WHERE project_id = ?", false, projectID)
    if err != nil {
        return err // Triggers rollback
    }
    
    _, err = tx.ExecContext(ctx, "UPDATE projects SET is_active = ? WHERE id = ?", false, projectID)
    if err != nil {
        return err // Triggers rollback
    }
    
    return nil // Commits on success
})
```

## Configuration

### SQLite Configuration

| Field | Description | Default |
|-------|-------------|---------|
| `Path` | Database file path | `data/llm-proxy.db` |
| `MaxOpenConns` | Max open connections | 10 |
| `MaxIdleConns` | Max idle connections | 5 |
| `ConnMaxLifetime` | Connection max lifetime | 1 hour |

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_DRIVER` | Database driver (`sqlite`, `postgres`) | `sqlite` |
| `DATABASE_PATH` | SQLite database path | `data/llm-proxy.db` |
| `DATABASE_URL` | PostgreSQL connection URL | - |
| `DATABASE_POOL_SIZE` | Max open connections | 10 |
| `DATABASE_MAX_IDLE_CONNS` | Max idle connections | 5 |
| `DATABASE_CONN_MAX_LIFETIME` | Connection lifetime | 1h |

### SQLite vs PostgreSQL

| Feature | SQLite | PostgreSQL |
|---------|--------|------------|
| Setup | Zero configuration | Requires server |
| Concurrency | Single writer | Multiple writers |
| Use case | Development, single instance | Production, multi-instance |
| Connection string | File path | URL with credentials |

## Migration Workflow

Migrations are managed using goose and stored in `migrations/sql/`:

```bash
# Run migrations (automatic on New())
# Migrations run automatically when creating a new DB connection

# Manual migration commands (if needed)
goose -dir internal/database/migrations/sql sqlite3 ./data/llm-proxy.db up
goose -dir internal/database/migrations/sql sqlite3 ./data/llm-proxy.db down
goose -dir internal/database/migrations/sql sqlite3 ./data/llm-proxy.db status
```

### Migration Files

Located in `internal/database/migrations/sql/`:

| Migration | Description |
|-----------|-------------|
| `001_create_projects.sql` | Create projects table |
| `002_create_tokens.sql` | Create tokens table |
| `003_create_audit_events.sql` | Create audit events table |
| `004_add_cache_hit_count.sql` | Add cache hit tracking |

See `migrations/README.md` for detailed migration documentation.

## Testing Guidance

### In-Memory Database for Tests

```go
package database_test

import (
    "context"
    "testing"

    "github.com/sofatutor/llm-proxy/internal/database"
)

func TestProjectCRUD(t *testing.T) {
    // Use in-memory SQLite for tests
    db, err := database.New(database.Config{Path: ":memory:"})
    if err != nil {
        t.Fatal(err)
    }
    defer db.Close()

    ctx := context.Background()

    // Create
    project := database.Project{
        ID:        "test-1",
        Name:      "Test Project",
        IsActive:  true,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    err = db.CreateProject(ctx, project)
    if err != nil {
        t.Fatal(err)
    }

    // Read
    got, err := db.GetProjectByID(ctx, "test-1")
    if err != nil {
        t.Fatal(err)
    }
    if got.Name != "Test Project" {
        t.Errorf("Expected 'Test Project', got %q", got.Name)
    }
}
```

### Mock Stores

For unit testing without database:

```go
// Use MockProjectStore from mock_project.go
store := database.NewMockProjectStore()
store.AddProject(project)

// Use MockTokenStore from mock_token.go
tokenStore := database.NewMockTokenStore()
tokenStore.AddToken(token)
```

### Integration Testing

```go
func TestDatabaseIntegration(t *testing.T) {
    // Create temp file for test database
    tmpFile, _ := os.CreateTemp("", "test-*.db")
    defer os.Remove(tmpFile.Name())

    db, _ := database.New(database.Config{Path: tmpFile.Name()})
    defer db.Close()

    // Run integration tests...
}
```

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `failed to create database directory` | Parent directory doesn't exist | Create directory or use absolute path |
| `failed to open database` | Invalid path or permissions | Check file permissions |
| `failed to run migrations` | Migration files not found | Verify migrations directory path |
| `database is locked` | Concurrent writes (SQLite) | Use PostgreSQL for concurrent access |
| `ErrProjectNotFound` | Project doesn't exist | Check project ID |
| `ErrTokenNotFound` | Token doesn't exist | Check token ID |

### Connection Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Connection timeouts | Pool exhaustion | Increase `MaxOpenConns` |
| Slow queries | Missing indexes | Check migration indexes |
| Memory growth | Connection leaks | Ensure `Close()` is called |

### SQLite-Specific Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| "database is locked" | Concurrent writes | Use WAL mode (enabled by default) |
| Slow writes | Missing WAL | Check connection string includes `?_journal=WAL` |
| Test failures | Shared in-memory DB | Use `MaxOpenConns = 1` for `:memory:` |

## Related Packages

| Package | Relationship |
|---------|--------------|
| [`token`](../token/README.md) | Uses TokenStore interface from database |
| [`proxy`](../proxy/README.md) | Uses ProjectStore interface from database |
| [`server`](../server/README.md) | Initializes database for API handlers |
| [`audit`](../audit/) | Stores audit events in database |

## Files

| File | Description |
|------|-------------|
| `database.go` | Core DB struct, connection, and migrations |
| `factory.go` | Multi-driver database factory |
| `factory_postgres.go` | PostgreSQL-specific factory |
| `models.go` | Data models (Project, Token, AuditEvent) |
| `project.go` | Project CRUD operations |
| `token.go` | Token CRUD operations |
| `audit.go` | Audit event operations |
| `utils.go` | Helper functions and query utilities |
| `mock_project.go` | Mock project store for testing |
| `mock_token.go` | Mock token store for testing |
| `migrations/` | Database migration files and runner |