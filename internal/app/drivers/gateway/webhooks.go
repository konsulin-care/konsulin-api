package gateway

import (
	"context"
	"crypto/ecdh"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"
	"github.com/konsulin-care/konsulin-api/internal/pkg/utils"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// webhookGuard proves at compile time that the adapter satisfies
// contracts.WebhookClient — the interface and the implementation must land
// together so the package compiles at every commit.
var _ contracts.WebhookClient = (*Client)(nil)

// webhookKeyTTL is how long the SDK EncKeyResolver serves one resolved
// recipient use:enc key before refetching GET /v1/parties/{id}/jwks.
const webhookKeyTTL = 5 * time.Minute

// recipientEncKey resolves the recipient party's use:enc key through the
// SDK EncKeyResolver over the party JWKS, fetched party-signed by the
// adapter. A JWKS without exactly one use:enc key fails closed with a nil
// key — no payload is ever sealed unprotected.
// Parameters: ctx bounds the JWKS fetch, partyID is the recipient party.
// Returns: the recipient's X25519 public key, its kid and nil, or an error
// explaining the failed resolution. Never panics; never logs key values.
func (c *Client) recipientEncKey(ctx context.Context, partyID string) (*ecdh.PublicKey, string, error) {
	resolver := envelope.NewEncKeyResolver(func(ctx context.Context, id string) ([]byte, error) {
		resp, err := c.do(ctx, http.MethodGet,
			"/v1/parties/"+url.PathEscape(id)+"/jwks", nil, false, "")
		if err != nil {
			return nil, err
		}
		return resp.Body, nil
	}, webhookKeyTTL, nil)
	return resolver.Resolve(ctx, partyID)
}

// sealWebhook seals data to the recipient's use:enc key with the SDK bound
// envelope (envelope.SealBound): the wrapper binds from the adapter's own
// party, to the recipient and to the service, so the recipient's OpenBound
// must carry the same three values. It fails closed before any webhook
// request when the recipient has no usable use:enc key.
// Parameters: ctx bounds the JWKS fetch, to the recipient party, service
// the listing name, data the plaintext payload.
// Returns: the compact JWE and nil, or an error. Never panics.
func (c *Client) sealWebhook(ctx context.Context, to, service string, data []byte) (string, error) {
	recipient, kid, err := c.recipientEncKey(ctx, to)
	if err != nil {
		return "", fmt.Errorf("gateway: webhook: recipient use:enc key: %w", err)
	}
	compact, err := envelope.SealBound(data, kid, recipient, envelope.Binding{
		From: c.partyID, To: to, Service: service,
	})
	if err != nil {
		return "", fmt.Errorf("gateway: webhook: seal: %w", err)
	}
	return compact, nil
}

// webhookBody renders one webhook request body: the payload member is the
// compact JWE as a JSON string, so the wire reads
// {"to","service","user_ref","payload":"<compact JWE>"}.
// Parameters: to the recipient party, service the listing name, userRef the
// peppered end-user hash, compact the sealed payload.
// Returns: the marshalled request bytes and nil, or an error. Never panics.
func webhookBody(to, service, userRef, compact string) ([]byte, error) {
	payload, err := json.Marshal(compact)
	if err != nil {
		return nil, fmt.Errorf("gateway: webhook: payload encode: %w", err)
	}
	body, err := json.Marshal(wire.EnqueueWebhookRequest{
		To: to, Service: service, UserRef: userRef, Payload: payload,
	})
	if err != nil {
		return nil, fmt.Errorf("gateway: webhook: request encode: %w", err)
	}
	return body, nil
}

// ownEncPrivateKey parses APP_PARTY_ENC_PRIVATE_KEY — the X25519 key the
// adapter opens sync replies with. The key value never appears in an error.
// Parameters: none; reads APP_PARTY_ENC_PRIVATE_KEY from the environment.
// Returns: the private key and nil, or an error naming only the variable.
// Never panics.
func ownEncPrivateKey() (*ecdh.PrivateKey, error) {
	raw := strings.TrimSpace(utils.GetEnvString("APP_PARTY_ENC_PRIVATE_KEY", ""))
	if raw == "" {
		return nil, errors.New("APP_PARTY_ENC_PRIVATE_KEY is empty")
	}
	key, err := envelope.ParseX25519PrivateKey(raw)
	if err != nil {
		return nil, errors.New("APP_PARTY_ENC_PRIVATE_KEY is unparseable")
	}
	return key, nil
}

// isJOSEContentType reports whether a sync reply media type carries a
// compact JWE envelope (application/jose, with or without a suffix or
// parameters).
// Parameters: ct is the reply's content_type value.
// Returns: whether the reply may be opened as a bound envelope. Never
// panics.
func isJOSEContentType(ct string) bool {
	mt, _, _ := strings.Cut(ct, ";")
	mt = strings.ToLower(strings.TrimSpace(mt))
	return mt == wire.ContentTypeJOSE || strings.HasPrefix(mt, "application/jose+")
}

// Enqueue queues one async invocation at POST /v1/webhooks through the SDK
// webhook route, party-signed and idempotent (one Idempotency-Key per
// logical create). The payload is sealed first: the body carries to,
// service, user_ref and payload — the compact JWE as a JSON string bound
// from the adapter's party, to the recipient and to the service. A disabled
// adapter reports contracts.ErrGatewayNotConfigured and a recipient
// without a use:enc key fails before any webhook request is sent.
// Implements contracts.WebhookClient.
// Parameters: ctx bounds the call, to the recipient party, service the
// listing name, userRef the peppered end-user hash, data the plaintext
// payload to seal.
// Returns: nil once the gateway accepts the enqueue, or the error to render
// through MapError. Never panics.
func (c *Client) Enqueue(ctx context.Context, to, service, userRef string, data []byte) error {
	if !c.enabled {
		return contracts.ErrGatewayNotConfigured
	}
	compact, err := c.sealWebhook(ctx, to, service, data)
	if err != nil {
		return fmt.Errorf("gateway: enqueue webhook: %w", err)
	}
	body, err := webhookBody(to, service, userRef, compact)
	if err != nil {
		return fmt.Errorf("gateway: enqueue webhook: %w", err)
	}
	if _, err := c.do(ctx, http.MethodPost, "/v1/webhooks", body, true, ""); err != nil {
		return fmt.Errorf("gateway: enqueue webhook: %w", err)
	}
	return nil
}

// Sync forwards one invocation inline at POST /v1/webhooks/sync through the
// SDK webhook route, party-signed and idempotent, then opens the reply: the
// request body is the same sealed payload as Enqueue, and an
// application/jose reply is opened with envelope.OpenBound under the
// swapped expectations — from the recipient party, to the adapter's own
// party, service unchanged — using APP_PARTY_ENC_PRIVATE_KEY. Everything
// else fails closed: a disabled adapter, a missing or unparseable
// APP_PARTY_ENC_PRIVATE_KEY, a recipient without a use:enc key, a
// non-jose content type, or an envelope that does not match the
// expectations. Implements contracts.WebhookClient.
// Parameters: ctx bounds the call, to the recipient party, service the
// listing name, userRef the peppered end-user hash, data the plaintext
// payload to seal.
// Returns: the opened reply data and nil, or nil and the error to render
// through MapError. Never panics; plaintext lives in the returned buffer
// only.
func (c *Client) Sync(ctx context.Context, to, service, userRef string, data []byte) ([]byte, error) {
	if !c.enabled {
		return nil, contracts.ErrGatewayNotConfigured
	}
	ownKey, err := ownEncPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("gateway: sync webhook: %w", err)
	}
	compact, err := c.sealWebhook(ctx, to, service, data)
	if err != nil {
		return nil, fmt.Errorf("gateway: sync webhook: %w", err)
	}
	body, err := webhookBody(to, service, userRef, compact)
	if err != nil {
		return nil, fmt.Errorf("gateway: sync webhook: %w", err)
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/webhooks/sync", body, true, "")
	if err != nil {
		return nil, fmt.Errorf("gateway: sync webhook: %w", err)
	}
	var out wire.SyncWebhookResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("gateway: sync webhook: reply decode: %w", err)
	}
	if !isJOSEContentType(out.ContentType) {
		return nil, fmt.Errorf("gateway: sync webhook: reply content type %q is not %s",
			out.ContentType, wire.ContentTypeJOSE)
	}
	opened, err := envelope.OpenBound(ctx, out.Body, ownKey, envelope.Expectations{
		From: to, To: c.partyID, Service: service,
	}, nil, time.Now)
	if err != nil {
		return nil, fmt.Errorf("gateway: sync webhook: open reply: %w", err)
	}
	return opened, nil
}

// Status reads one enqueued invocation's delivery state at GET
// /v1/webhooks/{id}/status — a read, so it sends no Idempotency-Key. A
// disabled adapter reports contracts.ErrGatewayNotConfigured before any
// I/O. Implements contracts.WebhookClient.
// Parameters: ctx bounds the call, id the enqueued webhook id.
// Returns: the gateway's state ("pending", "delivered" or "failed"), plus
// the error to render through MapError. Never panics.
func (c *Client) Status(ctx context.Context, id string) (string, error) {
	resp, err := c.do(ctx, http.MethodGet,
		"/v1/webhooks/"+url.PathEscape(id)+"/status", nil, false, "")
	if err != nil {
		return "", fmt.Errorf("gateway: webhook status: %w", err)
	}
	var out wire.WebhookStatusResponse
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return "", fmt.Errorf("gateway: webhook status: decode: %w", err)
	}
	return out.State, nil
}
