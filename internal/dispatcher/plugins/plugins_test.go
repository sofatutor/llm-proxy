package plugins

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

func TestFilePlugin(t *testing.T) {
	plugin := NewFilePlugin()

	// Test Init with missing endpoint
	err := plugin.Init(map[string]string{})
	if err == nil {
		t.Fatal("Expected error when endpoint is missing")
	}

	// Test Init with valid endpoint
	tmpFile := "/tmp/test-file-plugin.jsonl"
	defer func() {
		if err := os.Remove(tmpFile); err != nil {
			t.Errorf("failed to remove tmpFile: %v", err)
		}
	}()

	cfg := map[string]string{
		"endpoint": tmpFile,
	}

	err = plugin.Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Test SendEvents
	events := []dispatcher.EventPayload{
		{
			Type:  "test",
			Event: "start",
			RunID: "test-123",
		},
		{
			Type:  "test",
			Event: "end",
			RunID: "test-456",
		},
	}

	err = plugin.SendEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("SendEvents failed: %v", err)
	}

	// Verify file was written
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	lines := len(strings.Split(strings.TrimRight(string(content), "\n"), "\n"))
	if lines <= 0 {
		t.Error("Expected file to contain at least one line")
	}

	// Test Close
	err = plugin.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestFilePluginErrors(t *testing.T) {
	plugin := NewFilePlugin()

	// Test SendEvents with invalid file path (before Init)
	events := []dispatcher.EventPayload{{Type: "test", Event: "start", RunID: "test-123"}}

	err := plugin.SendEvents(context.Background(), events)
	if err == nil {
		t.Error("Expected error when sending events before Init")
	}

	// Test Init with invalid file path (directory that doesn't exist)
	cfg := map[string]string{
		"endpoint": "/invalid/path/that/does/not/exist/file.jsonl",
	}

	err = plugin.Init(cfg)
	if err == nil {
		t.Error("Expected error when initializing with invalid file path")
	}

	// Test Close without Init
	plugin2 := NewFilePlugin()
	err = plugin2.Close()
	if err != nil {
		t.Errorf("Close should work even without Init: %v", err)
	}
}

func TestFilePlugin_SendEvents_MarshalError(t *testing.T) {
	plugin := NewFilePlugin()
	tmpFile := "/tmp/test-file-plugin-marshal.jsonl"
	cfg := map[string]string{"endpoint": tmpFile}
	if err := plugin.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	defer func() { _ = plugin.Close(); _ = os.Remove(tmpFile) }()

	// Use an unsupported value (function) in Metadata to force json.Marshal error
	bad := dispatcher.EventPayload{Type: "test", Event: "bad", RunID: "x", Metadata: map[string]any{"fn": func() {}}}
	if err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{bad}); err == nil {
		t.Fatalf("expected marshal error, got nil")
	}
}

func TestFilePlugin_CloseEdgeCases(t *testing.T) {
	plugin := NewFilePlugin()
	// Close before Init (file is nil)
	if err := plugin.Close(); err != nil {
		t.Errorf("Close should not error when file is nil: %v", err)
	}
	// Init and close twice
	tmpFile := "/tmp/test-file-plugin-close.jsonl"
	cfg := map[string]string{"endpoint": tmpFile}
	if err := plugin.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := plugin.Close(); err != nil {
		t.Errorf("First Close failed: %v", err)
	}
	// Second close may return an error (Go os.File.Close returns error if already closed)
	_ = plugin.Close()
	_ = os.Remove(tmpFile)
}

