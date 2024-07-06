package constvars

// Validation messages for users, map it with respective tag field
var CustomValidationErrorMessages = map[string]string{
	"required": "is required",
	"email":    "must be a valid email",
	"alphanum": "must contain only alphanumeric characters",
	"min":      "must be at least %s characters long",
	"max":      "maximum at %s characters long",
	"eqfield":  "must match %s",
	"password": "must be at least 8 characters long, contain at least one special character, and one uppercase letter",
}

// Error messages for clients
const (
	ErrClientPasswordsDoNotMatch           = "passwords do not match"
	ErrClientEmailAlreadyExists            = "email already used"
	ErrClientUsernameAlreadyExists         = "username already used"
	ErrClientCannotProcessRequest          = "failed to process your request"
	ErrClientInvalidUsernameOrPassword     = "invalid username or password"
	ErrClientSomethingWrongWithApplication = "there is something wrong with the application"
	ErrClientServerLongRespond             = "the app taking too long to respond"
	ErrClientNotAuthorized                 = "you can't access this feature"
	ErrClientNotLoggedIn                   = "your session ended, please login again"
)

// Error messages for developers
const (
	ErrDevInvalidInput         = "invalid input"
	ErrDevCannotParseJSON      = "cannot parse JSON"
	ErrDevFailedToCreateUser   = "failed to create user"
	ErrDevFailedToHashPassword = "failed to hash password"
	ErrDevDocumentNotFound     = "document not found"
	ErrDevInvalidCredentials   = "invalid credentials"
	ErrDevUnauthorized         = "unauthorized access"
	ErrDevCreateHTTPRequest    = "failed to create HTTP request"
	ErrDevSendHTTPRequest      = "failed to send HTTP request"

	// Usecase messages
	ErrDevPasswordsDoNotMatch   = "passwords do not match"
	ErrDevEmailAlreadyExists    = "email already exists"
	ErrDevUsernameAlreadyExists = "username already exists"

	// Spark messages
	ErrDevSparkCreateFHIRPatient  = "failed to create FHIR patient from firesly spark"
	ErrDevSparkUpdateFHIRPatient  = "failed to update FHIR patient from firesly spark"
	ErrDevSparkGetFHIRPatient     = "failed to get FHIR patient from firely spark"
	ErrDevSparkDecodeFHIRResponse = "Failed to decode FHIR response from firely spark"

	// Validation messages
	ErrDevValidationFailed      = "validation failed"
	ErrDevInvalidRequestPayload = "invalid request payload"
	ErrDevMissingRequiredFields = "missing required fields"

	// Authentication messages
	ErrDevAuthSigningMethod    = "Unexpected signing method"
	ErrDevAuthTokenInvalid     = "invalid token"
	ErrDevAuthTokenExpired     = "token expired"
	ErrDevAuthTokenMissing     = "token missing"
	ErrDevAuthInvalidSession   = "invalid session"
	ErrDevAuthPermissionDenied = "permission denied"
	ErrDevAuthGenerateToken    = "Failed to generate token"

	// Database messages
	ErrDevDBFailedToInsertDocument = "failed to insert document into database"
	ErrDevDBFailedToUpdateDocument = "failed to update document into database"
	ErrDevDBFailedToFindDocument   = "failed when do find document on database"
	ErrDevDBConnectionFailed       = "failed to connect to database"
	ErrDevDBOperationFailed        = "database operation failed"
	ErrDevDBStringNotObjectID      = "given ID is not valid object ID"

	// Redis messages
	ErrDevRedisStoreSession = "Failed to store session data into redis"

	// Server messages
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
