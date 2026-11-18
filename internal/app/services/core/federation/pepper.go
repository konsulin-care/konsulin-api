package federation

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
)

// pepperFileName is the sealed pepper artifact at rest: the compact JWE the
// party stores verbatim under APP_DATA_DIR after the federation-pepper fetch
// (BK-PEPPER-001). It is the only pepper material ever written to disk.
const pepperFileName = "federation-pepper.jwe"

// pepperSize is the byte length the SDK identity HMAC (SDK-ID) requires from
// a pepper: HMAC-SHA256 key material, also the decoded size of the standard
// base64 delivery form (openssl rand -base64 32).
const pepperSize = 32

// LoadPepper opens the federation pepper into memory only. The sealed JWE at
// <APP_DATA_DIR>/federation-pepper.jwe, opened with APP_PARTY_ENC_PRIVATE_KEY,
// takes precedence; when no sealed file exists the APP_PEPPER environment
// value is the manual-delivery fallback (option A). A sealed file that exists
// but cannot be opened fails closed — it never silently falls back to a
// different pepper, which would split the federation's hashes. Neither the
// pepper, the key, nor the file contents are ever logged or written back.
//
// Parameters: none. APP_DATA_DIR (default ./data), APP_PARTY_ENC_PRIVATE_KEY
// and APP_PEPPER are read from the environment at call time, so a startup
// override is honored.
//
// Returns: the raw pepper bytes (a 32-byte value passes through; a standard
// base64 encoding of 32 bytes is decoded, so both delivery forms yield the
// same key), or an error when no pepper is available or the sealed JWE cannot
// be opened. Never panics.
func LoadPepper() ([]byte, error) {
	dataDir := utils.GetEnvString("APP_DATA_DIR", "./data")
	sealed, err := os.ReadFile(filepath.Join(dataDir, pepperFileName))
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("federation: read sealed pepper: %w", err)
		}
		return envPepper()
	}
	return openSealed(string(sealed))
}

// openSealed opens the compact JWE read from the sealed pepper file with the
// party's use:enc private key (APP_PARTY_ENC_PRIVATE_KEY) and normalizes the
// plaintext pepper.
// Parameters: compact is the raw file content (ciphertext exactly as
// received, tolerated trailing whitespace only).
// Returns: the pepper bytes and nil, or an error with a nil pepper — a
// missing or unparseable key, or an envelope that does not open under it,
// never yields plaintext. Never panics, never logs key material.
func openSealed(compact string) ([]byte, error) {
	trimmed := strings.TrimSpace(compact)
	if trimmed == "" {
		return nil, errors.New("federation: sealed pepper file is empty")
	}
	raw := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ENC_PRIVATE_KEY", ""))
	if raw == "" {
		return nil, errors.New("federation: APP_PARTY_ENC_PRIVATE_KEY is required to open the sealed pepper")
	}
	key, err := envelope.ParseX25519PrivateKey(raw)
	if err != nil {
		return nil, fmt.Errorf("federation: APP_PARTY_ENC_PRIVATE_KEY: %w", err)
	}
	plaintext, err := envelope.Open(trimmed, key)
	if err != nil {
		return nil, fmt.Errorf("federation: open sealed pepper: %w", err)
	}
	pepper := normalizePepper(plaintext)
	if len(pepper) == 0 {
		return nil, errors.New("federation: sealed pepper is empty")
	}
	return pepper, nil
}

// envPepper loads the APP_PEPPER manual-delivery fallback.
// Parameters: none; reads APP_PEPPER from the environment.
// Returns: the normalized pepper bytes and nil, or an error when the variable
// is unset or empty — no pepper is never treated as a valid key. Never
// panics.
func envPepper() ([]byte, error) {
	raw := strings.TrimSpace(utils.GetEnvString("APP_PEPPER", ""))
	if raw == "" {
		return nil, errors.New("federation: no pepper available: sealed federation-pepper.jwe absent and APP_PEPPER unset")
	}
	return normalizePepper([]byte(raw)), nil
}

// normalizePepper renders a loaded pepper as the raw bytes the SDK identity
// HMAC keys with: an already-raw value of exactly 32 bytes passes through, a
// standard base64 encoding of 32 bytes (the APP_PEPPER delivery form) is
// decoded, and anything else passes through unchanged so it fails closed at
// hash time instead of being silently reinterpreted. The same rule applies to
// the sealed and the env path, so both deliver identical hashes for the same
// pepper (BK-PEPPER-C01).
// Parameters: raw is the pepper as loaded from the file or the environment.
// Returns: the raw pepper bytes; never nil-tagged errors. Never panics.
func normalizePepper(raw []byte) []byte {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == pepperSize {
		return trimmed
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil && len(decoded) == pepperSize {
		return decoded
	}
	return trimmed
}
