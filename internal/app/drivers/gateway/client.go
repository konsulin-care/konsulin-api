// Package gateway is the sole importer of the external Konsulin gateway SDK
// (BK-FED-006): it adapts github.com/konsulin-care/gateway-sdk-go to backend
// config, keys and errors. No other package may import the SDK.
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
	"time"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"

	"github.com/konsulin-care/gateway-sdk-go/auth"
	"github.com/konsulin-care/gateway-sdk-go/client"
	"github.com/konsulin-care/gateway-sdk-go/wire"
	"go.uber.org/zap"
)

// enrolledTimeout bounds one Enrolled JWKS probe.
const enrolledTimeout = 5 * time.Second

// Client is the gateway adapter: an enabled client wraps the SDK client,
// a disabled one (empty APP_GATEWAY_URL) performs no I/O at all.
type Client struct {
	enabled bool
	partyID string
	kid     string
	sdk     *client.Client
	logger  *zap.Logger
}

// Request is one outbound gateway call in backend terms (the SDK's own
// request type never leaves this package).
type Request struct {
	// Method is the HTTP verb (e.g. http.MethodPost).
	Method string
	// Path is the request path relative to the gateway base URL.
	Path string
	// Body is the optional JSON request body.
	Body []byte
	// Idempotent marks a logical create that carries an Idempotency-Key.
	Idempotent bool
	// IdempotencyKey pins the key of an idempotent create; empty generates one.
	IdempotencyKey string
}

// Response is one gateway response with its body read in full.
type Response struct {
	// Status is the HTTP status code.
	Status int
	// Body is the complete response body.
	Body []byte
}

// New builds the gateway adapter from config: an empty APP_GATEWAY_URL
// yields a disabled adapter that requires no APP_PARTY_* values and never
// performs I/O; a configured gateway parses APP_PARTY_PRIVATE_KEY through
// the SDK key helper, derives the kid the SDK puts in every ks-party+jwt
// header, and builds the SDK client with WithSigner plus WithInsecureTransport
// only for local/dev/development/test environments.
// Parameters: cfg is the gateway config, env the app environment name,
// logger the adapter logger (nil becomes a no-op logger).
// Returns: the adapter and nil, or an error on missing party id, an
// unparseable key or an invalid SDK client option. Never panics.
func New(cfg config.AppGateway, env string, logger *zap.Logger) (*Client, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		logger.Info("gateway adapter disabled: APP_GATEWAY_URL is empty")
		return &Client{enabled: false, partyID: cfg.PartyID, logger: logger}, nil
	}
	if cfg.PartyID == "" {
		return nil, errors.New("gateway: APP_PARTY_ID is required when APP_GATEWAY_URL is set")
	}
	key, err := auth.ParseEd25519PrivateKey(strings.TrimSpace(cfg.PrivateKey))
	if err != nil {
		return nil, fmt.Errorf("gateway: APP_PARTY_PRIVATE_KEY: %w", err)
	}
	kid, err := sdkKid(key, cfg.PartyID)
	if err != nil {
		return nil, err
	}
	opts := []client.Option{
		client.WithBaseURL(rawURL),
		client.WithSigner(cfg.PartyID, key),
	}
	if isDevEnv(env) {
		opts = append(opts, client.WithInsecureTransport())
	}
	sdk, err := client.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("gateway: build sdk client: %w", err)
	}
	logger.Info("gateway adapter enabled", zap.String("party_id", cfg.PartyID), zap.String("env", env))
	return &Client{enabled: true, partyID: cfg.PartyID, kid: kid, sdk: sdk, logger: logger}, nil
}

// Enabled reports whether the gateway integration is configured (a non-empty
// APP_GATEWAY_URL). Implements contracts.GatewayStatus.
func (c *Client) Enabled() bool { return c.enabled }

