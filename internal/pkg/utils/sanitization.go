package utils

import (
	"konsulin-service/internal/pkg/dto/requests"
	"strings"
	"unicode"
)

func cleanWhiteSpaceFromEachStringOfAnArray(input []string) []string {
	sanitizedArray := make([]string, len(input))
	for i, v := range input {
		sanitizedArray[i] = strings.TrimSpace(v)
	}
	return sanitizedArray
}

func capitalize(input string) string {
	if len(input) == 0 {
		return input
	}
	runes := []rune(input)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

func SanitizeRegisterUserRequest(input *requests.RegisterUser) {
	input.Email = strings.TrimSpace(input.Email)
	input.Username = strings.TrimSpace(input.Username)
	input.Password = strings.TrimSpace(input.Password)
	input.RetypePassword = strings.TrimSpace(input.RetypePassword)
}

func SanitizeUpdateProfileRequest(input *requests.UpdateProfile) {
	input.Email = strings.TrimSpace(input.Email)
	input.Gender = strings.TrimSpace(input.Gender)
	input.Address = strings.TrimSpace(input.Address)
	input.Fullname = strings.TrimSpace(input.Fullname)
	input.BirthDate = strings.TrimSpace(input.BirthDate)
	input.WhatsAppNumber = strings.TrimSpace(input.WhatsAppNumber)

	input.Educations = cleanWhiteSpaceFromEachStringOfAnArray(input.Educations)
}

func SanitizeCreateMagicLinkRequest(input *requests.SupertokenPasswordlessCreateMagicLink) {
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	input.RedirectToPath = strings.TrimSpace(input.RedirectToPath)

	input.Roles = cleanWhiteSpaceFromEachStringOfAnArray(input.Roles)
}
