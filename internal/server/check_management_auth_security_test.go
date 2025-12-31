package server

import (
	"net/http/httptest"
	"testing"

	"github.com/sofatutor/llm-proxy/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestCheckManagementAuth_DoesNotLogSecrets(t *testing.T) {
	cfg := &config.Config{
		ManagementToken: "mgmt-secret-token-123",
		LogLevel:        "debug",
		EventBusBackend: "in-memory",
	}

	srv, err := New(cfg, &mockTokenStore{}, &mockProjectStore{})
	require.NoError(t, err)

	core, observed := observer.New(zapcore.DebugLevel)
	srv.logger = zap.New(core)

	providedToken := "provided-secret-token-456"
	req := httptest.NewRequest("GET", "/manage/projects", nil)
	req.Header.Set("Authorization", "Bearer "+providedToken)
	rr := httptest.NewRecorder()

	ok := srv.checkManagementAuth(rr, req)
	assert.False(t, ok)

	for _, entry := range observed.All() {
		assert.NotContains(t, entry.Message, providedToken)
		assert.NotContains(t, entry.Message, cfg.ManagementToken)
		for _, field := range entry.Context {
			if field.Type == zapcore.StringType {
				assert.NotContains(t, field.String, providedToken)
				assert.NotContains(t, field.String, cfg.ManagementToken)
			}
		}
	}
}
