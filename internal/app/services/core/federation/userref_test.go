package federation

import (
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/konsulin-care/gateway-sdk-go/identity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userRefPepper is the fixed 32-byte pepper the user_ref tests key with —
// test-only material, never a production pepper.
var userRefPepper = []byte("0123456789abcdef0123456789abcdef")

// useFallbackPepper points the loader at the APP_PEPPER path: a data dir
// without any sealed file plus the pepper in the base64 delivery form.
// Parameters: t is the test harness, pepper is the raw 32-byte pepper.
// Returns nothing. Never panics.
func useFallbackPepper(t *testing.T, pepper []byte) {
	t.Helper()
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "")
	t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(pepper))
}

// TestUserRef_HashesNormalizedEmail covers the T31 contract assertion that
// UserRef hashes the normalized email (trim + lowercase), fails closed on a
// malformed email, and ignores the phone while an email applies.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestUserRef_HashesNormalizedEmail(t *testing.T) {
	useFallbackPepper(t, userRefPepper)

	got, err := UserRef("  Person@Example.COM  ", "+62 812-3456-7890")
	require.NoError(t, err)
	want, err := identity.HashEmail("person@example.com", userRefPepper)
	require.NoError(t, err)
	assert.Equal(t, want, got, "user_ref must be the SDK identity HMAC of the normalized email")

	plain, err := UserRef("person@example.com", "")
	require.NoError(t, err)
	assert.Equal(t, got, plain, "case and whitespace must not change the user_ref")

	_, err = UserRef("not-an-email", "")
	require.Error(t, err, "a malformed email must fail closed, never hash")
}

// TestUserRef_PhoneFallbackWhenEmailEmpty covers the T31 contract assertion
// that UserRef falls back to the phone when the email is empty.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestUserRef_PhoneFallbackWhenEmailEmpty(t *testing.T) {
	useFallbackPepper(t, userRefPepper)

	got, err := UserRef("", "+62 812-3456-7890")
	require.NoError(t, err)
	want, err := identity.HashPhone("6281234567890", userRefPepper)
	require.NoError(t, err)
	assert.Equal(t, want, got, "an empty email must hash the normalized phone instead")

	empty, err := UserRef("", "")
	require.NoError(t, err)
	assert.Equal(t, identity.NAMarker, empty, "both fields empty must stay the SDK non-applicable marker")
}

// TestUserRef_ErrorsWhenNoPepperAvailable covers the T31 contract assertion
// that UserRef errors when no pepper is available.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestUserRef_ErrorsWhenNoPepperAvailable(t *testing.T) {
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "")
	t.Setenv("APP_PEPPER", "")

	ref, err := UserRef("person@example.com", "+6281234567890")
	require.Error(t, err, "no sealed pepper and no APP_PEPPER must fail closed")
	assert.Empty(t, ref, "an error must never carry a hash")
}

// TestUserRef_EnvFallbackMatchesSealedPath covers the T31 contract assertion
// that the APP_PEPPER fallback produces the same user_ref as the sealed path
// for the same pepper (BK-PEPPER-C01): the sealed JWE holds the raw pepper,
// the env carries its base64 delivery form, both must hash identically.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestUserRef_EnvFallbackMatchesSealedPath(t *testing.T) {
	pepper := sha256.Sum256([]byte("konsulin federation pepper"))
	raw := pepper[:]
	dataDir := t.TempDir()
	key, encValue := partyEncKey(t)

	t.Setenv("APP_DATA_DIR", dataDir)
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", encValue)
	t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(raw))
	writeSealedPepper(t, dataDir, key.PublicKey(), raw)

	sealedRef, err := UserRef("Person@Example.COM", "+62 812-3456-7890")
	require.NoError(t, err)
	require.NotEmpty(t, sealedRef)

	require.NoError(t, os.Remove(filepath.Join(dataDir, sealedPepperName)))
	fallbackRef, err := UserRef("Person@Example.COM", "+62 812-3456-7890")
	require.NoError(t, err)

	assert.Equal(t, sealedRef, fallbackRef,
		"the APP_PEPPER fallback must produce the same user_ref as the sealed path for the same pepper")
}
