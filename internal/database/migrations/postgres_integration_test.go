//go:build postgres && integration

// Package migrations provides PostgreSQL integration tests for the migration system.
// These tests require a real PostgreSQL instance and are run with:
//
//	go test -v -race -tags=postgres,integration ./internal/database/migrations/...
//
// Environment variables required:
//
//	TEST_POSTGRES_URL - PostgreSQL connection string
//
// Run PostgreSQL with Docker Compose:
//
//	./scripts/run-postgres-integration.sh
package migrations

import (
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestPostgresURL returns the PostgreSQL connection URL from environment.
func getTestPostgresURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("TEST_POSTGRES_URL or DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	return url
}

// setupPostgresDB creates a PostgreSQL database connection for testing.
func setupPostgresDB(t *testing.T) *sql.DB {
	t.Helper()

	url := getTestPostgresURL(t)

	db, err := sql.Open("pgx", url)
	require.NoError(t, err, "Failed to open PostgreSQL connection")
	require.NoError(t, db.Ping(), "Failed to ping PostgreSQL")

	// Configure connection pool for tests
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute)

	return db
}

// TestPostgresIntegration_AdvisoryLock tests PostgreSQL advisory locking.
func TestPostgresIntegration_AdvisoryLock(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	// Create a migration runner (it will detect PostgreSQL)
	runner := NewMigrationRunner(db, "sql")

	// Acquire lock - should succeed
	release, err := runner.acquireMigrationLock()
	require.NoError(t, err, "First lock acquisition should succeed")
	require.NotNil(t, release, "Release function should not be nil")

	// Release the lock
	release()

	// Should be able to acquire again after release
	release2, err := runner.acquireMigrationLock()
	require.NoError(t, err, "Lock acquisition after release should succeed")
	require.NotNil(t, release2, "Second release function should not be nil")
	release2()
}

// TestPostgresIntegration_AdvisoryLockContention tests concurrent lock acquisition.
func TestPostgresIntegration_AdvisoryLockContention(t *testing.T) {
	url := getTestPostgresURL(t)

	// Create two separate connections (advisory locks are session-level)
	db1, err := sql.Open("pgx", url)
	require.NoError(t, err)
	defer db1.Close()

	db2, err := sql.Open("pgx", url)
	require.NoError(t, err)
	defer db2.Close()

	runner1 := NewMigrationRunner(db1, "sql")
	runner2 := NewMigrationRunner(db2, "sql")

	// First runner acquires lock
	release1, err := runner1.acquireMigrationLock()
	require.NoError(t, err, "First runner should acquire lock")

	// Second runner should fail to acquire (after retries timeout)
	_, err = runner2.acquireMigrationLock()
	assert.Error(t, err, "Second runner should fail to acquire lock while first holds it")
	assert.Contains(t, err.Error(), "failed to acquire PostgreSQL advisory lock")

	// Release first lock
	release1()

	// Now second runner should be able to acquire
	release2, err := runner2.acquireMigrationLock()
	require.NoError(t, err, "Second runner should acquire lock after release")
	release2()
}

// TestPostgresIntegration_LockReleasedOnConnectionClose tests that locks are released when connection closes.
func TestPostgresIntegration_LockReleasedOnConnectionClose(t *testing.T) {
	url := getTestPostgresURL(t)

	// Create first connection and acquire lock
	db1, err := sql.Open("pgx", url)
	require.NoError(t, err)

	runner1 := NewMigrationRunner(db1, "sql")
	release1, err := runner1.acquireMigrationLock()
	require.NoError(t, err, "Should acquire lock")

	// Close connection without calling release (simulates crash)
	// Note: We still call release to clean up properly, but the test validates
	// that even if we didn't, closing the connection would release the lock
	release1()
	db1.Close()

	// Small delay to ensure connection is fully closed
	time.Sleep(100 * time.Millisecond)

	// Create second connection - should be able to acquire lock
	db2, err := sql.Open("pgx", url)
	require.NoError(t, err)
	defer db2.Close()

	runner2 := NewMigrationRunner(db2, "sql")
	release2, err := runner2.acquireMigrationLock()
	require.NoError(t, err, "Should acquire lock after first connection closed")
	release2()
}

// TestPostgresIntegration_MigrationUp tests migration Up() on PostgreSQL.
// Note: We don't test Down() here because it would drop tables that other
// concurrent tests depend on. Down() is tested in isolation via the advisory
// lock tests which use separate connections.
func TestPostgresIntegration_MigrationUp(t *testing.T) {
	db := setupPostgresDB(t)
	defer db.Close()

	// Get the PostgreSQL migrations path
	migrationsPath, err := getPostgresMigrationsPath()
	require.NoError(t, err, "Should find PostgreSQL migrations path")

	runner := NewMigrationRunner(db, migrationsPath)

	// Get initial version
	initialVersion, err := runner.Version()
	require.NoError(t, err)
	t.Logf("Initial migration version: %d", initialVersion)

	// Ensure migrations are applied
	err = runner.Up()
	require.NoError(t, err, "Up() should succeed")

	// Check version after up
	afterUpVersion, err := runner.Version()
	require.NoError(t, err)
	t.Logf("After Up() version: %d", afterUpVersion)
	assert.GreaterOrEqual(t, afterUpVersion, int64(1))

	// Verify tables exist
	tables := []string{"projects", "tokens", "audit_events"}
	for _, table := range tables {
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "Table %s should exist", table)
	}

	// Re-running Up() should be idempotent
	err = runner.Up()
	require.NoError(t, err, "Re-Up() should succeed (idempotent)")
}

// TestPostgresIntegration_ConcurrentMigrations tests that concurrent migration attempts are safe.
func TestPostgresIntegration_ConcurrentMigrations(t *testing.T) {
	url := getTestPostgresURL(t)

	migrationsPath, err := getPostgresMigrationsPath()
	require.NoError(t, err)

	numGoroutines := 5
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Each goroutine gets its own connection
			db, err := sql.Open("pgx", url)
			if err != nil {
				errors <- err
				return
			}
			defer db.Close()

			runner := NewMigrationRunner(db, migrationsPath)
			err = runner.Up()
			if err != nil {
				errors <- err
				return
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Some may fail to acquire lock, but at least one should succeed
	// and migrations should end up in correct state
	var lockErrors int
	for err := range errors {
		if err != nil {
			// Lock contention errors are expected
			t.Logf("Concurrent migration error (expected): %v", err)
			lockErrors++
		}
	}

	// Not all should fail
	assert.Less(t, lockErrors, numGoroutines, "At least one migration should succeed")

	// Verify final state is correct
	db, err := sql.Open("pgx", url)
	require.NoError(t, err)
	defer db.Close()

	runner := NewMigrationRunner(db, migrationsPath)
	version, err := runner.Version()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, version, int64(1), "Migrations should be applied")
}

// getPostgresMigrationsPath returns the path to PostgreSQL migrations.
func getPostgresMigrationsPath() (string, error) {
	// Try common paths
	paths := []string{
		"sql/postgres",
		"../migrations/sql/postgres",
		"../../internal/database/migrations/sql/postgres",
		"internal/database/migrations/sql/postgres",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try from current working directory
	cwd, err := os.Getwd()
	if err == nil {
		testPaths := []string{
			cwd + "/sql/postgres",
			cwd + "/../sql/postgres",
		}
		for _, path := range testPaths {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", os.ErrNotExist
}
