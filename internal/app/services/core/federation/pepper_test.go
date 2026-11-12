package federation

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sealedPepperName pins the artifact path the contract names: the sealed JWE
// lives at <APP_DATA_DIR>/federation-pepper.jwe.
const sealedPepperName = "federation-pepper.jwe"

// encSeed is the deterministic 32-byte seed the tests derive the party
// use:enc key from (test-only material, never a production key).
var encSeed = bytes.Repeat([]byte{0x42}, 32)

// partyEncKey builds the party's X25519 use:enc private key and the base64
// env value APP_PARTY_ENC_PRIVATE_KEY carries.
// Parameters: t is the test harness.
// Returns: the key and its env encoding. Never panics.
func partyEncKey(t *testing.T) (*ecdh.PrivateKey, string) {
	t.Helper()
	key, err := ecdh.X25519().NewPrivateKey(encSeed)
	require.NoError(t, err)
	return key, base64.StdEncoding.EncodeToString(encSeed)
}

// writeSealedPepper stores the compact JWE of pepper sealed to the party's
// use:enc key at <dataDir>/federation-pepper.jwe, exactly as the
// federation-pepper fetch would.
// Parameters: t is the test harness, dataDir is the APP_DATA_DIR stand-in,
// recipient is the party's X25519 public key, pepper is the plaintext to
// seal. Returns nothing. Never panics.
func writeSealedPepper(t *testing.T, dataDir string, recipient *ecdh.PublicKey, pepper []byte) {
	t.Helper()
	sealed, err := envelope.Seal(pepper, "test-enc-kid", recipient)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, sealedPepperName), []byte(sealed), 0o600))
}

// TestLoadPepper_SealedFileTakesPrecedence covers the T31 contract assertion
// that a sealed pepper file opened with the party enc key wins over the
// APP_PEPPER fallback.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestLoadPepper_SealedFileTakesPrecedence(t *testing.T) {
	dataDir := t.TempDir()
	key, encValue := partyEncKey(t)
	sealedPepper := bytes.Repeat([]byte{0x01}, 32)
	fallback := bytes.Repeat([]byte{0x02}, 32)

	t.Setenv("APP_DATA_DIR", dataDir)
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", encValue)
	t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(fallback))
	writeSealedPepper(t, dataDir, key.PublicKey(), sealedPepper)

	got, err := LoadPepper()
	require.NoError(t, err, "a sealed pepper openable with the party key must load")
	assert.Equal(t, sealedPepper, got, "the sealed JWE must take precedence over APP_PEPPER")
	assert.NotEqual(t, fallback, got, "the APP_PEPPER decoy must not leak into the loaded pepper")
}

// TestLoadPepper_EnvFallbackWithoutSealedFile covers the APP_PEPPER
// manual-delivery fallback (both delivery forms) and the fail-closed empty
// case.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestLoadPepper_EnvFallbackWithoutSealedFile(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("APP_DATA_DIR", dataDir)
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", "")

	t.Run("raw 32-byte value", func(t *testing.T) {
		raw := []byte("0123456789abcdef0123456789abcdef")
		t.Setenv("APP_PEPPER", string(raw))
		got, err := LoadPepper()
		require.NoError(t, err)
		assert.Equal(t, raw, got)
	})

	t.Run("standard base64 delivery form", func(t *testing.T) {
		raw := bytes.Repeat([]byte{0x03}, 32)
		t.Setenv("APP_PEPPER", base64.StdEncoding.EncodeToString(raw))
		got, err := LoadPepper()
		require.NoError(t, err)
		assert.Equal(t, raw, got, "base64 APP_PEPPER must decode to the raw 32-byte pepper")
	})

	t.Run("no pepper at all", func(t *testing.T) {
		t.Setenv("APP_PEPPER", "")
		got, err := LoadPepper()
		require.Error(t, err, "no sealed file and no APP_PEPPER must fail closed")
		assert.Nil(t, got)
	})
}

// TestLoadPepper_BrokenSealedFileFailsClosed pins that a sealed file that
// cannot be opened with APP_PARTY_ENC_PRIVATE_KEY errors instead of silently
// falling back to a different pepper.
// Parameters: t is the test harness.
// Returns nothing. Never panics.
func TestLoadPepper_BrokenSealedFileFailsClosed(t *testing.T) {
	dataDir := t.TempDir()
	_, encValue := partyEncKey(t)
	otherSeed := bytes.Repeat([]byte{0x07}, 32)
	otherKey, err := ecdh.X25519().NewPrivateKey(otherSeed)
	require.NoError(t, err)

	t.Setenv("APP_DATA_DIR", dataDir)
	t.Setenv("APP_PARTY_ENC_PRIVATE_KEY", encValue)
	t.Setenv("APP_PEPPER", string(bytes.Repeat([]byte{0x05}, 32)))
	// Sealed to a foreign use:enc key: the party key cannot open it.
	writeSealedPepper(t, dataDir, otherKey.PublicKey(), bytes.Repeat([]byte{0x01}, 32))

	got, err := LoadPepper()
	require.Error(t, err, "an unopenable sealed pepper must fail closed, not fall back")
	assert.Nil(t, got)
}
