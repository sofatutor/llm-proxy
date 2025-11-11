// Package main installs the database migration system.
// This is a self-contained installer that includes all necessary files.
//
// Usage: go run scripts/complete_migration_install.go
//
// This will:
// 1. Create internal/database/migrations/ directory
// 2. Create all necessary files (runner.go, runner_test.go, README.md)
// 3. Add goose dependency
// 4. Run tests
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const runnerGoContent = `// Package migrations provides database migration functionality using goose.
package migrations

import (
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
)

// MigrationRunner manages database migrations using goose.
type MigrationRunner struct {
	db             *sql.DB
	migrationsPath string
}

// NewMigrationRunner creates a new migration runner.
// db is the database connection, migrationsPath is the directory containing SQL migration files.
func NewMigrationRunner(db *sql.DB, migrationsPath string) *MigrationRunner {
	return &MigrationRunner{
		db:             db,
		migrationsPath: migrationsPath,
	}
}

// Up applies all pending migrations.
// Each migration runs in a transaction and will be rolled back if it fails.
func (m *MigrationRunner) Up() error {
	if m.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	if m.migrationsPath == "" {
		return fmt.Errorf("migrations path is empty")
	}

	// Set goose dialect to sqlite3
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Run migrations
	if err := goose.Up(m.db, m.migrationsPath); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// Down rolls back the most recently applied migration.
// The rollback runs in a transaction.
func (m *MigrationRunner) Down() error {
	if m.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	if m.migrationsPath == "" {
		return fmt.Errorf("migrations path is empty")
	}

	// Set goose dialect to sqlite3
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Roll back one migration
	if err := goose.Down(m.db, m.migrationsPath); err != nil {
		return fmt.Errorf("failed to roll back migration: %w", err)
	}

	return nil
}

// Status returns the current migration version.
// Returns 0 if no migrations have been applied.
func (m *MigrationRunner) Status() (int64, error) {
	if m.db == nil {
		return 0, fmt.Errorf("database connection is nil")
	}
	if m.migrationsPath == "" {
		return 0, fmt.Errorf("migrations path is empty")
	}

	// Set goose dialect to sqlite3
	if err := goose.SetDialect("sqlite3"); err != nil {
		return 0, fmt.Errorf("failed to set goose dialect: %w", err)
	}

	// Get current version
	version, err := goose.GetDBVersion(m.db)
	if err != nil {
		return 0, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, nil
}

// Version is an alias for Status(). Returns the current migration version.
func (m *MigrationRunner) Version() (int64, error) {
	return m.Status()
}
`

const runnerTestGoContent = `package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err, "failed to open test database")
	require.NoError(t, db.Ping(), "failed to ping test database")
	return db
}

// createTempMigrationsDir creates a temporary directory with test migrations
func createTempMigrationsDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	err := os.MkdirAll(migrationsDir, 0755)
	require.NoError(t, err, "failed to create temp migrations directory")
	return migrationsDir
}

// writeMigrationFile writes a migration file to the given directory
func writeMigrationFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "failed to write migration file")
}

func TestNewMigrationRunner(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	assert.NotNil(t, runner)
	assert.Equal(t, migrationsDir, runner.migrationsPath)
}

func TestMigrationRunner_Up_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", ` + "`" + `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	err := runner.Up()
	require.NoError(t, err, "Up() should succeed")
	
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&tableName)
	require.NoError(t, err, "test_table should exist")
	assert.Equal(t, "test_table", tableName)
}

func TestMigrationRunner_Up_MultipleMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_create_users.sql", ` + "`" + `
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
` + "`" + `)
	
	writeMigrationFile(t, migrationsDir, "00002_create_posts.sql", ` + "`" + `
-- +goose Up
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- +goose Down
DROP TABLE posts;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	err := runner.Up()
	require.NoError(t, err, "Up() should succeed with multiple migrations")
	
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('users', 'posts')").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both tables should exist")
}

func TestMigrationRunner_Up_AlreadyApplied(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", ` + "`" + `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	err := runner.Up()
	require.NoError(t, err, "first Up() should succeed")
	
	err = runner.Up()
	require.NoError(t, err, "second Up() should succeed (idempotent)")
}

func TestMigrationRunner_Down(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", ` + "`" + `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	err := runner.Up()
	require.NoError(t, err, "Up() should succeed")
	
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&tableName)
	require.NoError(t, err, "test_table should exist after Up()")
	
	err = runner.Down()
	require.NoError(t, err, "Down() should succeed")
	
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&tableName)
	assert.Error(t, err, "test_table should not exist after Down()")
	assert.Equal(t, sql.ErrNoRows, err, "should get no rows error")
}

func TestMigrationRunner_Version_NoMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	runner := NewMigrationRunner(db, migrationsDir)
	
	version, err := runner.Version()
	require.NoError(t, err, "Version() should succeed even with no migrations")
	assert.Equal(t, int64(0), version, "version should be 0 when no migrations applied")
}

func TestMigrationRunner_Version_AfterUp(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", ` + "`" + `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
` + "`" + `)
	
	writeMigrationFile(t, migrationsDir, "00002_add_email.sql", ` + "`" + `
-- +goose Up
ALTER TABLE test_table ADD COLUMN email TEXT;

-- +goose Down
SELECT 1;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	err := runner.Up()
	require.NoError(t, err)
	
	version, err := runner.Version()
	require.NoError(t, err)
	assert.Equal(t, int64(2), version, "version should be 2 after applying 2 migrations")
}

func TestMigrationRunner_Status(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", ` + "`" + `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	status, err := runner.Status()
	require.NoError(t, err)
	assert.Equal(t, int64(0), status, "initial status should be 0")
	
	err = runner.Up()
	require.NoError(t, err)
	
	status, err = runner.Status()
	require.NoError(t, err)
	assert.Equal(t, int64(1), status, "status should be 1 after applying migration")
}

func TestMigrationRunner_InvalidMigrationSQL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_invalid.sql", ` + "`" + `
-- +goose Up
THIS IS INVALID SQL;

-- +goose Down
DROP TABLE nonexistent;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	err := runner.Up()
	assert.Error(t, err, "Up() should fail with invalid SQL")
}

func TestMigrationRunner_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_partial_failure.sql", ` + "`" + `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY
);
INVALID SQL STATEMENT;

-- +goose Down
DROP TABLE test_table;
` + "`" + `)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	err := runner.Up()
	require.Error(t, err, "Up() should fail")
	
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&tableName)
	assert.Error(t, err, "test_table should not exist after rollback")
}

func TestMigrationRunner_NilDatabase(t *testing.T) {
	migrationsDir := createTempMigrationsDir(t)
	
	runner := NewMigrationRunner(nil, migrationsDir)
	assert.NotNil(t, runner, "should not panic with nil database")
	
	err := runner.Up()
	assert.Error(t, err, "Up() should return error with nil database")
	
	err = runner.Down()
	assert.Error(t, err, "Down() should return error with nil database")
	
	_, err = runner.Version()
	assert.Error(t, err, "Version() should return error with nil database")
}

func TestMigrationRunner_EmptyMigrationsPath(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	runner := NewMigrationRunner(db, "")
	
	err := runner.Up()
	assert.Error(t, err, "should return error for empty migrations path")
	
	err = runner.Down()
	assert.Error(t, err, "Down() should return error for empty migrations path")
	
	_, err = runner.Version()
	assert.Error(t, err, "Version() should return error for empty migrations path")
}

func TestMigrationRunner_NonexistentMigrationsPath(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	runner := NewMigrationRunner(db, "/nonexistent/path/to/migrations")
	
	err := runner.Up()
	assert.Error(t, err, "should return error for nonexistent migrations path")
}
`

