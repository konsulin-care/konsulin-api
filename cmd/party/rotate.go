package main

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/drivers/gateway"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
)

// rotateTimeout bounds one rotate call against the gateway.
const rotateTimeout = 30 * time.Second

// generateNextKeys creates the fresh Ed25519 signing key and X25519
// encryption key submitted by one rotation. Key material stays in memory.
// Parameters: none.
// Returns: the signing key, the encryption key, and nil — or the RNG error.
// Never panics.
func generateNextKeys() (ed25519.PrivateKey, *ecdh.PrivateKey, error) {
	_, signingKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("party rotate: ed25519: %w", err)
	}
	encKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("party rotate: x25519: %w", err)
	}
	return signingKey, encKey, nil
}

// rotate performs one self-service key rotation (BK-PARTYID-005): it reads
// the current key, party id and gateway URL from the environment, generates
// the next Ed25519 + X25519 keypair, and submits it through the gateway
// adapter's Rotate — signed with the current key — so the gateway installs
// the next kid. Only after the gateway accepts does it write the new
// paste-ready env values (APP_PARTY_NEXT_PRIVATE_KEY,
// APP_PARTY_NEXT_ENC_PRIVATE_KEY) plus the promote reminder to w. On any
// failure it returns the mapped error and writes no key material at all.
// Parameters: w is the output sink (os.Stdout in production).
// Returns: nil once the accepted rotation's output is written, otherwise
// the mapped error. Never panics; never writes key material on failure.
func rotate(w io.Writer) error {
	gwURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
	partyID := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ID", ""))
	currentKey := strings.TrimSpace(utils.GetEnvString("APP_PARTY_PRIVATE_KEY", ""))
	env := utils.GetEnvString("APP_ENV", constvars.EnvLocal)

	nextKey, nextEncKey, err := generateNextKeys()
	if err != nil {
		return err
	}
	// Env values use the SDK registration encoding (std base64): the seed
	// for APP_PARTY_NEXT_PRIVATE_KEY, the raw 32-byte scalar for
	// APP_PARTY_NEXT_ENC_PRIVATE_KEY — both parse back with the SDK helpers.
	nextSig := base64.StdEncoding.EncodeToString(nextKey.Seed())
	nextEnc := base64.StdEncoding.EncodeToString(nextEncKey.Bytes())

	adapter, err := gateway.New(config.AppGateway{
		Enabled:    gwURL != "",
		URL:        gwURL,
		PartyID:    partyID,
		PrivateKey: currentKey,
	}, env, nil)
	if err != nil {
		return fmt.Errorf("party rotate: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), rotateTimeout)
	defer cancel()
	if err := adapter.Rotate(ctx, nextSig, nextEnc); err != nil {
		return gateway.MapError(err)
	}

	// The gateway accepted the rotation — only now does key material reach
	// the output, as paste-ready NEXT_* env values plus the promote
	// reminder: the operator moves them into the main vars and clears the
	// NEXT_* values; the gateway's grace window covers the switch.
	nextPub := nextKey.Public().(ed25519.PublicKey)
	lines := []struct{ name, value string }{
		{"APP_PARTY_NEXT_PRIVATE_KEY", nextSig},
		{"APP_PARTY_NEXT_ENC_PRIVATE_KEY", nextEnc},
		{"public_key", base64.StdEncoding.EncodeToString(nextPub)},
		{"kid", rfc7638Thumbprint(nextPub)},
		{"enc_public_key", base64.StdEncoding.EncodeToString(nextEncKey.PublicKey().Bytes())},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "%s=%s\n", line.name, line.value); err != nil {
			return fmt.Errorf("party rotate: write output: %w", err)
		}
	}
	_, err = fmt.Fprintln(w, "promote: move APP_PARTY_NEXT_PRIVATE_KEY into APP_PARTY_PRIVATE_KEY and APP_PARTY_NEXT_ENC_PRIVATE_KEY into APP_PARTY_ENC_PRIVATE_KEY, then clear the APP_PARTY_NEXT_* values — the gateway grace window covers the switch")
	return err
}
