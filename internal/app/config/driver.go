package config

type (
	DriverConfig struct {
		MongoDB  MongoDB
		Redis    Redis
		Logger   Logger
		RabbitMQ RabbitMQ
		Minio    Minio
	}
	MongoDB struct {
		Port           string
		Host           string
		Username       string
		Password       string
		FhirDbName     string
		KonsulinDbName string
	}
	Redis struct {
		Host     string
		Port     string
		Password string
	}
	Logger struct {
		Level               string
		OutputFileName      string
		OutputErrorFileName string
	}
	RabbitMQ struct {
		Port     string
		Host     string
		Username string
		Password string
	}
	Minio struct {
		Port     string
		Host     string
		Username string
		Password string
		UseSSL   bool
	}
)
