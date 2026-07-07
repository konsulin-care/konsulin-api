package config

import (
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
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
		env = constvars.EnvLocal
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

			// General App Settings with Defaults
			Env:            utils.GetEnvString("APP_ENV", constvars.EnvLocal),
			Port:           utils.GetEnvString("APP_PORT", "3200"),
			Version:        utils.GetEnvString("APP_VERSION", "v1"),
			Address:        utils.GetEnvString("APP_ADDRESS", "localhost"),
			BaseUrl:        utils.GetEnvString("APP_BASE_URL", "http://localhost:3000/api/v1"),
			Timezone:       utils.GetEnvString("APP_TIMEZONE", "Asia/Jakarta"),
			FrontendDomain: utils.GetEnvString("APP_FRONTEND_DOMAIN", "http://localhost:3000"),
			EndpointPrefix: utils.GetEnvString("APP_ENDPOINT_PREFIX", "api"),

			// URLs & Timeouts
			MaxRequests:                           utils.GetEnvInt("APP_MAX_REQUESTS", 20),
			MaxTimeRequestsPerSeconds:             utils.GetEnvInt("APP_MAX_TIME_REQUESTS_PER_SECONDS", 30),
			RequestBodyLimitInMegabyte:            utils.GetEnvInt("APP_REQUEST_BODY_LIMIT_IN_MEGABYTE", 30),
			PaymentExpiredTimeInMinutes:           utils.GetEnvInt("APP_PAYMENT_EXPIRED_TIME_IN_MINUTES", 60),
			PaymentGatewayRequestTimeoutInSeconds: utils.GetEnvInt("APP_PAYMENT_GATEWAY_REQUEST_TIMEOUT_IN_SECONDS", 120),
			AccountDeactivationAgeInDays:          utils.GetEnvInt("APP_ACCOUNT_DEACTIVATION_AGE_IN_DAYS", 30),

			// Sensitive / Key Logic
			SuperadminAPIKey:           utils.GetEnvString("SUPERADMIN_API_KEY", ""), // Sensitive
			SuperadminAPIKeyRateLimit:  utils.GetEnvInt("SUPERADMIN_API_KEY_RATE_LIMIT", 10),
			WebhookInstantiateBasePath: fmt.Sprintf("/api/%s/%s", utils.GetEnvString("APP_VERSION", "v1"), utils.GetEnvString("APP_WEBHOOK_INSTANTIATE_BASE_PATH", "hook")), // expected format: /api/<app-version>/<hook-base-path>
			SlotWindowDays: func() int {
				v := utils.GetEnvInt("SLOT_WINDOW_DAYS", 30)
				if v <= 0 {
					return 30
				}
				return v
			}(),
			SlotWorkerCronSpec: utils.GetEnvString("SLOT_WORKER_CRON_SPEC", "@daily"),
		},
		FHIR: AppFHIR{
			BaseUrl:                  utils.GetEnvString("APP_FHIR_BASE_URL", "http://localhost:8080/fhir/"),
			TerminologyServerBaseUrl: utils.GetEnvString("APP_TERMINOLOGY_BASE_URL", "https://tx.konsulin.care/fhir"),
		},
		JWT: AppJWT{
			Secret:        utils.GetEnvString("APP_JWT_SECRET", ""),
			ExpTimeInHour: utils.GetEnvInt("APP_JWT_EXP_TIME_IN_HOUR", 1),
		},
		Mailer: AppMailer{
			EmailSender: utils.GetEnvString("APP_MAILER_EMAIL_SENDER", "konsulin.care@gmail.com"),
		},
		Konsulin: AppKonsulin{
			BankCode:           utils.GetEnvString("APP_KONSULIN_BANK_CODE", "014"),
			BankAccountNumber:  utils.GetEnvString("APP_KONSULIN_BANK_ACCOUNT_NUMBER", ""),
			FinanceEmail:       utils.GetEnvString("APP_KONSULIN_FINANCE_EMAIL", ""),
			PaymentDisplayName: utils.GetEnvString("APP_KONSULIN_PAYMENT_DISPLAY_NAME", "Konsulin"),
		},
		Supertoken: AppSupertoken{
			MagiclinkBaseUrl:           utils.GetEnvString("APP_SUPERTOKEN_MAGICLINK_BASE_URL", "http://localhost:3000/auth/verify"),
			KonsulinTenantID:           utils.GetEnvString("APP_SUPERTOKEN_KONSULIN_TENANT_ID", "public"),
			KonsulinDasboardAdminEmail: utils.GetEnvString("APP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL", ""),
		},
		PaymentGateway: AppPaymentGateway{
			Username:                utils.GetEnvString("APP_PAYMENT_GATEWAY_USERNAME", ""), // Sensitive
			ApiKey:                  utils.GetEnvString("APP_PAYMENT_GATEWAY_API_KEY", ""),  // Sensitive
			BaseUrl:                 utils.GetEnvString("APP_PAYMENT_GATEWAY_BASE_URL", ""),
			ListEnablePaymentMethod: utils.GetEnvString("OY_LIST_ENABLE_PAYMENT_METHOD", "BANK_TRANSFER,QRIS,EWALLET,CARDS"),
			ListEnableSOF:           utils.GetEnvString("OY_LIST_ENABLE_SOF", "QRIS,dana_ewallet,ovo_ewallet,shopeepay_ewallet,linkaja_ewallet,CC_DC"),
		},
		ServicePricing: AppServicePricing{
			// Default Pricing fallback
			AnalyzeBasePrice:           utils.GetEnvInt("BASE_PRICE_ANALYZE", 5000),
			ReportBasePrice:            utils.GetEnvInt("BASE_PRICE_REPORT", 20000),
			PerformanceReportBasePrice: utils.GetEnvInt("BASE_PRICE_PERFORMANCE_REPORT", 50000),
			AccessDatasetBasePrice:     utils.GetEnvInt("BASE_PRICE_ACCESS_DATASET", 100000),
		},
		Webhook: AppWebhook{
			RateLimit:                       utils.GetEnvInt("HOOK_RATE_LIMIT", -1),
			MonthlyQuota:                    utils.GetEnvInt("HOOK_QUOTA", -1),
			RateLimitedServices:             utils.GetEnvString("HOOK_RATE_LIMITED_SERVICES", "analyze,interpret"),
			PaidOnlyServices:                utils.GetEnvString("HOOK_PAID_ONLY_SERVICES", "analyze"),
			AsyncServiceNames:               parseCSVToLowerSlice(utils.GetEnvString("HOOK_ASYNC_SERVICE_NAMES", "")),
			SynchronousServiceNames:         parseCSVToLowerSlice(utils.GetEnvString("HOOK_SYNC_SERVICE_NAMES", "analyze")), // we default to "analyze" for satisfying the non-empty validation check
			SynchronousServiceRateLimit:     utils.GetEnvInt("HOOK_SYNCHRONOUS_SERVICE_RATE_LIMIT", 60),
			SynchronousServiceWindowSeconds: utils.GetEnvInt("HOOK_SYNCHRONOUS_SERVICE_WINDOW_SECONDS", 60),
			SynchronousServiceFailurePolicy: strings.ToLower(strings.TrimSpace(utils.GetEnvString("HOOK_SYNCHRONOUS_SERVICE_FAILURE_POLICY", "return_error"))),
			MaxQueue:                        utils.GetEnvInt("HOOK_MAX_QUEUE", 150),
			ThrottleRetry:                   utils.GetEnvInt("HOOK_THROTTLE_RETRY", 3),
			URL:                             utils.GetEnvString("HOOK_URL", ""),
			HTTPTimeoutInSeconds:            utils.GetEnvInt("HOOK_HTTP_TIMEOUT", 5),
			JWTAlg:                          utils.GetEnvString("HOOK_JWT_ALG", "ES256"),
			JWTHookKey:                      utils.GetEnvString("JWT_HOOK_KEY", ""), // Sensitive
		},
		Xendit: AppXendit{
			APIKey:       utils.GetEnvString("APP_XENDIT_API_KEY", ""), // Sensitive
			WebhookToken: utils.GetEnvString("APP_XENDIT_WEBHOOK_TOKEN", ""),
		},
	}

	if err := validateNonDevConfig(cfg); err != nil {
		log.Fatal(err.Error())
	}
	if err := validateBasePrices(cfg); err != nil {
		log.Fatal(err.Error())
	}
	if err := normalizeWebhookConfig(cfg); err != nil {
		log.Fatal(err.Error())
	}
	normalizeSlotCronSpec(cfg)

	return cfg
}

