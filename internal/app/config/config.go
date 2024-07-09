package config

import (
	"konsulin-service/internal/pkg/utils"

	"github.com/joho/godotenv"
)

func init() {
	godotenv.Load()
}

func NewDriverConfig() *DriverConfig {
	return &DriverConfig{
		MongoDB: MongoDB{
			Port:     utils.GetEnvString("MONGODB_PORT", "17017"),
			Host:     utils.GetEnvString("MONGODB_HOST", "localhost"),
			DbName:   utils.GetEnvString("MONGODB_DB_NAME", "spark"),
			Username: utils.GetEnvString("MONGODB_USERNAME", "defaultUsername"),
			Password: utils.GetEnvString("MONGODB_PASSWORD", "defaultPassword"),
		},
		Redis: Redis{
			Port:     utils.GetEnvString("REDIS_PORT", "6379"),
			Password: utils.GetEnvString("REDIS_PASSWORD", "customRedisPass"),
		},
	}
}

func NewInternalConfig() *InternalConfig {
	return &InternalConfig{
		App: App{
			Env:                        utils.GetEnvString("APP_ENV", "v1.0"),
			Port:                       utils.GetEnvString("APP_PORT", ":8080"),
			Version:                    utils.GetEnvString("APP_VERSION", "v1.0"),
			Timezone:                   utils.GetEnvString("APP_TIMEZONE", "Asia/Jakarta"),
			EndpointPrefix:             utils.GetEnvString("APP_ENDPOINT_PREFIX", "/v1"),
			MaxRequests:                utils.GetEnvInt("APP_MAX_REQUEST", 10),
			ShutdownTimeout:            utils.GetEnvInt("APP_SHUTDOWN_TIMEOUT", 10),
			MaxTimeRequestsPerSeconds:  utils.GetEnvInt("APP_MAX_TIME_REQUESTS_PER_SECONDS", 10),
			RequestBodyLimitInMegabyte: utils.GetEnvInt("APP_REQUEST_BODY_LIMIT_IN_MEGABYTE", 6),
		},
		FHIR: FHIR{
			BaseUrl: utils.GetEnvString("FHIR_BASE_URL", "http://localhost:5555/fhir"),
		},
		JWT: JWT{
			Secret: utils.GetEnvString("JWT_SECRET", "anyjwt"),
		},
	}
}
