package utils

import (
	"konsulin-service/internal/pkg/constvars"
	"regexp"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("username", validateUsername)
	validate.RegisterValidation("password", validatePassword)
	validate.RegisterValidation("user_type", validateUserType)
	validate.RegisterValidation("phone_number", validatePhoneNumber)
	validate.RegisterValidation("not_past_date", validateNotPastDate)
	validate.RegisterValidation("not_past_time", validateNotPastTime)
	validate.RegisterValidation("not_future_date", validateNotFutureDate)
}

func ValidateStruct(s interface{}) error {
	return validate.Struct(s)
}

func validateUsername(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)
	return re.MatchString(fl.Field().String())
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

func validateNotPastDate(fl validator.FieldLevel) bool {
	dateStr := fl.Field().String()

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	today := time.Now().Truncate(24 * time.Hour)
	return !parsedDate.Before(today)
}

func validateNotPastTime(fl validator.FieldLevel) bool {
	timeStr := fl.Field().String()

	dateStr := fl.Parent().FieldByName("Date").String()

	today := time.Now().Format("2006-01-02")
	if dateStr != today {
		return true
	}

	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return false
	}

	now := time.Now()
	currentTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)

	return !parsedTime.Before(currentTime)
}

func validateNotFutureDate(fl validator.FieldLevel) bool {
	dateStr := fl.Field().String()

	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	today := time.Now().Truncate(24 * time.Hour)

	return !parsedDate.After(today)
}
