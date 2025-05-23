package logging

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewLogger_FileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger("debug", "json", logFile)
	require.NoError(t, err)
	logger.Info("hello", zap.String("foo", "bar"))
	require.NoError(t, logger.Sync())

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(data), "\"foo\":\"bar\"")
}

func TestNewLogger_StdoutOutput(t *testing.T) {
	logger, err := NewLogger("info", "json", "")
	require.NoError(t, err)
	assert.NotNil(t, logger)
}

func TestNewLogger_AllLevels(t *testing.T) {
	tests := []struct {
		level string
	}{
		{"debug"},
		{"info"},
		{"warn"},
		{"error"},
		{""},        // defaults to info
		{"invalid"}, // defaults to info
		{"DEBUG"},   // case insensitive
		{"INFO"},
		{"WARN"},
		{"ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logger, err := NewLogger(tt.level, "json", "")
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLogger_AllFormats(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"json"},
		{"console"},
		{"JSON"},    // case insensitive
		{"CONSOLE"}, // case insensitive
		{"invalid"}, // defaults to json
		{""},        // defaults to json
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			logger, err := NewLogger("info", tt.format, "")
			require.NoError(t, err)
			assert.NotNil(t, logger)
		})
	}
}

func TestNewLogger_ConsoleFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "console.log")

	logger, err := NewLogger("debug", "console", logFile)
	require.NoError(t, err)
	logger.Info("test message", zap.String("key", "value"))
	require.NoError(t, logger.Sync())

	data, err := os.ReadFile(logFile)
	require.NoError(t, err)
	// Console format should contain the message and structured field
	assert.Contains(t, string(data), "test message")
	assert.Contains(t, string(data), "key")
}

func TestNewLogger_FileError(t *testing.T) {
	// Try to create a file in a directory that doesn't exist
	invalidPath := "/non/existent/directory/test.log"
	
	logger, err := NewLogger("info", "json", invalidPath)
	assert.Error(t, err)
	assert.Nil(t, logger)
}
