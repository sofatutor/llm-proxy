//go:build integration
// +build integration

// encryption_integration_test.go
// End-to-end integration tests verifying encryption works correctly with
// UsageStatsAggregator and CacheStatsAggregator when ENCRYPTION_KEY is set.
//
// These tests cover the bug class where token hashing was not applied to
// aggregator batch updates, causing "token not found" errors.

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/sofatutor/llm-proxy/internal/encryption"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupEncryptionIntegrationServer creates a server with encryption enabled.
// It returns the server, the DB, the hasher, and a cleanup function.
func setupEncryptionIntegrationServer(t *testing.T, enableCache bool) (*Server, *database.DB, encryption.TokenHasherInterface, func()) {
	t.Helper()

	// Set up environment variables for cache if needed
	if enableCache {
		os.Setenv("HTTP_CACHE_ENABLED", "true")
		os.Setenv("HTTP_CACHE_BACKEND", "in-memory")
	} else {
		os.Setenv("HTTP_CACHE_ENABLED", "false")
	}

	// Create a temporary database file
	dbFile, err := os.CreateTemp("", "llmproxy-encryption-integration-*.db")
	require.NoError(t, err)
	dbFile.Close()

	db, err := database.New(database.Config{Path: dbFile.Name()})
	require.NoError(t, err)

	// Create the token hasher for encryption
	hasher := encryption.NewTokenHasher()
	require.NotNil(t, hasher)

	// Create configuration
	cfg := &config.Config{
		ListenAddr:              ":0",
		RequestTimeout:          10 * time.Second,
		ManagementToken:         "integration-token",
		DatabasePath:            dbFile.Name(),
		EventBusBackend:         "in-memory",
		ObservabilityBufferSize: 100,
		UsageStatsBufferSize:    10, // Small buffer to trigger flushes quickly
		CacheStatsBufferSize:    10, // Small buffer to trigger flushes quickly
	}

	// Create stores with encryption wrapper
	tokenStore := encryption.NewSecureTokenStore(
		database.NewDBTokenStoreAdapter(db),
		hasher,
	)
	projectStore := db

	// Create server with token hasher for aggregators
	srv, err := NewWithDatabase(cfg, tokenStore, projectStore, db, WithTokenHasher(hasher))
	require.NoError(t, err)

	// Initialize API routes which sets up the aggregators
	err = srv.initializeAPIRoutes()
	require.NoError(t, err)

	cleanup := func() {
		// Gracefully stop aggregators to ensure final flush
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		db.Close()
		os.Remove(dbFile.Name())
		// Clean up env vars
		os.Unsetenv("HTTP_CACHE_ENABLED")
		os.Unsetenv("HTTP_CACHE_BACKEND")
	}

	return srv, db, hasher, cleanup
}

// TestEncryptionIntegration_UsageStatsWithHashedTokens verifies that when
// encryption is enabled, usage stats (request_count, last_used_at) are correctly
// recorded in the database using hashed token IDs.
func TestEncryptionIntegration_UsageStatsWithHashedTokens(t *testing.T) {
	srv, db, hasher, cleanup := setupEncryptionIntegrationServer(t, false)
	defer cleanup()

	ctx := context.Background()

	// Step 1: Create a project
	projectReq := map[string]string{
		"name":    "Encryption Test Project",
		"api_key": "sk-test1234567890abcdefghij",
	}
	projectBody, _ := json.Marshal(projectReq)

	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(projectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w := httptest.NewRecorder()

	srv.handleProjects(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Project creation failed: %s", w.Body.String())

	var projectResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &projectResp)
	require.NoError(t, err)
	projectID := projectResp["id"].(string)

	// Step 2: Create a token for the project
	tokenReq := map[string]interface{}{
		"project_id":       projectID,
		"name":             "Encryption Test Token",
		"duration_minutes": 60, // 1 hour expiry
	}
	tokenBody, _ := json.Marshal(tokenReq)

	req = httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(tokenBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w = httptest.NewRecorder()

	srv.handleTokens(w, req)
	require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
		"Token creation failed with status %d: %s", w.Code, w.Body.String())

	var tokenResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &tokenResp)
	require.NoError(t, err)
	rawTokenString := tokenResp["token"].(string)
	tokenID := tokenResp["id"].(string)

	// Step 3: Verify the token is stored with hashed lookup key
	// The token in the database should have a hashed "token" field
	hashedToken := hasher.CreateLookupKey(rawTokenString)
	t.Logf("Raw token: %s", rawTokenString[:20]+"...")
	t.Logf("Token ID: %s", tokenID)
	t.Logf("Hashed lookup key: %s", hashedToken[:20]+"...")

	// Step 4: Simulate requests that would increment usage stats
	// We can't easily make proxy requests in this test, but we can directly
	// call the UsageStatsAggregator if available
	if srv.usageStatsAgg != nil {
		// Record some usage via the aggregator
		for i := 0; i < 5; i++ {
			srv.usageStatsAgg.RecordTokenUsage(rawTokenString)
		}

		// Wait for flush
		time.Sleep(200 * time.Millisecond)

		// Force a flush by stopping the aggregator
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err = srv.usageStatsAgg.Stop(stopCtx)
		require.NoError(t, err)

		// Step 5: Verify the database has the correct counts for the hashed token
		// Query the database directly to check request_count
		var requestCount int
		var lastUsedAt *time.Time

		// The token should be stored with the hashed lookup key
		err = db.DB().QueryRowContext(ctx,
			"SELECT request_count, last_used_at FROM tokens WHERE token = ?",
			hashedToken,
		).Scan(&requestCount, &lastUsedAt)

		if err != nil {
			// If not found by hashed token, check what's actually in the DB
			rows, _ := db.DB().QueryContext(ctx, "SELECT id, token, request_count FROM tokens")
			t.Log("Tokens in database:")
			for rows.Next() {
				var id, tok string
				var count int
				rows.Scan(&id, &tok, &count)
				t.Logf("  id=%s, token=%s..., count=%d", id, tok[:20], count)
			}
			rows.Close()
			t.Fatalf("Failed to query token by hashed key: %v", err)
		}

		assert.Equal(t, 5, requestCount, "Expected 5 requests recorded for the hashed token")
		assert.NotNil(t, lastUsedAt, "Expected last_used_at to be set")
	} else {
		t.Skip("UsageStatsAggregator not initialized (db might be nil)")
	}
}

