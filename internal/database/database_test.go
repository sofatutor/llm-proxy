package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
)

// testDB creates a temporary database for testing.
func testDB(t *testing.T) (*DB, func()) {
	t.Helper()

	// Create a temporary database file
	dbFile, err := os.CreateTemp("", "llm-proxy-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	dbPath := dbFile.Name()
	dbFile.Close()

	// Create database
	db, err := New(Config{
		Path:            dbPath,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		os.Remove(dbPath)
		t.Fatalf("Failed to create database: %v", err)
	}

	// Return database and cleanup function
	return db, func() {
		db.Close()
		os.Remove(dbPath)
	}
}

// TestDatabaseInit tests database initialization.
func TestDatabaseInit(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	// Check if we can execute a simple query
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query projects: %v", err)
	}

	// Check if we can execute a simple query on tokens table
	err = db.db.QueryRow("SELECT COUNT(*) FROM tokens").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query tokens: %v", err)
	}
}

// TestTransaction tests transaction support.
func TestTransaction(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test successful transaction
	err := db.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO projects (id, name, openai_api_key) VALUES (?, ?, ?)", "test-id", "test-project", "test-key")
		return err
	})
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify the transaction was committed
	var count int
	err = db.db.QueryRow("SELECT COUNT(*) FROM projects WHERE id = ?", "test-id").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query projects: %v", err)
	}
	if count != 1 {
		t.Fatalf("Expected 1 project, got %d", count)
	}

	// Test failed transaction
	err = db.Transaction(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO projects (id, name, openai_api_key) VALUES (?, ?, ?)", "test-id2", "test-project2", "test-key2")
		if err != nil {
			return err
		}
		return errors.New("intentional error")
	})
	if err == nil {
		t.Fatalf("Expected transaction to fail")
	}

	// Verify the transaction was rolled back
	err = db.db.QueryRow("SELECT COUNT(*) FROM projects WHERE id = ?", "test-id2").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query projects: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 projects, got %d", count)
	}
}