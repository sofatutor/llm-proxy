package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestLoadAPIConfigFromFile(t *testing.T) {
	// Create a temporary test config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_api_config.yaml")

	// Sample API config for testing
	apiConfig := APIConfig{
		DefaultAPI: "test_api",
		APIs: map[string]*APIProviderConfig{
			"test_api": {
				BaseURL:          "https://api.example.com",
				AllowedEndpoints: []string{"/v1/test", "/v1/another_test"},
				AllowedMethods:   []string{"GET", "POST"},
				Timeouts: TimeoutConfig{
					Request:        60 * time.Second,
					ResponseHeader: 30 * time.Second,
					IdleConnection: 90 * time.Second,
					FlushInterval:  100 * time.Millisecond,
				},
				Connection: ConnectionConfig{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
				},
			},
		},
	}

	// Marshal the config to YAML
	configData, err := yaml.Marshal(apiConfig)
	require.NoError(t, err, "Failed to marshal test config to YAML")

	// Write the config to the temporary file
	err = os.WriteFile(configPath, configData, 0600)
	require.NoError(t, err, "Failed to write test config to file")

	// Test loading the config
	loadedConfig, err := LoadAPIConfigFromFile(configPath)
	require.NoError(t, err, "Failed to load API config from file")
	require.NotNil(t, loadedConfig, "Loaded config should not be nil")

	// Verify the loaded config
	assert.Equal(t, "test_api", loadedConfig.DefaultAPI, "Default API mismatch")
	assert.Contains(t, loadedConfig.APIs, "test_api", "API provider missing")

	testAPI := loadedConfig.APIs["test_api"]
	assert.Equal(t, "https://api.example.com", testAPI.BaseURL, "Base URL mismatch")
	assert.Equal(t, []string{"/v1/test", "/v1/another_test"}, testAPI.AllowedEndpoints, "Allowed endpoints mismatch")
	assert.Equal(t, []string{"GET", "POST"}, testAPI.AllowedMethods, "Allowed methods mismatch")
	assert.Equal(t, 60*time.Second, testAPI.Timeouts.Request, "Request timeout mismatch")
	assert.Equal(t, 100*time.Millisecond, testAPI.Timeouts.FlushInterval, "Flush interval mismatch")
	assert.Equal(t, 100, testAPI.Connection.MaxIdleConns, "Max idle connections mismatch")
}

func TestGetProxyConfigForAPI(t *testing.T) {
	// Create a test API config
	apiConfig := &APIConfig{
		DefaultAPI: "default_api",
		APIs: map[string]*APIProviderConfig{
			"default_api": {
				BaseURL:          "https://api.default.com",
				AllowedEndpoints: []string{"/v1/default"},
				AllowedMethods:   []string{"GET"},
				Timeouts: TimeoutConfig{
					Request:        30 * time.Second,
					ResponseHeader: 10 * time.Second,
					IdleConnection: 60 * time.Second,
					FlushInterval:  50 * time.Millisecond,
				},
				Connection: ConnectionConfig{
					MaxIdleConns:        50,
					MaxIdleConnsPerHost: 5,
				},
			},
			"other_api": {
				BaseURL:          "https://api.other.com",
				AllowedEndpoints: []string{"/v1/other"},
				AllowedMethods:   []string{"POST"},
				Timeouts: TimeoutConfig{
					Request:        60 * time.Second,
					ResponseHeader: 20 * time.Second,
					IdleConnection: 120 * time.Second,
					FlushInterval:  100 * time.Millisecond,
				},
				Connection: ConnectionConfig{
					MaxIdleConns:        100,
					MaxIdleConnsPerHost: 10,
				},
			},
		},
	}

	// Test getting config for the default API
	defaultProxyConfig, err := apiConfig.GetProxyConfigForAPI("")
	require.NoError(t, err, "Failed to get proxy config for default API")
	assert.Equal(t, "https://api.default.com", defaultProxyConfig.TargetBaseURL, "Default API base URL mismatch")
	assert.Equal(t, []string{"/v1/default"}, defaultProxyConfig.AllowedEndpoints, "Default API endpoints mismatch")
	assert.Equal(t, []string{"GET"}, defaultProxyConfig.AllowedMethods, "Default API methods mismatch")
	assert.Equal(t, 30*time.Second, defaultProxyConfig.RequestTimeout, "Default API request timeout mismatch")

	// Test getting config for a specific API
	otherProxyConfig, err := apiConfig.GetProxyConfigForAPI("other_api")
	require.NoError(t, err, "Failed to get proxy config for other API")
	assert.Equal(t, "https://api.other.com", otherProxyConfig.TargetBaseURL, "Other API base URL mismatch")
	assert.Equal(t, []string{"/v1/other"}, otherProxyConfig.AllowedEndpoints, "Other API endpoints mismatch")
	assert.Equal(t, []string{"POST"}, otherProxyConfig.AllowedMethods, "Other API methods mismatch")
	assert.Equal(t, 60*time.Second, otherProxyConfig.RequestTimeout, "Other API request timeout mismatch")

	// Test getting config for non-existent API
	_, err = apiConfig.GetProxyConfigForAPI("nonexistent_api")
	assert.Error(t, err, "Should return error for non-existent API")
}