// TestEncryptionIntegration_CacheStatsWithHashedTokens verifies that when
// encryption is enabled, cache hit stats (cache_hit_count) are correctly
// recorded in the database using hashed token IDs.
func TestEncryptionIntegration_CacheStatsWithHashedTokens(t *testing.T) {
	srv, db, hasher, cleanup := setupEncryptionIntegrationServer(t, true)
	defer cleanup()

	ctx := context.Background()

	// Step 1: Create a project
	projectReq := map[string]string{
		"name":    "Cache Stats Test Project",
		"api_key": "sk-cachetest1234567890abcd",
	}
	projectBody, _ := json.Marshal(projectReq)

	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(projectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w := httptest.NewRecorder()

	srv.handleProjects(w, req)
	require.Equal(t, http.StatusCreated, w.Code, "Project creation failed: %s", w.Body.String())

	var projectResp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &projectResp)
	require.NoError(t, err)
	projectID := projectResp["id"].(string)

	// Step 2: Create a token for the project
	tokenReq := map[string]interface{}{
		"project_id":       projectID,
		"name":             "Cache Stats Test Token",
		"duration_minutes": 60, // 1 hour expiry
	}
	tokenBody, _ := json.Marshal(tokenReq)

	req = httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(tokenBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w = httptest.NewRecorder()

	srv.handleTokens(w, req)
	require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
		"Token creation failed with status %d: %s", w.Code, w.Body.String())

	var tokenResp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &tokenResp)
	require.NoError(t, err)
	rawTokenString := tokenResp["token"].(string)
	tokenID := tokenResp["id"].(string)

	hashedToken := hasher.CreateLookupKey(rawTokenString)
	t.Logf("Raw token: %s", rawTokenString[:20]+"...")
	t.Logf("Token ID: %s", tokenID)
	t.Logf("Hashed lookup key: %s", hashedToken[:20]+"...")

	// Step 3: Record cache hits via the aggregator
	if srv.cacheStatsAgg != nil {
		// Record some cache hits via the aggregator
		for i := 0; i < 7; i++ {
			srv.cacheStatsAgg.RecordCacheHit(rawTokenString)
		}

		// Wait for flush
		time.Sleep(200 * time.Millisecond)

		// Force a flush by stopping the aggregator
		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		err = srv.cacheStatsAgg.Stop(stopCtx)
		require.NoError(t, err)

		// Step 4: Verify the database has the correct cache hit count for the hashed token
		var cacheHitCount int

		err = db.DB().QueryRowContext(ctx,
			"SELECT cache_hit_count FROM tokens WHERE token = ?",
			hashedToken,
		).Scan(&cacheHitCount)

		if err != nil {
			// If not found by hashed token, check what's actually in the DB
			rows, _ := db.DB().QueryContext(ctx, "SELECT id, token, cache_hit_count FROM tokens")
			t.Log("Tokens in database:")
			for rows.Next() {
				var id, tok string
				var count int
				rows.Scan(&id, &tok, &count)
				t.Logf("  id=%s, token=%s..., count=%d", id, tok[:20], count)
			}
			rows.Close()
			t.Fatalf("Failed to query token by hashed key: %v", err)
		}

		assert.Equal(t, 7, cacheHitCount, "Expected 7 cache hits recorded for the hashed token")
	} else {
		t.Skip("CacheStatsAggregator not initialized (cache might be disabled)")
	}
}

