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
	validate.RegisterValidation("user_type", validateUserType)
	validate.RegisterValidation("phone_number", validatePhoneNumber)
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

func validateUserType(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return value == "practitioner" || value == "patient"
}

func validatePhoneNumber(fl validator.FieldLevel) bool {
	phoneNumber := fl.Field().String()
	re := regexp.MustCompile(`^\+[1-9]\d{9,14}$`)
	return re.MatchString(phoneNumber)
}
