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

	// Initialize SQLite schema (not migrations - SQLite uses schema.sql directly)
	if err := initSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
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

// getSchemaPath returns the path to the SQLite schema file.
// SQLite uses a single schema file instead of migrations.
func getSchemaPath() (string, error) {
	// Strategy 1: Relative path from current working directory (development)
	cwdPath := filepath.Join("scripts", "schema.sql")
	if _, err := os.Stat(cwdPath); err == nil {
		return cwdPath, nil
	}

	// Strategy 2: Path relative to this source file (for tests)
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		srcPath := filepath.Join(repoRoot, "scripts", "schema.sql")
		if _, err := os.Stat(srcPath); err == nil {
			return srcPath, nil
		}
	}

	// Strategy 3: Relative path from executable location (production)
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		execSchemaPath := filepath.Join(execDir, "scripts", "schema.sql")
		if _, err := os.Stat(execSchemaPath); err == nil {
			return execSchemaPath, nil
		}
		// Also try parent directory (for bin/ structure)
		parentSchemaPath := filepath.Join(filepath.Dir(execDir), "scripts", "schema.sql")
		if _, err := os.Stat(parentSchemaPath); err == nil {
			return parentSchemaPath, nil
		}
	}

	return "", fmt.Errorf("schema.sql not found in any expected location")
}

// initSQLiteSchema initializes the SQLite database from schema.sql.
// SQLite does NOT use migrations - only the current schema file.
func initSQLiteSchema(db *sql.DB) error {
	schemaPath, err := getSchemaPath()
	if err != nil {
		return fmt.Errorf("failed to get schema path: %w", err)
	}

	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	_, err = db.Exec(string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	return nil
}

// DBInitForTests is a helper to ensure schema exists in tests. No-op if db is nil.
func DBInitForTests(d *DB) error {
	if d == nil || d.db == nil {
		return nil
	}
	return initSQLiteSchema(d.db)
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
