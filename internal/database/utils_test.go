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
