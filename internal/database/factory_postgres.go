//go:build postgres

package database

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
)

// newPostgresDB creates a new PostgreSQL database connection.
// This implementation is only available when built with the 'postgres' build tag.
func newPostgresDB(config FullConfig) (*DB, error) {
	if config.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required for PostgreSQL driver")
	}

	// Open connection using pgx stdlib
	db, err := sql.Open("pgx", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test the connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}

	// Run database migrations
	if err := runMigrationsForDriver(db, "postgres"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run PostgreSQL migrations: %w", err)
	}

	return &DB{db: db, driver: DriverPostgres}, nil
}
