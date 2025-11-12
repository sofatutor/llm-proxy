# Database Migrations

This package provides database migration functionality for the llm-proxy using [goose](https://github.com/pressly/goose).

## Migration Tool Selection

**Selected Tool**: goose (github.com/pressly/goose/v3)

### Decision Rationale

We evaluated three options: golang-migrate, goose, and a custom solution. We selected **goose** for the following reasons:

1. **Go-native design**: Built specifically for embedding in Go applications
2. **Simple API**: Clean, straightforward interface
3. **Light dependencies**: Minimal external dependencies
4. **Transaction support**: Built-in transaction handling for atomic migrations
5. **Both backends**: Supports SQLite (current) and PostgreSQL (future)
6. **Active maintenance**: Well-maintained, Go 1.23+ compatible
7. **Time to value**: Ready to use immediately

**Why not golang-migrate?** More complex than needed, heavier dependencies, CLI-first design.

**Why not custom?** Significant development time (3-5 days), ongoing maintenance burden, risk of bugs.

## Usage

### In Go Code

```go
import (
    "github.com/sofatutor/llm-proxy/internal/database/migrations"
)

// Create a migration runner
runner := migrations.NewMigrationRunner(db, "./internal/database/migrations/sql")

// Apply all pending migrations
if err := runner.Up(); err != nil {
    log.Fatal(err)
}

// Check current version
version, err := runner.Version()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Current migration version: %d\n", version)
```

## API Reference

### MigrationRunner

- `NewMigrationRunner(db *sql.DB, migrationsPath string) *MigrationRunner` - Create runner
- `Up() error` - Apply all pending migrations
- `Down() error` - Roll back last migration
- `Status() (int64, error)` - Get current version
- `Version() (int64, error)` - Alias for Status()

## Migration File Format

Format: `{version}_{description}.sql`

Example:
```sql
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
```

## Best Practices

1. Always include Down migrations
2. Test both directions
3. Keep migrations small
4. Never modify applied migrations
5. Use transactions (automatic via goose)

## References

- [goose Documentation](https://github.com/pressly/goose)
- [Story #117](https://github.com/sofatutor/llm-proxy/issues/117)
- [Epic #109](https://github.com/sofatutor/llm-proxy/issues/109)

