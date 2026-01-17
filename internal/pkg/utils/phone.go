package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reDigitsOnly = regexp.MustCompile(`^\d+$`)
	reNonDigits  = regexp.MustCompile(`\D+`)
)

// NormalizePhoneDigits returns a digits-only phone string by removing *all* non-digit characters.
// It is best-effort sanitization (e.g., it will turn "62812-34567-8901" into "62812345678901").
func NormalizePhoneDigits(input string) string {
	trimmed := strings.TrimSpace(input)
	return reNonDigits.ReplaceAllString(trimmed, "")
}

// ValidateInternationalPhoneDigits enforces "international digits" (E.164 without '+'):
// - digits only
// - 10..15 digits (E.164 max is 15; min is practical to avoid local/national numbers)
// - must not start with '0' (country codes don't start with 0)
//
// NOTE: Without a leading '+' or a separate region/country hint, we cannot *prove* a country code
// is present. This validation is a pragmatic guardrail.
func ValidateInternationalPhoneDigits(phoneDigits string) error {
	digits := strings.TrimSpace(phoneDigits)
	if digits == "" {
		return fmt.Errorf("phone is required")
	}
	if !reDigitsOnly.MatchString(digits) {
		return fmt.Errorf("phone must contain digits only")
	}
	if strings.HasPrefix(digits, "0") {
		return fmt.Errorf("phone must include country code (must not start with 0)")
	}
	if len(digits) < 10 || len(digits) > 15 {
		return fmt.Errorf("phone must be 10 to 15 digits (international format without '+')")
	}
	return nil
}

// FormatE164WithPlus returns a best-effort E.164-looking phone string by adding a leading '+'
// when missing. If input is empty/blank, it returns empty string.
//
// Note: This does not validate digits or length; use ValidateInternationalPhoneDigits if needed.
func FormatE164WithPlus(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "+") {
		return s
	}
	return "+" + s
}
