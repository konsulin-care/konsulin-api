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
			Port:       utils.GetEnvString("MINIO_PORT", "17017"),
			Host:       utils.GetEnvString("MINIO_HOST", "localhost"),
			Username:   utils.GetEnvString("MINIO_USERNAME", "defaultUsername"),
			Password:   utils.GetEnvString("MINIO_PASSWORD", "defaultPassword"),
			BucketName: utils.GetEnvString("MINIO_BUCKET_NAME", "defaultPassword"),
		},
	}
}

func NewInternalConfig() *InternalConfig {
	return &InternalConfig{
		App: App{
			Env:                                  utils.GetEnvString("APP_ENV", "v1.0"),
			Port:                                 utils.GetEnvString("APP_PORT", ":8080"),
			Version:                              utils.GetEnvString("APP_VERSION", "v1.0"),
			Address:                              utils.GetEnvString("APP_ADDRESS", "localhost"),
			Timezone:                             utils.GetEnvString("APP_TIMEZONE", "Asia/Jakarta"),
			EndpointPrefix:                       utils.GetEnvString("APP_ENDPOINT_PREFIX", "/v1"),
			ResetPasswordUrl:                     utils.GetEnvString("APP_RESET_PASSWORD_URL", ""),
			MailerEmailSender:                    utils.GetEnvString("APP_MAILER_EMAIL_SENDER", ""),
			RabbitMQMailerQueue:                  utils.GetEnvString("APP_RABBITMQ_MAILER_QUEUE", ""),
			RabbitMQWhatsAppQueue:                utils.GetEnvString("APP_RABBITMQ_WHATSAPP_QUEUE", ""),
			MaxRequests:                          utils.GetEnvInt("APP_MAX_REQUEST", 10),
			ShutdownTimeout:                      utils.GetEnvInt("APP_SHUTDOWN_TIMEOUT", 10),
			MaxTimeRequestsPerSeconds:            utils.GetEnvInt("APP_MAX_TIME_REQUESTS_PER_SECONDS", 10),
			RequestBodyLimitInMegabyte:           utils.GetEnvInt("APP_REQUEST_BODY_LIMIT_IN_MEGABYTE", 6),
			ForgotPasswordTokenExpTimeInMinute:   utils.GetEnvInt("APP_FORGOT_PASSWORD_TOKEN_EXP_TIME_IN_MINUTE", 2),
			MinioProfilePictureMaxUploadSizeInMB: utils.GetEnvInt64("APP_MINIO_PROFILE_PICTURE_UPLOAD_MAX_SIZE_IN_MB", 2),
		},
		FHIR: FHIR{
			BaseUrl: utils.GetEnvString("FHIR_BASE_URL", "http://localhost:5555/fhir"),
		},
		JWT: JWT{
			Secret:        utils.GetEnvString("JWT_SECRET", "anyjwt"),
			ExpTimeInHour: utils.GetEnvInt("JWT_EXP_TIME_IN_HOUR", 1),
		},
	}
}
