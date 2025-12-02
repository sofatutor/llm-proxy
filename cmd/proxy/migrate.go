package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database/migrations"
	"github.com/spf13/cobra"
)

// migrateCmd is the parent command for migration operations
var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Manage database migrations",
	Long:  `Database migration management commands for applying, rolling back, and checking migration status.`,
}

// migrateUpCmd applies all pending migrations
var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	Long:  `Apply all pending migrations to bring the database up to date.`,
	RunE:  runMigrateUp,
}

// migrateDownCmd rolls back the last migration
var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback the last migration",
	Long:  `Rollback the most recently applied migration.`,
	RunE:  runMigrateDown,
}

// migrateStatusCmd shows the current migration version
var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current migration version",
	Long:  `Display the current migration version. Returns 0 if no migrations have been applied.`,
	RunE:  runMigrateStatus,
}

// migrateVersionCmd is an alias for status
var migrateVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show current migration version (alias for status)",
	Long:  `Display the current migration version. This is an alias for the status command.`,
	RunE:  runMigrateStatus,
}

// runMigrateUp applies all pending migrations
func runMigrateUp(cmd *cobra.Command, args []string) error {
	db, migrationsPath, err := getMigrationResources()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			// Log but don't fail migration if close fails
			fmt.Printf("Warning: Failed to close database connection: %v\n", closeErr)
		}
	}()

	runner := migrations.NewMigrationRunner(db, migrationsPath)
	if err := runner.Up(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	fmt.Println("Migrations applied successfully")
	return nil
}

// runMigrateDown rolls back the last migration
func runMigrateDown(cmd *cobra.Command, args []string) error {
	db, migrationsPath, err := getMigrationResources()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			// Log but don't fail migration if close fails
			fmt.Printf("Warning: Failed to close database connection: %v\n", closeErr)
		}
	}()

	runner := migrations.NewMigrationRunner(db, migrationsPath)
	if err := runner.Down(); err != nil {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	fmt.Println("Migration rolled back successfully")
	return nil
}

// runMigrateStatus shows the current migration version
func runMigrateStatus(cmd *cobra.Command, args []string) error {
	db, migrationsPath, err := getMigrationResources()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			// Log but don't fail status check if close fails
			fmt.Printf("Warning: Failed to close database connection: %v\n", closeErr)
		}
	}()

	runner := migrations.NewMigrationRunner(db, migrationsPath)
	version, err := runner.Status()
	if err != nil {
		return fmt.Errorf("failed to get migration status: %w", err)
	}

	fmt.Printf("Current migration version: %d\n", version)
	return nil
}

// getMigrationResources opens a database connection and gets the migrations path
func getMigrationResources() (*sql.DB, string, error) {
	// Get database path from flag or environment
	dbPath := databasePath
	if dbPath == "" {
		dbPath = config.EnvOrDefault("DATABASE_PATH", "data/llm-proxy.db")
	}

	// Ensure database directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection (without running migrations automatically)
	db, err := sql.Open("sqlite3", dbPath+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, "", fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("failed to ping database: %w", err)
	}

	// Get migrations path using the same logic as database package
	migrationsPath, err := getMigrationsPathForCLI()
	if err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("failed to find migrations directory: %w", err)
	}

	return db, migrationsPath, nil
}

// getMigrationsPathForCLI returns the path to the migrations directory.
// This is a simplified version of database.getMigrationsPath() for CLI use.
func getMigrationsPathForCLI() (string, error) {
	// Try relative path from current working directory first (development)
	relPath := "internal/database/migrations/sql"
	if _, err := os.Stat(relPath); err == nil {
		return relPath, nil
	}

	// Try path relative to executable
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// Try relative to executable directory
		execRelPath := filepath.Join(execDir, "internal/database/migrations/sql")
		if _, err := os.Stat(execRelPath); err == nil {
			return execRelPath, nil
		}
		// Try relative to executable's parent (if executable is in bin/)
		binRelPath := filepath.Join(filepath.Dir(execDir), "internal/database/migrations/sql")
		if _, err := os.Stat(binRelPath); err == nil {
			return binRelPath, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found. Tried: %s", relPath)
}
