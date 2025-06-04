package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sofatutor/llm-proxy/internal/eventbus"
)

// FilePlugin writes events to a file.
type FilePlugin struct {
	filepath string
	file     *os.File
	mu       sync.Mutex
}

// NewFilePlugin creates a new file dispatcher plugin.
func NewFilePlugin() *FilePlugin {
	return &FilePlugin{}
}

// Init initializes the file plugin with configuration.
func (p *FilePlugin) Init(config map[string]string) error {
	filepath, ok := config["filepath"]
	if !ok || filepath == "" {
		return fmt.Errorf("filepath is required for file plugin")
	}
	
	p.filepath = filepath
	
	// Open file for appending
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filepath, err)
	}
	
	p.file = file
	return nil
}

// Send writes an event to the file.
func (p *FilePlugin) Send(ctx context.Context, event eventbus.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.file == nil {
		return fmt.Errorf("file plugin not initialized")
	}
	
	// Marshal event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Write to file with newline
	if _, err := p.file.Write(data); err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}
	if _, err := p.file.WriteString("\n"); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}
	
	// Sync to disk
	if err := p.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}
	
	return nil
}

// Name returns the plugin name.
func (p *FilePlugin) Name() string {
	return "file"
}

// Close closes the file handle.
func (p *FilePlugin) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}