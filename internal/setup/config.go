// Package setup provides configuration setup and management utilities.
package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sofatutor/llm-proxy/internal/utils"
)

// SetupConfig holds configuration parameters for setup
type SetupConfig struct {
	ConfigPath      string
	OpenAIAPIKey    string
	ManagementToken string
	DatabasePath    string
	ListenAddr      string
}

// ValidateConfig validates the setup configuration
func (sc *SetupConfig) ValidateConfig() error {
	if sc.OpenAIAPIKey == "" {
		return fmt.Errorf("OpenAI API key is required")
	}
	if sc.ConfigPath == "" {
		return fmt.Errorf("config path is required")
	}
	if sc.DatabasePath == "" {
		return fmt.Errorf("database path is required")
	}
	if sc.ListenAddr == "" {
		return fmt.Errorf("listen address is required")
	}
	return nil
}

// GenerateManagementToken generates a management token if not provided
func (sc *SetupConfig) GenerateManagementToken() error {
	if sc.ManagementToken == "" {
		token, err := utils.GenerateSecureToken(16)
		if err != nil {
			return fmt.Errorf("failed to generate management token: %w", err)
		}
		sc.ManagementToken = token
	}
	return nil
}

// WriteConfigFile writes the configuration to a file
func (sc *SetupConfig) WriteConfigFile() error {
	if err := sc.ValidateConfig(); err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(sc.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(sc.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	content := fmt.Sprintf(`# LLM Proxy Configuration
OPENAI_API_KEY=%s
MANAGEMENT_TOKEN=%s
DATABASE_PATH=%s
LISTEN_ADDR=%s
LOG_LEVEL=info
`, sc.OpenAIAPIKey, sc.ManagementToken, sc.DatabasePath, sc.ListenAddr)

	if err := os.WriteFile(sc.ConfigPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// RunNonInteractiveSetup performs non-interactive setup with the given configuration
func RunNonInteractiveSetup(sc *SetupConfig) error {
	if err := sc.GenerateManagementToken(); err != nil {
		return fmt.Errorf("failed to generate management token: %w", err)
	}

	if err := sc.WriteConfigFile(); err != nil {
		return fmt.Errorf("failed to write configuration: %w", err)
	}

	return nil
}