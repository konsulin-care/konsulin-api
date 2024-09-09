package logger

import (
	"konsulin-service/internal/app/config"
	"log"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewZapLogger(driverConfig *config.DriverConfig, internalConfig *config.InternalConfig) *zap.Logger {
	var logLevel zapcore.Level
	switch driverConfig.Logger.Level {
	case "debug":
		logLevel = zap.DebugLevel
	case "info":
		logLevel = zap.InfoLevel
	case "warn":
		logLevel = zap.WarnLevel
	case "error":
		logLevel = zap.ErrorLevel
	default:
		logLevel = zap.InfoLevel
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime:     customTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		EncodeLevel:    customLevelEncoder,
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Development:      internalConfig.App.Env == "development",
		Encoding:         "json",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if internalConfig.App.Env == "production" || internalConfig.App.Env == "development" {
		cfg.OutputPaths = []string{driverConfig.Logger.OutputFileName}
		cfg.ErrorOutputPaths = []string{"stderr", driverConfig.Logger.OutputErrorFileName}
		encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	}

	zapLogger, err := cfg.Build()
	if err != nil {
		log.Fatalf("Error while initializing zap logger: %v", err)
	}
	return zapLogger
}

func customTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006/01/02 15:04:05"))
}

func customLevelEncoder(level zapcore.Level, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(level.CapitalString())
}
