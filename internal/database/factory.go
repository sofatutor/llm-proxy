// Package database provides database operations for the LLM Proxy.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sofatutor/llm-proxy/internal/database/migrations"
)

// DriverType represents the database driver type.
type DriverType string

const (
	// DriverSQLite represents the SQLite database driver.
	DriverSQLite DriverType = "sqlite"
	// DriverPostgres represents the PostgreSQL database driver.
	DriverPostgres DriverType = "postgres"
)

// FullConfig contains the complete database configuration for all drivers.
type FullConfig struct {
	// Driver specifies which database driver to use (sqlite, postgres).
	Driver DriverType
	// SQLite-specific configuration
	// Path is the path to the SQLite database file.
	Path string
	// PostgreSQL-specific configuration
	// DatabaseURL is the PostgreSQL connection string.
	DatabaseURL string
	// Connection pool settings (used by both drivers)
	// MaxOpenConns is the maximum number of open connections.
	MaxOpenConns int
	// MaxIdleConns is the maximum number of idle connections.
	MaxIdleConns int
	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	ConnMaxLifetime time.Duration
}

// DefaultFullConfig returns a default database configuration.
func DefaultFullConfig() FullConfig {
	return FullConfig{
		Driver:          DriverSQLite,
		Path:            "data/llm-proxy.db",
		DatabaseURL:     "",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
	}
}

// ConfigFromEnv creates a FullConfig from environment variables.
// Invalid configuration values are logged as warnings and defaults are used.
func ConfigFromEnv() FullConfig {
	config := DefaultFullConfig()

	if driver := os.Getenv("DB_DRIVER"); driver != "" {
		driverType := DriverType(strings.ToLower(driver))
		if driverType != DriverSQLite && driverType != DriverPostgres {
			log.Printf("Warning: unsupported DB_DRIVER '%s', defaulting to sqlite", driver)
		} else {
			config.Driver = driverType
		}
	}

	if path := os.Getenv("DATABASE_PATH"); path != "" {
		config.Path = path
	}

	if url := os.Getenv("DATABASE_URL"); url != "" {
		config.DatabaseURL = url
	}

	if poolSize := os.Getenv("DATABASE_POOL_SIZE"); poolSize != "" {
		if size, err := parsePositiveInt(poolSize); err == nil {
			config.MaxOpenConns = size
		} else {
			log.Printf("Warning: invalid DATABASE_POOL_SIZE '%s': %v, using default %d", poolSize, err, config.MaxOpenConns)
		}
	}

	if idleConns := os.Getenv("DATABASE_MAX_IDLE_CONNS"); idleConns != "" {
		if size, err := parsePositiveInt(idleConns); err == nil {
			config.MaxIdleConns = size
		} else {
			log.Printf("Warning: invalid DATABASE_MAX_IDLE_CONNS '%s': %v, using default %d", idleConns, err, config.MaxIdleConns)
		}
	}

	if lifetime := os.Getenv("DATABASE_CONN_MAX_LIFETIME"); lifetime != "" {
		if duration, err := time.ParseDuration(lifetime); err == nil {
			config.ConnMaxLifetime = duration
		} else {
			log.Printf("Warning: invalid DATABASE_CONN_MAX_LIFETIME '%s': %v, using default %v", lifetime, err, config.ConnMaxLifetime)
		}
	}

	return config
}

// parsePositiveInt parses a string as a positive integer.
func parsePositiveInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil || i <= 0 {
		return 0, fmt.Errorf("invalid positive integer: %s", s)
	}
	return i, nil
}

// NewFromConfig creates a new database connection based on the configuration.
func NewFromConfig(config FullConfig) (*DB, error) {
	switch config.Driver {
	case DriverSQLite:
		return newSQLiteDB(config)
	case DriverPostgres:
		return newPostgresDB(config)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", config.Driver)
	}
}

// newSQLiteDB creates a new SQLite database connection.
func newSQLiteDB(config FullConfig) (*DB, error) {
	// Ensure database directory exists (skip for in-memory databases)
	if config.Path != ":memory:" {
		if err := ensureDirExists(filepath.Dir(config.Path)); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Open connection
	db, err := sql.Open("sqlite3", config.Path+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
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
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	// Run database migrations
	if err := runMigrationsForDriver(db, "sqlite3"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run SQLite migrations: %w", err)
	}

	return &DB{db: db, driver: DriverSQLite}, nil
}

// runMigrationsForDriver runs database migrations for the specified driver.
func runMigrationsForDriver(db *sql.DB, dialect string) error {
	migrationsPath, err := getMigrationsPathForDialect(dialect)
	if err != nil {
		return fmt.Errorf("failed to get migrations path: %w", err)
	}

	runner := migrations.NewMigrationRunner(db, migrationsPath)
	if err := runner.Up(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// getMigrationsPathForDialect returns the path to migrations for the specified dialect.
// It looks for dialect-specific migrations first (e.g., sql/postgres/), then falls back
// to the common migrations directory (sql/).
func getMigrationsPathForDialect(dialect string) (string, error) {
	// Common base paths to try
	basePaths := []string{
		"internal/database/migrations",
	}

	// Add path relative to this source file (for tests)
	_, filename, _, ok := runtime.Caller(0)
	if ok {
		sourceDir := filepath.Dir(filename)
		basePaths = append(basePaths, filepath.Join(sourceDir, "migrations"))
	}

	// Add paths relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		basePaths = append(basePaths, filepath.Join(execDir, "internal/database/migrations"))
		basePaths = append(basePaths, filepath.Join(filepath.Dir(execDir), "internal/database/migrations"))
	}

	// Dialect-specific subdirectory (e.g., "postgres", "sqlite3")
	dialectDir := dialect
	if dialect == "sqlite3" {
		dialectDir = "sqlite"
	}

	// Try each base path
	for _, basePath := range basePaths {
		// First, try dialect-specific directory
		dialectPath := filepath.Join(basePath, "sql", dialectDir)
		if _, err := os.Stat(dialectPath); err == nil {
			return dialectPath, nil
		}

		// Fall back to common SQL directory
		commonPath := filepath.Join(basePath, "sql")
		if _, err := os.Stat(commonPath); err == nil {
			return commonPath, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found for dialect: %s", dialect)
}
