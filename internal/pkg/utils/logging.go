package utils

import (
	"context"
	"time"

	"konsulin-service/internal/pkg/constvars"

	"go.uber.org/zap"
)

type LoggableError struct {
	Message string
	Code    string
	Context map[string]interface{}
	Err     error
}

func (e LoggableError) LogFields() []zap.Field {
	fields := []zap.Field{
		zap.String(constvars.LoggingErrorCodeKey, e.Code),
		zap.String(constvars.LoggingErrorMessageKey, e.Message),
	}

	for k, v := range e.Context {
		fields = append(fields, zap.Any(k, v))
	}

	if e.Err != nil {
		fields = append(fields, zap.Error(e.Err))
	}

	return fields
}

func LogOperation(logger *zap.Logger, operation string, requestID string, fn func() error) error {
	start := time.Now()

	logger.Debug("Operation started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOperationKey, operation),
	)

	err := fn()

	duration := time.Since(start)

	if err != nil {
		logger.Error("Operation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingOperationKey, operation),
			zap.Duration(constvars.LoggingDurationKey, duration),
			zap.Bool(constvars.LoggingSuccessKey, false),
			zap.Error(err),
		)
		return err
	}

	logger.Info("Operation completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOperationKey, operation),
		zap.Duration(constvars.LoggingDurationKey, duration),
		zap.Bool(constvars.LoggingSuccessKey, true),
	)

	return nil
}

func LogBusinessEvent(logger *zap.Logger, event string, requestID string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("business_event", event),
		zap.Time("timestamp", time.Now()),
	}
	allFields = append(allFields, fields...)

	logger.Info("Business event occurred", allFields...)
}

func LogSecurityEvent(logger *zap.Logger, event string, requestID string, severity string, fields ...zap.Field) {
	allFields := []zap.Field{
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("security_event", event),
		zap.String("severity", severity),
		zap.Time("timestamp", time.Now()),
	}
	allFields = append(allFields, fields...)

	logger.Warn("Security event detected", allFields...)
}

func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string); ok {
		return requestID
	}
	return ""
}
