//go:build postgres && integration

// Package database provides PostgreSQL integration tests.
// These tests require a real PostgreSQL instance and are run with:
//
//	go test -v -race -tags=postgres,integration ./internal/database/...
//
// Environment variables required:
//
//	TEST_POSTGRES_URL - PostgreSQL connection string (e.g., postgres://user:pass@localhost:5432/db?sslmode=disable)
//
// Run PostgreSQL with Docker Compose:
//
//	./scripts/run-postgres-integration.sh
package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sofatutor/llm-proxy/internal/database/migrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestPostgresURL returns the PostgreSQL connection URL from environment
// or skips the test if not available.
func getTestPostgresURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("TEST_POSTGRES_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("TEST_POSTGRES_URL or DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	return url
}

// setupPostgresTestDB creates a test database connection and runs migrations.
// It returns a cleanup function that should be deferred.
func setupPostgresTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	url := getTestPostgresURL(t)

	config := FullConfig{
		Driver:          DriverPostgres,
		DatabaseURL:     url,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err, "Failed to create PostgreSQL database connection")

	cleanup := func() {
		// Clean up test data
		// Note: projects deletion cascades to tokens (FK with ON DELETE CASCADE)
		ctx := context.Background()
		_, _ = db.db.ExecContext(ctx, "DELETE FROM audit_events")
		_, _ = db.db.ExecContext(ctx, "DELETE FROM projects") // Cascades to tokens
		_ = db.Close()
	}

	return db, cleanup
}

// TestPostgresIntegration_Connection tests basic PostgreSQL connectivity.
func TestPostgresIntegration_Connection(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test ping
	err := db.db.PingContext(ctx)
	require.NoError(t, err, "PostgreSQL ping failed")

	// Test simple query
	var result int
	err = db.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err, "PostgreSQL query failed")
	assert.Equal(t, 1, result)
}

// TestPostgresIntegration_Migrations tests that migrations run correctly on PostgreSQL.
func TestPostgresIntegration_Migrations(t *testing.T) {
	url := getTestPostgresURL(t)

	// Open a raw connection for migration testing
	rawDB, err := sql.Open("pgx", url)
	require.NoError(t, err, "Failed to open PostgreSQL connection")
	defer rawDB.Close()

	// Get migrations path
	migrationsPath, err := getMigrationsPathForDialect("postgres")
	require.NoError(t, err, "Failed to get migrations path")

	// Create migration runner
	runner := migrations.NewMigrationRunner(rawDB, migrationsPath)

	// Get current version
	version, err := runner.Version()
	require.NoError(t, err, "Failed to get migration version")
	t.Logf("Current migration version: %d", version)

	// Migrations should already be applied (by NewFromConfig)
	assert.GreaterOrEqual(t, version, int64(1), "Expected at least migration 1 to be applied")
}

