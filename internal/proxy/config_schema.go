// Package proxy provides the transparent proxy functionality for the LLM API.
package proxy

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// APIConfig represents the top-level configuration for API proxying
type APIConfig struct {
	// APIs contains configurations for different API providers
	APIs map[string]*APIProviderConfig `yaml:"apis"`
	// DefaultAPI is the default API provider to use if not specified
	DefaultAPI string `yaml:"default_api"`
}

// APIProviderConfig represents the configuration for a specific API provider
type APIProviderConfig struct {
	// BaseURL is the target API base URL
	BaseURL string `yaml:"base_url"`
	// AllowedEndpoints is a list of endpoint prefixes that are allowed to be accessed
	AllowedEndpoints []string `yaml:"allowed_endpoints"`
	// AllowedMethods is a list of HTTP methods that are allowed
	AllowedMethods []string `yaml:"allowed_methods"`
	// Timeouts for various operations
	Timeouts TimeoutConfig `yaml:"timeouts"`
	// Connection settings
	Connection ConnectionConfig `yaml:"connection"`
}

// TimeoutConfig contains timeout settings for the proxy
type TimeoutConfig struct {
	// Request is the overall request timeout
	Request time.Duration `yaml:"request"`
	// ResponseHeader is the timeout for receiving response headers
	ResponseHeader time.Duration `yaml:"response_header"`
	// IdleConnection is the timeout for idle connections
	IdleConnection time.Duration `yaml:"idle_connection"`
	// FlushInterval controls how often to flush streaming responses
	FlushInterval time.Duration `yaml:"flush_interval"`
}

// ConnectionConfig contains connection settings for the proxy
type ConnectionConfig struct {
	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns int `yaml:"max_idle_conns"`
	// MaxIdleConnsPerHost is the maximum number of idle connections per host
	MaxIdleConnsPerHost int `yaml:"max_idle_conns_per_host"`
}

// LoadAPIConfigFromFile loads API configuration from a YAML file
func LoadAPIConfigFromFile(filePath string) (*APIConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config APIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate the configuration
	if err := validateAPIConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validateAPIConfig checks that the API configuration is valid
func validateAPIConfig(config *APIConfig) error {
	if len(config.APIs) == 0 {
		return fmt.Errorf("no API providers configured")
	}

	if config.DefaultAPI != "" {
		if _, exists := config.APIs[config.DefaultAPI]; !exists {
			return fmt.Errorf("default API '%s' not found in configured APIs", config.DefaultAPI)
		}
	}

	for name, api := range config.APIs {
		if api.BaseURL == "" {
			return fmt.Errorf("API '%s' has empty base_url", name)
		}
		
		if len(api.AllowedEndpoints) == 0 {
			return fmt.Errorf("API '%s' has no allowed_endpoints", name)
		}
		
		if len(api.AllowedMethods) == 0 {
			return fmt.Errorf("API '%s' has no allowed_methods", name)
		}
	}

	return nil
}

// GetProxyConfigForAPI returns a ProxyConfig for the specified API provider
func (c *APIConfig) GetProxyConfigForAPI(apiName string) (*ProxyConfig, error) {
	// If no API name specified, use the default
	if apiName == "" {
		apiName = c.DefaultAPI
	}

	// Find the API configuration
	apiConfig, exists := c.APIs[apiName]
	if !exists {
		return nil, fmt.Errorf("API provider '%s' not found in configuration", apiName)
	}

	// Create the proxy configuration
	proxyConfig := ProxyConfig{
		TargetBaseURL:       apiConfig.BaseURL,
		AllowedEndpoints:    apiConfig.AllowedEndpoints,
		AllowedMethods:      apiConfig.AllowedMethods,
		RequestTimeout:      apiConfig.Timeouts.Request,
		ResponseHeaderTimeout: apiConfig.Timeouts.ResponseHeader,
		FlushInterval:       apiConfig.Timeouts.FlushInterval,
		IdleConnTimeout:     apiConfig.Timeouts.IdleConnection,
		MaxIdleConns:        apiConfig.Connection.MaxIdleConns,
		MaxIdleConnsPerHost: apiConfig.Connection.MaxIdleConnsPerHost,
	}

	return &proxyConfig, nil
}