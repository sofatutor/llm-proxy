package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/audit"
	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogging_Integration(t *testing.T) {
	// Create temporary directory for audit log
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "audit.log")

	// Create configuration with audit logging enabled
	cfg := &config.Config{
		ListenAddr:              ":8080",
		ManagementToken:         "test-management-token",
		AuditEnabled:            true,
		AuditLogFile:            auditLogPath,
		AuditCreateDir:          true,
		LogLevel:                "info",
		LogFormat:               "json",
		EventBusBackend:         "in-memory",
		ObservabilityBufferSize: 100,
	}

	// Create stores using the standard database package
	db, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	// Ensure table schemas exist explicitly for in-memory DB
	require.NoError(t, database.DBInitForTests(db))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create server (use DBTokenStoreAdapter for token store)
	server, err := New(cfg, database.NewDBTokenStoreAdapter(db), db)
	require.NoError(t, err)
	defer func() { _ = server.Shutdown(context.Background()) }()

	// Verify audit logger is initialized
	assert.NotNil(t, server.auditLogger)

	// Test project creation - this should generate audit events
	projectReq := map[string]string{
		"name":    "Test Project",
		"api_key": "sk-test1234567890abcdef",
	}
	projectBody, _ := json.Marshal(projectReq)

	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(projectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-management-token")
	w := httptest.NewRecorder()

	server.handleProjects(w, req)

	// Should be successful
	assert.Equal(t, http.StatusCreated, w.Code)

	// Parse response to get project ID
	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	projectID := response["id"].(string)

	// Test token creation - this should also generate audit events
	tokenReq := map[string]interface{}{
		"project_id":       projectID,
		"duration_minutes": 60,
	}
	tokenBody, _ := json.Marshal(tokenReq)

	req2 := httptest.NewRequest("POST", "/manage/tokens", bytes.NewReader(tokenBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer test-management-token")
	w2 := httptest.NewRecorder()

	server.handleTokens(w2, req2)

	// Should be successful
	assert.Equal(t, http.StatusOK, w2.Code)

	// Close the server to ensure audit logs are flushed
	_ = server.Shutdown(context.Background())

	// Read audit log file
	auditData, err := os.ReadFile(auditLogPath)
	require.NoError(t, err)

	// Parse audit events
	auditLines := strings.Split(strings.TrimSpace(string(auditData)), "\n")
	require.GreaterOrEqual(t, len(auditLines), 2, "Expected at least 2 audit events")

	// Parse first audit event (project creation)
	var projectAuditEvent audit.Event
	err = json.Unmarshal([]byte(auditLines[0]), &projectAuditEvent)
	require.NoError(t, err)

	// Verify project creation audit event
	assert.Equal(t, audit.ActionProjectCreate, projectAuditEvent.Action)
	assert.Equal(t, audit.ActorManagement, projectAuditEvent.Actor)
	assert.Equal(t, audit.ResultSuccess, projectAuditEvent.Result)
	assert.Equal(t, projectID, projectAuditEvent.ProjectID)
	assert.NotEmpty(t, projectAuditEvent.RequestID)
	assert.Equal(t, "POST", projectAuditEvent.Details["http_method"])
	assert.Equal(t, "/manage/projects", projectAuditEvent.Details["endpoint"])
	assert.Equal(t, "Test Project", projectAuditEvent.Details["project_name"])

	// Parse second audit event (token creation)
	var tokenAuditEvent audit.Event
	err = json.Unmarshal([]byte(auditLines[1]), &tokenAuditEvent)
	require.NoError(t, err)

	// Verify token creation audit event
	assert.Equal(t, audit.ActionTokenCreate, tokenAuditEvent.Action)
	assert.Equal(t, audit.ActorManagement, tokenAuditEvent.Actor)
	assert.Equal(t, audit.ResultSuccess, tokenAuditEvent.Result)
	assert.Equal(t, projectID, tokenAuditEvent.ProjectID)
	assert.NotEmpty(t, tokenAuditEvent.RequestID)
	assert.Equal(t, "POST", tokenAuditEvent.Details["http_method"])
	assert.Equal(t, "/manage/tokens", tokenAuditEvent.Details["endpoint"])
	assert.Equal(t, float64(60), tokenAuditEvent.Details["duration_minutes"]) // JSON unmarshals numbers as float64
	assert.NotEmpty(t, tokenAuditEvent.Details["token_id"])
	assert.Contains(t, tokenAuditEvent.Details["token_id"].(string), "...") // Should be obfuscated
}

func TestAuditLogging_Disabled(t *testing.T) {
	// Create configuration with audit logging disabled
	cfg := &config.Config{
		ListenAddr:              ":8080",
		ManagementToken:         "test-management-token",
		AuditEnabled:            false,
		AuditLogFile:            "",
		LogLevel:                "info",
		LogFormat:               "json",
		EventBusBackend:         "in-memory",
		ObservabilityBufferSize: 100,
	}

	// Create stores using the standard database package
	db, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, database.DBInitForTests(db))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create server (use DBTokenStoreAdapter for token store)
	server, err := New(cfg, database.NewDBTokenStoreAdapter(db), db)
	require.NoError(t, err)
	defer func() { _ = server.Shutdown(context.Background()) }()

	// Verify audit logger is null logger (no-op)
	assert.NotNil(t, server.auditLogger)

	// Test should still work but no audit events should be generated
	projectReq := map[string]string{
		"name":    "Test Project",
		"api_key": "sk-test1234567890abcdef",
	}
	projectBody, _ := json.Marshal(projectReq)

	req := httptest.NewRequest("POST", "/manage/projects", bytes.NewReader(projectBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-management-token")
	w := httptest.NewRecorder()

	server.handleProjects(w, req)

	// Should still be successful
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAuditLogging_FailureEvents(t *testing.T) {
	// Create temporary directory for audit log
	tmpDir := t.TempDir()
	auditLogPath := filepath.Join(tmpDir, "audit.log")

	// Create configuration with audit logging enabled
	cfg := &config.Config{
		ListenAddr:              ":8080",
		ManagementToken:         "test-management-token",
		AuditEnabled:            true,
		AuditLogFile:            auditLogPath,
		AuditCreateDir:          true,
		LogLevel:                "info",
		LogFormat:               "json",
		EventBusBackend:         "in-memory",
		ObservabilityBufferSize: 100,
	}

	// Create stores using the standard database package
	db, err := database.New(database.Config{Path: ":memory:"})
	require.NoError(t, err)
	require.NoError(t, database.DBInitForTests(db))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	// Create server (use DBTokenStoreAdapter for token store)
	server, err := New(cfg, database.NewDBTokenStoreAdapter(db), db)
	require.NoError(t, err)
	defer func() { _ = server.Shutdown(context.Background()) }()

	// Test invalid project creation - should generate failure audit event
	req := httptest.NewRequest("POST", "/manage/projects", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-management-token")
	w := httptest.NewRecorder()

	server.handleProjects(w, req)

	// Should fail
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Close the server to ensure audit logs are flushed
	_ = server.Shutdown(context.Background())

	// Read audit log file
	auditData, err := os.ReadFile(auditLogPath)
	require.NoError(t, err)

	// Parse audit events
	auditLines := strings.Split(strings.TrimSpace(string(auditData)), "\n")
	require.Len(t, auditLines, 1, "Expected exactly 1 audit event")

	// Parse audit event
	var auditEvent audit.Event
	err = json.Unmarshal([]byte(auditLines[0]), &auditEvent)
	require.NoError(t, err)

	// Verify failure audit event
	assert.Equal(t, audit.ActionProjectCreate, auditEvent.Action)
	assert.Equal(t, audit.ActorManagement, auditEvent.Actor)
	assert.Equal(t, audit.ResultFailure, auditEvent.Result)
	assert.NotEmpty(t, auditEvent.RequestID)
	assert.Equal(t, "POST", auditEvent.Details["http_method"])
	assert.Equal(t, "/manage/projects", auditEvent.Details["endpoint"])
	assert.Equal(t, "invalid request body", auditEvent.Details["validation_error"])
	assert.NotEmpty(t, auditEvent.Details["error"])
}
