package exceptions

import "net/http"

func ErrInvalidAPIKey(err error) error {
	return BuildNewCustomError(err, http.StatusUnauthorized, "Invalid API key", ErrDevInvalidAPIKey)
}

func ErrAPIKeyRequired(err error) error {
	return BuildNewCustomError(err, http.StatusUnauthorized, "API key is required", ErrDevAPIKeyRequired)
}

const (
	ErrDevInvalidAPIKey  = "INVALID_API_KEY"
	ErrDevAPIKeyRequired = "API_KEY_REQUIRED"
)