// TestEncryptionIntegration_TokenLookupByRawToken verifies that tokens can be
// looked up using the raw token string, even when stored with hashed values.
// This is critical for authentication to work correctly.
func TestEncryptionIntegration_TokenLookupByRawToken(t *testing.T) {
	srv, db, hasher, cleanup := setupEncryptionIntegrationServer(t, false)
	defer cleanup()

	ctx := context.Background()

	// Step 1: Create a project
	projectReq := map[string]string{
		"name":    "Lookup Test Project",
		"api_key": "sk-lookuptest1234567890abc",
	}
	projectBody, _ := json.Marshal(projectReq)

	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(projectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w := httptest.NewRecorder()

	srv.handleProjects(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var projectResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &projectResp)
	projectID := projectResp["id"].(string)

	// Step 2: Create a token
	tokenReq := map[string]interface{}{
		"project_id":       projectID,
		"name":             "Lookup Test Token",
		"duration_minutes": 60, // 1 hour expiry
	}
	tokenBody, _ := json.Marshal(tokenReq)

	req = httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(tokenBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w = httptest.NewRecorder()

	srv.handleTokens(w, req)
	require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
		"Token creation failed with status %d", w.Code)

	var tokenResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &tokenResp)
	rawTokenString := tokenResp["token"].(string)

	// Step 3: Verify we can look up the token using the raw token string
	// The SecureTokenStore should hash the token for lookup
	tokenData, err := srv.tokenStore.GetTokenByToken(ctx, rawTokenString)
	require.NoError(t, err, "Should be able to look up token by raw string")
	assert.Equal(t, projectID, tokenData.ProjectID)
	assert.True(t, tokenData.IsActive)

	// Step 4: Verify the database stores the hashed token
	hashedToken := hasher.CreateLookupKey(rawTokenString)
	var storedToken string
	err = db.DB().QueryRowContext(ctx,
		"SELECT token FROM tokens WHERE id = ?",
		tokenData.ID,
	).Scan(&storedToken)
	require.NoError(t, err)

	// The stored token should be the hashed version, not the raw token
	assert.Equal(t, hashedToken, storedToken, "Database should store hashed token, not raw")
	assert.NotEqual(t, rawTokenString, storedToken, "Database should NOT store raw token")
}

// TestEncryptionIntegration_MixedOperations tests a realistic scenario with
// multiple tokens, concurrent operations, and both usage and cache stats.
func TestEncryptionIntegration_MixedOperations(t *testing.T) {
	srv, db, hasher, cleanup := setupEncryptionIntegrationServer(t, true)
	defer cleanup()

	ctx := context.Background()

	// Create a project
	projectReq := map[string]string{
		"name":    "Mixed Operations Project",
		"api_key": "sk-mixedops1234567890abcde",
	}
	projectBody, _ := json.Marshal(projectReq)

	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(projectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer integration-token")
	w := httptest.NewRecorder()
	srv.handleProjects(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var projectResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &projectResp)
	projectID := projectResp["id"].(string)

	// Create multiple tokens
	var tokens []string
	for i := 0; i < 3; i++ {
		tokenReq := map[string]interface{}{
			"project_id":       projectID,
			"name":             fmt.Sprintf("Token %d", i),
			"duration_minutes": 60, // 1 hour expiry
		}
		tokenBody, _ := json.Marshal(tokenReq)

		req = httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(tokenBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer integration-token")
		w = httptest.NewRecorder()
		srv.handleTokens(w, req)
		require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated,
			"Token creation failed with status %d", w.Code)

		var tokenResp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &tokenResp)
		tokens = append(tokens, tokenResp["token"].(string))
	}

	// Record mixed usage and cache stats
	if srv.usageStatsAgg != nil && srv.cacheStatsAgg != nil {
		// Token 0: 10 requests, 3 cache hits
		for i := 0; i < 10; i++ {
			srv.usageStatsAgg.RecordTokenUsage(tokens[0])
		}
		for i := 0; i < 3; i++ {
			srv.cacheStatsAgg.RecordCacheHit(tokens[0])
		}

		// Token 1: 5 requests, 5 cache hits
		for i := 0; i < 5; i++ {
			srv.usageStatsAgg.RecordTokenUsage(tokens[1])
			srv.cacheStatsAgg.RecordCacheHit(tokens[1])
		}

		// Token 2: 2 requests, 0 cache hits
		for i := 0; i < 2; i++ {
			srv.usageStatsAgg.RecordTokenUsage(tokens[2])
		}

		// Wait and flush
		time.Sleep(200 * time.Millisecond)

		stopCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		_ = srv.usageStatsAgg.Stop(stopCtx)
		_ = srv.cacheStatsAgg.Stop(stopCtx)

		// Verify all counts
		expectedUsage := []int{10, 5, 2}
		expectedCacheHits := []int{3, 5, 0}

		for i, rawToken := range tokens {
			hashedToken := hasher.CreateLookupKey(rawToken)

			var requestCount, cacheHitCount int
			err := db.DB().QueryRowContext(ctx,
				"SELECT request_count, cache_hit_count FROM tokens WHERE token = ?",
				hashedToken,
			).Scan(&requestCount, &cacheHitCount)
			require.NoError(t, err, "Failed to query token %d", i)

			assert.Equal(t, expectedUsage[i], requestCount, "Token %d: wrong request_count", i)
			assert.Equal(t, expectedCacheHits[i], cacheHitCount, "Token %d: wrong cache_hit_count", i)
		}
	} else {
		t.Skip("Aggregators not initialized")
	}
}
