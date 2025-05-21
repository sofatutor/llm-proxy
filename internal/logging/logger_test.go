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
