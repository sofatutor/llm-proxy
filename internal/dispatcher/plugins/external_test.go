package plugins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeliconePlugin_Init(t *testing.T) {
	plugin := NewHeliconePlugin()

	// Test successful init
	config := map[string]string{
		"api_key":  "test-key",
		"endpoint": "https://api.example.com/v1/request",
	}
	err := plugin.Init(config)
	require.NoError(t, err)

	// Test missing API key
	configMissingKey := map[string]string{}
	err = plugin.Init(configMissingKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestHeliconePlugin_Name(t *testing.T) {
	plugin := NewHeliconePlugin()
	assert.Equal(t, "helicone", plugin.Name())
}

func TestHeliconePlugin_Send(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Initialize plugin
	plugin := NewHeliconePlugin()
	config := map[string]string{
		"api_key":  "test-key",
		"endpoint": server.URL,
	}
	err := plugin.Init(config)
	require.NoError(t, err)

	// Create test event
	event := eventbus.Event{
		RequestID:    "helicone-test-123",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       200,
		Duration:     time.Millisecond * 150,
		ResponseBody: []byte(`{"choices":[{"message":{"content":"Hello"}}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	// Send event
	ctx := context.Background()
	err = plugin.Send(ctx, event)
	require.NoError(t, err)
}

func TestLunaryPlugin_Init(t *testing.T) {
	plugin := NewLunaryPlugin()

	// Test successful init
	config := map[string]string{
		"api_key":  "test-key",
		"endpoint": "https://api.lunary.ai/v1/runs/ingest",
	}
	err := plugin.Init(config)
	require.NoError(t, err)

	// Test missing API key
	configMissingKey := map[string]string{}
	err = plugin.Init(configMissingKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api_key is required")
}

func TestLunaryPlugin_Name(t *testing.T) {
	plugin := NewLunaryPlugin()
	assert.Equal(t, "lunary", plugin.Name())
}

func TestLunaryPlugin_Send(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Initialize plugin
	plugin := NewLunaryPlugin()
	config := map[string]string{
		"api_key":  "test-key",
		"endpoint": server.URL,
	}
	err := plugin.Init(config)
	require.NoError(t, err)

	// Create test event
	event := eventbus.Event{
		RequestID:    "lunary-test-123",
		Method:       "POST",
		Path:         "/v1/chat/completions",
		Status:       200,
		Duration:     time.Millisecond * 150,
		ResponseBody: []byte(`{"choices":[{"message":{"content":"Hello"}}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
	}

	// Send event
	ctx := context.Background()
	err = plugin.Send(ctx, event)
	require.NoError(t, err)
}
