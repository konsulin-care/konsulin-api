package gateway

import (
	"context"
	"log/slog"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapCoreHandler is a slog.Handler that forwards records to a zapcore.Core so
// slog records reach the zap pipeline with their original level and message.
type zapCoreHandler struct {
	core   zapcore.Core
	fields []zapcore.Field
	groups []string
}

// NewSlogLogger bridges a zap logger to log/slog (a self-contained equivalent
// of go.uber.org/zap/exp/zapslog, which is not resolvable in this environment),
// so the SDK's client.WithLogger option receives an *slog.Logger whose records
// reach the underlying zap core with the original level and message.
// Parameters: logger is the zap logger to bridge; a nil logger becomes a no-op.
// Returns: an *slog.Logger writing through the zap core. Never panics.
func NewSlogLogger(logger *zap.Logger) *slog.Logger {
	if logger == nil {
		logger = zap.NewNop()
	}
	return slog.New(&zapCoreHandler{core: logger.Core()})
}

// Enabled reports whether the zap core enables the mapped zap level.
func (h *zapCoreHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.core.Enabled(zapLevel(level))
}

// Handle converts the slog record into a zapcore entry and writes it, together
// with previously attached attributes, to the zap core.
func (h *zapCoreHandler) Handle(_ context.Context, record slog.Record) error {
	fields := make([]zapcore.Field, 0, record.NumAttrs()+len(h.fields))
	fields = append(fields, h.fields...)
	record.Attrs(func(attr slog.Attr) bool {
		fields = append(fields, attrFields(attr, h.groups)...)
		return true
	})
	return h.core.Write(zapcore.Entry{
		Time:    record.Time,
		Level:   zapLevel(record.Level),
		Message: record.Message,
	}, fields)
}

// WithAttrs returns a handler carrying the given attributes as zap fields.
func (h *zapCoreHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	fields := make([]zapcore.Field, 0, len(h.fields)+len(attrs))
	fields = append(fields, h.fields...)
	for _, attr := range attrs {
		fields = append(fields, attrFields(attr, h.groups)...)
	}
	return &zapCoreHandler{core: h.core, fields: fields, groups: h.groups}
}

// WithGroup returns a handler that nests subsequent attributes under name.
func (h *zapCoreHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	groups := make([]string, 0, len(h.groups)+1)
	groups = append(groups, h.groups...)
	groups = append(groups, name)
	return &zapCoreHandler{core: h.core, fields: h.fields, groups: groups}
}

// zapLevel maps a slog.Level onto the nearest zapcore.Level.
func zapLevel(level slog.Level) zapcore.Level {
	switch {
	case level >= slog.Level(12): // slog has no Fatal; clamp high custom levels
		return zapcore.FatalLevel
	case level >= slog.LevelError:
		return zapcore.ErrorLevel
	case level >= slog.LevelWarn:
		return zapcore.WarnLevel
	case level >= slog.LevelInfo:
		return zapcore.InfoLevel
	default:
		return zapcore.DebugLevel
	}
}

// attrFields converts a slog attribute into zap fields, nesting keys under groups.
func attrFields(attr slog.Attr, groups []string) []zapcore.Field {
	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		sub := groups
		if attr.Key != "" {
			sub = make([]string, 0, len(groups)+1)
			sub = append(sub, groups...)
			sub = append(sub, attr.Key)
		}
		var out []zapcore.Field
		for _, inner := range value.Group() {
			out = append(out, attrFields(inner, sub)...)
		}
		return out
	}
	if attr.Key == "" {
		return nil
	}
	return []zapcore.Field{valueField(prefixKey(attr.Key, groups), value)}
}

// valueField renders a resolved slog.Value as a zap.Field under key.
func valueField(key string, value slog.Value) zapcore.Field {
	switch value.Kind() {
	case slog.KindString:
		return zap.String(key, value.String())
	case slog.KindInt64:
		return zap.Int64(key, value.Int64())
	case slog.KindUint64:
		return zap.Uint64(key, value.Uint64())
	case slog.KindFloat64:
		return zap.Float64(key, value.Float64())
	case slog.KindBool:
		return zap.Bool(key, value.Bool())
	case slog.KindTime:
		return zap.Time(key, value.Time())
	case slog.KindDuration:
		return zap.Duration(key, value.Duration())
	default:
		return zap.Any(key, value.Any())
	}
}

// prefixKey qualifies key with the current group path.
func prefixKey(key string, groups []string) string {
	if len(groups) == 0 {
		return key
	}
	return strings.Join(groups, ".") + "." + key
}
