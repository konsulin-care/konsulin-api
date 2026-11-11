package gateway

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/utils"

	"github.com/konsulin-care/gateway-sdk-go/auth"
	"github.com/konsulin-care/gateway-sdk-go/client"
	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/parties"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// partyRegistryGuard proves at compile time that the adapter satisfies
// contracts.PartyRegistry — the interface and the implementation must land
// together so the package compiles at every commit.
var _ contracts.PartyRegistry = (*Client)(nil)

// partyJWKS is the minimal party JWKS document IsEnrolled decodes: only the
// kid list is read — key material never enters this package or any log.
type partyJWKS struct {
	Keys []struct {
		Kid string `json:"kid"`
	} `json:"keys"`
}

// IsEnrolled fetches GET /v1/parties/{id}/jwks for the adapter's own party
// through the SDK client and reports whether the served JWKS contains the
// adapter's own kid (client.go Kid(), RFC 7638) — the only enrollment check,
// with the gateway as the source of truth (BK-PARTYID-002). It fails closed:
// a disabled adapter returns contracts.ErrGatewayNotConfigured before any
// I/O, a transport or decode failure returns false with the error, and a JWKS
// without the adapter's kid returns (false, nil). Implements
// contracts.PartyRegistry.
// Parameters: ctx bounds the JWKS fetch.
// Returns: whether the party JWKS serves the adapter's own kid, and the
// error explaining a failed probe. Never panics; never logs key values.
func (c *Client) IsEnrolled(ctx context.Context) (bool, error) {
	if !c.enabled || c.kid == "" {
		return false, contracts.ErrGatewayNotConfigured
	}
	resp, err := c.do(ctx, http.MethodGet,
		"/v1/parties/"+url.PathEscape(c.partyID)+"/jwks", nil, false, "")
	if err != nil {
		return false, fmt.Errorf("gateway: party jwks: fetch: %w", err)
	}
	var jwks partyJWKS
	if err := json.Unmarshal(resp.Body, &jwks); err != nil {
		return false, fmt.Errorf("gateway: party jwks: decode: %w", err)
	}
	for _, k := range jwks.Keys {
		if k.Kid == c.kid {
			return true, nil
		}
	}
	return false, nil
}

// deriveEncPublicKey derives enc_public_key (base64.StdEncoding, raw 32
// bytes — the SDK's registration encoding) from APP_PARTY_ENC_PRIVATE_KEY,
// the X25519 key the party opens JWE envelopes with. The key value never
// appears in the return value beyond its public half or in any error.
// Parameters: none; reads APP_PARTY_ENC_PRIVATE_KEY from the environment.
// Returns: the derived enc_public_key and nil, or an error naming only the
// variable — never its value. Never panics.
func deriveEncPublicKey() (string, error) {
	raw := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ENC_PRIVATE_KEY", ""))
	if raw == "" {
		return "", errors.New("APP_PARTY_ENC_PRIVATE_KEY is empty")
	}
	priv, err := envelope.ParseX25519PrivateKey(raw)
	if err != nil {
		return "", errors.New("APP_PARTY_ENC_PRIVATE_KEY is unparseable")
	}
	return base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()), nil
}

// Enroll completes first-startup enrollment (BK-PARTYID-002): it submits
// the single-use enrollment token, the adapter's signing public key and
// the enc_public_key derived from APP_PARTY_ENC_PRIVATE_KEY through the
// SDK parties.Enroll route (POST /v1/parties/{id}/enroll), signed by the
// party key that derives the adapter's own kid. It fails closed: a disabled
// adapter reports contracts.ErrGatewayNotConfigured before any I/O, an
// empty token and missing or unparseable key material are rejected before
// any I/O. Implements contracts.PartyRegistry. The key values are never
// logged; errors name variables, never values.
// Parameters: ctx bounds the enrollment call, token is the single-use
// enrollment token delivered out of band by the gateway admin.
// Returns: nil once the gateway accepts the enrollment, or the error
// explaining the failure. Never panics.
func (c *Client) Enroll(ctx context.Context, token string) error {
	if !c.enabled || c.kid == "" {
		return contracts.ErrGatewayNotConfigured
	}
	if strings.TrimSpace(token) == "" {
		return errors.New("gateway: enroll: enrollment token is required")
	}
	key, err := auth.ParseEd25519PrivateKey(
		strings.TrimSpace(utils.GetEnvString("APP_PARTY_PRIVATE_KEY", "")))
	if err != nil {
		return fmt.Errorf("gateway: enroll: APP_PARTY_PRIVATE_KEY: %w",
			contracts.ErrGatewayNotConfigured)
	}
	encPub, err := deriveEncPublicKey()
	if err != nil {
		return fmt.Errorf("gateway: enroll: %w", err)
	}
	rawURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
	if rawURL == "" {
		return contracts.ErrGatewayNotConfigured
	}
	routes, err := parties.New(c.sdk, rawURL)
	if err != nil {
		return fmt.Errorf("gateway: enroll: party routes: %w", err)
	}
	if _, err := routes.Enroll(ctx, parties.EnrollInput{
		PartyID:         c.partyID,
		EnrollmentToken: token,
		EncPublicKey:    encPub,
	}, key); err != nil {
		return fmt.Errorf("gateway: enroll: %w", err)
	}
	return nil
}

