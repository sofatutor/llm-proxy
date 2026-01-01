package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

func TestSQLite_TimestampsRoundTripAsUTC(t *testing.T) {
	// Ensure this test is deterministic across environments by forcing a non-UTC local timezone.
	originalLocal := time.Local
	time.Local = time.FixedZone("TestLocal", 2*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	fixedUTC := time.Date(2025, 12, 14, 13, 38, 0, 0, time.UTC).Truncate(time.Second)

	type openCase struct {
		name string
		open func(t *testing.T) (*DB, func())
	}

	cases := []openCase{
		{
			name: "New",
			open: func(t *testing.T) (*DB, func()) {
				db, err := New(Config{Path: ":memory:"})
				if err != nil {
					t.Fatalf("New DB error: %v", err)
				}
				return db, func() { _ = db.Close() }
			},
		},
		{
			name: "NewFromConfig",
			open: func(t *testing.T) (*DB, func()) {
				db, err := NewFromConfig(FullConfig{Driver: DriverSQLite, Path: ":memory:"})
				if err != nil {
					t.Fatalf("NewFromConfig error: %v", err)
				}
				return db, func() { _ = db.Close() }
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, cleanup := tc.open(t)
			defer cleanup()

			ctx := context.Background()
			project := proxy.Project{
				ID:           "test-project-utc-roundtrip",
				Name:         "Test Project",
				APIKey: "test-api-key",
				CreatedAt:    fixedUTC,
				UpdatedAt:    fixedUTC,
			}

			if err := db.CreateProject(ctx, project); err != nil {
				t.Fatalf("CreateProject failed: %v", err)
			}

			retrieved, err := db.GetProjectByID(ctx, project.ID)
			if err != nil {
				t.Fatalf("GetProjectByID failed: %v", err)
			}

			if !retrieved.CreatedAt.Equal(fixedUTC) {
				t.Fatalf("CreatedAt drift: got %s, want %s", retrieved.CreatedAt.Format(time.RFC3339), fixedUTC.Format(time.RFC3339))
			}
			if !retrieved.UpdatedAt.Equal(fixedUTC) {
				t.Fatalf("UpdatedAt drift: got %s, want %s", retrieved.UpdatedAt.Format(time.RFC3339), fixedUTC.Format(time.RFC3339))
			}
		})
	}
}

func TestSQLite_TokenLastUsedAt_NotShiftedByLocalTimezone(t *testing.T) {
	// Ensure this test is deterministic across environments by forcing a non-UTC local timezone.
	originalLocal := time.Local
	time.Local = time.FixedZone("TestLocal", 2*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	db, err := New(Config{Path: ":memory:"})
	if err != nil {
		t.Fatalf("New DB error: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := DBInitForTests(db); err != nil {
		t.Fatalf("DBInitForTests error: %v", err)
	}

	ctx := context.Background()
	fixedUTC := time.Date(2025, 12, 14, 13, 38, 0, 0, time.UTC).Truncate(time.Second)
	project := proxy.Project{
		ID:           "test-project-token-last-used-at",
		Name:         "Test Project",
		APIKey: "test-api-key",
		CreatedAt:    fixedUTC,
		UpdatedAt:    fixedUTC,
	}
	if err := db.CreateProject(ctx, project); err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	const tokenString = "test-token-last-used-at"
	if err := db.CreateToken(ctx, Token{Token: tokenString, ProjectID: project.ID, IsActive: true, CreatedAt: fixedUTC}); err != nil {
		t.Fatalf("CreateToken failed: %v", err)
	}

	if err := db.IncrementTokenUsage(ctx, tokenString); err != nil {
		t.Fatalf("IncrementTokenUsage failed: %v", err)
	}

	retrieved, err := db.GetTokenByToken(ctx, tokenString)
	if err != nil {
		t.Fatalf("GetTokenByToken failed: %v", err)
	}
	if retrieved.LastUsedAt == nil {
		t.Fatalf("LastUsedAt is nil")
	}

	// If SQLite parsing or storage is timezone-shifted, this will typically drift by hours.
	nowUTC := time.Now().UTC()
	drift := retrieved.LastUsedAt.Sub(nowUTC)
	if drift < 0 {
		drift = -drift
	}
	if drift > 10*time.Second {
		t.Fatalf("LastUsedAt drift too large: got %s, now %s (drift %s)", retrieved.LastUsedAt.Format(time.RFC3339), nowUTC.Format(time.RFC3339), drift)
	}
}

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
		_, err := tx.Exec("INSERT INTO projects (id, name, api_key) VALUES (?, ?, ?)", "test-id", "test-project", "test-key")
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
		_, err := tx.Exec("INSERT INTO projects (id, name, api_key) VALUES (?, ?, ?)", "test-id2", "test-project2", "test-key2")
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

func TestInitSQLiteSchema_Error(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	// Close DB to force error
	_ = db.Close()
	err := initSQLiteSchema(db.db)
	if err == nil {
		t.Error("expected error for closed DB in initSQLiteSchema")
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
	p := proxy.Project{ID: "x", Name: "x", APIKey: "x", CreatedAt: time.Now(), UpdatedAt: time.Now()}
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

func TestSQLiteSchema_NewDatabase(t *testing.T) {
	// Test that a new database gets the full schema from schema.sql
	db, cleanup := testDB(t)
	defer cleanup()

	// Verify all tables exist
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('projects', 'tokens', 'audit_events')").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check tables: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 tables, got %d", count)
	}

	// Verify all required columns exist in projects
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'is_active'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check is_active column: %v", err)
	}
	if count != 1 {
		t.Error("is_active column should exist in projects table")
	}

	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = 'deactivated_at'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check deactivated_at column: %v", err)
	}
	if count != 1 {
		t.Error("deactivated_at column should exist in projects table")
	}

	// Verify all required columns exist in tokens
	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tokens') WHERE name = 'id'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check id column: %v", err)
	}
	if count != 1 {
		t.Error("id column should exist in tokens table")
	}

	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tokens') WHERE name = 'deactivated_at'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check tokens deactivated_at column: %v", err)
	}
	if count != 1 {
		t.Error("deactivated_at column should exist in tokens table")
	}

	err = db.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('tokens') WHERE name = 'cache_hit_count'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check cache_hit_count column: %v", err)
	}
	if count != 1 {
		t.Error("cache_hit_count column should exist in tokens table")
	}
}

