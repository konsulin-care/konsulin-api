package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// TestNewSlogLoggerBridgesRecordsToZapCore asserts records logged through the
// slog bridge reach the zap core with the matching level and message.
func TestNewSlogLoggerBridgesRecordsToZapCore(t *testing.T) {
	core, logs := observer.New(zapcore.InfoLevel)
	slogLogger := NewSlogLogger(zap.New(core))

	slogLogger.Info("slog info message")
	slogLogger.Warn("slog warn message")
	slogLogger.Error("slog error message")

	entries := logs.All()
	require.Len(t, entries, 3, "every slog record must reach the zap core")

	assert.Equal(t, zapcore.InfoLevel, entries[0].Level, "info level must match")
	assert.Equal(t, "slog info message", entries[0].Message, "info message must match")
	assert.Equal(t, zapcore.WarnLevel, entries[1].Level, "warn level must match")
	assert.Equal(t, "slog warn message", entries[1].Message, "warn message must match")
	assert.Equal(t, zapcore.ErrorLevel, entries[2].Level, "error level must match")
	assert.Equal(t, "slog error message", entries[2].Message, "error message must match")
}