// Enrolled reports whether the gateway serves this party's kid in
// GET /v1/parties/{id}/jwks — the gateway is the only enrollment source of
// truth (BK-PARTYID-002). It fails closed: a disabled adapter, any transport
// error or a JWKS without our kid reports false. Implements
// contracts.GatewayStatus.
func (c *Client) Enrolled() bool {
	if !c.enabled || c.kid == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), enrolledTimeout)
	defer cancel()
	resp, err := c.do(ctx, http.MethodGet,
		"/v1/parties/"+url.PathEscape(c.partyID)+"/jwks", nil, false, "")
	if err != nil {
		return false
	}
	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(resp.Body, &jwks); err != nil {
		return false
	}
	for _, k := range jwks.Keys {
		if k.Kid == c.kid {
			return true
		}
	}
	return false
}

// Kid returns the RFC 7638 thumbprint the SDK derives from
// APP_PARTY_PRIVATE_KEY — the kid header of every outbound ks-party+jwt.
// It is empty when the adapter is disabled (no key is parsed in standalone
// mode).
func (c *Client) Kid() string { return c.kid }

// PartyID returns APP_PARTY_ID, the party id every signed request is
// issued for.
func (c *Client) PartyID() string { return c.partyID }

// Do issues one party-signed request through the SDK client. A disabled
// adapter returns contracts.ErrGatewayNotConfigured before any I/O.
// Parameters: ctx is the call context, req describes the outbound call.
// Returns: the response, or an error — contracts.ErrGatewayNotConfigured
// when disabled, otherwise the SDK's error (including *wire.Error for a
// gateway failure). Never panics.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	return c.do(ctx, req.Method, req.Path, req.Body, req.Idempotent, req.IdempotencyKey)
}

// do is the shared call path behind Do and Enrolled: it short-circuits a
// disabled adapter with contracts.ErrGatewayNotConfigured, then runs the
// request party-signed through the SDK client.
func (c *Client) do(ctx context.Context, method, path string, body []byte, idempotent bool, key string) (*Response, error) {
	if !c.enabled {
		return nil, contracts.ErrGatewayNotConfigured
	}
	req := client.Request{
		Method:         method,
		Path:           path,
		Body:           body,
		Auth:           wire.AuthParty,
		Idempotent:     idempotent,
		IdempotencyKey: key,
	}
	resp, err := c.sdk.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	return &Response{Status: resp.StatusCode, Body: resp.Body}, nil
}

// sdkKid derives the kid exactly as the SDK does: it signs one throwaway
// ks-party+jwt with the key and reads the kid out of the protected header,
// so Kid() can never drift from the header the gateway sees.
// Parameters: key is the parsed Ed25519 signing key, partyID its issuer.
// Returns: the base64url RFC 7638 thumbprint, or an error.
func sdkKid(key ed25519.PrivateKey, partyID string) (string, error) {
	iat := time.Now().Unix()
	tok, err := auth.SignPartyRequest(key, auth.PartyClaims{
		Aud: "gateway",
		Exp: iat + auth.MaxPartyLifetime,
		Iat: iat,
		Iss: partyID,
		Jti: "kid-derivation",
	})
	if err != nil {
		return "", fmt.Errorf("gateway: derive kid: %w", err)
	}
	hdr, err := decodeHeader(tok)
	if err != nil {
		return "", err
	}
	if hdr.Kid == "" {
		return "", errors.New("gateway: derive kid: sdk token header has no kid")
	}
	return hdr.Kid, nil
}

// isDevEnv reports whether env permits http:// gateway URLs
// (WithInsecureTransport): local, dev, development or test.
func isDevEnv(env string) bool {
	switch env {
	case constvars.EnvLocal, constvars.EnvDev, constvars.EnvDevelopment, constvars.EnvTest:
		return true
	}
	return false
}

// jwsHeader is the minimal compact-JWS protected header read by tests and
// kid derivation.
type jwsHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

// decodeHeader decodes the protected header of a compact JWS.
func decodeHeader(tok string) (jwsHeader, error) {
	var hdr jwsHeader
	seg, _, ok := strings.Cut(tok, ".")
	if !ok {
		return hdr, errors.New("gateway: token is not a compact JWS")
	}
	raw, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return hdr, fmt.Errorf("gateway: token header: %w", err)
	}
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return hdr, fmt.Errorf("gateway: token header: %w", err)
	}
	return hdr, nil
}
