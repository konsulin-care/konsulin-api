package constvars

// Validation messages mapper
var CustomValidationErrorMessages = map[string]string{
	"required":             "is required",
	"email":                "must be a valid email",
	"alphanum":             "must contain only alphanumeric characters",
	"min":                  "must be at least %s characters long",
	"max":                  "maximum at %s characters long",
	"eqfield":              "must match %s",
	"password":             "must be at least 8 characters long, contain at least one special character, and one uppercase letter",
	"numeric":              "must be a number",
	"len":                  "must be %s characters long",
	"oneof":                "must be one of [%s]",
	"gt":                   "must be greater than %s",
	"gte":                  "must be greater than or equal to %s",
	"lt":                   "must be less than %s",
	"lte":                  "must be less than or equal to %s",
	"url":                  "must be a valid URL",
	"uuid":                 "must be a valid UUID",
	"file":                 "must be a valid file",
	"base64":               "must be a valid base64 string",
	"excludes":             "must not contain %s",
	"excludesall":          "must not contain any of [%s]",
	"excludesrune":         "must not contain the rune %s",
	"required_if":          "is required when %s is %s",
	"required_unless":      "is required unless %s is %s",
	"required_with":        "is required when %s is present",
	"required_with_all":    "is required when all of [%s] are present",
	"required_without":     "is required when %s is not present",
	"required_without_all": "is required when none of [%s] are present",
	"user_type":            "must be either 'practitioner' or 'patient'",
}

// Tags that require parameter substitution
var TagsWithParams = map[string]bool{
	"min":                  true,
	"max":                  true,
	"len":                  true,
	"eqfield":              true,
	"gt":                   true,
	"gte":                  true,
	"lt":                   true,
	"lte":                  true,
	"excludes":             true,
	"user_type":            true,
	"oneof":                true,
	"excludesrune":         true,
	"required_if":          true,
	"required_unless":      true,
	"required_with":        true,
	"required_with_all":    true,
	"required_without":     true,
	"required_without_all": true,
}

// Error messages for clients
const (
	ErrClientPasswordsDoNotMatch           = "passwords do not match"
	ErrClientEmailAlreadyExists            = "email already used"
	ErrClientUsernameAlreadyExists         = "username already used"
	ErrClientCannotProcessRequest          = "failed to process your request"
	ErrClientInvalidUsernameOrPassword     = "invalid username or password"
	ErrClientInvalidImageFormat            = "the image you uploaded does not meet the specified standards"
	ErrClientSomethingWrongWithApplication = "there is something wrong with the application"
	ErrClientServerLongRespond             = "the app taking too long to respond"
	ErrClientNotAuthorized                 = "you can't access this feature"
	ErrClientNotLoggedIn                   = "your session ended, please login again"
	ErrClientResetPasswordToken            = "your reset password request already expired"
)

