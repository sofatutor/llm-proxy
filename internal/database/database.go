// Package database provides SQLite database operations for the LLM Proxy.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
	"github.com/sofatutor/llm-proxy/internal/database/migrations"
)

// DB represents the database connection.
type DB struct {
	db     *sql.DB
	driver DriverType
}

// Config contains the database configuration.
type Config struct {
	// Path is the path to the SQLite database file.
	Path string
	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	ConnMaxLifetime time.Duration
}

// DefaultConfig returns a default database configuration.
func DefaultConfig() Config {
	return Config{
		Path:            "data/llm-proxy.db",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	}
}

// New creates a new database connection.
func New(config Config) (*DB, error) {
	// Ensure database directory exists
	if err := ensureDirExists(filepath.Dir(config.Path)); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open connection
	db, err := sql.Open("sqlite3", config.Path+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	// Special case: in-memory SQLite databases are per-connection. Use a single connection
	// to ensure schema and data are visible across queries within the same *sql.DB handle.
	if config.Path == ":memory:" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxOpenConns(config.MaxOpenConns)
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test the connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run database migrations
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{db: db, driver: DriverSQLite}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.db != nil {
		_ = d.db.Close()
	}
	return nil
}

// ensureDirExists creates the directory if it doesn't exist.
func ensureDirExists(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, 0755)
	} else if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("path %s exists and is not a directory", dir)
	}
	return nil
}

// getMigrationsPath returns the path to the migrations directory.
// It tries multiple strategies to locate the migrations:
// 1. Relative path from current working directory (for development)
// 2. Path relative to this source file (for tests)
// 3. Relative path from executable location (for production)
// Debug logging is included to help diagnose path resolution issues in production.
func getMigrationsPath() (string, error) {
	var triedPaths []string

	// Try relative path from current working directory first (development)
	relPath := "internal/database/migrations/sql"
	triedPaths = append(triedPaths, relPath)
	if _, err := os.Stat(relPath); err == nil {
		return relPath, nil
	}

	// Try path relative to this source file (for tests)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		// Get directory of this file (database.go)
		sourceDir := filepath.Dir(filename)
		// migrations/sql is sibling to database package
		sourceRelPath := filepath.Join(sourceDir, "migrations", "sql")
		triedPaths = append(triedPaths, sourceRelPath)
		if _, err := os.Stat(sourceRelPath); err == nil {
			return sourceRelPath, nil
		}
	}

	// Try to get path relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// Try relative to executable directory
		execRelPath := filepath.Join(execDir, "internal/database/migrations/sql")
		triedPaths = append(triedPaths, execRelPath)
		if _, err := os.Stat(execRelPath); err == nil {
			return execRelPath, nil
		}
		// Try relative to executable's parent (if executable is in bin/)
		binRelPath := filepath.Join(filepath.Dir(execDir), "internal/database/migrations/sql")
		triedPaths = append(triedPaths, binRelPath)
		if _, err := os.Stat(binRelPath); err == nil {
			return binRelPath, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found: tried paths %v", triedPaths)
}

// runMigrations runs database migrations using the migration runner.
func runMigrations(db *sql.DB) error {
	migrationsPath, err := getMigrationsPath()
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	runner := migrations.NewMigrationRunner(db, migrationsPath)
	if err := runner.Up(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// initDatabase is deprecated. Use runMigrations instead.
// Kept for backward compatibility with DBInitForTests.
func initDatabase(db *sql.DB) error {
	return runMigrations(db)
}

// DBInitForTests is a helper to ensure schema exists in tests. No-op if db is nil.
func DBInitForTests(d *DB) error {
	if d == nil || d.db == nil {
		return nil
	}
	return initDatabase(d.db)
}

// Transaction executes the given function within a transaction.
func (d *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("database is nil")
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// If the function panics, rollback the transaction
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p) // Re-throw the panic after rolling back
		}
	}()

	// Execute the function
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DB returns the underlying sql.DB instance.
func (d *DB) DB() *sql.DB {
	return d.db
}

// Driver returns the database driver type.
func (d *DB) Driver() DriverType {
	return d.driver
}

// HealthCheck performs a health check on the database connection.
// It verifies that the database is reachable and responsive.
func (d *DB) HealthCheck(ctx context.Context) error {
	if d == nil || d.db == nil {
		return fmt.Errorf("database is nil")
	}

	// Test the connection with a simple query
	if err := d.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Verify we can execute a simple query
	var result int
	err := d.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	return nil
}