// TestPostgresIntegration_ProjectCRUD tests Project CRUD operations on PostgreSQL.
func TestPostgresIntegration_ProjectCRUD(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create project
	project := Project{
		ID:        "test-project-pg-" + time.Now().Format("20060102150405"),
		Name:      "PostgreSQL Test Project",
		APIKey:    "test-api-key-12345",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create project")

	// Read project
	retrieved, err := db.GetProjectByID(ctx, project.ID)
	require.NoError(t, err, "Failed to get project by ID")
	assert.Equal(t, project.ID, retrieved.ID)
	assert.Equal(t, project.Name, retrieved.Name)
	assert.Equal(t, project.APIKey, retrieved.APIKey)
	assert.True(t, retrieved.IsActive)

	// Update project
	err = db.UpdateProject(ctx, project.ID, map[string]interface{}{
		"name": "Updated PostgreSQL Project",
	})
	require.NoError(t, err, "Failed to update project")

	updated, err := db.GetProjectByID(ctx, project.ID)
	require.NoError(t, err, "Failed to get updated project")
	assert.Equal(t, "Updated PostgreSQL Project", updated.Name)

	// List projects
	projects, err := db.ListProjects(ctx)
	require.NoError(t, err, "Failed to list projects")
	assert.NotEmpty(t, projects)

	// Delete project (deactivate)
	err = db.DeleteProject(ctx, project.ID)
	require.NoError(t, err, "Failed to delete project")

	deleted, err := db.GetProjectByID(ctx, project.ID)
	require.NoError(t, err, "Failed to get deleted project")
	assert.False(t, deleted.IsActive)
}

// TestPostgresIntegration_TokenCRUD tests Token CRUD operations on PostgreSQL.
func TestPostgresIntegration_TokenCRUD(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// First create a project (required for token foreign key)
	project := Project{
		ID:        "token-test-project-" + time.Now().Format("20060102150405"),
		Name:      "Token Test Project",
		APIKey:    "test-api-key-tokens",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create project for token test")

	// Create token
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)
	token := Token{
		Token:        "test-token-pg-" + time.Now().Format("20060102150405"),
		ProjectID:    project.ID,
		ExpiresAt:    &expiresAt,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  ptrInt(1000),
		CreatedAt:    now,
	}

	err = db.CreateToken(ctx, token)
	require.NoError(t, err, "Failed to create token")

	// Read token
	retrieved, err := db.GetTokenByID(ctx, token.Token)
	require.NoError(t, err, "Failed to get token by ID")
	assert.Equal(t, token.Token, retrieved.Token)
	assert.Equal(t, token.ProjectID, retrieved.ProjectID)
	assert.True(t, retrieved.IsActive)
	assert.Equal(t, 0, retrieved.RequestCount)

	// Increment usage
	err = db.IncrementTokenUsage(ctx, token.Token)
	require.NoError(t, err, "Failed to increment token usage")

	incremented, err := db.GetTokenByID(ctx, token.Token)
	require.NoError(t, err, "Failed to get incremented token")
	assert.Equal(t, 1, incremented.RequestCount)
	assert.NotNil(t, incremented.LastUsedAt)

	// Reset usage
	err = db.ResetTokenUsage(ctx, token.Token)
	require.NoError(t, err, "Failed to reset token usage")

	reset, err := db.GetTokenByID(ctx, token.Token)
	require.NoError(t, err, "Failed to get reset token")
	assert.Equal(t, 0, reset.RequestCount)

	// List tokens for project
	tokens, err := db.ListTokensForProject(ctx, project.ID)
	require.NoError(t, err, "Failed to list tokens")
	assert.NotEmpty(t, tokens)

	// Revoke token
	err = db.RevokeToken(ctx, token.Token)
	require.NoError(t, err, "Failed to revoke token")

	revoked, err := db.GetTokenByID(ctx, token.Token)
	require.NoError(t, err, "Failed to get revoked token")
	assert.False(t, revoked.IsActive)
}

// TestPostgresIntegration_AuditEvents tests audit event operations on PostgreSQL.
func TestPostgresIntegration_AuditEvents(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create audit event
	event := AuditEvent{
		ID:            "audit-event-pg-" + time.Now().Format("20060102150405"),
		Timestamp:     time.Now(),
		Action:        "test_action",
		Actor:         "test_actor",
		ProjectID:     "test-project",
		RequestID:     "req-123",
		CorrelationID: "corr-456",
		ClientIP:      "192.168.1.1",
		Method:        "POST",
		Path:          "/v1/chat/completions",
		UserAgent:     "test-agent/1.0",
		Outcome:       "success",
		Reason:        "",
		TokenID:       "token-123",
		Metadata:      `{"key": "value"}`,
	}

	err := db.CreateAuditEvent(ctx, event)
	require.NoError(t, err, "Failed to create audit event")

	// Query audit events
	events, err := db.GetAuditEvents(ctx, AuditEventQuery{
		Action: "test_action",
		Limit:  10,
	})
	require.NoError(t, err, "Failed to query audit events")
	assert.NotEmpty(t, events)

	// Verify event data
	found := false
	for _, e := range events {
		if e.ID == event.ID {
			found = true
			assert.Equal(t, event.Action, e.Action)
			assert.Equal(t, event.Actor, e.Actor)
			assert.Equal(t, event.Outcome, e.Outcome)
			break
		}
	}
	assert.True(t, found, "Created audit event not found in query results")
}

// TestPostgresIntegration_PlaceholderRebinding tests that $1, $2 placeholders work correctly.
func TestPostgresIntegration_PlaceholderRebinding(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a project using the rebinding helper
	project := Project{
		ID:        "rebind-test-" + time.Now().Format("20060102150405"),
		Name:      "Rebind Test",
		APIKey:    "rebind-api-key",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create project with rebinding")

	// Verify the project was created correctly
	retrieved, err := db.GetProjectByID(ctx, project.ID)
	require.NoError(t, err, "Failed to retrieve project")
	assert.Equal(t, project.Name, retrieved.Name)
}

// TestPostgresIntegration_ConcurrentOperations tests concurrent database access.
func TestPostgresIntegration_ConcurrentOperations(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a project for concurrent token operations
	project := Project{
		ID:        "concurrent-test-" + time.Now().Format("20060102150405"),
		Name:      "Concurrent Test Project",
		APIKey:    "concurrent-api-key",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := db.CreateProject(ctx, project)
	require.NoError(t, err)

	// Create a token
	now := time.Now()
	token := Token{
		Token:        "concurrent-token-" + time.Now().Format("20060102150405"),
		ProjectID:    project.ID,
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    now,
	}
	err = db.CreateToken(ctx, token)
	require.NoError(t, err)

	// Run concurrent increment operations
	numGoroutines := 10
	done := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			done <- db.IncrementTokenUsage(ctx, token.Token)
		}()
	}

	// Wait for all goroutines and check errors
	for i := 0; i < numGoroutines; i++ {
		err := <-done
		assert.NoError(t, err, "Concurrent increment failed")
	}

	// Verify final count
	final, err := db.GetTokenByID(ctx, token.Token)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines, final.RequestCount, "Request count should match number of increments")
}

// TestPostgresIntegration_TransactionRollback tests that failed operations don't leave partial state.
func TestPostgresIntegration_TransactionRollback(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to create a token without a valid project (should fail due to foreign key)
	token := Token{
		Token:        "invalid-project-token-" + time.Now().Format("20060102150405"),
		ProjectID:    "non-existent-project-id",
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    time.Now(),
	}

	err := db.CreateToken(ctx, token)
	assert.Error(t, err, "Creating token with non-existent project should fail")

	// Verify token wasn't created
	_, err = db.GetTokenByID(ctx, token.Token)
	assert.Error(t, err, "Token should not exist after failed creation")
}

// TestPostgresIntegration_GetStats tests database statistics on PostgreSQL.
func TestPostgresIntegration_GetStats(t *testing.T) {
	db, cleanup := setupPostgresTestDB(t)
	defer cleanup()

	stats := db.GetStats()

	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.MaxOpenConnections, 0)
	assert.GreaterOrEqual(t, stats.OpenConnections, 0)
}

// Helper function to create a pointer to an int
func ptrInt(i int) *int {
	return &i
}
