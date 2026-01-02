//go:build mysql && integration

// Package database provides MySQL integration tests.
// These tests require a real MySQL instance and are run with:
//
//	go test -v -race -tags=mysql,integration ./internal/database/...
//
// Environment variables required:
//
//	TEST_MYSQL_URL - MySQL connection string (e.g., user:pass@tcp(localhost:3306)/db?parseTime=true)
//
// Run MySQL with Docker Compose:
//
//	./scripts/run-mysql-integration.sh
package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/google/uuid"
	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/database/migrations"
	"github.com/sofatutor/llm-proxy/internal/proxy"
	tokenpkg "github.com/sofatutor/llm-proxy/internal/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestMySQLURL returns the MySQL connection URL from environment
// or skips the test if not available.
func getTestMySQLURL(t *testing.T) string {
	t.Helper()

	url := os.Getenv("TEST_MYSQL_URL")
	if url == "" {
		url = os.Getenv("DATABASE_URL")
	}
	if url == "" {
		t.Skip("TEST_MYSQL_URL or DATABASE_URL not set; skipping MySQL integration test")
	}

	return url
}

// setupMySQLTestDB creates a test database connection and runs migrations.
// It returns a cleanup function that should be deferred.
func setupMySQLTestDB(t *testing.T) (*DB, func()) {
	t.Helper()

	url := getTestMySQLURL(t)

	config := FullConfig{
		Driver:          DriverMySQL,
		DatabaseURL:     url,
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Minute,
	}

	db, err := NewFromConfig(config)
	require.NoError(t, err, "Failed to create MySQL database connection")

	cleanup := func() {
		// Clean up test data
		// Note: projects deletion cascades to tokens (FK with ON DELETE CASCADE)
		ctx := context.Background()
		if _, err := db.db.ExecContext(ctx, "DELETE FROM audit_events"); err != nil {
			t.Logf("Warning: Failed to clean up audit_events: %v", err)
		}
		if _, err := db.db.ExecContext(ctx, "DELETE FROM projects"); err != nil {
			t.Logf("Warning: Failed to clean up projects: %v", err)
		}
		if err := db.Close(); err != nil {
			t.Logf("Warning: Failed to close DB: %v", err)
		}
	}

	return db, cleanup
}

// TestMySQLIntegration_Connection tests basic MySQL connectivity.
func TestMySQLIntegration_Connection(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Test ping
	err := db.db.PingContext(ctx)
	require.NoError(t, err, "MySQL ping failed")

	// Test simple query
	var result int
	err = db.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err, "MySQL query failed")
	assert.Equal(t, 1, result)
}

