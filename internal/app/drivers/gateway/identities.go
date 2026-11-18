package gateway

import (
	"context"
	"fmt"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/identities"
	"github.com/konsulin-care/gateway-sdk-go/identity"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// identityGuard proves at compile time that the adapter satisfies
// contracts.IdentityClient — the interface and the implementation must
// land together so the package compiles at every commit.
var _ contracts.IdentityClient = (*Client)(nil)

// identityRoutes binds the SDK identity routes (SDK-API-001, BK-FED-003)
// to the adapter's SDK client. It fails closed: a disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O.
// Parameters: none — reads the adapter's own state.
// Returns: the SDK identity routes, or the error explaining why they
// cannot be built. Never panics.
func (c *Client) identityRoutes() (*identities.Client, error) {
	if !c.enabled {
		return nil, contracts.ErrGatewayNotConfigured
	}
	return identities.New(c.sdk)
}

// validateHashedFields fail-closes a non-hashfmt identity field before any
// request is sent: only the gateway hashfmt (the "N/A" marker or standard
// base64 of exactly 32 bytes) may reach the wire, so plaintext PII never
// leaves the adapter.
// Parameters: emailHash and phoneHash the candidate hashfmt values.
// Returns: nil when both fields are hashfmt, or the error naming the
// violation. Never panics.
func validateHashedFields(emailHash, phoneHash string) error {
	if err := identity.ValidateHash(emailHash); err != nil {
		return err
	}
	return identity.ValidateHash(phoneHash)
}

// Register registers one party-local subject's hashed identity at
// POST /v1/identities/register through the SDK identities Register route,
// party-signed by the adapter. The SDK marks the call idempotent, so one
// logical create carries one Idempotency-Key (SDK-CLIENT-002). The body
// carries only the peppered email and phone hashes plus the subject's
// resource type and local id — never plaintext PII. A disabled adapter
// reports contracts.ErrGatewayNotConfigured before any I/O; a non-hashfmt
// field fails closed before any request is sent. Implements
// contracts.IdentityClient.
// Parameters: ctx bounds the call, resourceType the local resource type,
// localResourceID the party-local resource reference, emailHash and
// phoneHash the peppered hashfmt values.
// Returns: the gateway's global_key, plus the error — render it through
// MapError. Never panics.
func (c *Client) Register(ctx context.Context, resourceType, localResourceID,
	emailHash, phoneHash string,
) (string, error) {
	routes, err := c.identityRoutes()
	if err != nil {
		return "", fmt.Errorf("gateway: register identity: %w", err)
	}
	if err := validateHashedFields(emailHash, phoneHash); err != nil {
		return "", fmt.Errorf("gateway: register identity: %w", err)
	}
	out, err := routes.Register(ctx, wire.RegisterIdentityRequest{
		ResourceType:    resourceType,
		LocalResourceID: localResourceID,
		EmailHash:       emailHash,
		PhoneHash:       phoneHash,
	})
	if err != nil {
		return "", fmt.Errorf("gateway: register identity: %w", err)
	}
	return out.GlobalKey, nil
}

// Resolve resolves hashed identity fields to candidate links at
// POST /v1/identities/resolve through the SDK identities Resolve route,
// party-signed by the adapter — a read, so it sends no Idempotency-Key. A
// disabled adapter reports contracts.ErrGatewayNotConfigured before any
// I/O; a non-hashfmt field fails closed before any request is sent.
// Implements contracts.IdentityClient.
// Parameters: ctx bounds the call, emailHash and phoneHash the peppered
// hashfmt values.
// Returns: the gateway's candidates (empty when none match), plus the
// error — render it through MapError. Never panics.
func (c *Client) Resolve(ctx context.Context, emailHash, phoneHash string,
) ([]contracts.IdentityCandidate, error) {
	routes, err := c.identityRoutes()
	if err != nil {
		return nil, fmt.Errorf("gateway: resolve identity: %w", err)
	}
	if err := validateHashedFields(emailHash, phoneHash); err != nil {
		return nil, fmt.Errorf("gateway: resolve identity: %w", err)
	}
	out, err := routes.Resolve(ctx, wire.ResolveIdentityRequest{
		EmailHash: emailHash,
		PhoneHash: phoneHash,
	})
	if err != nil {
		return nil, fmt.Errorf("gateway: resolve identity: %w", err)
	}
	cands := make([]contracts.IdentityCandidate, 0, len(out.Candidates))
	for _, cand := range out.Candidates {
		cands = append(cands, contracts.IdentityCandidate{
			GlobalKey:       cand.GlobalKey,
			PartyID:         cand.PartyID,
			LocalResourceID: cand.LocalResourceID,
			MatchField:      cand.MatchField,
		})
	}
	return cands, nil
}

// Link declares equivalence between a confirmed global key and a
// party-local resource at POST /v1/identities/link through the SDK
// identities Link route, party-signed by the adapter. The SDK marks the
// call idempotent, so one logical create carries one Idempotency-Key
// (SDK-CLIENT-002). A disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.IdentityClient.
// Parameters: ctx bounds the call, globalKey the confirmed candidate key,
// localResourceID the party-local resource to link.
// Returns: whether the gateway wrote the equivalence row, plus the error —
// render it through MapError. Never panics.
func (c *Client) Link(ctx context.Context, globalKey, localResourceID string) (bool, error) {
	routes, err := c.identityRoutes()
	if err != nil {
		return false, fmt.Errorf("gateway: link identity: %w", err)
	}
	out, err := routes.Link(ctx, wire.LinkIdentityRequest{
		GlobalKey:       globalKey,
		LocalResourceID: localResourceID,
	})
	if err != nil {
		return false, fmt.Errorf("gateway: link identity: %w", err)
	}
	return out.Linked, nil
}
