package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	// Create a minimal config for testing
	cfg := &config.Config{
		ListenAddr:     ":8080",
		RequestTimeout: 30 * time.Second,
	}

	// Create a new server
	server := New(cfg)

	// Create a request to the health endpoint
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Create a ResponseRecorder to record the response
	rr := httptest.NewRecorder()

	// Call the health endpoint directly
	server.handleHealth(rr, req)

	// Check the status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, status)
	}

	// Check the content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type %s, got %s", "application/json", contentType)
	}

	// Parse the response body
	var response HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate the response fields
	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}

	if response.Version != Version {
		t.Errorf("Expected version '%s', got '%s'", Version, response.Version)
	}

	// Timestamp should be recent
	now := time.Now()
	diff := now.Sub(response.Timestamp)
	if diff > 5*time.Second {
		t.Errorf("Timestamp is too old: %v", response.Timestamp)
	}
}

func TestServerLifecycle(t *testing.T) {
	// Use a random port for this test to avoid conflicts
	cfg := &config.Config{
		ListenAddr:     ":0", // Random port
		RequestTimeout: 1 * time.Second,
	}

	// Create a new server
	server := New(cfg)

	// Start the server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start()
	}()

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Failed to shutdown server: %v", err)
	}

	// Check server error
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("Unexpected server error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not shut down within timeout")
	}
}