// TestMySQLIntegration_Migrations tests that migrations run correctly on MySQL.
func TestMySQLIntegration_Migrations(t *testing.T) {
	url := getTestMySQLURL(t)

	// Open a raw connection for migration testing
	rawDB, err := sql.Open("mysql", url)
	require.NoError(t, err, "Failed to open MySQL connection")
	defer rawDB.Close()

	// Get migrations path
	migrationsPath, err := getMigrationsPathForDialect("mysql")
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

// TestMySQLIntegration_ProjectCRUD tests Project CRUD operations on MySQL.
func TestMySQLIntegration_ProjectCRUD(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// Create project using proxy.Project (what the interface expects)
	project := proxy.Project{
		ID:        "test-project-mysql-" + time.Now().Format("20060102150405"),
		Name:      "MySQL Test Project",
		APIKey:    "test-api-key-12345",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
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

	// Update project (UpdateProject takes a full proxy.Project)
	updatedProject := retrieved
	updatedProject.Name = "Updated MySQL Project"
	err = db.UpdateProject(ctx, updatedProject)
	require.NoError(t, err, "Failed to update project")

	updated, err := db.GetProjectByID(ctx, project.ID)
	require.NoError(t, err, "Failed to get updated project")
	assert.Equal(t, "Updated MySQL Project", updated.Name)

	// List projects
	projects, err := db.ListProjects(ctx)
	require.NoError(t, err, "Failed to list projects")
	assert.NotEmpty(t, projects)

	// Delete project (this does a hard delete in current implementation)
	err = db.DeleteProject(ctx, project.ID)
	require.NoError(t, err, "Failed to delete project")

	// After deletion, project should not be found
	_, err = db.GetProjectByID(ctx, project.ID)
	assert.Error(t, err, "Expected error when getting deleted project")
}

// TestMySQLIntegration_TokenCRUD tests Token CRUD operations on MySQL.
func TestMySQLIntegration_TokenCRUD(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// First create a project (required for token foreign key)
	project := proxy.Project{
		ID:        uuid.NewString(),
		Name:      "Token Test Project",
		APIKey:    "test-api-key-tokens",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create project for token test")

	// Create token
	expiresAt := now.Add(24 * time.Hour)
	tokenID := uuid.NewString()
	tokenSecret := "tok-" + uuid.NewString()
	token := Token{
		ID:           tokenID,
		Token:        tokenSecret,
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
	retrieved, err := db.GetTokenByID(ctx, token.ID)
	require.NoError(t, err, "Failed to get token by ID")
	assert.Equal(t, token.Token, retrieved.Token)
	assert.Equal(t, token.ID, retrieved.ID)
	assert.Equal(t, token.ProjectID, retrieved.ProjectID)
	assert.True(t, retrieved.IsActive)
	assert.Equal(t, 0, retrieved.RequestCount)

	// Increment usage
	err = db.IncrementTokenUsage(ctx, token.Token)
	require.NoError(t, err, "Failed to increment token usage")

	incremented, err := db.GetTokenByID(ctx, token.ID)
	require.NoError(t, err, "Failed to get incremented token")
	assert.Equal(t, 1, incremented.RequestCount)
	assert.NotNil(t, incremented.LastUsedAt)

	// Update token to reset usage (no ResetTokenUsage method, use UpdateToken)
	incremented.RequestCount = 0
	err = db.UpdateToken(ctx, incremented)
	require.NoError(t, err, "Failed to reset token usage via UpdateToken")

	reset, err := db.GetTokenByID(ctx, token.ID)
	require.NoError(t, err, "Failed to get reset token")
	assert.Equal(t, 0, reset.RequestCount)

	// List tokens for project (use GetTokensByProjectID)
	tokens, err := db.GetTokensByProjectID(ctx, project.ID)
	require.NoError(t, err, "Failed to list tokens")
	assert.NotEmpty(t, tokens)

	// Revoke token via UpdateToken (set IsActive = false)
	reset.IsActive = false
	nowPtr := time.Now()
	reset.DeactivatedAt = &nowPtr
	err = db.UpdateToken(ctx, reset)
	require.NoError(t, err, "Failed to revoke token")

	revoked, err := db.GetTokenByID(ctx, token.ID)
	require.NoError(t, err, "Failed to get revoked token")
	assert.False(t, revoked.IsActive)
}

// TestMySQLIntegration_AuditEvents tests audit event operations on MySQL.
func TestMySQLIntegration_AuditEvents(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create audit event using the audit.Event type
	event := audit.NewEvent("test_action", "test_actor", audit.ResultSuccess).
		WithProjectID("test-project").
		WithRequestID("req-123").
		WithCorrelationID("corr-456").
		WithClientIP("192.168.1.1").
		WithHTTPMethod("POST").
		WithEndpoint("/v1/chat/completions").
		WithUserAgent("test-agent/1.0").
		WithDetail("key", "value")

	err := db.StoreAuditEvent(ctx, event)
	require.NoError(t, err, "Failed to store audit event")

	// Query audit events using AuditEventFilters
	events, err := db.ListAuditEvents(ctx, AuditEventFilters{
		Action: "test_action",
		Limit:  10,
	})
	require.NoError(t, err, "Failed to list audit events")
	assert.NotEmpty(t, events)

	// Verify event data
	found := false
	for _, e := range events {
		if e.Action == "test_action" && e.Actor == "test_actor" {
			found = true
			assert.Equal(t, "success", e.Outcome)
			break
		}
	}
	assert.True(t, found, "Created audit event not found in query results")
}

// TestMySQLIntegration_PlaceholderRebinding tests that ? placeholders work correctly.
func TestMySQLIntegration_PlaceholderRebinding(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// Create a project using the rebinding helper
	project := proxy.Project{
		ID:        "rebind-test-" + time.Now().Format("20060102150405"),
		Name:      "Rebind Test",
		APIKey:    "rebind-api-key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := db.CreateProject(ctx, project)
	require.NoError(t, err, "Failed to create project with rebinding")

	// Verify the project was created correctly
	retrieved, err := db.GetProjectByID(ctx, project.ID)
	require.NoError(t, err, "Failed to retrieve project")
	assert.Equal(t, project.Name, retrieved.Name)
}

// TestMySQLIntegration_ConcurrentOperations tests concurrent database access.
func TestMySQLIntegration_ConcurrentOperations(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	// Create a project for concurrent token operations
	project := proxy.Project{
		ID:        uuid.NewString(),
		Name:      "Concurrent Test Project",
		APIKey:    "concurrent-api-key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err := db.CreateProject(ctx, project)
	require.NoError(t, err)

	// Create a token
	tokenID := uuid.NewString()
	tokenSecret := "tok-" + uuid.NewString()
	token := Token{
		ID:           tokenID,
		Token:        tokenSecret,
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
	final, err := db.GetTokenByID(ctx, token.ID)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines, final.RequestCount, "Request count should match number of increments")
}

// TestMySQLIntegration_ConcurrentMaxRequestsEnforcement tests rate limiting under concurrent load.
func TestMySQLIntegration_ConcurrentMaxRequestsEnforcement(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	now := time.Now().Truncate(time.Second)

	project := proxy.Project{
		ID:        uuid.NewString(),
		Name:      "Concurrent Quota Test Project",
		APIKey:    "concurrent-quota-api-key",
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	require.NoError(t, db.CreateProject(ctx, project))

	maxRequests := 25
	tokenID := uuid.NewString()
	tokenSecret := "tok-" + uuid.NewString()
	tk := Token{
		ID:           tokenID,
		Token:        tokenSecret,
		ProjectID:    project.ID,
		IsActive:     true,
		RequestCount: 0,
		MaxRequests:  ptrInt(maxRequests),
		CreatedAt:    now,
	}
	require.NoError(t, db.CreateToken(ctx, tk))

	// Fan out more increments than the quota allows.
	numGoroutines := maxRequests * 4

	var successCount atomic.Int64
	var rateLimitCount atomic.Int64

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errorsCh := make(chan error, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			err := db.IncrementTokenUsage(ctx, tk.Token)
			if err == nil {
				successCount.Add(1)
				return
			}
			if errors.Is(err, ErrTokenNotFound) {
				errorsCh <- err
				return
			}
			if errors.Is(err, tokenpkg.ErrTokenRateLimit) {
				rateLimitCount.Add(1)
				return
			}
			errorsCh <- err
		}()
	}

	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		require.NoError(t, err)
	}

	final, err := db.GetTokenByID(ctx, tk.ID)
	require.NoError(t, err)
	assert.Equal(t, maxRequests, final.RequestCount)
	assert.Equal(t, int64(maxRequests), successCount.Load())
	assert.Equal(t, int64(numGoroutines-maxRequests), rateLimitCount.Load())
}

// TestMySQLIntegration_TransactionRollback tests that failed operations don't leave partial state.
func TestMySQLIntegration_TransactionRollback(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Try to create a token without a valid project (should fail due to foreign key)
	token := Token{
		ID:           uuid.NewString(),
		Token:        "tok-" + uuid.NewString(),
		ProjectID:    uuid.NewString(),
		IsActive:     true,
		RequestCount: 0,
		CreatedAt:    time.Now(),
	}

	err := db.CreateToken(ctx, token)
	assert.Error(t, err, "Creating token with non-existent project should fail")

	// Verify token wasn't created
	_, err = db.GetTokenByID(ctx, token.ID)
	assert.Error(t, err, "Token should not exist after failed creation")
}

// TestMySQLIntegration_GetStats tests database statistics on MySQL.
func TestMySQLIntegration_GetStats(t *testing.T) {
	db, cleanup := setupMySQLTestDB(t)
	defer cleanup()

	ctx := context.Background()

	stats, err := db.GetStats(ctx)
	require.NoError(t, err, "GetStats should not return error")

	assert.NotNil(t, stats)
	assert.Contains(t, stats, "database_size_bytes")
	assert.Contains(t, stats, "project_count")
	assert.Contains(t, stats, "active_token_count")
}

// Helper function to create a pointer to an int
func ptrInt(i int) *int {
	return &i
}
