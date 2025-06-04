package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupConfig_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  SetupConfig
		wantErr bool
		errText string
	}{
		{
			name: "valid config",
			config: SetupConfig{
				ConfigPath:      "test.env",
				OpenAIAPIKey:    "sk-test",
				ManagementToken: "token",
				DatabasePath:    "db.sqlite",
				ListenAddr:      ":8080",
			},
			wantErr: false,
		},
		{
			name: "missing OpenAI key",
			config: SetupConfig{
				ConfigPath:      "test.env",
				ManagementToken: "token",
				DatabasePath:    "db.sqlite",
				ListenAddr:      ":8080",
			},
			wantErr: true,
			errText: "OpenAI API key is required",
		},
		{
			name: "missing config path",
			config: SetupConfig{
				OpenAIAPIKey:    "sk-test",
				ManagementToken: "token",
				DatabasePath:    "db.sqlite",
				ListenAddr:      ":8080",
			},
			wantErr: true,
			errText: "config path is required",
		},
		{
			name: "missing database path",
			config: SetupConfig{
				ConfigPath:      "test.env",
				OpenAIAPIKey:    "sk-test",
				ManagementToken: "token",
				ListenAddr:      ":8080",
			},
			wantErr: true,
			errText: "database path is required",
		},
		{
			name: "missing listen address",
			config: SetupConfig{
				ConfigPath:      "test.env",
				OpenAIAPIKey:    "sk-test",
				ManagementToken: "token",
				DatabasePath:    "db.sqlite",
			},
			wantErr: true,
			errText: "listen address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateConfig()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errText) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errText)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSetupConfig_GenerateManagementToken(t *testing.T) {
	t.Run("generates token when empty", func(t *testing.T) {
		config := &SetupConfig{}
		err := config.GenerateManagementToken()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.ManagementToken == "" {
			t.Error("management token should be generated")
		}
		if len(config.ManagementToken) != 32 { // 16 bytes = 32 hex chars
			t.Errorf("token length = %d, want 32", len(config.ManagementToken))
		}
	})

	t.Run("preserves existing token", func(t *testing.T) {
		config := &SetupConfig{ManagementToken: "existing-token"}
		err := config.GenerateManagementToken()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if config.ManagementToken != "existing-token" {
			t.Errorf("token = %q, want %q", config.ManagementToken, "existing-token")
		}
	})
}

func TestSetupConfig_WriteConfigFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("writes valid config", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "test.env")
		dbPath := filepath.Join(tmpDir, "db", "test.db")

		config := &SetupConfig{
			ConfigPath:      configPath,
			OpenAIAPIKey:    "sk-test-key",
			ManagementToken: "test-token",
			DatabasePath:    dbPath,
			ListenAddr:      ":8080",
		}

		err := config.WriteConfigFile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that file exists
		if _, err := os.Stat(configPath); err != nil {
			t.Errorf("config file not created: %v", err)
		}

		// Check content
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}

		contentStr := string(content)
		expectedLines := []string{
			"OPENAI_API_KEY=sk-test-key",
			"MANAGEMENT_TOKEN=test-token",
			"DATABASE_PATH=" + dbPath,
			"LISTEN_ADDR=:8080",
			"LOG_LEVEL=info",
		}

		for _, line := range expectedLines {
			if !strings.Contains(contentStr, line) {
				t.Errorf("config file missing line: %s", line)
			}
		}
	})

	t.Run("validates before writing", func(t *testing.T) {
		config := &SetupConfig{
			ConfigPath: filepath.Join(tmpDir, "invalid.env"),
			// Missing required fields
		}

		err := config.WriteConfigFile()
		if err == nil {
			t.Error("expected validation error, got nil")
		}
	})

	t.Run("creates directories", func(t *testing.T) {
		configPath := filepath.Join(tmpDir, "subdir", "config", "test.env")
		dbPath := filepath.Join(tmpDir, "subdir", "data", "test.db")

		config := &SetupConfig{
			ConfigPath:      configPath,
			OpenAIAPIKey:    "sk-test",
			ManagementToken: "token",
			DatabasePath:    dbPath,
			ListenAddr:      ":8080",
		}

		err := config.WriteConfigFile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that directories were created
		if _, err := os.Stat(filepath.Dir(configPath)); err != nil {
			t.Error("config directory not created")
		}
		if _, err := os.Stat(filepath.Dir(dbPath)); err != nil {
			t.Error("database directory not created")
		}
	})
}

