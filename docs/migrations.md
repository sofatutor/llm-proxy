# Database Migrations Guide

This guide covers the database migration system for llm-proxy, including how to create, run, and manage migrations.

## Overview

The llm-proxy uses [goose](https://github.com/pressly/goose) for database migrations. Migrations are versioned SQL files that allow you to:

- Track schema changes over time
- Apply changes consistently across environments
- Roll back changes when needed
- Prevent schema drift between environments

## Migration Files

Migration files are located in `internal/database/migrations/sql/` and follow the naming pattern:

```
{version}_{description}.sql
```

Example: `00001_initial_schema.sql`

### Migration File Format

Each migration file contains both "up" and "down" migrations:

```sql
-- +goose Up
-- SQL in this section is executed when the migration is applied
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back
DROP TABLE IF NOT EXISTS projects;
```

**Important Notes:**
- `-- +goose Up` marks the beginning of the "up" migration
- `-- +goose Down` marks the beginning of the "down" migration
- Both sections are required
- Use `IF NOT EXISTS` / `IF EXISTS` clauses for idempotency
- Test both up and down migrations before committing

## CLI Commands

### Apply Migrations

Apply all pending migrations:

```bash
llm-proxy migrate up
```

Or specify a custom database path:

```bash
llm-proxy migrate up --db /path/to/database.db
```

### Check Migration Status

View the current migration version:

```bash
llm-proxy migrate status
# or
llm-proxy migrate version
```

### Rollback Migrations

Roll back the most recently applied migration:

```bash
llm-proxy migrate down
```

**Warning:** Only roll back migrations in development. Production rollbacks should be carefully planned and tested.

## Automatic Migrations

Migrations run automatically in two scenarios:

1. **During Setup**: When you run `llm-proxy setup`, migrations are applied automatically
2. **Server Startup**: When the server starts, migrations are applied automatically via `database.New()`

If migrations fail during setup, you'll see a warning message and can run them manually later.

### Concurrency in Distributed Systems

**Important**: In distributed systems with multiple server instances, migrations run automatically on each instance startup. While migrations are idempotent and SQLite/PostgreSQL provide locking mechanisms, for production deployments with multiple instances, consider:

1. **Running migrations before starting instances** (using init containers or deployment scripts)
2. **Using advisory locks** (see [Migration Concurrency Guide](migrations-concurrency.md) for details)

The current implementation is safe for most scenarios due to:
- Idempotent migrations (`IF NOT EXISTS` clauses)
- Database-level locking (SQLite file locks, PostgreSQL row locks)
- Version tracking in `goose_db_version` table

For production deployments with many concurrent instances, see the [Migration Concurrency Guide](migrations-concurrency.md) for best practices and enhancement options.

## Creating New Migrations

### Step 1: Create Migration File

Create a new migration file in `internal/database/migrations/sql/`:

```bash
# Example: Create migration 00002_add_user_table.sql
touch internal/database/migrations/sql/00002_add_user_table.sql
```

**Naming Convention:**
- Use sequential version numbers (00001, 00002, etc.)
- Use descriptive names (snake_case)
- Never reuse version numbers
- Never modify existing migration files after they've been applied

### Step 2: Write Up Migration

Add the SQL to create/modify your schema:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
```

### Step 3: Write Down Migration

Add the SQL to reverse your changes:

```sql
-- +goose Down
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
```

### Step 4: Test Locally

Test your migration:

```bash
# Create a test database
TEST_DB="test_migration.db"

# Apply migration
llm-proxy migrate up --db "$TEST_DB"

# Check status
llm-proxy migrate status --db "$TEST_DB"

# Rollback
llm-proxy migrate down --db "$TEST_DB"

# Clean up
rm "$TEST_DB"
```

### Step 5: Commit and Test in CI

After committing, CI will automatically validate:
- Migration files are valid SQL
- Migrations can be applied to a clean database
- Migrations can be rolled back

## Best Practices

### 1. Always Write Down Migrations

Every migration must have a corresponding down migration. This enables:
- Safe rollbacks in development
- Testing migration reversibility
- Recovery from failed migrations

### 2. Use Idempotent SQL

Prefer `IF NOT EXISTS` / `IF EXISTS` clauses:

```sql
-- Good: Idempotent
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Bad: Will fail if index already exists
CREATE INDEX idx_users_email ON users(email);
```

### 3. Test Both Directions

Always test both up and down migrations:

```bash
# Test up
llm-proxy migrate up --db test.db

# Test down
llm-proxy migrate down --db test.db

# Test up again
llm-proxy migrate up --db test.db
```

### 4. Never Modify Applied Migrations

Once a migration has been applied to any environment:
- **Never** modify the migration file
- **Never** delete the migration file
- Create a new migration to fix issues

### 5. Use Transactions When Possible

Goose wraps each migration in a transaction automatically. However, some SQLite operations (like `ALTER TABLE`) cannot be rolled back. Be careful with:
- `ALTER TABLE` operations
- `DROP TABLE` operations
- Schema changes that affect existing data

### 6. Handle Existing Data

When adding columns or constraints to existing tables:

```sql
-- +goose Up
-- Add column with default value for existing rows
ALTER TABLE projects ADD COLUMN is_active BOOLEAN NOT NULL DEFAULT 1;

-- +goose Down
-- Note: SQLite doesn't support DROP COLUMN directly
-- You may need to recreate the table or use a workaround
```

### 7. Document Complex Migrations

Add comments explaining complex logic:

```sql
-- +goose Up
-- This migration adds a new column and backfills data from an existing table
-- We use a transaction to ensure atomicity
BEGIN;

ALTER TABLE projects ADD COLUMN metadata TEXT;

-- Backfill existing data
UPDATE projects SET metadata = '{}' WHERE metadata IS NULL;

COMMIT;
```

## Troubleshooting

### Migration Fails During Setup

If migrations fail during `llm-proxy setup`:

1. Check the error message
2. Run migrations manually: `llm-proxy migrate up`
3. Verify database permissions
4. Check that migrations directory is accessible

### Migration Version Mismatch

If you see version mismatch errors:

```bash
# Check current version
llm-proxy migrate status

# Check migration files
ls -la internal/database/migrations/sql/
```

### Cannot Rollback Migration

Some migrations cannot be rolled back (e.g., data loss operations). In these cases:

1. Create a new migration to restore data
2. Document the limitation in migration comments
3. Consider using a backup before applying risky migrations

### Migration File Not Found

If you see "migrations directory not found":

1. Verify you're running from the project root
2. Check that `internal/database/migrations/sql/` exists
3. Try specifying the full path to migrations (if using custom setup)

## CI Validation

The CI pipeline automatically validates migrations:

- ✅ Migration files exist and are valid SQL
- ✅ Migrations can be applied to a clean database
- ✅ Migrations can be rolled back

If CI validation fails, fix the migration before merging.

## Related Documentation

- [CLI Reference](cli-reference.md) - Complete CLI command documentation
- [Database Package](../internal/database/README.md) - Database package overview
- [Migration Runner](../internal/database/migrations/README.md) - Technical migration runner details

## Migration History

Current migrations:

- `00001_initial_schema.sql` - Initial database schema (projects, tokens, audit_events tables)

To see all migrations:

```bash
ls -la internal/database/migrations/sql/
```

