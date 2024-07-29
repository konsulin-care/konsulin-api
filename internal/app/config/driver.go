package config

type (
	DriverConfig struct {
		MongoDB  MongoDB
		Redis    Redis
		Logger   Logger
		SMTP     SMTP
		RabbitMQ RabbitMQ
		Minio    Minio
	}
	MongoDB struct {
		Port     string
		Host     string
		DbName   string
		Username string
		Password string
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
	SMTP struct {
		Host        string
		Username    string
		Password    string
		EmailSender string
		Port        int
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
