package config

import (
	"fmt"
	"konsulin-service/internal/pkg/utils"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

var (
	internalCfg *InternalConfig
	driverCfg   *DriverConfig
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v\n", err)
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	internalCfg = loadInternalConfigWithEnv()
	driverCfg = loadDriverConfigWithEnv()
}

func loadViperConfig(env string) error {
	viper.SetConfigName(fmt.Sprintf("config.%s", env))
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	return viper.ReadInConfig()
}

func loadInternalConfigWithYAML() *InternalConfig {
	var config InternalConfig
	err := viper.UnmarshalKey("internal_config", &config)
	if err != nil {
		log.Fatalf("unable to decode into InternalConfig: %s", err)
	}
	return &config
}

func loadInternalConfigWithEnv() *InternalConfig {
	cfg := &InternalConfig{
		App: App{
			Env:                                      utils.GetEnvString("APP_ENV", ""),
			Port:                                     utils.GetEnvString("APP_PORT", ""),
			Version:                                  utils.GetEnvString("APP_VERSION", ""),
			Address:                                  utils.GetEnvString("APP_ADDRESS", ""),
			BaseUrl:                                  utils.GetEnvString("APP_BASE_URL", ""),
			Timezone:                                 utils.GetEnvString("APP_TIMEZONE", ""),
			FrontendDomain:                           utils.GetEnvString("APP_FRONTEND_DOMAIN", ""),
			EndpointPrefix:                           utils.GetEnvString("APP_ENDPOINT_PREFIX", ""),
			ResetPasswordUrl:                         utils.GetEnvString("APP_RESET_PASSWORD_URL", ""),
			MaxRequests:                              utils.GetEnvInt("APP_MAX_REQUESTS", 0),
			ShutdownTimeoutInSeconds:                 utils.GetEnvInt("APP_SHUTDOWN_TIMEOUT_IN_SECONDS", 0),
			MaxTimeRequestsPerSeconds:                utils.GetEnvInt("APP_MAX_TIME_REQUESTS_PER_SECONDS", 0),
			SessionMultiplierInMinutes:               utils.GetEnvInt("APP_SESSION_MULTIPLIER_IN_MINUTES", 0),
			RequestBodyLimitInMegabyte:               utils.GetEnvInt("APP_REQUEST_BODY_LIMIT_IN_MEGABYTE", 0),
			PaymentExpiredTimeInMinutes:              utils.GetEnvInt("APP_PAYMENT_EXPIRED_TIME_IN_MINUTES", 0),
			PaymentGatewayRequestTimeoutInSeconds:    utils.GetEnvInt("APP_PAYMENT_GATEWAY_REQUEST_TIMEOUT_IN_SECONDS", 60),
			AccountDeactivationAgeInDays:             utils.GetEnvInt("APP_ACCOUNT_DEACTIVATION_AGE_IN_DAYS", 0),
			LoginSessionExpiredTimeInHours:           utils.GetEnvInt("APP_LOGIN_SESSION_EXPIRED_TIME_IN_HOURS", 0),
			WhatsAppOTPExpiredTimeInMinutes:          utils.GetEnvInt("APP_WHATSAPP_OTP_EXPIRED_TIME_IN_MINUTES", 0),
			ForgotPasswordTokenExpiredTimeInMinutes:  utils.GetEnvInt("APP_FORGOT_PASSWORD_TOKEN_EXPIRED_TIME_IN_MINUTES", 0),
			MinioPreSignedUrlObjectExpiryTimeInHours: utils.GetEnvInt("APP_MINIO_PRE_SIGNED_URL_OBJECT_EXPIRY_TIME_IN_HOURS", 0),
			QuestionnaireGuestResponseExpiredTimeInMinutes: utils.GetEnvInt("APP_QUESTIONNAIRE_GUEST_RESPONSE_EXPIRED_TIME_IN_MINUTES", 0),
			SuperadminAPIKey:          utils.GetEnvString("SUPERADMIN_API_KEY", ""),
			SuperadminAPIKeyRateLimit: utils.GetEnvInt("SUPERADMIN_API_KEY_RATE_LIMIT", 100),
		},
		FHIR: AppFHIR{
			BaseUrl: utils.GetEnvString("APP_FHIR_BASE_URL", ""),
		},
		JWT: AppJWT{
			Secret:        utils.GetEnvString("APP_JWT_SECRET", ""),
			ExpTimeInHour: utils.GetEnvInt("APP_JWT_EXP_TIME_IN_HOUR", 0),
		},
		Mailer: AppMailer{
			EmailSender: utils.GetEnvString("APP_MAILER_EMAIL_SENDER", ""),
		},
		Minio: AppMinio{
			ProfilePictureMaxUploadSizeInMB: utils.GetEnvInt("APP_MINIO_PROFILE_PICTURE_MAX_UPLOAD_SIZE_IN_MB", 0),
			BucketName:                      utils.GetEnvString("APP_MINIO_BUCKET_NAME", ""),
		},
		RabbitMQ: AppRabbitMQ{
			MailerQueue:   utils.GetEnvString("APP_RABBITMQ_MAILER_QUEUE", ""),
			WhatsAppQueue: utils.GetEnvString("APP_RABBITMQ_WHATSAPP_QUEUE", ""),
		},
		MongoDB: AppMongoDB{
			FhirDBName:     utils.GetEnvString("APP_MONGODB_FHIR_DB_NAME", ""),
			KonsulinDBName: utils.GetEnvString("APP_MONGODB_KONSULIN_DB_NAME", ""),
		},
		Konsulin: AppKonsulin{
			BankCode:           utils.GetEnvString("APP_KONSULIN_BANK_CODE", ""),
			BankAccountNumber:  utils.GetEnvString("APP_KONSULIN_BANK_ACCOUNT_NUMBER", ""),
			FinanceEmail:       utils.GetEnvString("APP_KONSULIN_FINANCE_EMAIL", ""),
			PaymentDisplayName: utils.GetEnvString("APP_KONSULIN_PAYMENT_DISPLAY_NAME", ""),
		},
		Supertoken: AppSupertoken{
			MagiclinkBaseUrl:           utils.GetEnvString("APP_SUPERTOKEN_MAGICLINK_BASE_URL", ""),
			KonsulinTenantID:           utils.GetEnvString("APP_SUPERTOKEN_KONSULIN_TENANT_ID", ""),
			KonsulinDasboardAdminEmail: utils.GetEnvString("APP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL", ""),
		},
		PaymentGateway: AppPaymentGateway{
			Username:                utils.GetEnvString("APP_PAYMENT_GATEWAY_USERNAME", ""),
			ApiKey:                  utils.GetEnvString("APP_PAYMENT_GATEWAY_API_KEY", ""),
			BaseUrl:                 utils.GetEnvString("APP_PAYMENT_GATEWAY_BASE_URL", ""),
			ListEnablePaymentMethod: utils.GetEnvString("OY_LIST_ENABLE_PAYMENT_METHOD", ""),
			ListEnableSOF:           utils.GetEnvString("OY_LIST_ENABLE_SOF", ""),
		},
		ServicePricing: AppServicePricing{
			AnalyzeBasePrice:           utils.GetEnvInt("BASE_PRICE_ANALYZE", 0),
			ReportBasePrice:            utils.GetEnvInt("BASE_PRICE_REPORT", 0),
			PerformanceReportBasePrice: utils.GetEnvInt("BASE_PRICE_PERFORMANCE_REPORT", 0),
			AccessDatasetBasePrice:     utils.GetEnvInt("BASE_PRICE_ACCESS_DATASET", 0),
		},
		Webhook: AppWebhook{
			RateLimit:            utils.GetEnvInt("HOOK_RATE_LIMIT", 0),
			MonthlyQuota:         utils.GetEnvInt("HOOK_QUOTA", 0),
			RateLimitedServices:  utils.GetEnvString("HOOK_RATE_LIMITED_SERVICES", ""),
			MaxQueue:             utils.GetEnvInt("HOOK_MAX_QUEUE", 1),
			ThrottleRetry:        utils.GetEnvInt("HOOK_THROTTLE_RETRY", 15),
			URL:                  utils.GetEnvString("HOOK_URL", ""),
			HTTPTimeoutInSeconds: utils.GetEnvInt("HOOK_HTTP_TIMEOUT", 10),
			JWTAlg:               utils.GetEnvString("HOOK_JWT_ALG", "ES256"),
			JWTHookKey:           utils.GetEnvString("JWT_HOOK_KEY", ""),
		},
	}

	// this is a safe guard to ensure that no base price is left unset
	// this must be prevented because it will trigger failed payment
	// if the amount calculation resulting in 0
	if cfg.ServicePricing.AnalyzeBasePrice <= 0 ||
		cfg.ServicePricing.ReportBasePrice <= 0 ||
		cfg.ServicePricing.PerformanceReportBasePrice <= 0 ||
		cfg.ServicePricing.AccessDatasetBasePrice <= 0 {
		log.Fatalf("invalid service base price configuration: all BASE_PRICE_* must be > 0")
	}

	return cfg
}

func loadDriverConfigWithYAML() *DriverConfig {
	var config DriverConfig
	err := viper.UnmarshalKey("driver_config", &config)
	if err != nil {
		log.Fatalf("unable to decode into DriverConfig: %s", err)
	}
	return &config
}

func loadDriverConfigWithEnv() *DriverConfig {
	return &DriverConfig{
		Redis: Redis{
			Host:     utils.GetEnvString("REDIS_HOST", "localhost"),
			Port:     utils.GetEnvString("REDIS_PORT", "6379"),
			Password: utils.GetEnvString("REDIS_PASSWORD", ""),
		},
		Logger: Logger{
			Level:               utils.GetEnvString("LOGGER_LEVEL", "info"),
			OutputFileName:      utils.GetEnvString("LOGGER_OUTPUT_FILE_NAME", "app.log"),
			OutputErrorFileName: utils.GetEnvString("LOGGER_OUTPUT_ERROR_FILE_NAME", "error.log"),
		},
		RabbitMQ: RabbitMQ{
			Host:     utils.GetEnvString("RABBITMQ_HOST", "localhost"),
			Port:     utils.GetEnvString("RABBITMQ_PORT", "5672"),
			Username: utils.GetEnvString("RABBITMQ_USERNAME", "guest"),
			Password: utils.GetEnvString("RABBITMQ_PASSWORD", "guest"),
		},
		Minio: Minio{
			Host:     utils.GetEnvString("MINIO_HOST", "localhost"),
			Port:     utils.GetEnvString("MINIO_PORT", "9000"),
			Username: utils.GetEnvString("MINIO_USERNAME", "minioadmin"),
			Password: utils.GetEnvString("MINIO_PASSWORD", "minioadmin"),
			UseSSL:   utils.GetEnvBool("MINIO_USE_SSL", false),
		},
		Supertoken: Supertoken{
			ApiBasePath:     utils.GetEnvString("SUPERTOKEN_API_BASE_PATH", "/auth"),
			WebsiteBasePath: utils.GetEnvString("SUPERTOKEN_WEBSITE_BASE_PATH", "/"),
			ConnectionURI:   utils.GetEnvString("SUPERTOKEN_CONNECTION_URI", ""),
			AppName:         utils.GetEnvString("SUPERTOKEN_APP_NAME", "MyApp"),
			ApiDomain:       utils.GetEnvString("SUPERTOKEN_API_DOMAIN", "http://localhost:3000"),
			WebsiteDomain:   utils.GetEnvString("SUPERTOKEN_WEBSITE_DOMAIN", "http://localhost:3001"),
		},
	}
}

func NewInternalConfig() *InternalConfig {
	return internalCfg
}

func NewDriverConfig() *DriverConfig {
	return driverCfg
}

func GetInternalConfig() *InternalConfig {
	return internalCfg
}
func GetDriverConfig() *DriverConfig {
	return driverCfg
}
