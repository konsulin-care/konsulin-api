package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/konsulin-care/gateway-sdk-go/auth"
	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// output runs the keygen subcommand through the CLI dispatcher and returns
// its stdout, failing if keygen reports an error or writes to stderr.
// Parameters: t is the test harness.
// Returns: the complete keygen stdout.
func output(t *testing.T) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	require.NoError(t, run([]string{"keygen"}, &stdout, &stderr), "keygen must succeed")
	require.Empty(t, stderr.String(), "keygen must not write to stderr")
	return stdout.String()
}

// envLine returns the value of the first `key=` line in out.
// Parameters: t is the test harness, out the keygen output, key the line key.
// Returns: the value after `key=`; fails the test when the line is missing.
func envLine(t *testing.T, out, key string) string {
	t.Helper()
	prefix := key + "="
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("keygen output must contain a %s line", key)
	return ""
}

// TestKeygenEnvValuesParseWithSDKHelpers covers assertion 1: keygen prints
// APP_PARTY_PRIVATE_KEY and APP_PARTY_ENC_PRIVATE_KEY lines whose values
// parse with the SDK key helpers.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestKeygenEnvValuesParseWithSDKHelpers(t *testing.T) {
	out := output(t)

	signing, err := auth.ParseEd25519PrivateKey(envLine(t, out, "APP_PARTY_PRIVATE_KEY"))
	require.NoError(t, err, "printed APP_PARTY_PRIVATE_KEY must parse with the SDK helper")
	require.Len(t, signing, ed25519.PrivateKeySize)

	enc, err := envelope.ParseX25519PrivateKey(envLine(t, out, "APP_PARTY_ENC_PRIVATE_KEY"))
	require.NoError(t, err, "printed APP_PARTY_ENC_PRIVATE_KEY must parse with the SDK helper")
	require.NotNil(t, enc, "printed encryption key must be usable")
}

// TestKeygenKidAndEncPublicKeyCoverAssertions covers assertion 2: the printed
// kid equals the RFC 7638 thumbprint of the printed signing key and
// enc_public_key is the base64 X25519 public half of the printed encryption
// key.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestKeygenKidAndEncPublicKeyCoverAssertions(t *testing.T) {
	out := output(t)

	signing, err := auth.ParseEd25519PrivateKey(envLine(t, out, "APP_PARTY_PRIVATE_KEY"))
	require.NoError(t, err)
	enc, err := envelope.ParseX25519PrivateKey(envLine(t, out, "APP_PARTY_ENC_PRIVATE_KEY"))
	require.NoError(t, err)

	// Expected kid computed independently per RFC 7638 section 3.1:
	// SHA-256 over the canonical OKP JWK JSON, base64url without padding.
	pub := signing.Public().(ed25519.PublicKey)
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` +
		base64.RawURLEncoding.EncodeToString(pub) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	wantKid := base64.RawURLEncoding.EncodeToString(sum[:])
	assert.Equal(t, wantKid, envLine(t, out, "kid"),
		"kid must be the RFC 7638 thumbprint of the printed signing key")

	assert.Equal(t, base64.StdEncoding.EncodeToString(enc.PublicKey().Bytes()),
		envLine(t, out, "enc_public_key"),
		"enc_public_key must be the base64 X25519 public half")
}

// TestKeygenCreatesNoFiles covers assertion 3: keygen creates no files in
// the working directory.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestKeygenCreatesNoFiles(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	_ = output(t)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "keygen must create no files in the working directory")
}