// normalizeSlotCronSpec validates the slot worker cron spec and defaults to @daily if invalid.
func normalizeSlotCronSpec(cfg *InternalConfig) {
	spec := cfg.App.SlotWorkerCronSpec
	if _, err := cron.ParseStandard(spec); err != nil {
		log.Printf("slot worker: invalid cron spec '%s': %v, defaulting to @daily", spec, err)
		cfg.App.SlotWorkerCronSpec = "@daily"
	}
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
	cfg := &DriverConfig{
		Redis: Redis{
			Host:     utils.GetEnvString("REDIS_HOST", "localhost"),
			Port:     utils.GetEnvString("REDIS_PORT", "6379"),
			Password: utils.GetEnvString("REDIS_PASSWORD", ""),
		},
		Logger: Logger{
			Level:               utils.GetEnvString("LOGGER_LEVEL", "info"),
			OutputFileName:      utils.GetEnvString("LOGGER_OUTPUT_FILE_NAME", "logger.log"),
			OutputErrorFileName: utils.GetEnvString("LOGGER_OUTPUT_ERROR_FILE_NAME", "logger-error.log"),
		},
		RabbitMQ: RabbitMQ{
			// Defaults for connectivity (Standard Localhost)
			Host: utils.GetEnvString("RABBITMQ_HOST", "localhost"),
			Port: utils.GetEnvString("RABBITMQ_PORT", "5672"),
			// These must be set in .env
			Username: utils.GetEnvString("RABBITMQ_USERNAME", ""),
			Password: utils.GetEnvString("RABBITMQ_PASSWORD", ""),
		},
		Supertoken: Supertoken{
			ApiBasePath:     utils.GetEnvString("SUPERTOKEN_API_BASE_PATH", "/auth"),
			WebsiteBasePath: utils.GetEnvString("SUPERTOKEN_WEBSITE_BASE_PATH", "/auth"),
			ConnectionURI:   utils.GetEnvString("SUPERTOKEN_CONNECTION_URI", "http://localhost:3567"),
			APIKey:          utils.GetEnvString("SUPERTOKEN_API_KEY", ""),
			AppName:         utils.GetEnvString("SUPERTOKEN_APP_NAME", "Konsulin"),
			ApiDomain:       utils.GetEnvString("SUPERTOKEN_API_DOMAIN", "http://localhost:3000"),
			WebsiteDomain:   utils.GetEnvString("SUPERTOKEN_WEBSITE_DOMAIN", "http://localhost:3000"),
		},
	}

	// Check current environment
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = constvars.EnvLocal
	}

	if err := validateNonDevDriverConfig(cfg, env); err != nil {
		log.Fatal(err.Error())
	}

	return cfg
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

