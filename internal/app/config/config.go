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
			Port:           utils.GetEnvString("MONGODB_PORT", "17017"),
			Host:           utils.GetEnvString("MONGODB_HOST", "localhost"),
			Username:       utils.GetEnvString("MONGODB_USERNAME", "defaultUsername"),
			Password:       utils.GetEnvString("MONGODB_PASSWORD", "defaultPassword"),
			FhirDbName:     utils.GetEnvString("MONGODB_FHIR_DB_NAME", "spark"),
			KonsulinDbName: utils.GetEnvString("MONGODB_KONSULIN_DB_NAME", "konsulin"),
		},
		Redis: Redis{
			Host:     utils.GetEnvString("REDIS_HOST", "localhost"),
			Port:     utils.GetEnvString("REDIS_PORT", "6379"),
			Password: utils.GetEnvString("REDIS_PASSWORD", "customRedisPass"),
		},
		Logger: Logger{
			Level:               utils.GetEnvString("LOGGER_LEVEL", "debug"),
			OutputFileName:      utils.GetEnvString("LOGGER_OUTPUT_FILENAME", "logger.log"),
			OutputErrorFileName: utils.GetEnvString("LOGGER_OUTPUT_ERROR_FILENAME", "logger_error.log"),
		},
		SMTP: SMTP{
			Host:        utils.GetEnvString("SMTP_HOST", "smtp_host"),
			Username:    utils.GetEnvString("SMTP_USERNAME", ""),
			Password:    utils.GetEnvString("SMTP_PASSWORD", ""),
			EmailSender: utils.GetEnvString("SMTP_EMAIL_SENDER", ""),
			Port:        utils.GetEnvInt("SMTP_PORT", 2525),
		},
		RabbitMQ: RabbitMQ{
			Port:     utils.GetEnvString("RABBITMQ_PORT", "17017"),
			Host:     utils.GetEnvString("RABBITMQ_HOST", "localhost"),
			Username: utils.GetEnvString("RABBITMQ_USERNAME", "defaultUsername"),
			Password: utils.GetEnvString("RABBITMQ_PASSWORD", "defaultPassword"),
		},
		Minio: Minio{
			Port:     utils.GetEnvString("MINIO_PORT", "17017"),
			Host:     utils.GetEnvString("MINIO_HOST", "localhost"),
			Username: utils.GetEnvString("MINIO_USERNAME", "defaultUsername"),
			Password: utils.GetEnvString("MINIO_PASSWORD", "defaultPassword"),
			UseSSL:   utils.GetEnvBool("MINIO_USE_SSL", false),
		},
	}
}

func NewInternalConfig() *InternalConfig {
	return &InternalConfig{
		App: App{
			Env:                                utils.GetEnvString("APP_ENV", "v1.0"),
			Port:                               utils.GetEnvString("APP_PORT", ":8080"),
			Version:                            utils.GetEnvString("APP_VERSION", "v1.0"),
			Address:                            utils.GetEnvString("APP_ADDRESS", "localhost"),
			Timezone:                           utils.GetEnvString("APP_TIMEZONE", "Asia/Jakarta"),
			EndpointPrefix:                     utils.GetEnvString("APP_ENDPOINT_PREFIX", "/v1"),
			ResetPasswordUrl:                   utils.GetEnvString("APP_RESET_PASSWORD_URL", ""),
			MaxRequests:                        utils.GetEnvInt("APP_MAX_REQUEST", 10),
			ShutdownTimeoutInSecond:            utils.GetEnvInt("APP_SHUTDOWN_TIMEOUT_IN_SECONDS", 10),
			MaxTimeRequestsPerSeconds:          utils.GetEnvInt("APP_MAX_TIME_REQUESTS_PER_SECONDS", 10),
			RequestBodyLimitInMegabyte:         utils.GetEnvInt("APP_REQUEST_BODY_LIMIT_IN_MEGABYTE", 6),
			ForgotPasswordTokenExpTimeInMinute: utils.GetEnvInt("APP_FORGOT_PASSWORD_TOKEN_EXP_TIME_IN_MINUTE", 2),
		},
		FHIR: AppFHIR{
			BaseUrl: utils.GetEnvString("APP_FHIR_BASE_URL", "http://localhost:5555/fhir"),
		},
		JWT: AppJWT{
			Secret:        utils.GetEnvString("APP_JWT_SECRET", "anyjwt"),
			ExpTimeInHour: utils.GetEnvInt("APP_JWT_EXP_TIME_IN_HOUR", 1),
		},
		Mailer: AppMailer{
			EmailSender: utils.GetEnvString("APP_MAILER_EMAIL_SENDER", ""),
		},
		Minio: AppMinio{
			BucketName:                      utils.GetEnvString("APP_MINIO_BUCKET_NAME", "konsulin-dev"),
			ProfilePictureMaxUploadSizeInMB: utils.GetEnvInt64("APP_MINIO_PROFILE_PICTURE_UPLOAD_MAX_SIZE_IN_MB", 2),
		},
		RabbitMQ: AppRabbitMQ{
			MailerQueue:   utils.GetEnvString("APP_RABBITMQ_MAILER_QUEUE", ""),
			WhatsAppQueue: utils.GetEnvString("APP_RABBITMQ_WHATSAPP_QUEUE", ""),
		},
		MongoDB: AppMongoDB{
			FhirDBName:     utils.GetEnvString("APP_MONGODB_FHIR_DB_NAME", "spark"),
			KonsulinDBName: utils.GetEnvString("APP_MONGODB_KONSULIN_DB_NAME", "konsulin"),
		},
	}
}
