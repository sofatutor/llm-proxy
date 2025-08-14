package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	t.Run("creates logger with valid file path", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		assert.Equal(t, logPath, logger.GetPath())
		assert.FileExists(t, logPath)
	})

	t.Run("creates parent directories when createDir is true", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "logs", "audit", "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		assert.FileExists(t, logPath)
		assert.DirExists(t, filepath.Dir(logPath))
	})

	t.Run("fails when parent directory doesn't exist and createDir is false", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "nonexistent", "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: false,
		}

		_, err := NewLogger(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open audit log file")
	})

	t.Run("fails with empty file path", func(t *testing.T) {
		config := LoggerConfig{
			FilePath:  "",
			CreateDir: true,
		}

		_, err := NewLogger(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "audit log file path cannot be empty")
	})

	t.Run("fails when directory creation fails", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a file where we want to create a directory
		blockingFile := filepath.Join(tmpDir, "blocking-file")
		err := os.WriteFile(blockingFile, []byte("content"), 0644)
		require.NoError(t, err)

		// Try to create logger with path that requires creating directory where file exists
		logPath := filepath.Join(blockingFile, "audit", "audit.log")
		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		_, err = NewLogger(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create audit log directory")
	})
}

func TestLogger_Log(t *testing.T) {
	t.Run("writes event to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess).
			WithProjectID("proj-123").
			WithRequestID("req-456")

		err = logger.Log(event)
		require.NoError(t, err)

		// Read back the logged event
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		require.Len(t, lines, 1)

		var loggedEvent Event
		err = json.Unmarshal([]byte(lines[0]), &loggedEvent)
		require.NoError(t, err)

		assert.Equal(t, ActionTokenCreate, loggedEvent.Action)
		assert.Equal(t, ActorManagement, loggedEvent.Actor)
		assert.Equal(t, ResultSuccess, loggedEvent.Result)
		assert.Equal(t, "proj-123", loggedEvent.ProjectID)
		assert.Equal(t, "req-456", loggedEvent.RequestID)
	})

	t.Run("writes multiple events as JSONL", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		events := []*Event{
			NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess),
			NewEvent(ActionProjectDelete, ActorAdmin, ResultFailure),
		}

		for _, event := range events {
			err = logger.Log(event)
			require.NoError(t, err)
		}

		// Read back the logged events
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		require.Len(t, lines, 2)

		// Verify first event
		var event1 Event
		err = json.Unmarshal([]byte(lines[0]), &event1)
		require.NoError(t, err)
		assert.Equal(t, ActionTokenCreate, event1.Action)

		// Verify second event
		var event2 Event
		err = json.Unmarshal([]byte(lines[1]), &event2)
		require.NoError(t, err)
		assert.Equal(t, ActionProjectDelete, event2.Action)
	})

	t.Run("appends to existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		// Write initial event
		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger1, err := NewLogger(config)
		require.NoError(t, err)

		event1 := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
		err = logger1.Log(event1)
		require.NoError(t, err)
		_ = logger1.Close()

		// Create new logger and write another event
		logger2, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger2.Close() }()

		event2 := NewEvent(ActionProjectDelete, ActorAdmin, ResultFailure)
		err = logger2.Log(event2)
		require.NoError(t, err)

		// Verify both events are present
		data, err := os.ReadFile(logPath)
		require.NoError(t, err)

		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		require.Len(t, lines, 2)
	})

	t.Run("fails with nil event", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		err = logger.Log(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "audit event cannot be nil")
	})

	t.Run("fails when file is closed", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)

		// Close the file
		err = logger.Close()
		require.NoError(t, err)

		// Try to log after closing
		event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
		err = logger.Log(event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to log to file")
	})

	t.Run("handles file write errors gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)
		defer func() { _ = logger.Close() }()

		// Create an event with problematic data that might cause JSON marshal issues
		// While Event normally marshals fine, let's test by manually setting bad file state
		event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)

		// Close file manually to simulate write error
		_ = logger.file.Close()

		err = logger.Log(event)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to log to file")
	})
}

func TestLogger_Close(t *testing.T) {
	t.Run("closes file successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)

		err = logger.Close()
		assert.NoError(t, err)
	})

	t.Run("can be called multiple times", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "audit.log")

		config := LoggerConfig{
			FilePath:  logPath,
			CreateDir: true,
		}

		logger, err := NewLogger(config)
		require.NoError(t, err)

		err = logger.Close()
		assert.NoError(t, err)

		err = logger.Close()
		assert.NoError(t, err)
	})
}

func TestLogger_ThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	config := LoggerConfig{
		FilePath:  logPath,
		CreateDir: true,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Write events from multiple goroutines
	numGoroutines := 10
	eventsPerGoroutine := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < eventsPerGoroutine; j++ {
				event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess).
					WithDetail("goroutine_id", id).
					WithDetail("event_id", j)
				err := logger.Log(event)
				assert.NoError(t, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all events were written
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, numGoroutines*eventsPerGoroutine)

	// Verify each line is valid JSON
	for i, line := range lines {
		var event Event
		err := json.Unmarshal([]byte(line), &event)
		assert.NoError(t, err, "line %d is not valid JSON: %s", i, line)
	}
}

func TestNewNullLogger(t *testing.T) {
	logger := NewNullLogger()
	assert.NotNil(t, logger)

	// Should not error when logging
	event := NewEvent(ActionTokenCreate, ActorManagement, ResultSuccess)
	err := logger.Log(event)
	assert.NoError(t, err)

	// Close should not error
	err = logger.Close()
	assert.NoError(t, err)
}

func TestLogger_JSONLFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	config := LoggerConfig{
		FilePath:  logPath,
		CreateDir: true,
	}

	logger, err := NewLogger(config)
	require.NoError(t, err)
	defer func() { _ = logger.Close() }()

	// Create an event with various field types
	timestamp := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)
	event := &Event{
		Timestamp:     timestamp,
		Action:        ActionTokenCreate,
		Actor:         ActorManagement,
		ProjectID:     "proj-123",
		RequestID:     "req-456",
		CorrelationID: "corr-789",
		Result:        ResultSuccess,
		Details: map[string]interface{}{
			"duration_ms": 150,
			"method":      "POST",
			"success":     true,
		},
	}

	err = logger.Log(event)
	require.NoError(t, err)

	// Read and verify the JSON structure
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)

	var loggedEvent Event
	err = json.Unmarshal(data[:len(data)-1], &loggedEvent) // Remove trailing newline
	require.NoError(t, err)

	assert.Equal(t, timestamp.Format(time.RFC3339Nano), loggedEvent.Timestamp.Format(time.RFC3339Nano))
	assert.Equal(t, ActionTokenCreate, loggedEvent.Action)
	assert.Equal(t, ActorManagement, loggedEvent.Actor)
	assert.Equal(t, "proj-123", loggedEvent.ProjectID)
	assert.Equal(t, "req-456", loggedEvent.RequestID)
	assert.Equal(t, "corr-789", loggedEvent.CorrelationID)
	assert.Equal(t, ResultSuccess, loggedEvent.Result)
	assert.Equal(t, float64(150), loggedEvent.Details["duration_ms"]) // JSON unmarshals numbers as float64
	assert.Equal(t, "POST", loggedEvent.Details["method"])
	assert.Equal(t, true, loggedEvent.Details["success"])
}
