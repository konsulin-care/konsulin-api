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
	ServicePricing AppServicePricing `mapstructure:"service_pricing"`
	Webhook        AppWebhook        `mapstructure:"webhook"`
	Xendit         AppXendit         `mapstructure:"xendit"`
}

type App struct {
	Env                                            string `mapstructure:"env"`
	Port                                           string `mapstructure:"port"`
	Version                                        string `mapstructure:"version"`
	Address                                        string `mapstructure:"address"`
	BaseUrl                                        string `mapstructure:"base_url"`
	Timezone                                       string `mapstructure:"timezone"`
	FrontendDomain                                 string `mapstructure:"frontend_domain"`
	EndpointPrefix                                 string `mapstructure:"endpoint_prefix"`
	ResetPasswordUrl                               string `mapstructure:"reset_password_url"`
	MaxRequests                                    int    `mapstructure:"max_requests"`
	ShutdownTimeoutInSeconds                       int    `mapstructure:"shutdown_timeout_in_seconds"`
	MaxTimeRequestsPerSeconds                      int    `mapstructure:"max_time_requests_per_seconds"`
	SessionMultiplierInMinutes                     int    `mapstructure:"session_multiplier_in_minutes"`
	RequestBodyLimitInMegabyte                     int    `mapstructure:"request_body_limit_in_megabyte"`
	PaymentExpiredTimeInMinutes                    int    `mapstructure:"payment_expired_time_in_minutes"`
	PaymentGatewayRequestTimeoutInSeconds          int    `mapstructure:"payment_gateway_request_timeout_in_seconds"`
	AccountDeactivationAgeInDays                   int    `mapstructure:"account_deactivation_age_in_days"`
	LoginSessionExpiredTimeInHours                 int    `mapstructure:"login_session_expired_time_in_hours"`
	WhatsAppOTPExpiredTimeInMinutes                int    `mapstructure:"whatsapp_otp_expired_time_in_minutes"`
	ForgotPasswordTokenExpiredTimeInMinutes        int    `mapstructure:"forgot_password_token_expired_time_in_minutes"`
	MinioPreSignedUrlObjectExpiryTimeInHours       int    `mapstructure:"minio_pre_signed_url_object_expiry_time_in_hours"`
	QuestionnaireGuestResponseExpiredTimeInMinutes int    `mapstructure:"questionnaire_guest_response_expired_time_in_minutes"`
	SuperadminAPIKey                               string `mapstructure:"superadmin_api_key"`
	SuperadminAPIKeyRateLimit                      int    `mapstructure:"superadmin_api_key_rate_limit"`
	WebhookInstantiateBasePath                     string `mapstructure:"webhook_instantiate_base_path"`
	// SlotWindowDays controls rolling window days for Slot generation (default 30 if unset)
	SlotWindowDays int `mapstructure:"slot_window_days"`
	// SlotWorkerCronSpec defines the cron expression for the slot worker schedule (e.g., "@daily")
	SlotWorkerCronSpec string `mapstructure:"slot_worker_cron_spec"`
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
	MagiclinkBaseUrl           string `mapstructure:"magiclink_base_url"`
	KonsulinTenantID           string `mapstructure:"konsulin_tenant_id"`
	KonsulinDasboardAdminEmail string `mapstructure:"konsulin_dashboard_admin_email"`
}

type AppPaymentGateway struct {
	Username                string `mapstructure:"username"`
	ApiKey                  string `mapstructure:"api_key"`
	BaseUrl                 string `mapstructure:"base_url"`
	ListEnablePaymentMethod string `mapstructure:"list_enable_payment_method"`
	ListEnableSOF           string `mapstructure:"list_enable_sof"`
}

// AppServicePricing represents per-service base prices for service-based payments.
type AppServicePricing struct {
	AnalyzeBasePrice           int `mapstructure:"analyze_base_price"`
	ReportBasePrice            int `mapstructure:"report_base_price"`
	PerformanceReportBasePrice int `mapstructure:"performance_report_base_price"`
	AccessDatasetBasePrice     int `mapstructure:"access_dataset_base_price"`
}

// AppWebhook holds configuration for the Webhook Service Integration feature.
type AppWebhook struct {
	// RateLimit is the number of allowed requests per 60-second window (by service name)
	RateLimit int `mapstructure:"rate_limit"`
	// MonthlyQuota is the number of allowed requests per calendar month UTC (by service name)
	MonthlyQuota int `mapstructure:"monthly_quota"`
	// RateLimitedServices is a CSV list of service names subject to rate limiting
	RateLimitedServices string `mapstructure:"rate_limited_services"`
	// PaidOnlyServices is a CSV list of service names that require a forwarded JWT from payment service
	PaidOnlyServices string `mapstructure:"paid_only_services"`
	// AsyncServiceNames is a parsed list of service names that trigger async ServiceRequest creation
	AsyncServiceNames []string
	// MaxQueue defines how many items the worker processes per tick
	MaxQueue int `mapstructure:"max_queue"`
	// ThrottleRetry is the failedCount threshold before sending to DLQ
	ThrottleRetry int `mapstructure:"throttle_retry"`
	// URL is the base URL of the external webhook service
	URL string `mapstructure:"url"`
	// HTTPTimeoutInSeconds is the HTTP client timeout when calling the webhook
	HTTPTimeoutInSeconds int `mapstructure:"http_timeout_in_seconds"`
	// JWTAlg selects the signing algorithm (ES256|RS256)
	JWTAlg string `mapstructure:"jwt_alg"`
	// JWTHookKey is the private key PEM for signing webhook JWTs
	JWTHookKey string `mapstructure:"jwt_hook_key"`
	// KonsulinOmnichannelContactSyncURL is the full URL of the Konsulin Omnichannel Contact Sync service endpoint
	KonsulinOmnichannelContactSyncURL string `mapstructure:"konsulin_omnichannel_contact_sync_url"`
}

// AppXendit holds Xendit SDK configuration
type AppXendit struct {
	APIKey       string `mapstructure:"api_key"`
	WebhookToken string `mapstructure:"webhook_token"`
}
