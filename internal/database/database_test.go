package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

func TestMaintainAndStatsOnInMemoryDB(t *testing.T) {
	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("New DB error: %v", err)
	}
	defer func() { _ = db.Close() }()
	// Ensure schema exists
	if err := DBInitForTests(db); err != nil {
		t.Fatalf("DBInitForTests error: %v", err)
	}

	// Maintain should succeed
	if err := db.MaintainDatabase(context.Background()); err != nil {
		t.Fatalf("MaintainDatabase error: %v", err)
	}

	// Stats should return a map with expected keys
	stats, err := db.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	for _, k := range []string{"database_size_bytes", "project_count", "active_token_count", "expired_token_count", "total_request_count"} {
		if _, ok := stats[k]; !ok {
			t.Fatalf("missing stats key: %s", k)
		}
	}
}

func TestBackupDatabaseCreatesFile(t *testing.T) {
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup.sqlite")

	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("New DB error: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := DBInitForTests(db); err != nil {
		t.Fatalf("DBInitForTests error: %v", err)
	}

	if err := db.BackupDatabase(context.Background(), backup); err != nil {
		t.Fatalf("BackupDatabase error: %v", err)
	}
	if _, err := os.Stat(backup); err != nil {
		t.Fatalf("expected backup file to exist: %v", err)
	}
}

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
	if err := f.Close(); err != nil {
		t.Fatalf("Failed to close file: %v", err)
	}
	defer func() { _ = os.Remove(file) }()
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

func TestInitDatabase_Error(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	// Close DB to force error
	_ = db.Close()
	err := initDatabase(db.db)
	if err == nil {
		t.Error("expected error for closed DB in initDatabase")
	}
}

func TestNew_PermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission test not reliable")
	}
	dir, err := os.MkdirTemp("", "llm-proxy-test-perm")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	// Make dir read-only
	if err := os.Chmod(dir, 0400); err != nil {
		t.Fatalf("Failed to chmod temp dir: %v", err)
	}
	badPath := dir + "/subdir/dbfile.db"
	_, err = New(Config{Path: badPath, MaxOpenConns: 1, MaxIdleConns: 1, ConnMaxLifetime: time.Second})
	if err == nil {
		t.Error("expected error for permission denied in New")
	}
	_ = os.Chmod(dir, 0700) // restore permissions for cleanup
}

func TestTransaction_NilDB(t *testing.T) {
	d := &DB{db: nil}
	err := d.Transaction(context.Background(), func(tx *sql.Tx) error { return nil })
	if err == nil {
		t.Error("expected error for nil DB in Transaction")
	}
}

func TestCRUD_ClosedDB(t *testing.T) {
	db, cleanup := testDB(t)
	cleanup()
	ctx := context.Background()
	p := proxy.Project{ID: "x", Name: "x", OpenAIAPIKey: "x", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := db.CreateProject(ctx, p); err == nil {
		t.Error("expected error for CreateProject on closed DB")
	}
	_, err := db.GetProjectByID(ctx, "x")
	if err == nil {
		t.Error("expected error for GetProjectByID on closed DB")
	}
	if err := db.UpdateProject(ctx, p); err == nil {
		t.Error("expected error for UpdateProject on closed DB")
	}
	if err := db.DeleteProject(ctx, "x"); err == nil {
		t.Error("expected error for DeleteProject on closed DB")
	}
	_, err = db.ListProjects(ctx)
	if err == nil {
		t.Error("expected error for ListProjects on closed DB")
	}
}

func TestDBInitForTests_NilDB(t *testing.T) {
	// Test with nil DB - should be no-op
	err := DBInitForTests(nil)
	if err != nil {
		t.Errorf("DBInitForTests with nil DB should not error, got: %v", err)
	}

	// Test with DB with nil internal db
	db := &DB{db: nil}
	err = DBInitForTests(db)
	if err != nil {
		t.Errorf("DBInitForTests with nil internal DB should not error, got: %v", err)
	}
}

func TestTransaction_NilDBVariants(t *testing.T) {
	// Test with nil DB struct
	var db *DB
	err := db.Transaction(context.Background(), func(tx *sql.Tx) error { return nil })
	if err == nil {
		t.Error("expected error for nil DB struct in Transaction")
	}
}

func TestMigrations_AddColumn(t *testing.T) {
	// Create a temporary database file
	dbFile, err := os.CreateTemp("", "llm-proxy-migration-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	dbPath := dbFile.Name()
	_ = dbFile.Close()
	defer func() { _ = os.Remove(dbPath) }()

	// Open raw database connection
	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer func() { _ = rawDB.Close() }()

	// Create basic schema without the new columns
	basicSchema := `
		CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			openai_api_key TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE IF NOT EXISTS tokens (
			token TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			expires_at DATETIME,
			max_requests INTEGER,
			used_requests INTEGER DEFAULT 0,
			FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
		);
	`
	_, err = rawDB.Exec(basicSchema)
	if err != nil {
		t.Fatalf("Failed to create basic schema: %v", err)
	}

	// Check columns don't exist yet
	var count int
	err = rawDB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'deactivated_at'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check column existence: %v", err)
	}
	if count != 0 {
		t.Error("deactivated_at column should not exist yet")
	}

	// Close raw DB and open with our DB wrapper to trigger migrations
	_ = rawDB.Close()

	db, err := New(Config{
		Path:            dbPath,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Check that migrations added the column
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'deactivated_at'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check column existence after migration: %v", err)
	}
	if count != 1 {
		t.Error("deactivated_at column should exist after migration")
	}

	// Check is_active column was also added to projects
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'is_active'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check is_active column existence: %v", err)
	}
	if count != 1 {
		t.Error("is_active column should exist after migration")
	}

	// Check deactivated_at column was added to tokens
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tokens') WHERE name = 'deactivated_at'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check tokens deactivated_at column existence: %v", err)
	}
	if count != 1 {
		t.Error("deactivated_at column should exist in tokens table after migration")
	}
}
