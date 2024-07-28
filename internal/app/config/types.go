package config

type (
	InternalConfig struct {
		App  App
		FHIR FHIR
		JWT  JWT
	}

	DriverConfig struct {
		MongoDB  MongoDB
		Redis    Redis
		Logger   Logger
		SMTP     SMTP
		RabbitMQ RabbitMQ
		Minio    Minio
	}

	App struct {
		Env                                  string
		Port                                 string
		Version                              string
		Address                              string
		Timezone                             string
		EndpointPrefix                       string
		ResetPasswordUrl                     string
		MailerEmailSender                    string
		RabbitMQMailerQueue                  string
		RabbitMQWhatsAppQueue                string
		MaxRequests                          int
		ShutdownTimeout                      int
		MaxTimeRequestsPerSeconds            int
		RequestBodyLimitInMegabyte           int
		ForgotPasswordTokenExpTimeInMinute   int
		MinioProfilePictureMaxUploadSizeInMB int64
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
		Port       string
		Host       string
		Username   string
		Password   string
		BucketName string
	}
	FHIR struct {
		BaseUrl string
	}

	JWT struct {
		Secret        string
		ExpTimeInHour int
	}
)
