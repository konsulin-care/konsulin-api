package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reDigitsOnly = regexp.MustCompile(`^\d+$`)
)

// NormalizePhoneDigits trims spaces, removes all inner spaces, and strips a single leading '+'.
// It returns a digits-only string if the caller validates it.
func NormalizePhoneDigits(input string) string {
	s := strings.TrimSpace(input)
	// Best-effort: remove spaces anywhere.
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimPrefix(s, "+")
	return s
}

// ValidateInternationalPhoneDigits enforces "international digits" (E.164 without '+'):
// - digits only
// - 10..15 digits (E.164 max is 15; min is practical to avoid local/national numbers)
// - must not start with '0' (country codes don't start with 0)
//
// NOTE: Without a leading '+' or a separate region/country hint, we cannot *prove* a country code
// is present. This validation is a pragmatic guardrail.
func ValidateInternationalPhoneDigits(phoneDigits string) error {
	if strings.TrimSpace(phoneDigits) == "" {
		return fmt.Errorf("phone is required")
	}
	if !reDigitsOnly.MatchString(phoneDigits) {
		return fmt.Errorf("phone must contain digits only")
	}
	if strings.HasPrefix(phoneDigits, "0") {
		return fmt.Errorf("phone must include country code (must not start with 0)")
	}
	if len(phoneDigits) < 10 || len(phoneDigits) > 15 {
		return fmt.Errorf("phone must be 10 to 15 digits (international format without '+')")
	}
	return nil
}
