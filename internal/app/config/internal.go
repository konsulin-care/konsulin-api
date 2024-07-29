package config

type (
	InternalConfig struct {
		App      App
		FHIR     AppFHIR
		JWT      AppJWT
		Mailer   AppMailer
		Minio    AppMinio
		RabbitMQ AppRabbitMQ
	}
	App struct {
		Env                                string
		Port                               string
		Version                            string
		Address                            string
		Timezone                           string
		EndpointPrefix                     string
		ResetPasswordUrl                   string
		MaxRequests                        int
		ShutdownTimeoutInSecond            int
		MaxTimeRequestsPerSeconds          int
		RequestBodyLimitInMegabyte         int
		ForgotPasswordTokenExpTimeInMinute int
	}
	AppFHIR struct {
		BaseUrl string
	}

	AppJWT struct {
		Secret        string
		ExpTimeInHour int
	}

	AppMailer struct {
		EmailSender string
	}

	AppMinio struct {
		ProfilePictureMaxUploadSizeInMB int64
		BucketName                      string
	}
	AppRabbitMQ struct {
		MailerQueue   string
		WhatsAppQueue string
	}
)
