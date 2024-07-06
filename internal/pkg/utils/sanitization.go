package utils

import (
	"konsulin-service/internal/pkg/dto/requests"
	"strings"
)

func SanitizeString(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func SanitizeCreatePatientRequest(input *requests.RegisterPatient) {
	input.Email = SanitizeString(input.Email)
	input.Username = SanitizeString(input.Username)
}
