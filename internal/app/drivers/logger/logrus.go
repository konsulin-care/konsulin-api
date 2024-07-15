package logger

import (
	"konsulin-service/internal/app/config"
	"os"

	"github.com/sirupsen/logrus"
)

func NewLogrusLogger(internalConfig *config.InternalConfig) *logrus.Logger {
	logger := logrus.New()
	switch internalConfig.App.Env {
	case "production":
		logger.SetFormatter(&logrus.JSONFormatter{})
		file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logger.SetOutput(file)
		} else {
			logger.Info("Failed to log to file, using default stderr")
		}
	default:
		logger.SetFormatter(&logrus.TextFormatter{})
	}
	return logger
}