func TestValidateAPIConfig(t *testing.T) {
	// Test valid config
	validConfig := &APIConfig{
		DefaultAPI: "api1",
		APIs: map[string]*APIProviderConfig{
			"api1": {
				BaseURL:          "https://api.example.com",
				AllowedEndpoints: []string{"/v1/test"},
				AllowedMethods:   []string{"GET"},
			},
		},
	}
	assert.NoError(t, validateAPIConfig(validConfig), "Valid config should not return error")

	// Test empty APIs
	emptyAPIsConfig := &APIConfig{
		DefaultAPI: "api1",
		APIs:       map[string]*APIProviderConfig{},
	}
	assert.Error(t, validateAPIConfig(emptyAPIsConfig), "Config with no APIs should return error")

	// Test invalid default API
	invalidDefaultAPIConfig := &APIConfig{
		DefaultAPI: "nonexistent",
		APIs: map[string]*APIProviderConfig{
			"api1": {
				BaseURL:          "https://api.example.com",
				AllowedEndpoints: []string{"/v1/test"},
				AllowedMethods:   []string{"GET"},
			},
		},
	}
	assert.Error(t, validateAPIConfig(invalidDefaultAPIConfig), "Config with invalid default API should return error")

	// Test missing base URL
	missingBaseURLConfig := &APIConfig{
		DefaultAPI: "api1",
		APIs: map[string]*APIProviderConfig{
			"api1": {
				BaseURL:          "",
				AllowedEndpoints: []string{"/v1/test"},
				AllowedMethods:   []string{"GET"},
			},
		},
	}
	assert.Error(t, validateAPIConfig(missingBaseURLConfig), "Config with missing base URL should return error")

	// Test missing allowed endpoints
	missingEndpointsConfig := &APIConfig{
		DefaultAPI: "api1",
		APIs: map[string]*APIProviderConfig{
			"api1": {
				BaseURL:          "https://api.example.com",
				AllowedEndpoints: []string{},
				AllowedMethods:   []string{"GET"},
			},
		},
	}
	assert.Error(t, validateAPIConfig(missingEndpointsConfig), "Config with no allowed endpoints should return error")

	// Test missing allowed methods
	missingMethodsConfig := &APIConfig{
		DefaultAPI: "api1",
		APIs: map[string]*APIProviderConfig{
			"api1": {
				BaseURL:          "https://api.example.com",
				AllowedEndpoints: []string{"/v1/test"},
				AllowedMethods:   []string{},
			},
		},
	}
	assert.Error(t, validateAPIConfig(missingMethodsConfig), "Config with no allowed methods should return error")
}

func TestLoadAPIConfigFromFile_Valid(t *testing.T) {
	tmp, err := os.CreateTemp("", "apiconfig-*.yaml")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmp.Name()) }()
	content := `
apis:
  openai:
    base_url: "https://api.openai.com"
    allowed_endpoints:
      - "/v1/completions"
    allowed_methods:
      - "POST"
    timeouts:
      request: 30s
      response_header: 10s
      idle_connection: 60s
      flush_interval: 100ms
    connection:
      max_idle_conns: 10
      max_idle_conns_per_host: 2
  other:
    base_url: "https://api.other.com"
    allowed_endpoints: ["/foo"]
    allowed_methods: ["GET"]
    timeouts:
      request: 10s
      response_header: 5s
      idle_connection: 20s
      flush_interval: 50ms
    connection:
      max_idle_conns: 5
      max_idle_conns_per_host: 1
default_api: "openai"
`
	_, err = tmp.WriteString(content)
	assert.NoError(t, err)
	_ = tmp.Close()

	cfg, err := LoadAPIConfigFromFile(tmp.Name())
	assert.NoError(t, err)
	assert.Equal(t, "openai", cfg.DefaultAPI)
	assert.Contains(t, cfg.APIs, "openai")
	assert.Contains(t, cfg.APIs, "other")
	assert.Equal(t, "https://api.openai.com", cfg.APIs["openai"].BaseURL)
	assert.Equal(t, []string{"/v1/completions"}, cfg.APIs["openai"].AllowedEndpoints)
	assert.Equal(t, []string{"POST"}, cfg.APIs["openai"].AllowedMethods)
	assert.Equal(t, 10, cfg.APIs["openai"].Connection.MaxIdleConns)
	assert.Equal(t, 2, cfg.APIs["openai"].Connection.MaxIdleConnsPerHost)
}

func TestLoadAPIConfigFromFile_InvalidFile(t *testing.T) {
	_, err := LoadAPIConfigFromFile("/does/not/exist.yaml")
	assert.Error(t, err)
}

func TestLoadAPIConfigFromFile_InvalidYAML(t *testing.T) {
	tmp, err := os.CreateTemp("", "apiconfig-bad-*.yaml")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString("not: [valid: yaml")
	assert.NoError(t, err)
	_ = tmp.Close()
	_, err = LoadAPIConfigFromFile(tmp.Name())
	assert.Error(t, err)
}

func TestLoadAPIConfigFromFile_InvalidConfig(t *testing.T) {
	tmp, err := os.CreateTemp("", "apiconfig-empty-*.yaml")
	assert.NoError(t, err)
	defer func() { _ = os.Remove(tmp.Name()) }()
	_, err = tmp.WriteString("apis: {}\n")
	assert.NoError(t, err)
	_ = tmp.Close()
	_, err = LoadAPIConfigFromFile(tmp.Name())
	assert.Error(t, err)
}
