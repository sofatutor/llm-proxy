//go:build !postgres && !mysql

package migrations

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationRunner_AcquirePostgresLock_Stub tests the PostgreSQL lock stub
// which is used when the postgres build tag is not set.
// The real PostgreSQL locking will be tested via Docker Compose integration tests (issue #139).
func TestMigrationRunner_AcquirePostgresLock_Stub(t *testing.T) {
	db, err := setupTestDBForStub()
	require.NoError(t, err, "failed to setup test database")
	defer func() { _ = db.Close() }()

	runner := NewMigrationRunner(db, "")

	// The stub should return an error indicating PostgreSQL build tag is required
	release, err := runner.acquirePostgresLock()
	assert.Error(t, err)
	assert.Nil(t, release)
	assert.Contains(t, err.Error(), "postgres")
}

// TestMigrationRunner_AcquireMySQLLock_Stub tests the MySQL lock stub
// which is used when the mysql build tag is not set.
// The real MySQL locking is tested via Docker Compose integration tests (issue #245).
func TestMigrationRunner_AcquireMySQLLock_Stub(t *testing.T) {
	db, err := setupTestDBForStub()
	require.NoError(t, err, "failed to setup test database")
	defer func() { _ = db.Close() }()

	runner := NewMigrationRunner(db, "")

	// The stub should return an error indicating MySQL build tag is required
	release, err := runner.acquireMySQLLock()
	assert.Error(t, err)
	assert.Nil(t, release)
	assert.Contains(t, err.Error(), "mysql")
}

// setupTestDBForStub creates a test database for stub tests
func setupTestDBForStub() (*sql.DB, error) {
	return sql.Open("sqlite3", ":memory:")
}