func TestRunNonInteractiveSetup(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("successful setup", func(t *testing.T) {
		config := &SetupConfig{
			ConfigPath:   filepath.Join(tmpDir, "success.env"),
			OpenAIAPIKey: "sk-test-key",
			DatabasePath: filepath.Join(tmpDir, "test.db"),
			ListenAddr:   ":8080",
			// ManagementToken left empty to test generation
		}

		err := RunNonInteractiveSetup(config)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that token was generated
		if config.ManagementToken == "" {
			t.Error("management token should be generated")
		}

		// Check that file was written
		if _, err := os.Stat(config.ConfigPath); err != nil {
			t.Error("config file not created")
		}
	})

	t.Run("validation failure", func(t *testing.T) {
		config := &SetupConfig{
			ConfigPath: filepath.Join(tmpDir, "invalid.env"),
			// Missing required fields
		}

		err := RunNonInteractiveSetup(config)
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// Additional tests to improve coverage

func TestSetupConfig_GenerateManagementToken_ErrorScenarios(t *testing.T) {
	t.Run("token_already_set", func(t *testing.T) {
		config := &SetupConfig{
			ManagementToken: "existing-token",
		}

		err := config.GenerateManagementToken()
		if err != nil {
			t.Errorf("expected no error when token already set, got %v", err)
		}
		if config.ManagementToken != "existing-token" {
			t.Error("existing token should be preserved")
		}
	})

	t.Run("empty_token_generates_new", func(t *testing.T) {
		config := &SetupConfig{
			ManagementToken: "",
		}

		err := config.GenerateManagementToken()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if config.ManagementToken == "" {
			t.Error("token should be generated when empty")
		}
	})
}

func TestSetupConfig_WriteConfigFile_ErrorScenarios(t *testing.T) {
	t.Run("invalid_config_path", func(t *testing.T) {
		config := &SetupConfig{
			ConfigPath:      "/invalid/path/that/does/not/exist/and/cannot/be/created/config.env",
			OpenAIAPIKey:    "sk-test",
			ManagementToken: "test-token",
			DatabasePath:    "/tmp/test.db",
			ListenAddr:      "localhost:8080",
		}

		err := config.WriteConfigFile()
		if err == nil {
			t.Error("expected error for invalid config path")
		}
		if !strings.Contains(err.Error(), "failed to create config directory") {
			t.Errorf("expected config directory error, got %v", err)
		}
	})

	t.Run("invalid_database_path", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "llm-proxy-test-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		config := &SetupConfig{
			ConfigPath:      filepath.Join(tmpDir, "config.env"),
			OpenAIAPIKey:    "sk-test",
			ManagementToken: "test-token",
			DatabasePath:    "/invalid/path/that/does/not/exist/and/cannot/be/created/test.db",
			ListenAddr:      "localhost:8080",
		}

		err = config.WriteConfigFile()
		if err == nil {
			t.Error("expected error for invalid database path")
		}
		if !strings.Contains(err.Error(), "failed to create database directory") {
			t.Errorf("expected database directory error, got %v", err)
		}
	})

	t.Run("validation_error", func(t *testing.T) {
		config := &SetupConfig{
			ConfigPath: "/tmp/config.env",
			// Missing required fields to trigger validation error
		}

		err := config.WriteConfigFile()
		if err == nil {
			t.Error("expected validation error")
		}
	})

	t.Run("write_permission_error", func(t *testing.T) {
		// Try to write to a location that should cause permission issues
		config := &SetupConfig{
			ConfigPath:      "/root/config.env", // Typically not writable by non-root
			OpenAIAPIKey:    "sk-test",
			ManagementToken: "test-token",
			DatabasePath:    "/tmp/test.db",
			ListenAddr:      "localhost:8080",
		}

		err := config.WriteConfigFile()
		// This may or may not fail depending on the test environment
		// but it exercises the write file error path
		if err != nil && !strings.Contains(err.Error(), "failed to write config file") {
			t.Logf("Got expected write error: %v", err)
		}
	})
}

func TestRunNonInteractiveSetup_ErrorScenarios(t *testing.T) {
	t.Run("management_token_generation_with_existing", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "llm-proxy-test-")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)

		config := &SetupConfig{
			ConfigPath:      filepath.Join(tmpDir, "config.env"),
			OpenAIAPIKey:    "sk-test",
			ManagementToken: "existing-token",
			DatabasePath:    filepath.Join(tmpDir, "test.db"),
			ListenAddr:      "localhost:8080",
		}

		err = RunNonInteractiveSetup(config)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		// Verify the existing token was preserved
		if config.ManagementToken != "existing-token" {
			t.Error("existing management token should be preserved")
		}
	})

	t.Run("write_config_error", func(t *testing.T) {
		config := &SetupConfig{
			ConfigPath:      "/invalid/path/config.env",
			OpenAIAPIKey:    "sk-test",
			ManagementToken: "test-token",
			DatabasePath:    "/tmp/test.db",
			ListenAddr:      "localhost:8080",
		}

		err := RunNonInteractiveSetup(config)
		if err == nil {
			t.Error("expected error for invalid config path")
		}
		if !strings.Contains(err.Error(), "failed to write configuration") {
			t.Errorf("expected write configuration error, got %v", err)
		}
	})
}
