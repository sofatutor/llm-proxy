package database

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDatabaseUtils tests the database utility functions.
func TestDatabaseUtils(t *testing.T) {
	db, cleanup := testDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a test project
	project := Project{
		ID:           "test-project-id",
		Name:         "Test Project",
		OpenAIAPIKey: "test-api-key",
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
	isValid, err := db.IsTokenValid(ctx, validToken.Token)
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
	project := Project{ID: "p", Name: "P", OpenAIAPIKey: "k", CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
