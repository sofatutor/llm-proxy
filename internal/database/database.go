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
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
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
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, 0755)
	} else if err != nil {
		return err
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
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Transaction executes the given function within a transaction.
func (d *DB) Transaction(ctx context.Context, fn func(*sql.Tx) error) error {
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