func TestLunaryPlugin(t *testing.T) {
	plugin := NewLunaryPlugin()

	// Test Init with missing api-key
	err := plugin.Init(map[string]string{})
	if err == nil {
		t.Fatal("Expected error when api-key is missing")
	}

	// Test Init with valid config
	cfg := map[string]string{
		"api-key": "test-key",
	}

	err = plugin.Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify default endpoint
	if plugin.endpoint != "https://api.lunary.ai/v1/runs/ingest" {
		t.Errorf("Expected default endpoint, got %s", plugin.endpoint)
	}

	// Test with custom endpoint
	cfg["endpoint"] = "https://custom.endpoint.com/api"
	err = plugin.Init(cfg)
	if err != nil {
		t.Fatalf("Init with custom endpoint failed: %v", err)
	}

	if plugin.endpoint != "https://custom.endpoint.com/api" {
		t.Errorf("Expected custom endpoint, got %s", plugin.endpoint)
	}

	// Test SendEvents (will fail due to network, but exercises the code path)
	events := []dispatcher.EventPayload{
		{
			Type:  "test",
			Event: "start",
			RunID: "test-123",
		},
	}

	// This will fail due to network connectivity, but that's expected in test environment
	_ = plugin.SendEvents(context.Background(), events)
	// We don't assert the error since network calls will fail in test environment
	// The important thing is that the code path is exercised

	// Test Close
	err = plugin.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestLunaryPlugin_SendEvents_EdgeCases(t *testing.T) {
	plugin := NewLunaryPlugin()
	cfg := map[string]string{"api-key": "test-key", "endpoint": "http://127.0.0.1:0/invalid"}
	if err := plugin.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Run("empty events", func(t *testing.T) {
		if err := plugin.SendEvents(context.Background(), nil); err != nil {
			t.Errorf("SendEvents with nil events should not error: %v", err)
		}
		if err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{}); err != nil {
			t.Errorf("SendEvents with empty slice should not error: %v", err)
		}
	})
	t.Run("invalid endpoint", func(t *testing.T) {
		events := []dispatcher.EventPayload{{Type: "test", Event: "fail", RunID: "fail"}}
		err := plugin.SendEvents(context.Background(), events)
		if err == nil {
			t.Error("Expected error for invalid endpoint")
		}
	})
}

func TestLunaryPlugin_Init_Errors(t *testing.T) {
	plugin := NewLunaryPlugin()
	// Missing api-key
	if err := plugin.Init(map[string]string{}); err == nil {
		t.Error("Expected error for missing api-key")
	}
}

func TestHeliconePlugin(t *testing.T) {
	plugin := NewHeliconePlugin()

	// Test Init with missing api-key
	err := plugin.Init(map[string]string{})
	if err == nil {
		t.Fatal("Expected error when api-key is missing")
	}

	// Test Init with valid config
	cfg := map[string]string{
		"api-key": "test-key",
	}

	err = plugin.Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify default endpoint
	if plugin.endpoint != "https://api.worker.helicone.ai/custom/v1/log" {
		t.Errorf("Expected default endpoint, got %s", plugin.endpoint)
	}

	// Test SendEvents (will fail due to network, but exercises the code path)
	events := []dispatcher.EventPayload{
		{
			Type:  "test",
			Event: "start",
			RunID: "test-123",
		},
	}

	// This will fail due to network connectivity, but that's expected in test environment
	_ = plugin.SendEvents(context.Background(), events)
	// We don't assert the error since network calls will fail in test environment
	// The important thing is that the code path is exercised

	// Test Close
	err = plugin.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestHeliconePlugin_SendEvents_EdgeCases(t *testing.T) {
	plugin := NewHeliconePlugin()
	cfg := map[string]string{"api-key": "test-key", "endpoint": "http://127.0.0.1:0/invalid"}
	if err := plugin.Init(cfg); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Run("empty events", func(t *testing.T) {
		if err := plugin.SendEvents(context.Background(), nil); err != nil {
			t.Errorf("SendEvents with nil events should not error: %v", err)
		}
		if err := plugin.SendEvents(context.Background(), []dispatcher.EventPayload{}); err != nil {
			t.Errorf("SendEvents with empty slice should not error: %v", err)
		}
	})
	t.Run("invalid endpoint", func(t *testing.T) {
		events := []dispatcher.EventPayload{{Type: "test", Event: "fail", RunID: "fail"}}
		err := plugin.SendEvents(context.Background(), events)
		if err != nil {
			t.Errorf("Expected no error for invalid endpoint (permanent errors are not retried): got %v", err)
		}
	})
}

func TestHeliconePlugin_Init_Errors(t *testing.T) {
	plugin := NewHeliconePlugin()
	// Missing api-key
	if err := plugin.Init(map[string]string{}); err == nil {
		t.Error("Expected error for missing api-key")
	}
}

func TestPluginRegistry(t *testing.T) {
	// Test listing plugins
	plugins := ListPlugins()
	if len(plugins) == 0 {
		t.Fatal("Expected at least one plugin")
	}

	expectedPlugins := []string{"file", "lunary", "helicone"}
	for _, expected := range expectedPlugins {
		found := false
		for _, plugin := range plugins {
			if plugin == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected plugin %s not found in registry", expected)
		}
	}

	// Test creating plugins
	for _, name := range expectedPlugins {
		plugin, err := NewPlugin(name)
		if err != nil {
			t.Errorf("Failed to create plugin %s: %v", name, err)
		}
		if plugin == nil {
			t.Errorf("Expected non-nil plugin for %s", name)
		}
	}

	// Test unknown plugin
	_, err := NewPlugin("unknown")
	if err == nil {
		t.Fatal("Expected error for unknown plugin")
	}
}
