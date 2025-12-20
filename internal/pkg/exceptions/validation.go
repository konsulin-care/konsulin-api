package exceptions

import (
	"konsulin-service/internal/pkg/constvars"
	"strings"

	"github.com/go-playground/validator/v10"
)

func FormatAllValidationErrors(err error) string {
	if err == nil {
		return constvars.ErrClientCannotProcessRequest
	}

	var errors []string
	for _, err := range err.(validator.ValidationErrors) {
		fieldName := strings.ToLower(err.Field())
		tag := err.Tag()
		customMessage, ok := constvars.CustomValidationErrorMessages[tag]
		if !ok {
			customMessage = "is invalid"
		}
		if constvars.TagsWithParams[tag] {
			if tag == "oneof" {
				customMessage = strings.Replace(customMessage, "%s", strings.Join(strings.Fields(err.Param()), ", "), 1)
			} else {
				customMessage = strings.Replace(customMessage, "%s", err.Param(), 1)
			}
		}
		errors = append(errors, fieldName+" "+customMessage)
	}
	return strings.Join(errors, ", ")
}

func FormatFirstValidationError(err error) string {
	if err == nil {
		return constvars.ErrClientCannotProcessRequest
	}

	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		firstErr := validationErrors[0]
		fieldName := strings.ToLower(firstErr.Field())
		tag := firstErr.Tag()
		customMessage, ok := constvars.CustomValidationErrorMessages[tag]
		if tag == "phone_number" || tag == "not_past_date" || tag == "not_past_time" {
			return customMessage
		}
		if !ok {
			customMessage = "is invalid"
		}

		if constvars.TagsWithParams[tag] {
			if tag == "oneof" {
				customMessage = strings.Replace(customMessage, "%s", strings.Join(strings.Fields(firstErr.Param()), ", "), 1)
			} else {
				customMessage = strings.Replace(customMessage, "%s", firstErr.Param(), 1)
			}
		}
		return fieldName + " " + customMessage
	}
	return constvars.ErrDevInvalidInput
}
