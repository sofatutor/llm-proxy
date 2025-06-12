package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sofatutor/llm-proxy/internal/dispatcher"
)

// FilePlugin implements file-based event logging
type FilePlugin struct {
	filePath string
	file     *os.File
}

// NewFilePlugin creates a new file plugin
func NewFilePlugin() *FilePlugin {
	return &FilePlugin{}
}

// Init initializes the file plugin with configuration
func (p *FilePlugin) Init(cfg map[string]string) error {
	filePath, ok := cfg["endpoint"]
	if !ok || filePath == "" {
		return fmt.Errorf("file plugin requires 'endpoint' configuration (file path)")
	}

	p.filePath = filePath

	// Open file for writing
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	p.file = file
	return nil
}

// SendEvents writes events to the file as JSONL (JSON Lines)
func (p *FilePlugin) SendEvents(ctx context.Context, events []dispatcher.EventPayload) error {
	if p.file == nil {
		return fmt.Errorf("file plugin not initialized")
	}

	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}

		// Write JSON line with newline
		if _, err := p.file.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	// Ensure data is written to disk
	if err := p.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}

// Close closes the file
func (p *FilePlugin) Close() error {
	if p.file != nil {
		return p.file.Close()
	}
	return nil
}
