package controllers

import (
	"encoding/json"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"

	"go.uber.org/zap"
)

// requireRequestID extracts the request ID from the request context and, when
// missing or empty, logs the failure with request metadata and writes the
// standard error response. Returns the request ID and whether it was present,
// so handlers can return early when it was not.
func requireRequestID(log *zap.Logger, w http.ResponseWriter, r *http.Request) (string, bool) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		log.Error("Request ID missing from context",
			zap.String(constvars.LoggingEndpointKey, r.URL.Path),
			zap.String(constvars.LoggingMethodKey, r.Method),
			zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		)
		utils.BuildErrorResponse(log, w, exceptions.ErrMissingRequestID(nil))
		return "", false
	}
	return requestID, true
}

// decodeJSONBody decodes the request body into dest, logging contextMsg on
// failure and writing the standard parse-error response. When withErrorType is
// true the log entry carries a JSON-parsing error-type field. Returns false
// when the body could not be decoded so the caller can return early.
func decodeJSONBody(log *zap.Logger, w http.ResponseWriter, r *http.Request, requestID string, dest any, contextMsg string, withErrorType bool) bool {
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		if withErrorType {
			log.Error(contextMsg,
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
				zap.Error(err),
			)
		} else {
			log.Error(contextMsg,
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
		}
		utils.BuildErrorResponse(log, w, exceptions.ErrCannotParseJSON(err))
		return false
	}
	return true
}

// rejectRateLimited writes the standard 429 response with a Retry-After header
// when the rate limiter denied the request (allowed is false). Returns true
// when the response was written so the caller can return early.
func rejectRateLimited(log *zap.Logger, w http.ResponseWriter, allowed bool, retryAfterSecs int, errorCode string) bool {
	if allowed {
		return false
	}
	if retryAfterSecs < 0 {
		retryAfterSecs = 0
	}
	w.Header().Set(constvars.HeaderRetryAfter, fmt.Sprintf("%d", retryAfterSecs))
	utils.BuildErrorResponse(log, w, exceptions.BuildNewCustomError(nil, constvars.StatusTooManyRequests, "Too many requests", errorCode))
	return true
}