const readmeMdContent = `# Database Migrations

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

` + "```go" + `
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
` + "```" + `

## API Reference

### MigrationRunner

- ` + "`NewMigrationRunner(db *sql.DB, migrationsPath string) *MigrationRunner`" + ` - Create runner
- ` + "`Up() error`" + ` - Apply all pending migrations
- ` + "`Down() error`" + ` - Roll back last migration
- ` + "`Status() (int64, error)`" + ` - Get current version
- ` + "`Version() (int64, error)`" + ` - Alias for Status()

## Migration File Format

Format: ` + "`{version}_{description}.sql`" + `

Example:
` + "```sql" + `
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
` + "```" + `

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
`

func main() {
	projectRoot := "/home/runner/work/llm-proxy/llm-proxy"
	migrationsDir := filepath.Join(projectRoot, "internal", "database", "migrations")
	sqlDir := filepath.Join(migrationsDir, "sql")

	fmt.Println("=== Complete Migration Installation ===\n")

	// Step 1
	fmt.Println("Step 1: Creating directories...")
	if err := os.MkdirAll(sqlDir, 0755); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Created %s\n", migrationsDir)
	fmt.Printf("✓ Created %s\n\n", sqlDir)

	// Step 2
	fmt.Println("Step 2: Writing implementation files...")
	files := map[string]string{
		"runner.go":      runnerGoContent,
		"runner_test.go": runnerTestGoContent,
		"README.md":      readmeMdContent,
	}

	for filename, content := range files {
		path := filepath.Join(migrationsDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Printf("❌ Error writing %s: %v\n", filename, err)
			os.Exit(1)
		}
		fmt.Printf("✓ Created %s\n", filename)
	}
	fmt.Println()

	// Step 3
	fmt.Println("Step 3: Adding goose dependency...")
	cmd := exec.Command("go", "get", "-u", "github.com/pressly/goose/v3")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ Error: %v\n%s\n", err, output)
		os.Exit(1)
	}
	fmt.Println("✓ Added github.com/pressly/goose/v3\n")

	// Step 4
	fmt.Println("Step 4: Tidying dependencies...")
	cmd = exec.Command("go", "mod", "tidy")
	cmd.Dir = projectRoot
	if output, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("❌ Error: %v\n%s\n", err, output)
		os.Exit(1)
	}
	fmt.Println("✓ Dependencies tidied\n")

	// Step 5
	fmt.Println("Step 5: Running migration tests...")
	cmd = exec.Command("go", "test", "./internal/database/migrations/...", "-v", "-race")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("\n⚠️  Tests failed: %v\n", err)
		fmt.Println("Check test output above for details.")
		os.Exit(1)
	}

	fmt.Println("\n✅ Migration system installation complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Run 'make test' to verify full test suite")
	fmt.Println("  2. Run 'make test-coverage-ci' to check coverage")
	fmt.Println("  3. Proceed to Story 1.2: Convert existing schema")
}
`

func main() {
	projectRoot := "/home/runner/work/llm-proxy/llm-proxy"
	scriptPath := filepath.Join(projectRoot, "scripts", "complete_migration_install.go")
	
	if err := os.WriteFile(scriptPath, []byte(installerContent), 0644); err != nil {
		fmt.Printf("Error creating installer: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("✓ Created complete_migration_install.go")
	fmt.Println("\nTo complete installation, run:")
	fmt.Println("  go run scripts/complete_migration_install.go")
}

const installerContent = `INSTALLER_CODE_HERE`
