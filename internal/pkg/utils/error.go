package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"strings"

	"github.com/go-playground/validator/v10"
)

func FormatAllValidationErrors(err error) string {
	var errors []string
	for _, err := range err.(validator.ValidationErrors) {
		fieldName := strings.ToLower(err.Field())
		tag := err.Tag()
		customMessage, ok := constvars.CustomValidationErrorMessages[tag]
		if !ok {
			customMessage = "is invalid"
		}
		if tag == "min" || tag == "eqfield" {
			customMessage = strings.Replace(customMessage, "%s", err.Param(), 1)
		}
		errors = append(errors, fieldName+" "+customMessage)
	}
	return strings.Join(errors, ", ")
}

func FormatFirstValidationError(err error) string {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		firstErr := validationErrors[0]
		fieldName := strings.ToLower(firstErr.Field())
		tag := firstErr.Tag()
		customMessage, ok := constvars.CustomValidationErrorMessages[tag]
		if !ok {
			customMessage = "is invalid"
		}
		if tag == "min" || tag == "eqfield" || tag == "max" {
			customMessage = strings.Replace(customMessage, "%s", firstErr.Param(), 1)
		}
		return fieldName + " " + customMessage
	}
	return constvars.ErrDevInvalidInput
}
