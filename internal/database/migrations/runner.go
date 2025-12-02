// Package migrations provides database migration functionality using goose.
package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"time"

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
// Advisory locking is used to prevent concurrent migrations in distributed systems.
func (m *MigrationRunner) Up() error {
	if m.db == nil {
		return fmt.Errorf("database connection is nil")
	}
	if m.migrationsPath == "" {
		return fmt.Errorf("migrations path is empty")
	}

	// Acquire advisory lock to prevent concurrent migrations
	release, err := m.acquireMigrationLock()
	if err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer release()

	// Detect database driver and set goose dialect
	driverName, err := m.detectDriver()
	if err != nil {
		return fmt.Errorf("failed to detect database driver: %w", err)
	}

	if err := goose.SetDialect(driverName); err != nil {
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

	// Acquire migration lock to prevent concurrent operations
	release, err := m.acquireMigrationLock()
	if err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer release()

	// Detect database driver and set goose dialect
	driverName, err := m.detectDriver()
	if err != nil {
		return fmt.Errorf("failed to detect database driver: %w", err)
	}

	if err := goose.SetDialect(driverName); err != nil {
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

	// Detect database driver and set goose dialect
	driverName, err := m.detectDriver()
	if err != nil {
		return 0, fmt.Errorf("failed to detect database driver: %w", err)
	}

	if err := goose.SetDialect(driverName); err != nil {
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

// detectDriver detects the database driver from the connection.
// Returns the goose dialect name: "sqlite3" or "postgres".
func (m *MigrationRunner) detectDriver() (string, error) {
	if m.db == nil {
		return "", fmt.Errorf("database connection is nil")
	}

	// Get driver name from connection
	driverName := m.db.Driver()
	if driverName == nil {
		return "", fmt.Errorf("driver is nil")
	}

	driverType := fmt.Sprintf("%T", driverName)

	// Detect SQLite
	if driverType == "*sqlite3.SQLiteDriver" || driverType == "*sqlite3.SQLiteConn" {
		return "sqlite3", nil
	}

	// Detect PostgreSQL (common drivers: lib/pq, pgx)
	if driverType == "*pq.driver" || driverType == "*pgx.Conn" || driverType == "*pgxpool.Pool" {
		return "postgres", nil
	}

	// Try to detect via connection string or query
	// For SQLite, try a SQLite-specific query
	var result int
	err := m.db.QueryRow("SELECT 1").Scan(&result)
	if err == nil {
		// Try SQLite-specific pragma
		var journalMode string
		pragmaErr := m.db.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
		if pragmaErr == nil {
			return "sqlite3", nil
		}
	}

	// Default to sqlite3 for backward compatibility
	// This will be updated when PostgreSQL support is added
	return "sqlite3", nil
}

// acquireMigrationLock acquires an advisory lock to prevent concurrent migrations.
// Returns a release function that must be called to release the lock.
func (m *MigrationRunner) acquireMigrationLock() (func(), error) {
	driverName, err := m.detectDriver()
	if err != nil {
		return nil, fmt.Errorf("failed to detect driver for locking: %w", err)
	}

	switch driverName {
	case "sqlite3":
		return m.acquireSQLiteLock()
	case "postgres":
		return m.acquirePostgresLock()
	default:
		return m.acquireSQLiteLock() // Default to SQLite lock for backward compatibility
	}
}

// acquireSQLiteLock acquires a lock using a SQLite lock table.
// This prevents concurrent migrations when multiple instances start simultaneously.
func (m *MigrationRunner) acquireSQLiteLock() (func(), error) {
	// Create lock table if it doesn't exist
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS migration_lock (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			locked BOOLEAN NOT NULL DEFAULT 0,
			locked_at DATETIME,
			locked_by TEXT,
			process_id INTEGER
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock table: %w", err)
	}

	// Initialize lock row if it doesn't exist
	_, _ = m.db.Exec(`INSERT OR IGNORE INTO migration_lock (id, locked) VALUES (1, 0)`)

	// Try to acquire lock with retries
	maxRetries := 10
	retryDelay := 100 * time.Millisecond
	processID := os.Getpid()

	for i := 0; i < maxRetries; i++ {
		// Use a transaction to atomically check and acquire lock
		tx, err := m.db.Begin()
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}

		var locked bool
		err = tx.QueryRow(`SELECT locked FROM migration_lock WHERE id = 1`).Scan(&locked)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to read lock status: %w", err)
		}

		if locked {
			_ = tx.Rollback()
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("migration lock is already held by another process (retried %d times)", maxRetries)
		}

		// Acquire lock
		result, err := tx.Exec(`
			UPDATE migration_lock 
			SET locked = 1, locked_at = CURRENT_TIMESTAMP, locked_by = ?, process_id = ?
			WHERE id = 1 AND locked = 0
		`, fmt.Sprintf("pid-%d", processID), processID)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}

		// Verify that the update actually affected a row (lock was acquired)
		rowsAffected, err := result.RowsAffected()
		if err != nil || rowsAffected == 0 {
			_ = tx.Rollback()
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("failed to acquire lock: another process may have acquired it")
		}

		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit lock acquisition: %w", err)
		}

		// Verify lock was acquired
		var isLocked bool
		err = m.db.QueryRow(`SELECT locked FROM migration_lock WHERE id = 1`).Scan(&isLocked)
		if err != nil || !isLocked {
			if i < maxRetries-1 {
				time.Sleep(retryDelay)
				continue
			}
			return nil, fmt.Errorf("failed to verify lock acquisition")
		}

		// Return release function
		release := func() {
			_, _ = m.db.Exec(`UPDATE migration_lock SET locked = 0 WHERE id = 1`)
		}

		return release, nil
	}

	return nil, fmt.Errorf("failed to acquire migration lock after %d retries", maxRetries)
}

// acquirePostgresLock acquires an advisory lock using PostgreSQL's pg_advisory_lock.
// This prevents concurrent migrations when multiple instances start simultaneously.
// The lock is automatically released when the connection closes.
func (m *MigrationRunner) acquirePostgresLock() (func(), error) {
	// Use a fixed lock ID for migrations (derived from "llm-proxy-migrations")
	// This ID is unique enough for our purposes and consistent across instances
	const lockID = 3141592653 // A fixed number to identify this application's migration lock

	maxRetries := 10
	retryDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		// Try to acquire the advisory lock (non-blocking)
		var acquired bool
		err := m.db.QueryRow("SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
		if err != nil {
			return nil, fmt.Errorf("failed to try advisory lock: %w", err)
		}

		if acquired {
			// Lock acquired successfully
			release := func() {
				_, _ = m.db.Exec("SELECT pg_advisory_unlock($1)", lockID)
			}
			return release, nil
		}

		// Lock not acquired, wait and retry
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil, fmt.Errorf("failed to acquire PostgreSQL advisory lock after %d retries", maxRetries)
}
