package migrations

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
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
`)
	
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
	
	writeMigrationFile(t, migrationsDir, "00001_create_users.sql", `
-- +goose Up
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE users;
`)
	
	writeMigrationFile(t, migrationsDir, "00002_create_posts.sql", `
-- +goose Up
CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

-- +goose Down
DROP TABLE posts;
`)
	
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
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
`)
	
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
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
`)
	
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
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
`)
	
	writeMigrationFile(t, migrationsDir, "00002_add_email.sql", `
-- +goose Up
ALTER TABLE test_table ADD COLUMN email TEXT;

-- +goose Down
SELECT 1;
`)
	
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
	
	writeMigrationFile(t, migrationsDir, "00001_create_test_table.sql", `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL
);

-- +goose Down
DROP TABLE test_table;
`)
	
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
	
	writeMigrationFile(t, migrationsDir, "00001_invalid.sql", `
-- +goose Up
THIS IS INVALID SQL;

-- +goose Down
DROP TABLE nonexistent;
`)
	
	runner := NewMigrationRunner(db, migrationsDir)
	
	err := runner.Up()
	assert.Error(t, err, "Up() should fail with invalid SQL")
}

func TestMigrationRunner_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	migrationsDir := createTempMigrationsDir(t)
	
	writeMigrationFile(t, migrationsDir, "00001_partial_failure.sql", `
-- +goose Up
CREATE TABLE test_table (
    id INTEGER PRIMARY KEY
);
INVALID SQL STATEMENT;

-- +goose Down
DROP TABLE test_table;
`)
	
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

