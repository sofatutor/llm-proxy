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
	_ = dbFile.Close()

	// Create database
	db, err := New(Config{
		Path:            dbPath,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		_ = os.Remove(dbPath)
		t.Fatalf("Failed to create database: %v", err)
	}

	// Return database and cleanup function
	return db, func() {
		_ = db.Close()
		_ = os.Remove(dbPath)
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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Path == "" || cfg.MaxOpenConns <= 0 || cfg.MaxIdleConns <= 0 || cfg.ConnMaxLifetime <= 0 {
		t.Errorf("DefaultConfig returned invalid config: %+v", cfg)
	}
}

func TestDB_DB(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	if db.DB() == nil {
		t.Error("DB() returned nil")
	}
}

func TestEnsureDirExists(t *testing.T) {
	dir := os.TempDir() + "/llm-proxy-test-dir"
	_ = os.RemoveAll(dir)
	err := ensureDirExists(dir)
	if err != nil {
		t.Fatalf("ensureDirExists failed: %v", err)
	}
	// Should succeed if called again (already exists)
	if err := ensureDirExists(dir); err != nil {
		t.Fatalf("ensureDirExists failed on existing dir: %v", err)
	}
	_ = os.RemoveAll(dir)
}

func TestEnsureDirExists_Error(t *testing.T) {
	// Try to create a dir where parent doesn't exist and can't be created (simulate with a file)
	file := os.TempDir() + "/llm-proxy-test-file"
	f, err := os.Create(file)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	f.Close()
	defer os.Remove(file)
	// Now try to create a dir with the same name as the file
	err = ensureDirExists(file)
	if err == nil {
		t.Error("expected error when ensureDirExists called on file path")
	}
}

func TestNew_Error(t *testing.T) {
	// Invalid path (directory as file)
	dir := os.TempDir() + "/llm-proxy-test-baddir"
	_ = os.MkdirAll(dir, 0755)
	_, err := New(Config{Path: dir, MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxLifetime: time.Second})
	if err == nil {
		t.Error("expected error for directory as DB file")
	}
	_ = os.RemoveAll(dir)
}

func TestTransaction_Panic(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic in transaction")
		}
	}()
	_ = db.Transaction(ctx, func(tx *sql.Tx) error {
		panic("test panic")
	})
}

func TestTransaction_CommitError(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	// Close DB to force commit error
	_ = db.Close()
	err := db.Transaction(ctx, func(tx *sql.Tx) error {
		return nil
	})
	if err == nil {
		t.Error("expected error on commit after DB closed")
	}
}
