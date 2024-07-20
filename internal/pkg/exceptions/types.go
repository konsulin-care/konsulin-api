package exceptions

import (
	"fmt"
	"konsulin-service/internal/pkg/constvars"
)

var (
	ErrInputValidation = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, FormatFirstValidationError(err), constvars.ErrDevValidationFailed)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevValidationFailed)
	}
	ErrHashPassword = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevFailedToHashPassword)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevFailedToHashPassword)
	}
	ErrCannotParseJSON = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCannotParseJSON)
	}
	ErrCannotMarshalJSON = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevCannotMarshalJSON)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevCannotMarshalJSON)
	}
	ErrServerDeadlineExceeded = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
		}
		return WrapWithoutError(constvars.StatusRequestTimeout, constvars.ErrClientServerLongRespond, constvars.ErrDevServerDeadlineExceeded)
	}
	ErrInvalidUsernameOrPassword = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusUnauthorized, constvars.ErrClientInvalidUsernameOrPassword, constvars.ErrDevInvalidCredentials)
		}
		return WrapWithoutError(constvars.StatusUnauthorized, constvars.ErrClientInvalidUsernameOrPassword, constvars.ErrDevInvalidCredentials)
	}
	ErrPasswordDoNotMatch = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientPasswordsDoNotMatch, constvars.ErrDevPasswordsDoNotMatch)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientPasswordsDoNotMatch, constvars.ErrDevPasswordsDoNotMatch)
	}
	ErrEmailAlreadyExist = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientEmailAlreadyExists, constvars.ErrDevEmailAlreadyExists)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientEmailAlreadyExists, constvars.ErrDevEmailAlreadyExists)
	}
	ErrUsernameAlreadyExist = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusNotFound, constvars.ErrClientUsernameAlreadyExists, constvars.ErrDevUsernameAlreadyExists)
		}
		return WrapWithoutError(constvars.StatusNotFound, constvars.ErrClientUsernameAlreadyExists, constvars.ErrDevUsernameAlreadyExists)
	}
	ErrUserNotExist = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevUserNotExists)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevUserNotExists)
	}
	ErrTokenMissing = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusUnauthorized, constvars.ErrClientNotAuthorized, constvars.ErrDevAuthTokenMissing)
		}
		return WrapWithoutError(constvars.StatusUnauthorized, constvars.ErrClientNotAuthorized, constvars.ErrDevAuthTokenMissing)
	}
	ErrTokenGenerate = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthGenerateToken)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthGenerateToken)
	}
	ErrTokenSigningMethod = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthSigningMethod)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevAuthSigningMethod)
	}
	ErrTokenInvalid = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusUnauthorized, constvars.ErrClientNotLoggedIn, constvars.ErrDevAuthTokenInvalid)
		}
		return WrapWithoutError(constvars.StatusUnauthorized, constvars.ErrClientNotLoggedIn, constvars.ErrDevAuthTokenInvalid)
	}
	ErrInvalidRoleType = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevInvalidRoleType)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, constvars.ErrDevInvalidRoleType)
	}
	ErrNotMatchRoleType = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, constvars.ErrDevRoleTypeDoesntMatch)
		}
		return WrapWithoutError(constvars.StatusForbidden, constvars.ErrClientNotAuthorized, constvars.ErrDevRoleTypeDoesntMatch)
	}

	// Auth
	ErrAuthInvalidRole = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusBadRequest, constvars.ErrClientNotAuthorized, constvars.ErrDevRoleTypeDoesntMatch)
		}
		return WrapWithoutError(constvars.StatusBadRequest, constvars.ErrClientNotAuthorized, constvars.ErrDevRoleTypeDoesntMatch)
	}

	// Mongo DB
	ErrMongoDBFindDocument = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevDBFailedToFindDocument)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevDBFailedToFindDocument)
	}
	ErrMongoDBDeleteDocument = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevDBFailedToDeleteDocument)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevDBFailedToDeleteDocument)
	}
	ErrMongoDBIterateDocuments = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToIterateDocuments)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToIterateDocuments)
	}
	ErrMongoDBNotObjectID = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBStringNotObjectID)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBStringNotObjectID)
	}
	ErrMongoDBUpdateDocument = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToUpdateDocument)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToUpdateDocument)
	}
	ErrMongoDBInsertDocument = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToInsertDocument)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevDBFailedToInsertDocument)
	}

	// Redis
	ErrRedisDelete = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisDeleteData)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisDeleteData)
	}
	ErrRedisGet = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisGetData)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisGetData)
	}
	ErrRedisSet = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSetData)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSetData)
	}
	ErrRedisIncrement = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisIncrementValue)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisIncrementValue)
	}
	ErrRedisPushToList = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisRightPushToList)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisRightPushToList)
	}
	ErrRedisPopFromList = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisLeftPopList)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisLeftPopList)
	}
	ErrRedisAddToSet = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSAdd)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSAdd)
	}
	ErrRedisGetSetMembers = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSMembers)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, constvars.ErrDevRedisSMembers)
	}

	// HTTP
	ErrCreateHTTPRequest = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCreateHTTPRequest)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevCreateHTTPRequest)
	}
	ErrSendHTTPRequest = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevSendHTTPRequest)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevSendHTTPRequest)
	}

	// SMTP
	ErrSMTPSendEmail = func(err error, hostname string) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, fmt.Sprintf(constvars.ErrDevSMTPSendEmail, hostname))
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, fmt.Sprintf(constvars.ErrDevSMTPSendEmail, hostname))
	}

	// FHIR
	ErrCreateFHIRResource = func(err error, resource string) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkCreateFHIRResource, resource))
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkCreateFHIRResource, resource))
	}
	ErrGetFHIRResource = func(err error, resource string) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkGetFHIRResource, resource))
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkGetFHIRResource, resource))
	}
	ErrUpdateFHIRResource = func(err error, resource string) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkUpdateFHIRResource, resource))
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkUpdateFHIRResource, resource))
	}
	ErrDecodeResponse = func(err error, resource string) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkDecodeFHIRResourceResponse, resource))
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, fmt.Sprintf(constvars.ErrDevSparkDecodeFHIRResourceResponse, resource))
	}

	// Deault Server
	ErrServerProcess = func(err error) *CustomError {
		if err != nil {
			return WrapWithError(err, constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevServerProcess)
		}
		return WrapWithoutError(constvars.StatusInternalServerError, constvars.ErrClientCannotProcessRequest, constvars.ErrDevServerProcess)
	}
)
