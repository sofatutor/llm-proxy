//go:build mysql

package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
)

// newMySQLDB creates a new MySQL database connection.
// This implementation is only available when built with the 'mysql' build tag.
func newMySQLDB(config FullConfig) (*DB, error) {
	if config.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required for MySQL driver")
	}

	// Open connection using MySQL driver
	db, err := sql.Open("mysql", config.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test the connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping MySQL database: %w", err)
	}

	// Run database migrations
	if err := runMigrationsForDriver(db, "mysql"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run MySQL migrations: %w", err)
	}

	return &DB{db: db, driver: DriverMySQL}, nil
}
