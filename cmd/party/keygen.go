package main

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// keygen generates a fresh Ed25519 signing key and a fresh X25519
// encryption key (BK-PARTYID-004) and writes paste-ready output to w: the
// APP_PARTY_PRIVATE_KEY and APP_PARTY_ENC_PRIVATE_KEY env lines, followed
// by the public key, kid (RFC 7638 thumbprint) and enc_public_key the
// operator hands the gateway admin. It creates no files and uses no logger
// — key material reaches w and nowhere else.
// Parameters: w is the output sink (os.Stdout in production).
// Returns: nil once the output is written, or the RNG/write error. Never
// panics.
func keygen(w io.Writer) error {
	_, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("party keygen: ed25519: %w", err)
	}
	encKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("party keygen: x25519: %w", err)
	}
	signingPub := signingKey.Public().(ed25519.PublicKey)

	// Env values use the SDK registration encoding (std base64): the seed
	// for APP_PARTY_PRIVATE_KEY, the raw 32-byte scalar for
	// APP_PARTY_ENC_PRIVATE_KEY — both parse back with the SDK helpers.
	lines := []struct{ name, value string }{
		{"APP_PARTY_PRIVATE_KEY", base64.StdEncoding.EncodeToString(signingKey.Seed())},
		{"APP_PARTY_ENC_PRIVATE_KEY", base64.StdEncoding.EncodeToString(encKey.Bytes())},
		{"public_key", base64.StdEncoding.EncodeToString(signingPub)},
		{"kid", rfc7638Thumbprint(signingPub)},
		{"enc_public_key", base64.StdEncoding.EncodeToString(encKey.PublicKey().Bytes())},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%s=%s\n", line.name, line.value); err != nil {
			return fmt.Errorf("party keygen: write output: %w", err)
		}
	}
	return nil
}

// rfc7638Thumbprint computes the RFC 7638 section 3.1 thumbprint of an
// Ed25519 public key — the kid of every ks-party+jwt the SDK signs: SHA-256
// over the canonical OKP JWK JSON (crv, kty, x in lexicographic order),
// base64url without padding.
// Parameters: pub is the signing public key.
// Returns: the base64url thumbprint. Never panics.
func rfc7638Thumbprint(pub ed25519.PublicKey) string {
	canonical := `{"crv":"Ed25519","kty":"OKP","x":"` +
		base64.RawURLEncoding.EncodeToString(pub) + `"}`
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