// validateNonDevConfig ensures mandatory sensitive fields are set in non-dev environments.
func validateNonDevConfig(cfg *InternalConfig) error {
	if cfg.App.Env == constvars.EnvLocal || cfg.App.Env == constvars.EnvDev || cfg.App.Env == constvars.EnvDevelopment || cfg.App.Env == constvars.EnvTest {
		return nil
	}
	if cfg.JWT.Secret == "" {
		return fmt.Errorf("APP_JWT_SECRET is required in %s environment", cfg.App.Env)
	}
	if cfg.Webhook.JWTHookKey == "" {
		return fmt.Errorf("JWT_HOOK_KEY is required in %s environment", cfg.App.Env)
	}
	if cfg.PaymentGateway.Username == "" || cfg.PaymentGateway.ApiKey == "" {
		return fmt.Errorf("payment gateway credentials (APP_PAYMENT_GATEWAY_USERNAME, APP_PAYMENT_GATEWAY_API_KEY) are required in %s environment", cfg.App.Env)
	}
	if cfg.Xendit.APIKey == "" {
		return fmt.Errorf("APP_XENDIT_API_KEY is required in %s environment", cfg.App.Env)
	}
	if cfg.PaymentGateway.BaseUrl == "" {
		return fmt.Errorf("APP_PAYMENT_GATEWAY_BASE_URL is required in %s environment", cfg.App.Env)
	}
	return nil
}

// validateBasePrices ensures no service base price is left unset (would cause $0 payments).
func validateBasePrices(cfg *InternalConfig) error {
	if cfg.ServicePricing.AnalyzeBasePrice <= 0 ||
		cfg.ServicePricing.ReportBasePrice <= 0 ||
		cfg.ServicePricing.PerformanceReportBasePrice <= 0 ||
		cfg.ServicePricing.AccessDatasetBasePrice <= 0 {
		return fmt.Errorf("invalid service base price configuration: all BASE_PRICE_* must be > 0")
	}
	return nil
}

// normalizeWebhookConfig validates and sets defaults for webhook synchronous service config.
func normalizeWebhookConfig(cfg *InternalConfig) error {
	if len(cfg.Webhook.SynchronousServiceNames) == 0 {
		return fmt.Errorf("invalid webhook configuration: HOOK_SYNC_SERVICE_NAMES must be set and non-empty")
	}
	if cfg.Webhook.SynchronousServiceRateLimit <= 0 {
		cfg.Webhook.SynchronousServiceRateLimit = 60
	}
	if cfg.Webhook.SynchronousServiceWindowSeconds <= 0 {
		cfg.Webhook.SynchronousServiceWindowSeconds = 60
	}
	switch cfg.Webhook.SynchronousServiceFailurePolicy {
	case "return_error", "enqueue_request":
	default:
		cfg.Webhook.SynchronousServiceFailurePolicy = "return_error"
	}
	return nil
}

// validateNonDevDriverConfig ensures mandatory sensitive fields are set in non-dev environments.
func validateNonDevDriverConfig(cfg *DriverConfig, env string) error {
	if env == constvars.EnvLocal || env == constvars.EnvDev || env == constvars.EnvDevelopment || env == constvars.EnvTest {
		return nil
	}
	if cfg.Redis.Password == "" {
		return fmt.Errorf("REDIS_PASSWORD is required in %s environment", env)
	}
	if cfg.RabbitMQ.Username == "" || cfg.RabbitMQ.Password == "" {
		return fmt.Errorf("RabbitMQ credentials (RABBITMQ_USERNAME, RABBITMQ_PASSWORD) are required in %s environment", env)
	}
	if cfg.Supertoken.APIKey == "" {
		return fmt.Errorf("Supertoken API key is required in %s environment", env)
	}
	return nil
}

// parseCSVToLowerSlice parses a comma-separated string into a slice of trimmed, lowercased strings.
// Returns an empty slice if the input is empty or contains only whitespace.
func parseCSVToLowerSlice(csv string) []string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return []string{}
	}
	parts := strings.Split(csv, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, strings.ToLower(trimmed))
		}
	}
	return result
}
