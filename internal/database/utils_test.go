package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/proxy"
)

// TestDatabaseUtils tests the database utility functions.
func TestDatabaseUtils(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project
	project := proxy.Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		APIKey: "test-api-key",
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
		UpdatedAt:    time.Now().UTC().Truncate(time.Second),
	}
	err := db.CreateProject(ctx, project)
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create test tokens with different states
	now := time.Now().UTC()
	futureExpiry := now.Add(24 * time.Hour)
	maxRequests := 10

	validToken := Token{
		ID:           "valid-token-id",
		Token:        "valid-token",
		ProjectID:    project.ID,
		ExpiresAt:    &futureExpiry,
		IsActive:     true,
		RequestCount: 5,
		MaxRequests:  &maxRequests,
		CreatedAt:    now.Truncate(time.Second),
	}
	err = db.CreateToken(ctx, validToken)
	if err != nil {
		t.Fatalf("Failed to create valid token: %v", err)
	}

	// Test IsTokenValid
	isValid, err := db.IsTokenValid(ctx, validToken.ID)
	if err != nil {
		t.Fatalf("IsTokenValid failed: %v", err)
	}
	if !isValid {
		t.Fatalf("Expected valid token to be valid")
	}

	// Test IsTokenValid with non-existent token
	isValid, err = db.IsTokenValid(ctx, "non-existent")
	if err != nil {
		t.Fatalf("IsTokenValid with non-existent token failed: %v", err)
	}
	if isValid {
		t.Fatalf("Expected non-existent token to be invalid")
	}

	// Test GetStats
	stats, err := db.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats["project_count"].(int) != 1 {
		t.Fatalf("Expected 1 project, got %d", stats["project_count"].(int))
	}
	if stats["active_token_count"].(int) != 1 {
		t.Fatalf("Expected 1 active token, got %d", stats["active_token_count"].(int))
	}

	// Test MaintainDatabase
	err = db.MaintainDatabase(ctx)
	if err != nil {
		t.Fatalf("MaintainDatabase failed: %v", err)
	}

	// Test BackupDatabase
	backupPath := os.TempDir() + "/llm-proxy-test-backup.db"
	defer func() { _ = os.Remove(backupPath) }()

	err = db.BackupDatabase(ctx, backupPath)
	if err != nil {
		t.Fatalf("BackupDatabase failed: %v", err)
	}

	// Verify backup file was created
	_, err = os.Stat(backupPath)
	if err != nil {
		t.Fatalf("Backup file was not created: %v", err)
	}
}

func TestBackupDatabase_Error(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	// Use an invalid path
	err := db.BackupDatabase(ctx, "/invalid/path/backup.db")
	if err == nil {
		t.Error("expected error for invalid backup path")
	}
}

func TestMaintainDatabase_Error(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	// Close DB to force error
	_ = db.Close()
	err := db.MaintainDatabase(ctx)
	if err == nil {
		t.Error("expected error for closed DB in MaintainDatabase")
	}
}

func TestGetStats_Error(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	// Close DB to force error
	_ = db.Close()
	_, err := db.GetStats(ctx)
	if err == nil {
		t.Error("expected error for closed DB in GetStats")
	}
}

