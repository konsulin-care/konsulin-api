package logger

import (
	"konsulin-service/internal/app/config"
	"os"

	"github.com/sirupsen/logrus"
)

func InitLogger(driverConfig *config.DriverConfig) {
	switch driverConfig.App.Env {
	case "production":
		logrus.SetFormatter(&logrus.JSONFormatter{})
		file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			logrus.SetOutput(file)
		} else {
			logrus.Info("Failed to log to file, using default stderr")
		}
	default:
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
}
