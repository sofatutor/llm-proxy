//go:build integration
// +build integration

// management_api_integration_test.go
// Integration/E2E tests for the Management API: covers full HTTP stack, real SQLite DB, and all endpoints.

package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func setupIntegrationServer(t *testing.T) (*Server, *httptest.Server, func()) {
	dbFile, err := os.CreateTemp("", "llmproxy-integration-*.db")
	require.NoError(t, err)
	dbFile.Close()

	db, err := sql.Open("sqlite3", dbFile.Name())
	require.NoError(t, err)

	cfg := testConfigWithDB(dbFile.Name())
	srv, err := NewWithDB(cfg, db)
	require.NoError(t, err)

	ts := httptest.NewServer(srv.server.Handler)

	cleanup := func() {
		ts.Close()
		db.Close()
		os.Remove(dbFile.Name())
	}
	return srv, ts, cleanup
}

func testConfigWithDB(dbPath string) *Config {
	return &Config{
		ListenAddr:      ":0",
		RequestTimeout:  10 * time.Second,
		ManagementToken: "integration-token",
		DatabasePath:    dbPath,
	}
}

func TestManagementAPI_Integration(t *testing.T) {
	srv, ts, cleanup := setupIntegrationServer(t)
	defer cleanup()

	client := &http.Client{Timeout: 5 * time.Second}
	headers := map[string]string{
		"Authorization": "Bearer integration-token",
		"Content-Type":  "application/json",
	}

	// 1. Create Project
	projReq := map[string]string{"name": "Integration Project", "openai_api_key": "sk-test"}
	projBody, _ := json.Marshal(projReq)
	resp, err := http.Post(ts.URL+"/manage/projects", "application/json", bytes.NewReader(projBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var projResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&projResp))
	resp.Body.Close()
	projID := projResp["id"].(string)

	// 2. Get Project
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/manage/projects/%s", ts.URL, projID), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
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
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 5. Create Token
	tokReq := map[string]interface{}{"project_id": projID, "duration_hours": 24}
	tokBody, _ := json.Marshal(tokReq)
	resp, err = http.Post(ts.URL+"/manage/tokens", "application/json", bytes.NewReader(tokBody))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var tokResp map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tokResp))
	resp.Body.Close()
	tokenID := tokResp["token"].(string)

	// 6. Get Token
	req, _ = http.NewRequest("GET", fmt.Sprintf("%s/manage/tokens/%s", ts.URL, tokenID), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 7. List Tokens
	req, _ = http.NewRequest("GET", ts.URL+"/manage/tokens", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// 8. Delete Token
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/manage/tokens/%s", ts.URL, tokenID), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	// 9. Delete Project
	req, _ = http.NewRequest("DELETE", fmt.Sprintf("%s/manage/projects/%s", ts.URL, projID), nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()
}
