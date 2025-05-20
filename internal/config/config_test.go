package config

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, env := range os.Environ() {
		for i := 0; i < len(env); i++ {
			if env[i] == '=' {
				originalEnv[env[:i]] = env[i+1:]
				break
			}
		}
	}

	// Restore environment after test
	defer func() {
		os.Clearenv()
		for k, v := range originalEnv {
			os.Setenv(k, v)
		}
	}()

	t.Run("DefaultValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Set required fields
		os.Setenv("MANAGEMENT_TOKEN", "test-token")
		
		config, err := New()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check default values
		if config.ListenAddr != ":8080" {
			t.Errorf("Expected ListenAddr to be :8080, got %s", config.ListenAddr)
		}
		if config.RequestTimeout != 30*time.Second {
			t.Errorf("Expected RequestTimeout to be 30s, got %s", config.RequestTimeout)
		}
		if config.MaxRequestSize != 10*1024*1024 {
			t.Errorf("Expected MaxRequestSize to be 10MB, got %d", config.MaxRequestSize)
		}
		if config.DatabasePath != "./data/llm-proxy.db" {
			t.Errorf("Expected DatabasePath to be ./data/llm-proxy.db, got %s", config.DatabasePath)
		}
		if config.ManagementToken != "test-token" {
			t.Errorf("Expected ManagementToken to be test-token, got %s", config.ManagementToken)
		}
	})

	t.Run("CustomValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Set custom values
		os.Setenv("LISTEN_ADDR", ":9090")
		os.Setenv("REQUEST_TIMEOUT", "45s")
		os.Setenv("MAX_REQUEST_SIZE", "5242880") // 5MB
		os.Setenv("DATABASE_PATH", "./test.db")
		os.Setenv("MANAGEMENT_TOKEN", "custom-token")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("ENABLE_METRICS", "false")
		
		config, err := New()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		
		// Check custom values
		if config.ListenAddr != ":9090" {
			t.Errorf("Expected ListenAddr to be :9090, got %s", config.ListenAddr)
		}
		if config.RequestTimeout != 45*time.Second {
			t.Errorf("Expected RequestTimeout to be 45s, got %s", config.RequestTimeout)
		}
		if config.MaxRequestSize != 5*1024*1024 {
			t.Errorf("Expected MaxRequestSize to be 5MB, got %d", config.MaxRequestSize)
		}
		if config.DatabasePath != "./test.db" {
			t.Errorf("Expected DatabasePath to be ./test.db, got %s", config.DatabasePath)
		}
		if config.ManagementToken != "custom-token" {
			t.Errorf("Expected ManagementToken to be custom-token, got %s", config.ManagementToken)
		}
		if config.LogLevel != "debug" {
			t.Errorf("Expected LogLevel to be debug, got %s", config.LogLevel)
		}
		if config.EnableMetrics != false {
			t.Errorf("Expected EnableMetrics to be false, got %v", config.EnableMetrics)
		}
	})

	t.Run("MissingRequiredValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Don't set required fields
		config, err := New()
		if err == nil {
			t.Fatalf("Expected error for missing MANAGEMENT_TOKEN, got none")
		}
		if config != nil {
			t.Errorf("Expected nil config when validation fails, got %+v", config)
		}
	})

	t.Run("InvalidValues", func(t *testing.T) {
		// Clear environment
		os.Clearenv()
		
		// Set required fields
		os.Setenv("MANAGEMENT_TOKEN", "test-token")
		
		// Set invalid values that should be replaced with defaults
		os.Setenv("REQUEST_TIMEOUT", "invalid")
		os.Setenv("MAX_REQUEST_SIZE", "invalid")
		os.Setenv("ENABLE_METRICS", "invalid")
		
		config, err := New()
		if err != nil {
			t.Fatalf("Expected no error despite invalid values, got %v", err)
		}
		
		// Should use defaults for invalid values
		if config.RequestTimeout != 30*time.Second {
			t.Errorf("Expected default RequestTimeout (30s) for invalid input, got %s", config.RequestTimeout)
		}
		if config.MaxRequestSize != 10*1024*1024 {
			t.Errorf("Expected default MaxRequestSize (10MB) for invalid input, got %d", config.MaxRequestSize)
		}
		if config.EnableMetrics != true {
			t.Errorf("Expected default EnableMetrics (true) for invalid input, got %v", config.EnableMetrics)
		}
	})
}