package utils

import (
	"konsulin-service/internal/pkg/dto/requests"
	"strings"
)

func SanitizeString(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func SanitizeRegisterUserRequest(input *requests.RegisterUser) {
	input.Email = SanitizeString(input.Email)
	input.Username = SanitizeString(input.Username)
	input.UserType = SanitizeString(input.UserType)
}
func SanitizeLoginUserRequest(input *requests.LoginUser) {
	input.UserType = SanitizeString(input.UserType)
}