// Error messages for developers
const (
	ErrDevInvalidInput                  = "invalid input"
	ErrDevCannotParseJSON               = "cannot parse JSON into struct or other data types"
	ErrDevCannotParseTime               = "cannot parse time into the given format"
	ErrDevCannotMarshalJSON             = "cannot convert struct or other data types to JSON"
	ErrDevInvalidFormat                 = "invalid %s format"
	ErrDevCannotParseMultipartForm      = "cannot parse multipart form body"
	ErrDevCannotParsedate               = "cannot parse the requested date"
	ErrDevBuildRequest                  = "encountering error while building request DTO"
	ErrDevInvalidRoleType               = "invalid role type, should be 'practitioner' or 'patient'"
	ErrDevRoleTypeDoesntMatch           = "invalid role type, request done by user with different type"
	ErrDevFailedToCreateUser            = "failed to create user"
	ErrDevFailedToHashPassword          = "failed to hash password"
	ErrDevDocumentNotFound              = "document not found"
	ErrDevInvalidCredentials            = "invalid credentials"
	ErrDevUnauthorized                  = "unauthorized access"
	ErrDevCreateHTTPRequest             = "failed to create HTTP request"
	ErrDevSendHTTPRequest               = "failed to send HTTP request"
	ErrDevAccountDeactivationAgeExpired = "Account is no longer on the system and is in the process of being removed completely"

	// SMTP
	ErrDevSMTPSendEmail = "failed to send email via SMTP client hostname %s"

	// Usecase messages
	ErrDevPasswordsDoNotMatch   = "passwords do not match"
	ErrDevEmailAlreadyExists    = "email already exists"
	ErrDevUsernameAlreadyExists = "username already exists"
	ErrDevUserNotExists         = "user not exists in our system"

	// Spark messages
	ErrDevSparkCreateFHIRResource                 = "failed to create FHIR %s from `BLAZE` service"
	ErrDevSparkUpdateFHIRResource                 = "failed to update FHIR %s from `BLAZE` service"
	ErrDevSparkGetFHIRResource                    = "failed to get FHIR %s from `BLAZE` service"
	ErrDevSparkNoDataFHIRResource                 = "no data found from FHIR %s"
	ErrDevSparkFetchedResultNotUniqueFHIRResource = "result fetched for %s response contain more than 1 data (not unique)"
	ErrDevSparkGetFHIRResourceDuplicate           = "got more than one document when get FHIR %s from `BLAZE` service, which should be unique and contain only one result"
	ErrDevSparkDecodeFHIRResourceResponse         = "failed to decode FHIR %s response from `BLAZE` service"

	// Validation messages
	ErrDevValidationFailed           = "validation failed"
	ErrDevImageValidationFailed      = "image validation failed"
	ErrDevInvalidRequestPayload      = "invalid request payload"
	ErrDevMissingRequiredFields      = "missing required fields"
	ErrDevURLParamIDValidationFailed = "parameter %s validation failed"

	// Authentication messages
	ErrDevAuthSigningMethod         = "unexpected signing method"
	ErrDevAuthTokenInvalidOrExpired = "invalid or expired token"
	ErrDevAuthTokenExpired          = "token expired"
	ErrDevAuthTokenMissing          = "token missing"
	ErrDevAuthInvalidSession        = "invalid session"
	ErrDevAuthPermissionDenied      = "permission denied"
	ErrDevAuthGenerateToken         = "failed to generate token"
	ErrDevAuthRoleNotExists         = "role doesn't exist on the system"

	// Database messages
	ErrDevDBFailedToInsertDocument   = "failed to insert document into database"
	ErrDevDBFailedToUpdateDocument   = "failed to update document into database"
	ErrDevDBFailedToFindDocument     = "failed when do find document on database"
	ErrDevDBFailedToDeleteDocument   = "failed when do delete document on database"
	ErrDevDBFailedToIterateDocuments = "failed when iterating documents from database"
	ErrDevDBConnectionFailed         = "failed to connect to database"
	ErrDevDBOperationFailed          = "database operation failed"
	ErrDevDBStringNotObjectID        = "given ID is not valid object ID"

	// Minio messages
	ErrDevMinioFailedToCreateObject          = "failed to create object into minio storage with bucket name '%s'"
	ErrDevMinioFailedToGetObjectPresignedURL = "failed to get object URL from minio storage with bucket name '%s'"

	// Redis messages
	ErrDevRedisSetData         = "failed to SET data into redis"
	ErrDevRedisGetData         = "failed to GET data from redis"
	ErrDevRedisGetNoData       = "failed to GET data from redis, there is no data associated with key %s"
	ErrDevRedisDeleteData      = "failed to DELETE data from redis"
	ErrDevRedisIncrementValue  = "failed to INCR data in redis"
	ErrDevRedisRightPushToList = "failed to RPUSH data into list in redis"
	ErrDevRedisLeftPopList     = "failed to LPOP data from list in redis"
	ErrDevRedisSAdd            = "failed to SAdd data into set in redis"
	ErrDevRedisSMembers        = "failed to SMembers data from set in redis"

	// Server messages
	ErrDevServerProcess          = "server failed to process something related to machine system"
	ErrDevServerInternalError    = "internal server error"
	ErrDevServerNotImplemented   = "feature not implemented"
	ErrDevServerBadRequest       = "bad request"
	ErrDevServerNotFound         = "resource not found"
	ErrDevServerDeadlineExceeded = "deadline exceeded"
	ErrDevServerParseSessionData = "failed to parse session data"

	// File handling messages
	ErrDevFileUploadSuccess = "file uploaded successfully"
	ErrDevFileUploadFailed  = "file upload failed"
	ErrDevFileNotFound      = "file not found"
	ErrDevFileInvalidType   = "invalid file type"

	// Miscellaneous messages
	ErrDevActionNotAllowed     = "action not allowed"
	ErrDevServiceUnavailable   = "service temporarily unavailable"
	ErrDevOperationTimedOut    = "operation timed out"
	ErrDevRequestLimitExceeded = "request limit exceeded"
)

const (
	ErrFileLocationUnknown = "file location unknown"
	ErrLineLocationUnknown = "line location unknown"
	ErrFunctionNameUnknown = "function name unknown"
)

const (
	ErrEnvParsing     = "Error parsing %s: %v, will use default value"
	ErrEnvKeyNotExist = "Error getting env key: %s, will use default value"
)
