package config

type InternalConfig struct {
	App            App               `mapstructure:"app"`
	FHIR           AppFHIR           `mapstructure:"fhir"`
	JWT            AppJWT            `mapstructure:"jwt"`
	Mailer         AppMailer         `mapstructure:"mailer"`
	Minio          AppMinio          `mapstructure:"minio"`
	RabbitMQ       AppRabbitMQ       `mapstructure:"rabbitmq"`
	MongoDB        AppMongoDB        `mapstructure:"mongodb"`
	Konsulin       AppKonsulin       `mapstructure:"konsulin"`
	Supertoken     AppSupertoken     `mapstructure:"supertoken"`
	PaymentGateway AppPaymentGateway `mapstructure:"payment_gateway"`
}

type App struct {
	Env                                            string `mapstructure:"env"`
	Port                                           string `mapstructure:"port"`
	Version                                        string `mapstructure:"version"`
	Address                                        string `mapstructure:"address"`
	BaseUrl                                        string `mapstructure:"base_url"`
	Timezone                                       string `mapstructure:"timezone"`
	EndpointPrefix                                 string `mapstructure:"endpoint_prefix"`
	ResetPasswordUrl                               string `mapstructure:"reset_password_url"`
	MaxRequests                                    int    `mapstructure:"max_requests"`
	ShutdownTimeoutInSeconds                       int    `mapstructure:"shutdown_timeout_in_seconds"`
	MaxTimeRequestsPerSeconds                      int    `mapstructure:"max_time_requests_per_seconds"`
	SessionMultiplierInMinutes                     int    `mapstructure:"session_multiplier_in_minutes"`
	RequestBodyLimitInMegabyte                     int    `mapstructure:"request_body_limit_in_megabyte"`
	PaymentExpiredTimeInMinutes                    int    `mapstructure:"payment_expired_time_in_minutes"`
	AccountDeactivationAgeInDays                   int    `mapstructure:"account_deactivation_age_in_days"`
	LoginSessionExpiredTimeInHours                 int    `mapstructure:"login_session_expired_time_in_hours"`
	WhatsAppOTPExpiredTimeInMinutes                int    `mapstructure:"whatsapp_otp_expired_time_in_minutes"`
	ForgotPasswordTokenExpiredTimeInMinutes        int    `mapstructure:"forgot_password_token_expired_time_in_minutes"`
	MinioPreSignedUrlObjectExpiryTimeInHours       int    `mapstructure:"minio_pre_signed_url_object_expiry_time_in_hours"`
	QuestionnaireGuestResponseExpiredTimeInMinutes int    `mapstructure:"questionnaire_guest_response_expired_time_in_minutes"`
}

type AppFHIR struct {
	BaseUrl string `mapstructure:"base_url"`
}

type AppJWT struct {
	Secret        string `mapstructure:"secret"`
	ExpTimeInHour int    `mapstructure:"exp_time_in_hour"`
}

type AppMailer struct {
	EmailSender string `mapstructure:"email_sender"`
}

type AppMinio struct {
	ProfilePictureMaxUploadSizeInMB int    `mapstructure:"profile_picture_max_upload_size_in_mb"`
	BucketName                      string `mapstructure:"bucket_name"`
}

type AppRabbitMQ struct {
	MailerQueue   string `mapstructure:"mailer_queue"`
	WhatsAppQueue string `mapstructure:"whatsapp_queue"`
}

type AppMongoDB struct {
	FhirDBName     string `mapstructure:"fhir_db_name"`
	KonsulinDBName string `mapstructure:"konsulin_db_name"`
}
type AppKonsulin struct {
	BankCode           string `mapstructure:"bank_code"`
	BankAccountNumber  string `mapstructure:"bank_account_number"`
	FinanceEmail       string `mapstructure:"finance_email"`
	PaymentDisplayName string `mapstructure:"payment_display_name"`
}

type AppSupertoken struct {
	MagiclinkBaseUrl string `mapstructure:"magiclink_base_url"`
	KonsulinTenantID string `mapstructure:"konsulin_tenant_id"`
}
type AppPaymentGateway struct {
	Username string `mapstructure:"username"`
	ApiKey   string `mapstructure:"api_key"`
	BaseUrl  string `mapstructure:"base_url"`
}
