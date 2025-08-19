package exceptions

import "net/http"

func ErrInvalidAPIKey(err error) error {
	return BuildNewCustomError(err, http.StatusUnauthorized, "Invalid API key", ErrDevInvalidAPIKey)
}

const (
	ErrDevInvalidAPIKey = "INVALID_API_KEY"
)
