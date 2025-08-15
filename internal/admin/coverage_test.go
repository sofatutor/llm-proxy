package admin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test the API client methods that have lower coverage
func TestAPIClient_CoverageImprovements(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{
			name: "GetProject_error_scenario",
			testFunc: func(t *testing.T) {
				client := NewAPIClient("http://example.com", "test-token")
				ctx := context.Background()
				
				// This should fail due to invalid URL/unreachable server
				project, err := client.GetProject(ctx, "test-id")
				assert.Error(t, err)
				assert.Nil(t, project)
			},
		},
		{
			name: "CreateProject_error_scenario", 
			testFunc: func(t *testing.T) {
				client := NewAPIClient("http://example.com", "test-token")
				ctx := context.Background()
				
				// This should fail due to invalid URL/unreachable server
				project, err := client.CreateProject(ctx, "test-name", "test-key")
				assert.Error(t, err)
				assert.Nil(t, project)
			},
		},
		{
			name: "UpdateProject_error_scenario",
			testFunc: func(t *testing.T) {
				client := NewAPIClient("http://example.com", "test-token")
				ctx := context.Background()
				
				// This should fail due to invalid URL/unreachable server
				project, err := client.UpdateProject(ctx, "test-id", "test-name", "test-key")
				assert.Error(t, err)
				assert.Nil(t, project)
			},
		},
		{
			name: "CreateToken_error_scenario",
			testFunc: func(t *testing.T) {
				client := NewAPIClient("http://example.com", "test-token")
				ctx := context.Background()
				
				// This should fail due to invalid URL/unreachable server
				token, err := client.CreateToken(ctx, "test-project", 60)
				assert.Error(t, err)
				assert.Nil(t, token)
			},
		},
		{
			name: "GetAuditEvent_error_scenario",
			testFunc: func(t *testing.T) {
				client := NewAPIClient("http://example.com", "test-token")
				ctx := context.Background()
				
				// This should fail due to invalid URL/unreachable server  
				event, err := client.GetAuditEvent(ctx, "test-id")
				assert.Error(t, err)
				assert.Nil(t, event)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

// Test utility functions for coverage
func TestUtilityFunctions_Coverage(t *testing.T) {
	t.Run("ObfuscateAPIKey", func(t *testing.T) {
		result := ObfuscateAPIKey("sk-1234567890abcdef")
		assert.Contains(t, result, "...")
		assert.NotEqual(t, "sk-1234567890abcdef", result)
	})

	t.Run("ObfuscateToken", func(t *testing.T) {
		result := ObfuscateToken("token123456789")
		assert.Contains(t, result, "...")
		assert.NotEqual(t, "token123456789", result)
	})
}