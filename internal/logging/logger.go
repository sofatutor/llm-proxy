package logging

import (
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a zap.Logger with the specified level, format, and optional file output.
// level can be debug, info, warn, or error. format can be json or console.
// If filePath is empty, logs are written to stdout.
func NewLogger(level, format, filePath string) (*zap.Logger, error) {
	var lvl zapcore.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = zapcore.DebugLevel
	case "info", "":
		lvl = zapcore.InfoLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		CallerKey:      "caller",
		StacktraceKey:  "stacktrace",
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
	}

	var encoder zapcore.Encoder
	if strings.ToLower(format) == "console" {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	var ws = zapcore.AddSync(os.Stdout)
	if filePath != "" {
		f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, err
		}
		ws = f
	}

	core := zapcore.NewCore(encoder, ws, lvl)
	return zap.New(core), nil
}
