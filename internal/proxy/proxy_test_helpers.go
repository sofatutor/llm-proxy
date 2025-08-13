package proxy

import (
	"go.uber.org/zap"
)

// newNoopLogger returns a zap logger that does nothing, for tests
func newNoopLogger() *zap.Logger { return zap.NewNop() }
