package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Logger handles writing audit events to multiple backends (file and database)
// with immutable semantics. It provides thread-safe operations and ensures 
// all audit events are persisted.
type Logger struct {
	file      *os.File
	writer    io.Writer
	mutex     sync.Mutex
	path      string
	dbStore   DatabaseStore
	dbEnabled bool
}

// DatabaseStore defines the interface for database audit storage
type DatabaseStore interface {
	StoreAuditEvent(ctx context.Context, event *Event) error
}

// LoggerConfig holds configuration for the audit logger
type LoggerConfig struct {
	// FilePath is the path to the audit log file
	FilePath string
	// CreateDir determines whether to create parent directories if they don't exist
	CreateDir bool
	// DatabaseStore is an optional database backend for audit events
	DatabaseStore DatabaseStore
	// EnableDatabase determines whether to store events in database
	EnableDatabase bool
}

// NewLogger creates a new audit logger that writes to the specified file.
// If createDir is true, it will create parent directories if they don't exist.
// If a database store is provided, events will also be persisted to the database.
func NewLogger(config LoggerConfig) (*Logger, error) {
	if config.FilePath == "" {
		return nil, fmt.Errorf("audit log file path cannot be empty")
	}

	// Create parent directories if needed
	if config.CreateDir {
		dir := filepath.Dir(config.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}
	}

	// Open file for appending with appropriate permissions
	file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &Logger{
		file:      file,
		writer:    file,
		path:      config.FilePath,
		dbStore:   config.DatabaseStore,
		dbEnabled: config.EnableDatabase && config.DatabaseStore != nil,
	}, nil
}

// Log writes an audit event to both file and database backends.
// Events are written as JSON lines (JSONL format) for easy parsing.
// This method is thread-safe.
func (l *Logger) Log(event *Event) error {
	if event == nil {
		return fmt.Errorf("audit event cannot be nil")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	// Write to file backend
	if err := l.logToFile(event); err != nil {
		return fmt.Errorf("failed to log to file: %w", err)
	}

	// Write to database backend if enabled
	if l.dbEnabled {
		// Use background context with timeout for database operations
		ctx := context.Background()
		if err := l.dbStore.StoreAuditEvent(ctx, event); err != nil {
			// Log database errors but don't fail the audit operation
			// File logging is the primary reliable audit trail
			fmt.Printf("Warning: Failed to store audit event to database: %v\n", err)
		}
	}

	return nil
}

// logToFile writes an audit event to the file backend
func (l *Logger) logToFile(event *Event) error {
	// Encode event as JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Write JSON line followed by newline
	if _, err := l.writer.Write(data); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	if _, err := l.writer.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Sync to ensure data is written to disk
	if syncer, ok := l.writer.(interface{ Sync() error }); ok {
		if err := syncer.Sync(); err != nil {
			return fmt.Errorf("failed to sync audit log: %w", err)
		}
	}

	return nil
}

// Close closes the audit log file.
// After calling Close, the logger should not be used for logging.
func (l *Logger) Close() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// GetPath returns the file path of the audit log
func (l *Logger) GetPath() string {
	return l.path
}

// NewNullLogger creates a logger that discards all events.
// Useful for testing or when audit logging is disabled.
func NewNullLogger() *Logger {
	return &Logger{
		writer:    io.Discard,
		dbEnabled: false,
	}
}