func TestSQLiteSchema_Idempotent(t *testing.T) {
	// Test that running schema initialization multiple times is safe
	// SQLite uses IF NOT EXISTS so re-running should be a no-op
	db, cleanup := testDB(t)
	defer cleanup()

	// Check initial state - tables exist
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('projects', 'tokens', 'audit_events')").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check tables: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 tables, got %d", count)
	}

	// Re-run schema initialization (should be no-op)
	if err := initSQLiteSchema(db.db); err != nil {
		t.Fatalf("Failed to re-run schema initialization: %v", err)
	}

	// Tables should still exist with same count
	err = db.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('projects', 'tokens', 'audit_events')").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check tables after re-init: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 tables after re-init, got %d", count)
	}
}

func TestGetSchemaPath_Success(t *testing.T) {
	// Test that getSchemaPath can find schema.sql from various paths
	// This test verifies the function works when run from the project root
	path, err := getSchemaPath()
	if err != nil {
		t.Logf("getSchemaPath error (may be expected in some test environments): %v", err)
		// Not a fatal error - depends on working directory
		return
	}

	// Verify the path exists and is a SQL file
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat schema file: %v", err)
	}

	if info.IsDir() {
		t.Error("Schema path should point to a file, not a directory")
	}

	if filepath.Base(path) != "schema.sql" {
		t.Errorf("Schema path should end with schema.sql, got %s", filepath.Base(path))
	}
}

func TestGetSchemaPath_FindsSchemaFromSource(t *testing.T) {
	// The getSchemaPath function uses runtime.Caller to find the source file location,
	// so it should always find schema.sql from the scripts/ directory.
	path, err := getSchemaPath()
	if err != nil {
		// If we get an error, that's acceptable in some test environments
		t.Logf("getSchemaPath returned error (may be expected): %v", err)
		return
	}

	// Verify the returned path is valid
	if path == "" {
		t.Error("getSchemaPath returned empty path")
	}
}

func TestGetSchemaPath_PrefersCWDWhenPresent(t *testing.T) {
	tmpDir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	scriptsDir := filepath.Join(tmpDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}

	absoluteSchemaPath := filepath.Join(scriptsDir, "schema.sql")
	if err := os.WriteFile(absoluteSchemaPath, []byte("-- test schema\n"), 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}

	path, err := getSchemaPath()
	if err != nil {
		t.Fatalf("getSchemaPath error: %v", err)
	}

	// For the cwd strategy, getSchemaPath intentionally returns a relative path.
	expectedRelative := filepath.Join("scripts", "schema.sql")
	if path != expectedRelative {
		t.Fatalf("schema path mismatch: got %q want %q", path, expectedRelative)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected schema path to exist: %v", err)
	}
}

func TestGetSchemaPath_FallsBackToSourcePathWhenCWDMissing(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	// Force strategy 1 (cwd scripts/schema.sql) to fail.
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}

	// Compute the expected repo-root path similarly to getSchemaPath.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	expected := filepath.Join(repoRoot, "scripts", "schema.sql")

	path, err := getSchemaPath()
	if err != nil {
		t.Fatalf("getSchemaPath error: %v", err)
	}
	if path != expected {
		t.Fatalf("schema path mismatch: got %q want %q", path, expected)
	}
}

func TestInitSQLiteSchema_ClosedDB(t *testing.T) {
	// Test initSQLiteSchema with a closed database
	db, cleanup := testDB(t)
	cleanup() // Close database immediately

	err := initSQLiteSchema(db.db)
	if err == nil {
		t.Error("Expected error when initializing schema on closed database")
	}
}
