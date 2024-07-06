package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"regexp"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("password", validatePassword)
}

func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	hasMinLen := len(password) >= 8
	hasSpecialChar := regexp.MustCompile(constvars.RegexContainAtLeastOneSpecialChar).MatchString(password)
	hasUppercase := regexp.MustCompile(constvars.RegexContainAtLeastOneUppercase).MatchString(password)
	return hasMinLen && hasSpecialChar && hasUppercase
}
