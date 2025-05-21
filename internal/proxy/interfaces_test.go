package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyConfig_Validate(t *testing.T) {
	// All fields valid
	cfg := &ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedMethods:   []string{"GET"},
		AllowedEndpoints: []string{"/foo"},
	}
	assert.NoError(t, cfg.Validate())

	// Missing TargetBaseURL
	cfg = &ProxyConfig{
		AllowedMethods:   []string{"GET"},
		AllowedEndpoints: []string{"/foo"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "TargetBaseURL")

	// Missing AllowedMethods
	cfg = &ProxyConfig{
		TargetBaseURL:    "https://api.example.com",
		AllowedEndpoints: []string{"/foo"},
	}
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AllowedMethods")

	// Missing AllowedEndpoints
	cfg = &ProxyConfig{
		TargetBaseURL:  "https://api.example.com",
		AllowedMethods: []string{"GET"},
	}
	err = cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AllowedEndpoints")
}
