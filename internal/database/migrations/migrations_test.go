package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestMigration001Up tests applying the initial schema migration.
func TestMigration001Up(t *testing.T) {
	migrationSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration: %v", err)
	}

	verifySchemaExists(t, db)
}

// TestMigration001UpIdempotent tests that applying migration twice works (IF NOT EXISTS).
func TestMigration001UpIdempotent(t *testing.T) {
	migrationSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	// Apply migration twice
	_, err := db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration first time: %v", err)
	}

	_, err = db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration second time: %v", err)
	}

	verifySchemaExists(t, db)
}

// TestMigration001Down tests the rollback migration.
func TestMigration001Down(t *testing.T) {
	upSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	downSQL := readMigrationFile(t, "001_initial_schema.down.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	// First apply up migration
	_, err := db.Exec(upSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration: %v", err)
	}

	// Verify schema exists
	verifySchemaExists(t, db)

	// Apply down migration
	_, err = db.Exec(downSQL)
	if err != nil {
		t.Fatalf("Failed to apply down migration: %v", err)
	}

	// Verify all tables are dropped
	verifySchemaNotExists(t, db)
}

// TestMigration001RoundTrip tests full migration round trip (up, down, up).
func TestMigration001RoundTrip(t *testing.T) {
	upSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	downSQL := readMigrationFile(t, "001_initial_schema.down.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	// Up -> Down -> Up
	_, err := db.Exec(upSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration (first): %v", err)
	}
	verifySchemaExists(t, db)

	_, err = db.Exec(downSQL)
	if err != nil {
		t.Fatalf("Failed to apply down migration: %v", err)
	}
	verifySchemaNotExists(t, db)

	_, err = db.Exec(upSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration (second): %v", err)
	}
	verifySchemaExists(t, db)
}

// TestMigration001SchemaMatchesDatabase verifies migration creates same schema as database.go.
func TestMigration001SchemaMatchesDatabase(t *testing.T) {
	migrationSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(migrationSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration: %v", err)
	}

	// Verify projects table columns
	projectsCols := getTableColumns(t, db, "projects")
	expectedProjectsCols := []string{
		"id", "name", "openai_api_key", "is_active", "deactivated_at",
		"created_at", "updated_at",
	}
	verifyColumns(t, "projects", projectsCols, expectedProjectsCols)

	// Verify tokens table columns
	tokensCols := getTableColumns(t, db, "tokens")
	expectedTokensCols := []string{
		"token", "project_id", "expires_at", "is_active", "deactivated_at",
		"request_count", "max_requests", "created_at", "last_used_at",
	}
	verifyColumns(t, "tokens", tokensCols, expectedTokensCols)

	// Verify audit_events table columns
	auditCols := getTableColumns(t, db, "audit_events")
	expectedAuditCols := []string{
		"id", "timestamp", "action", "actor", "project_id", "request_id",
		"correlation_id", "client_ip", "method", "path", "user_agent",
		"outcome", "reason", "token_id", "metadata",
	}
	verifyColumns(t, "audit_events", auditCols, expectedAuditCols)

	// Verify indexes exist
	indexes := getIndexes(t, db)
	expectedIndexes := []string{
		"idx_projects_name",
		"idx_tokens_project_id",
		"idx_tokens_expires_at",
		"idx_tokens_is_active",
		"idx_audit_timestamp",
		"idx_audit_action",
		"idx_audit_project_id",
		"idx_audit_client_ip",
		"idx_audit_request_id",
		"idx_audit_outcome",
		"idx_audit_ip_action",
	}
	verifyIndexes(t, indexes, expectedIndexes)
}

// TestMigration001DataPersistence tests that data survives migration operations.
func TestMigration001DataPersistence(t *testing.T) {
	upSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(upSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`INSERT INTO projects (id, name, openai_api_key) VALUES ('test-id', 'test-project', 'test-key')`)
	if err != nil {
		t.Fatalf("Failed to insert project: %v", err)
	}

	_, err = db.Exec(`INSERT INTO tokens (token, project_id) VALUES ('test-token', 'test-id')`)
	if err != nil {
		t.Fatalf("Failed to insert token: %v", err)
	}

	// Verify data exists
	var projectCount, tokenCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM projects`).Scan(&projectCount)
	if err != nil {
		t.Fatalf("Failed to count projects: %v", err)
	}
	if projectCount != 1 {
		t.Errorf("Expected 1 project, got %d", projectCount)
	}

	err = db.QueryRow(`SELECT COUNT(*) FROM tokens`).Scan(&tokenCount)
	if err != nil {
		t.Fatalf("Failed to count tokens: %v", err)
	}
	if tokenCount != 1 {
		t.Errorf("Expected 1 token, got %d", tokenCount)
	}
}

// TestMigration001ForeignKeys tests that foreign key constraints are enforced.
func TestMigration001ForeignKeys(t *testing.T) {
	upSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	// Enable foreign keys
	_, err := db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	_, err = db.Exec(upSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration: %v", err)
	}

	// Try to insert token without valid project_id - should fail
	_, err = db.Exec(`INSERT INTO tokens (token, project_id) VALUES ('orphan-token', 'nonexistent-project')`)
	if err == nil {
		t.Error("Expected foreign key constraint violation, but insert succeeded")
	}
}

// TestMigration001AuditEventConstraint tests the outcome CHECK constraint.
func TestMigration001AuditEventConstraint(t *testing.T) {
	upSQL := readMigrationFile(t, "001_initial_schema.up.sql")
	db := createTestDB(t)
	defer func() { _ = db.Close() }()

	_, err := db.Exec(upSQL)
	if err != nil {
		t.Fatalf("Failed to apply up migration: %v", err)
	}

	// Valid outcomes should work
	for _, outcome := range []string{"success", "failure"} {
		_, err = db.Exec(`INSERT INTO audit_events (id, timestamp, action, actor, outcome) VALUES (?, datetime('now'), 'test', 'test', ?)`, outcome+"-id", outcome)
		if err != nil {
			t.Errorf("Failed to insert audit event with outcome '%s': %v", outcome, err)
		}
	}

	// Invalid outcome should fail
	_, err = db.Exec(`INSERT INTO audit_events (id, timestamp, action, actor, outcome) VALUES ('invalid-id', datetime('now'), 'test', 'test', 'invalid')`)
	if err == nil {
		t.Error("Expected CHECK constraint violation for invalid outcome, but insert succeeded")
	}
}

// Helper functions

func readMigrationFile(t *testing.T, filename string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(".", filename))
	if err != nil {
		t.Fatalf("Failed to read migration file %s: %v", filename, err)
	}
	return string(content)
}

func createTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}
	return db
}

func verifySchemaExists(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{"projects", "tokens", "audit_events"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query sqlite_master for table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Expected table %s to exist", table)
		}
	}
}

func verifySchemaNotExists(t *testing.T, db *sql.DB) {
	t.Helper()
	tables := []string{"projects", "tokens", "audit_events"}
	for _, table := range tables {
		var count int
		err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query sqlite_master for table %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("Expected table %s to not exist", table)
		}
	}
}

func getTableColumns(t *testing.T, db *sql.DB, tableName string) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, tableName)
	if err != nil {
		t.Fatalf("Failed to get columns for table %s: %v", tableName, err)
	}
	defer func() { _ = rows.Close() }()

	var columns []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Failed to scan column name: %v", err)
		}
		columns = append(columns, name)
	}
	return columns
}

func verifyColumns(t *testing.T, tableName string, actual, expected []string) {
	t.Helper()
	actualSet := make(map[string]bool)
	for _, col := range actual {
		actualSet[col] = true
	}

	for _, col := range expected {
		if !actualSet[col] {
			t.Errorf("Table %s: expected column %s not found", tableName, col)
		}
	}
}

func getIndexes(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%'`)
	if err != nil {
		t.Fatalf("Failed to get indexes: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("Failed to scan index name: %v", err)
		}
		indexes = append(indexes, name)
	}
	return indexes
}

func verifyIndexes(t *testing.T, actual, expected []string) {
	t.Helper()
	actualSet := make(map[string]bool)
	for _, idx := range actual {
		actualSet[idx] = true
	}

	for _, idx := range expected {
		if !actualSet[idx] {
			t.Errorf("Expected index %s not found", idx)
		}
	}
}
