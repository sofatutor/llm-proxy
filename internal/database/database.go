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
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// DB represents the database connection.
type DB struct {
	db *sql.DB
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

	// Initialize database (create tables, indexes)
	if err := initDatabase(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &DB{db: db}, nil
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

// initDatabase initializes the database with the necessary schema.
func initDatabase(db *sql.DB) error {
	// Create tables and indexes if they don't exist
	_, err := db.Exec(`
	-- Projects table
	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL UNIQUE,
		openai_api_key TEXT NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT 1,
		deactivated_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Create index on project name
	CREATE INDEX IF NOT EXISTS idx_projects_name ON projects(name);

	-- Tokens table
	CREATE TABLE IF NOT EXISTS tokens (
		token TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		expires_at DATETIME,
		is_active BOOLEAN NOT NULL DEFAULT 1,
		deactivated_at DATETIME,
		request_count INTEGER NOT NULL DEFAULT 0,
		max_requests INTEGER,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	-- Create indexes on tokens
	CREATE INDEX IF NOT EXISTS idx_tokens_project_id ON tokens(project_id);
	CREATE INDEX IF NOT EXISTS idx_tokens_expires_at ON tokens(expires_at);
	CREATE INDEX IF NOT EXISTS idx_tokens_is_active ON tokens(is_active);

	-- Audit events table for security logging and firewall rule derivation
	CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		action TEXT NOT NULL,
		actor TEXT NOT NULL,
		project_id TEXT,
		request_id TEXT,
		correlation_id TEXT,
		client_ip TEXT,
		method TEXT,
		path TEXT,
		user_agent TEXT,
		outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failure')),
		reason TEXT,
		token_id TEXT,
		metadata TEXT
	);

	-- Create indexes on audit events for performance and firewall rule queries
	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_events(action);
	CREATE INDEX IF NOT EXISTS idx_audit_project_id ON audit_events(project_id);
	CREATE INDEX IF NOT EXISTS idx_audit_client_ip ON audit_events(client_ip);
	CREATE INDEX IF NOT EXISTS idx_audit_request_id ON audit_events(request_id);
	CREATE INDEX IF NOT EXISTS idx_audit_outcome ON audit_events(outcome);
	CREATE INDEX IF NOT EXISTS idx_audit_ip_action ON audit_events(client_ip, action);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Apply schema migrations for existing databases
	if err := applyMigrations(db); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// applyMigrations applies database schema changes for existing databases
func applyMigrations(db *sql.DB) error {
	// Add is_active and deactivated_at columns to projects table if they don't exist
	err := addColumnIfNotExists(db, "projects", "is_active", "BOOLEAN NOT NULL DEFAULT 1")
	if err != nil {
		return fmt.Errorf("failed to add is_active column to projects: %w", err)
	}

	err = addColumnIfNotExists(db, "projects", "deactivated_at", "DATETIME")
	if err != nil {
		return fmt.Errorf("failed to add deactivated_at column to projects: %w", err)
	}

	// Add deactivated_at column to tokens table if it doesn't exist
	err = addColumnIfNotExists(db, "tokens", "deactivated_at", "DATETIME")
	if err != nil {
		return fmt.Errorf("failed to add deactivated_at column to tokens: %w", err)
	}

	return nil
}

// addColumnIfNotExists adds a column to a table if it doesn't already exist
func addColumnIfNotExists(db *sql.DB, tableName, columnName, columnType string) error {
	// Check if column exists
	query := `SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`
	var count int
	err := db.QueryRow(query, tableName, columnName).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check if column exists: %w", err)
	}

	// Add column if it doesn't exist
	if count == 0 {
		alterQuery := fmt.Sprintf("ALTER TABLE \"%s\" ADD COLUMN \"%s\" %s", tableName, columnName, columnType)
		_, err = db.Exec(alterQuery)
		if err != nil {
			return fmt.Errorf("failed to add column %s to table %s: %w", columnName, tableName, err)
		}
	}

	return nil
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
