package plugins

import (
	"context"
	"os"
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
	defer os.Remove(tmpFile)

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

	lines := len(content)
	if lines == 0 {
		t.Error("Expected file to contain data")
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
	events := []dispatcher.EventPayload{
		{
			Type:  "test",
			Event: "start",
			RunID: "test-123",
		},
	}

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
	if plugin.endpoint != "https://api.hconeai.com/v1/request" {
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