// Rotate performs one self-service key rotation (BK-PARTYID-003): it submits
// the next signing key's public key and derived kid (RFC 7638 thumbprint)
// through the SDK parties.Rotate route (POST /v1/parties/{id}/keys), signed
// with the CURRENT adapter key, then switches the adapter signer to the next
// key so every subsequent request carries the next kid. It fails closed: a
// disabled adapter reports contracts.ErrGatewayNotConfigured before any I/O,
// as do missing APP_GATEWAY_URL and missing or unparseable next key material
// (errors name variables, never values). The rotate route body carries the
// next signing key only — the SDK wire.RotateKeyRequest has no enc member —
// while nextEnc is validated so an unusable APP_PARTY_NEXT_ENC_PRIVATE_KEY
// fails before any I/O. Implements contracts.PartyRegistry; never logs key
// values.
// Parameters: ctx bounds the rotation call, nextSig is the next signing key
// value (APP_PARTY_NEXT_PRIVATE_KEY encoding), nextEnc the next X25519 key
// value (APP_PARTY_NEXT_ENC_PRIVATE_KEY encoding).
// Returns: nil once the gateway accepts the rotation and the signer has
// switched, or the error explaining the failure. Never panics.
func (c *Client) Rotate(ctx context.Context, nextSig, nextEnc string) error {
	if !c.enabled || c.kid == "" {
		return contracts.ErrGatewayNotConfigured
	}
	nextKey, err := auth.ParseEd25519PrivateKey(strings.TrimSpace(nextSig))
	if err != nil {
		return fmt.Errorf("gateway: rotate: APP_PARTY_NEXT_PRIVATE_KEY: %w",
			contracts.ErrGatewayNotConfigured)
	}
	if strings.TrimSpace(nextEnc) == "" {
		return errors.New("gateway: rotate: APP_PARTY_NEXT_ENC_PRIVATE_KEY is empty")
	}
	if _, err := envelope.ParseX25519PrivateKey(strings.TrimSpace(nextEnc)); err != nil {
		return errors.New("gateway: rotate: APP_PARTY_NEXT_ENC_PRIVATE_KEY is unparseable")
	}
	nextKid, err := sdkKid(nextKey, c.partyID)
	if err != nil {
		return err
	}
	rawURL := strings.TrimSpace(utils.GetEnvString("APP_GATEWAY_URL", ""))
	if rawURL == "" {
		return contracts.ErrGatewayNotConfigured
	}
	routes, err := parties.New(c.sdk, rawURL)
	if err != nil {
		return fmt.Errorf("gateway: rotate: party routes: %w", err)
	}
	if _, err := routes.Rotate(ctx, c.partyID, wire.RotateKeyRequest{
		PublicKey: base64.StdEncoding.EncodeToString(
			nextKey.Public().(ed25519.PublicKey)),
		Kid: nextKid,
		Alg: "Ed25519",
	}); err != nil {
		return fmt.Errorf("gateway: rotate: %w", err)
	}
	return c.switchSigner(nextKey, nextKid, rawURL)
}

// switchSigner rebuilds the SDK client with the next signing key and adopts
// the next kid, so every subsequent party-signed request carries it. The
// rebuild mirrors New's options (base URL, signer, insecure transport for
// http loopback URLs); no key value is logged.
// Parameters: key is the next Ed25519 signing key, kid its RFC 7638
// thumbprint, rawURL the gateway base URL.
// Returns: nil once the adapter signer switched, or an error naming only
// facts. Never panics.
func (c *Client) switchSigner(key ed25519.PrivateKey, kid, rawURL string) error {
	sdk, err := client.New(
		client.WithBaseURL(rawURL),
		client.WithSigner(c.partyID, key),
		client.WithInsecureTransport(),
	)
	if err != nil {
		return fmt.Errorf("gateway: rotate: switch signer: %w", err)
	}
	c.sdk = sdk
	c.kid = kid
	return nil
}