func TestIsTokenValid_EdgeCases(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	// Create a project and token
	project := proxy.Project{ID: "p", Name: "P", APIKey: "k", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	_ = db.CreateProject(ctx, project)
	max := 1
	expired := time.Now().Add(-time.Hour)
	tokens := []Token{
		{Token: "inactive", ProjectID: project.ID, IsActive: false, CreatedAt: time.Now()},
		{Token: "expired", ProjectID: project.ID, IsActive: true, ExpiresAt: &expired, CreatedAt: time.Now()},
		{Token: "limited", ProjectID: project.ID, IsActive: true, MaxRequests: &max, RequestCount: 1, CreatedAt: time.Now()},
	}
	for _, tk := range tokens {
		_ = db.CreateToken(ctx, tk)
		valid, err := db.IsTokenValid(ctx, tk.Token)
		if err != nil {
			t.Errorf("IsTokenValid error: %v", err)
		}
		if valid {
			t.Errorf("Expected token %s to be invalid", tk.Token)
		}
	}
}

func TestMaintainDatabase_EmptyDB(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	if err := db.MaintainDatabase(ctx); err != nil {
		t.Fatalf("MaintainDatabase failed on empty DB: %v", err)
	}
}

func TestGetStats_EmptyDB(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	stats, err := db.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed on empty DB: %v", err)
	}
	if stats["project_count"].(int) != 0 {
		t.Errorf("expected 0 projects, got %d", stats["project_count"].(int))
	}
	if stats["active_token_count"].(int) != 0 {
		t.Errorf("expected 0 active tokens, got %d", stats["active_token_count"].(int))
	}
	if stats["expired_token_count"].(int) != 0 {
		t.Errorf("expected 0 expired tokens, got %d", stats["expired_token_count"].(int))
	}
	if stats["total_request_count"].(int64) != 0 {
		t.Errorf("expected 0 total requests, got %d", stats["total_request_count"].(int64))
	}
}

func TestMaintainDatabase_ClosedDB(t *testing.T) {
	db, cleanup := testDB(t)
	cleanup()
	ctx := context.Background()
	if err := db.MaintainDatabase(ctx); err == nil {
		t.Error("expected error for MaintainDatabase on closed DB")
	}
}

func TestIsTokenValid_ClosedDB(t *testing.T) {
	db, cleanup := testDB(t)
	cleanup()
	ctx := context.Background()
	_, err := db.IsTokenValid(ctx, "x")
	if err == nil {
		t.Error("expected error for IsTokenValid on closed DB")
	}
}

func TestBackupDatabase_Errors(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	// Empty path
	if err := db.BackupDatabase(ctx, ""); err == nil {
		t.Error("expected error for empty backup path")
	}
	// Invalid path (starts with -)
	if err := db.BackupDatabase(ctx, "-badpath.db"); err == nil {
		t.Error("expected error for invalid backup path")
	}
	// Simulate DB error by closing DB
	_ = db.Close()
	if err := db.BackupDatabase(ctx, "/tmp/llm-proxy-test-backup.db"); err == nil {
		t.Error("expected error for closed DB in BackupDatabase")
	}
}

func TestMaintainDatabase_DBError(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	_ = db.Close()
	if err := db.MaintainDatabase(ctx); err == nil {
		t.Error("expected error for closed DB in MaintainDatabase")
	}
}

func TestGetStats_DBError(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()
	ctx := context.Background()
	_ = db.Close()
	_, err := db.GetStats(ctx)
	if err == nil {
		t.Error("expected error for closed DB in GetStats")
	}
}

func TestBoolValue(t *testing.T) {
	sqliteDB := &DB{driver: DriverSQLite}
	postgresDB := &DB{driver: DriverPostgres}
	mysqlDB := &DB{driver: DriverMySQL}

	// SQLite uses 1/0 for boolean values
	if sqliteDB.boolValue(true) != 1 {
		t.Errorf("SQLite boolValue(true) = %v, want 1", sqliteDB.boolValue(true))
	}
	if sqliteDB.boolValue(false) != 0 {
		t.Errorf("SQLite boolValue(false) = %v, want 0", sqliteDB.boolValue(false))
	}

	// PostgreSQL uses true/false for boolean values
	if postgresDB.boolValue(true) != true {
		t.Errorf("PostgreSQL boolValue(true) = %v, want true", postgresDB.boolValue(true))
	}
	if postgresDB.boolValue(false) != false {
		t.Errorf("PostgreSQL boolValue(false) = %v, want false", postgresDB.boolValue(false))
	}

	// MySQL uses true/false for boolean values
	if mysqlDB.boolValue(true) != true {
		t.Errorf("MySQL boolValue(true) = %v, want true", mysqlDB.boolValue(true))
	}
	if mysqlDB.boolValue(false) != false {
		t.Errorf("MySQL boolValue(false) = %v, want false", mysqlDB.boolValue(false))
	}
}
