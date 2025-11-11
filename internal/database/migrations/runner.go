// Package migrations provides database migration functionality using goose.
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
