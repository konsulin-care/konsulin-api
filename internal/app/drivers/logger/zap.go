package logger

import (
	"konsulin-service/internal/app/config"
	"log"

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

	var outputPaths []string
	var errorOutputPaths []string

	switch internalConfig.App.Env {
	case "development":
		outputPaths = []string{"stdout"}
		errorOutputPaths = []string{"stderr"}
	case "production":
		outputPaths = []string{driverConfig.Logger.OutputFileName}
		errorOutputPaths = []string{"stderr", driverConfig.Logger.OutputErrorFileName}
	default:
		outputPaths = []string{"stdout"}
		errorOutputPaths = []string{"stderr"}
	}

	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(logLevel),
		Development:      internalConfig.App.Env == "development",
		Encoding:         "json",
		EncoderConfig:    encoderConfig,
		OutputPaths:      outputPaths,
		ErrorOutputPaths: errorOutputPaths,
	}

	zapLogger, err := cfg.Build()
	if err != nil {
		log.Fatalf("Error while initializing zap logger: %v", err)
	}
	return zapLogger
}
