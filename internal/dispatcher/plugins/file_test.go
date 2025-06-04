package plugins

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilePlugin(t *testing.T) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test-events-*.jsonl")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Create and initialize plugin
	plugin := NewFilePlugin()
	config := map[string]string{
		"filepath": tmpFile.Name(),
	}

	err = plugin.Init(config)
	require.NoError(t, err)
	defer plugin.Close()

	// Create test event
	event := eventbus.Event{
		RequestID: "file-test-123",
		Method:    "POST",
		Path:      "/v1/test",
		Status:    200,
		Duration:  time.Millisecond * 100,
	}

	// Send event
	ctx := context.Background()
	err = plugin.Send(ctx, event)
	require.NoError(t, err)

	// Read file and verify content
	content, err := os.ReadFile(tmpFile.Name())
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	require.Len(t, lines, 1)

	var receivedEvent eventbus.Event
	err = json.Unmarshal([]byte(lines[0]), &receivedEvent)
	require.NoError(t, err)

	assert.Equal(t, event.RequestID, receivedEvent.RequestID)
	assert.Equal(t, event.Method, receivedEvent.Method)
	assert.Equal(t, event.Path, receivedEvent.Path)
	assert.Equal(t, event.Status, receivedEvent.Status)
}

func TestFilePlugin_Name(t *testing.T) {
	plugin := NewFilePlugin()
	assert.Equal(t, "file", plugin.Name())
}

func TestFilePlugin_InitError(t *testing.T) {
	plugin := NewFilePlugin()

	// Test missing filepath
	err := plugin.Init(map[string]string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "filepath is required")
}
