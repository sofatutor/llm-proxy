---
title: Migration Concurrency
parent: Database
nav_order: 3
---

# Migration Concurrency in Distributed Systems

## Current Situation

The llm-proxy runs migrations automatically during server startup via `database.New()`. In distributed systems with multiple server instances, this can lead to concurrency issues.

## The Problem

When multiple server instances start simultaneously:

1. Each instance calls `database.New()` during initialization
2. Each instance calls `runMigrations()` which executes `goose.Up()`
3. Multiple processes may attempt to apply the same migration concurrently
4. This can lead to:
   - Race conditions on the `goose_db_version` table
   - Duplicate migration attempts
   - Database locking conflicts
   - Potential data corruption (though unlikely with idempotent migrations)

## How Goose Handles Concurrency

**Goose v3** relies on database-level locking mechanisms:

### SQLite (Current)
- **File-level locking**: SQLite uses file locks to serialize writes
- **WAL mode**: Allows concurrent readers, but writes are serialized
- **Implicit protection**: The `goose_db_version` table updates are serialized by SQLite's locking
- **Risk level**: **Low** - SQLite's locking prevents most issues, but there's still a small window for race conditions

### PostgreSQL (Future)
- **Row-level locking**: PostgreSQL provides better concurrency control
- **Transaction isolation**: Each migration runs in a transaction
- **Risk level**: **Very Low** - PostgreSQL's MVCC and locking handle concurrency well

## Current Implementation Behavior

```go
// internal/database/database.go
func New(config Config) (*DB, error) {
    // ... connection setup ...
    
    // Run database migrations
    if err := runMigrations(db); err != nil {
        // Migration fails, database connection is closed
        return nil, fmt.Errorf("failed to run migrations: %w", err)
    }
    
    return &DB{db: db}, nil
}
```

**What happens with concurrent starts:**
1. Instance A opens database connection
2. Instance B opens database connection (SQLite allows this in WAL mode)
3. Instance A reads `goose_db_version` → version 1
4. Instance B reads `goose_db_version` → version 1 (before A commits)
5. Instance A applies migration 2, updates version to 2
6. Instance B tries to apply migration 2 → **May fail or succeed depending on timing**

## Mitigation Strategies

### 1. **Idempotent Migrations (Current Best Practice)**

All migrations use `IF NOT EXISTS` / `IF EXISTS` clauses:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS projects (...);
CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);
```

**Pros:**
- Safe to run multiple times
- Works with current implementation
- No code changes needed

**Cons:**
- Doesn't prevent race conditions, just makes them harmless
- Still causes unnecessary work and potential locking contention

### 2. **Advisory Locks (Recommended Enhancement)**

Add explicit locking before running migrations:

```go
// Pseudo-code for advisory lock implementation
func runMigrationsWithLock(db *sql.DB) error {
    // Acquire advisory lock (database-specific)
    lockID := 12345 // Arbitrary lock ID for migrations
    
    // SQLite: Use a lock table
    // PostgreSQL: Use pg_advisory_lock()
    
    defer releaseLock(db, lockID)
    
    return runMigrations(db)
}
```

**Implementation for SQLite:**
```go
func acquireMigrationLock(db *sql.DB) (func(), error) {
    // Create lock table if not exists
    _, _ = db.Exec(`CREATE TABLE IF NOT EXISTS migration_lock (
        id INTEGER PRIMARY KEY CHECK (id = 1),
        locked BOOLEAN NOT NULL DEFAULT 0,
        locked_at DATETIME,
        locked_by TEXT
    )`)
    
    // Try to acquire lock with timeout
    tx, _ := db.Begin()
    var locked bool
    err := tx.QueryRow(`SELECT locked FROM migration_lock WHERE id = 1`).Scan(&locked)
    
    if locked {
        tx.Rollback()
        return nil, fmt.Errorf("migration already in progress")
    }
    
    tx.Exec(`UPDATE migration_lock SET locked = 1, locked_at = CURRENT_TIMESTAMP, locked_by = ? WHERE id = 1`, os.Getpid())
    tx.Commit()
    
    release := func() {
        db.Exec(`UPDATE migration_lock SET locked = 0 WHERE id = 1`)
    }
    
    return release, nil
}
```

**Implementation for PostgreSQL:**
```go
func acquireMigrationLock(db *sql.DB) (func(), error) {
    lockID := int64(12345) // Advisory lock ID
    
    var acquired bool
    err := db.QueryRow(`SELECT pg_try_advisory_lock($1)`, lockID).Scan(&acquired)
    if err != nil || !acquired {
        return nil, fmt.Errorf("could not acquire migration lock")
    }
    
    release := func() {
        db.Exec(`SELECT pg_advisory_unlock($1)`, lockID)
    }
    
    return release, nil
}
```

### 3. **Leader Election (For Kubernetes/Orchestrated Deployments)**

Use Kubernetes init containers or leader election:

```yaml
# Kubernetes example
initContainers:
  - name: migrate
    image: llm-proxy:latest
    command: ["llm-proxy", "migrate", "up"]
```

**Pros:**
- Migrations run once before any instances start
- Clean separation of concerns
- No code changes needed

**Cons:**
- Requires orchestration platform
- Additional deployment complexity

### 4. **External Migration Runner**

Run migrations separately from application startup:

```bash
# Deployment script
llm-proxy migrate up --db /path/to/db
# Then start all server instances
```

**Pros:**
- Simple and explicit
- No concurrency issues
- Clear migration step in deployment

**Cons:**
- Requires deployment process changes
- Manual step (can be automated)

## Recommended Approach

### For Current Implementation (SQLite)

**Short-term**: Current implementation is **mostly safe** because:
1. Migrations are idempotent (`IF NOT EXISTS`)
2. SQLite file locking serializes writes
3. Goose tracks versions in a table (prevents duplicate application)

**Enhancement**: Add advisory locking for extra safety:

```go
// internal/database/migrations/runner.go
func (m *MigrationRunner) Up() error {
    // Acquire lock
    release, err := acquireMigrationLock(m.db)
    if err != nil {
        return fmt.Errorf("failed to acquire migration lock: %w", err)
    }
    defer release()
    
    // ... existing migration code ...
}
```

### For Future PostgreSQL Implementation

PostgreSQL's `pg_advisory_lock()` provides built-in advisory locking:

```go
func (m *MigrationRunner) Up() error {
    // PostgreSQL advisory lock
    lockID := int64(0x4C4D50524F5859) // "LLMPROXY" in hex
    
    var acquired bool
    err := m.db.QueryRow(`SELECT pg_try_advisory_lock($1)`, lockID).Scan(&acquired)
    if err != nil || !acquired {
        return fmt.Errorf("migration already in progress by another instance")
    }
    defer m.db.Exec(`SELECT pg_advisory_unlock($1)`, lockID)
    
    // ... existing migration code ...
}
```

## Best Practices for Production

1. **Run migrations before starting instances** (init containers, deployment scripts)
2. **Use idempotent migrations** (already implemented)
3. **Monitor migration status** (`llm-proxy migrate status`)
4. **Add advisory locking** (recommended enhancement)
5. **Test concurrent startup scenarios** in staging

## Testing Concurrent Migrations

To test concurrent migration behavior:

```bash
# Terminal 1
llm-proxy server --db test.db

# Terminal 2 (start immediately after)
llm-proxy server --db test.db

# Both should start successfully, migrations should only run once
```

## Related Documentation

- [Migration Guide](migrations.md) - General migration workflow
- [CLI Reference](cli-reference.md) - Migration commands
- [Epic #109](../docs/epics/epic-109-database-migration-system.md) - Migration system epic

