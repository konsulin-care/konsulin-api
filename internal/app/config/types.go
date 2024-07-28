package config

type (
	InternalConfig struct {
		App  App
		FHIR FHIR
		JWT  JWT
	}

	DriverConfig struct {
		MongoDB MongoDB
		Redis   Redis
		Logger  Logger
		SMTP    SMTP
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
		ShutdownTimeout                    int
		MaxTimeRequestsPerSeconds          int
		RequestBodyLimitInMegabyte         int
		ForgotPasswordTokenExpTimeInMinute int
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
	FHIR struct {
		BaseUrl string
	}

	JWT struct {
		Secret        string
		ExpTimeInHour int
	}
)
