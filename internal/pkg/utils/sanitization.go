package utils

import (
	"konsulin-service/internal/pkg/dto/requests"
	"strings"
)

func sanitizeString(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func sanitizeStringArray(input []string) []string {
	sanitizedArray := make([]string, len(input))
	for i, v := range input {
		sanitizedArray[i] = strings.TrimSpace(v)
	}
	return sanitizedArray
}

func SanitizeRegisterUserRequest(input *requests.RegisterUser) {
	input.Email = sanitizeString(input.Email)
	input.Username = sanitizeString(input.Username)
	input.Password = sanitizeString(input.Password)
	input.RetypePassword = sanitizeString(input.RetypePassword)
}

func SanitizeUpdateProfileRequest(input *requests.UpdateProfile) {
	input.Email = sanitizeString(input.Email)
	input.Gender = sanitizeString(input.Gender)
	input.Address = sanitizeString(input.Address)
	input.Fullname = sanitizeString(input.Fullname)
	input.BirthDate = sanitizeString(input.BirthDate)
	input.WhatsAppNumber = sanitizeString(input.WhatsAppNumber)
	input.ProfilePictureName = sanitizeString(input.ProfilePictureName)

	input.Educations = sanitizeStringArray(input.Educations)
}
