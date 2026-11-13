package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"

	"github.com/joho/godotenv"
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

	var loadErr error
	internalCfg, loadErr = loadInternalConfigWithEnv()
	if loadErr != nil {
		log.Fatal(loadErr.Error())
	}

	driverCfg, loadErr = loadDriverConfigWithEnv()
	if loadErr != nil {
		log.Fatal(loadErr.Error())
	}
}

func loadInternalConfigWithEnv() (*InternalConfig, error) {
	gatewayURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
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
			MaxRequests:                 utils.GetEnvInt("APP_MAX_REQUESTS", 20),
			PaymentExpiredTimeInMinutes: utils.GetEnvInt("APP_PAYMENT_EXPIRED_TIME_IN_MINUTES", 60),
			ShutdownTimeoutInSeconds:    utils.GetEnvInt("APP_SHUTDOWN_TIMEOUT_IN_SECONDS", 30),

			// Sensitive / Key Logic
			SuperadminAPIKey:          utils.GetEnvString("SUPERADMIN_API_KEY", ""), // Sensitive
			SuperadminAPIKeyRateLimit: utils.GetEnvInt("SUPERADMIN_API_KEY_RATE_LIMIT", 10),
			RBACPolicyPath:            utils.GetEnvString("APP_RBAC_POLICY_PATH", "resources/rbac_policy.csv"),
			OrganizationName:          utils.GetEnvString("APP_ORGANIZATION_NAME", "Konsulin"),
			DataDir:                   utils.GetEnvString("APP_DATA_DIR", "./data"),
			Pepper:                    utils.GetEnvString("APP_PEPPER", ""), // Sensitive
		},
		FHIR: AppFHIR{
			BaseUrl: utils.GetEnvString("APP_FHIR_BASE_URL", "http://localhost:8080/fhir/"),
		},
		JWT: AppJWT{
			Secret: utils.GetEnvString("APP_JWT_SECRET", ""),
		},
		Supertoken: AppSupertoken{
			KonsulinTenantID:           utils.GetEnvString("APP_SUPERTOKEN_KONSULIN_TENANT_ID", "public"),
			KonsulinDasboardAdminEmail: utils.GetEnvString("APP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL", ""),
		},
		Gateway: AppGateway{
			Enabled:                gatewayURL != "",
			URL:                    gatewayURL,
			PartyID:                utils.GetEnvString("APP_PARTY_ID", ""),
			PrivateKey:             utils.GetEnvString("APP_PARTY_PRIVATE_KEY", ""),          // Sensitive — never logged
			EncPrivateKey:          utils.GetEnvString("APP_PARTY_ENC_PRIVATE_KEY", ""),      // Sensitive — never logged
			EnrollmentToken:        utils.GetEnvString("APP_PARTY_ENROLLMENT_TOKEN", ""),     // Sensitive — never logged
			NextPrivateKey:         utils.GetEnvString("APP_PARTY_NEXT_PRIVATE_KEY", ""),     // Sensitive — never logged
			NextEncPrivateKey:      utils.GetEnvString("APP_PARTY_NEXT_ENC_PRIVATE_KEY", ""), // Sensitive — never logged
			AutomationPartyID:      utils.GetEnvString("APP_AUTOMATION_PARTY_ID", ""),
			ServiceProviderAliases: utils.GetEnvString("APP_SERVICE_PROVIDER_ALIASES", ""),
		},
	}

	if err := validateNonDevConfig(cfg); err != nil {
		return nil, err
	}
	if err := validateGatewaySettings(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func loadDriverConfigWithEnv() (*DriverConfig, error) {
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
		return nil, err
	}

	return cfg, nil
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
		return fmt.Errorf("env var APP_JWT_SECRET is required in %s environment", cfg.App.Env)
	}
	return nil
}

// validateNonDevDriverConfig ensures mandatory sensitive fields are set in non-dev environments.
func validateNonDevDriverConfig(cfg *DriverConfig, env string) error {
	if env == constvars.EnvLocal || env == constvars.EnvDev || env == constvars.EnvDevelopment || env == constvars.EnvTest {
		return nil
	}
	if cfg.Redis.Password == "" {
		return fmt.Errorf("env var REDIS_PASSWORD is required in %s environment", env)
	}
	if cfg.Supertoken.APIKey == "" {
		return fmt.Errorf("env var SUPERTOKEN_API_KEY is required in %s environment", env)
	}
	return nil
}
