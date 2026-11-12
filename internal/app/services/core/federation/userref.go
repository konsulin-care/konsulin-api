package federation

import (
	"fmt"

	"github.com/konsulin-care/gateway-sdk-go/identity"
)

// UserRef returns the federation user_ref for an end user: the SDK identity
// HMAC (SDK-ID) over the normalized email, or — when the email is empty and
// therefore non-applicable — over the normalized phone. The pepper comes from
// LoadPepper (the sealed JWE first, APP_PEPPER as fallback) and is used in
// memory only; neither the pepper nor the plaintext identity is ever logged
// or persisted (BK-PEPPER-C01).
//
// Parameters: email is the end user's email; "" marks the field as
// non-applicable and hands over to phone. phone is the end user's phone,
// consulted only when email is empty.
//
// Returns: the standard base64 user_ref (or the SDK "N/A" marker when the
// chosen field is empty), or an error when no pepper is available, the pepper
// is not exactly 32 bytes, or the value is malformed — never a partial or
// unpeppered hash. Never panics.
func UserRef(email, phone string) (string, error) {
	pepper, err := LoadPepper()
	if err != nil {
		return "", fmt.Errorf("federation: user_ref: %w", err)
	}
	if email != "" {
		ref, err := identity.HashEmail(email, pepper)
		if err != nil {
			return "", fmt.Errorf("federation: user_ref: %w", err)
		}
		return ref, nil
	}
	ref, err := identity.HashPhone(phone, pepper)
	if err != nil {
		return "", fmt.Errorf("federation: user_ref: %w", err)
	}
	return ref, nil
}
