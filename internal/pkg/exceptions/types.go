package exceptions

import (
	"fmt"
	"konsulin-service/internal/pkg/constvars"
)

var (
	ErrURLParamIDValidation = func(err error, paramName string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevURLParamIDValidationFailed, paramName))
	}
	ErrImageValidation = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientInvalidImageFormat, constvars.ErrDevImageValidationFailed)
	}
	ErrInputValidation = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, FormatFirstValidationError(err), constvars.ErrDevValidationFailed)
	}
	ErrHashPassword = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevFailedToHashPassword)
	}
	ErrBuildRequest = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevBuildRequest)
	}
	ErrCannotParseMultipartForm = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseMultipartForm)
	}
	ErrCannotParseDate = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseMultipartForm)
	}
	ErrInvalidFormat = func(err error, source string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevInvalidFormat, source))
	}
	ErrCannotMarshalJSON = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevCannotMarshalJSON)
	}
	ErrServerDeadlineExceeded = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusGatewayTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
	}
	ErrInvalidUsernameOrPassword = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusUnauthorized, constvars.ErrClientInvalidUsernameOrPassword, constvars.ErrDevInvalidCredentials)
	}
	ErrAccountDeactivationAgeExpired = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusUnauthorized, constvars.ErrClientInvalidUsernameOrPassword, constvars.ErrDevAccountDeactivationAgeExpired)
	}
	ErrPasswordDoNotMatch = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientPasswordsDoNotMatch, constvars.ErrDevPasswordsDoNotMatch)
	}
	ErrEmailAlreadyExist = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientEmailAlreadyExists, constvars.ErrDevEmailAlreadyExists)
	}
	ErrPhoneNumberAlreadyRegistered = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientPhoneNumberAlreadyRegistered, constvars.ErrDevPhoneNumberAlreadyRegistered)
	}
	ErrUsernameAlreadyExist = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientUsernameAlreadyExists, constvars.ErrDevUsernameAlreadyExists)
	}
	ErrUserNotExist = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusNotFound, constvars.ErrClientCannotProcessRequest, constvars.ErrDevUserNotExists)
	}
	ErrTokenMissing = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusUnauthorized, constvars.ErrClientNotAuthorized, constvars.ErrDevAuthTokenMissing)
	}
	ErrTokenGenerate = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthGenerateToken)
	}
	ErrTokenInvalidOrExpired = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusUnauthorized, constvars.ErrClientNotLoggedIn, constvars.ErrDevAuthTokenInvalidOrExpired)
	}
	ErrTokenResetPasswordExpired = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusGone, constvars.ErrClientResetPasswordTokenExpired, constvars.ErrDevAuthTokenExpired)
	}
	ErrWhatsAppOTPExpired = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusGone, constvars.ErrClientWhatsAppOTPExpired, constvars.ErrDevAuthWhatsAppOTPExpired)
	}
	ErrWhatsAppOTPInvalid = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientWhatsAppOTPInvalid, constvars.ErrDevAuthWhatsAppOTPInvalid)
	}
	ErrInvalidRoleType = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevInvalidRoleType)
	}
	ErrNotMatchRoleType = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, constvars.ErrDevRoleTypeDoesntMatch)
	}

	// Parse
	ErrCannotParseJSON = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
	}
	ErrCannotParseTime = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
	}

	// Auth
	ErrAuthInvalidRole = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientNotAuthorized, constvars.ErrDevRoleTypeDoesntMatch)
	}

	// Mongo DB
	ErrMongoDBFindDocument = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToFindDocument)
	}
	ErrMongoDBDeleteDocument = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToDeleteDocument)
	}
	ErrMongoDBIterateDocuments = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToIterateDocuments)
	}
	ErrMongoDBNotObjectID = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBStringNotObjectID)
	}
	ErrMongoDBUpdateDocument = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToUpdateDocument)
	}
	ErrMongoDBInsertDocument = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToInsertDocument)
	}

	// Postgres DB
	ErrPostgresDBFindData = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToFindData)
	}
	ErrPostgresDBDeleteData = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToDeleteData)
	}
	ErrPostgresDBIterateDataset = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToIterateDataset)
	}
	ErrPostgresDBUpdateData = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToUpdateData)
	}
	ErrPostgresDBInsertData = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToInsertData)
	}

	// Minio
	ErrMinioCreateObject = func(err error, bucketName string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, fmt.Sprintf(constvars.ErrDevMinioFailedToCreateObject, bucketName))
	}
	ErrMinioFindObjectPresignedURL = func(err error, bucketName string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, fmt.Sprintf(constvars.ErrDevMinioFailedToCreateObject, bucketName))
	}

	// Redis
	ErrRedisGetNoData = func(err error, redisKey string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, fmt.Sprintf(constvars.ErrDevRedisGetNoData, redisKey))
	}
	ErrRedisDelete = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisDeleteData)
	}
	ErrRedisGet = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisGetData)
	}
	ErrRedisSet = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSetData)
	}
	ErrRedisIncrement = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisIncrementValue)
	}
	ErrRedisPushToList = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisRightPushToList)
	}
	ErrRedisPopFromList = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisLeftPopList)
	}
	ErrRedisAddToSet = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSAdd)
	}
	ErrRedisGetSetMembers = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSMembers)
	}

	// RabbitMQ
	ErrRabbitMQPublishMessage = func(err error, queueName string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSMembers)
	}

	// HTTP
	ErrCreateHTTPRequest = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCreateHTTPRequest)
	}
	ErrSendHTTPRequest = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevSendHTTPRequest)
	}

	// SMTP
	ErrSMTPSendEmail = func(err error, hostname string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, fmt.Sprintf(constvars.ErrDevSMTPSendEmail, hostname))
	}

	// FHIR
	ErrCreateFHIRResource = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkCreateFHIRResource, resource))
	}
	ErrGetFHIRResource = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkGetFHIRResource, resource))
	}
	ErrNoDataFHIRResource = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkNoDataFHIRResource, resource))
	}
	ErrGetFHIRResourceDuplicate = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkGetFHIRResourceDuplicate, resource))
	}
	ErrUpdateFHIRResource = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkUpdateFHIRResource, resource))
	}
	ErrResultFetchedNotUniqueFhirResource = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkFetchedResultNotUniqueFHIRResource, resource))
	}

	ErrDecodeResponse = func(err error, resource string) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkDecodeFHIRResourceResponse, resource))
	}

	// Default Server
	ErrServerProcess = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevServerProcess)
	}

	ErrClientCustomMessage = func(err error) *CustomError {
		return BuildNewCustomError(err, constvars.StatusBadRequest, err.Error(), constvars.ErrDevServerProcess)
	}
)
