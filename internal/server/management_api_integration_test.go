//go:build integration
// +build integration

// management_api_integration_test.go
// Integration/E2E tests for the Management API: covers full HTTP stack, real SQLite DB, and all endpoints.

package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/sofatutor/llm-proxy/internal/database"
	"github.com/stretchr/testify/require"
)

func setupIntegrationServer(t *testing.T) (*httptest.Server, func()) {
	dbFile, err := os.CreateTemp("", "llmproxy-integration-*.db")
	require.NoError(t, err)
	dbFile.Close()

	db, err := database.New(database.Config{Path: dbFile.Name()})
	require.NoError(t, err)

	cfg := &config.Config{
		ListenAddr:      ":0",
		RequestTimeout:  10 * time.Second,
		ManagementToken: "integration-token",
		DatabasePath:    dbFile.Name(),
	}
	tokenStore := database.NewDBTokenStoreAdapter(db)
	projectStore := db

	srv, err := New(cfg, tokenStore, projectStore)
	require.NoError(t, err)

	ts := httptest.NewServer(srv.server.Handler)

	cleanup := func() {
		ts.Close()
		db.Close()
		os.Remove(dbFile.Name())
	}
	return ts, cleanup
}

func TestManagementAPI_Integration(t *testing.T) {
	ts, cleanup := setupIntegrationServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 5 * time.Second}
	headers := map[string]string{
		"Authorization": "Bearer integration-token",
		"Content-Type":  "application/json",
	}

	// 1. Create Project
	projReq := map[string]string{"name": "Integration Project", "openai_api_key": "sk-test"}
	projBody, _ := json.Marshal(projReq)
	req, _ := http.NewRequest("POST", ts.URL+"/manage/projects", bytes.NewReader(projBody))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var projResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&projResp))
	resp.Body.Close()
	projID := projResp["id"].(string)

	// 2. Get Project
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/manage/projects/%s", ts.URL, projID), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 3. Update Project
	updReq := map[string]string{"name": "Updated Name"}
	updBody, _ := json.Marshal(updReq)
	req, _ = http.NewRequest("PATCH", fmt.Sprintf("%s/manage/projects/%s", ts.URL, projID), bytes.NewReader(updBody))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 4. List Projects
	req, _ = http.NewRequest("GET", ts.URL+"/manage/projects", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	getClient := &http.Client{Timeout: 5 * time.Second}
	resp, err = getClient.Do(req)
	if err != nil {
		t.Fatalf("List projects request failed: %v", err)
	}
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 5. Create Token
	tokReq := map[string]interface{}{"project_id": projID, "duration_minutes": 60 * 24}
	tokBody, _ := json.Marshal(tokReq)
	req, _ = http.NewRequest("POST", ts.URL+"/manage/tokens", bytes.NewReader(tokBody))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tokResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tokResp))
	resp.Body.Close()

	// 6. List Tokens
	req, _ = http.NewRequest("GET", ts.URL+"/manage/tokens", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 7. Delete Project
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/manage/projects/%s", ts.URL, projID), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}
