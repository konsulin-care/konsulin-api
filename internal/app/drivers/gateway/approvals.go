package gateway

import (
	"context"
	"fmt"

	"konsulin-service/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/approvals"
	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// approvalGuard proves at compile time that the adapter satisfies
// contracts.ApprovalClient — the interface and the implementation must
// land together so the package compiles at every commit.
var _ contracts.ApprovalClient = (*Client)(nil)

// approvalRoutes binds the SDK approval routes (SDK-API-001, SDK-API-002)
// to the adapter's SDK client. It fails closed: a disabled adapter reports
// contracts.ErrGatewayNotConfigured before any I/O.
// Parameters: none — reads the adapter's own state.
// Returns: the SDK approval routes, or the error explaining why they
// cannot be built. Never panics.
func (c *Client) approvalRoutes() (*approvals.Client, error) {
	if !c.enabled {
		return nil, contracts.ErrGatewayNotConfigured
	}
	return approvals.New(c.sdk)
}

// Mint mints an approval handoff ticket at POST /v1/approvals through the
// SDK approvals Mint route, party-signed by the adapter. The SDK marks the
// call idempotent, so one logical create carries one Idempotency-Key
// (SDK-CLIENT-002). The body carries the recipient party, the allowlisted
// scope, the literal purpose and the researcher hash — no other fields. A
// disabled adapter reports contracts.ErrGatewayNotConfigured before any
// I/O. Implements contracts.ApprovalClient.
// Parameters: ctx bounds the call, to the recipient party id, scope the
// allowlisted scope ("referral" or "export:<study>"), purpose the literal
// purpose string, researcherHash the base64 32-byte researcher hash or
// "N/A".
// Returns: the minted ks-approval+jwt ticket (the response's access_token),
// plus the error — render it through MapError. Never panics.
func (c *Client) Mint(ctx context.Context, to, scope, purpose,
	researcherHash string,
) (string, error) {
	routes, err := c.approvalRoutes()
	if err != nil {
		return "", fmt.Errorf("gateway: mint approval: %w", err)
	}
	out, err := routes.Mint(ctx, wire.MintApprovalRequest{
		To:             to,
		Scope:          scope,
		Purpose:        purpose,
		ResearcherHash: researcherHash,
	})
	if err != nil {
		return "", fmt.Errorf("gateway: mint approval: %w", err)
	}
	return out.AccessToken, nil
}

// VerifyTicket checks one directly presented ks-approval+jwt through the
// SDK verifier's Ticket rule set: exact typ ks-approval+jwt, Ed25519
// signature by the token's own header kid against the gateway JWKS
// (APP_GATEWAY_URL/.well-known/jwks), iss "gateway", aud equal to this
// party (APP_PARTY_ID), exp inside the 60-second skew, scope exactly
// equal to expectedScope, and finally the ticket's single-use jti
// consumed in the Redis replay cache — a replayed jti or a Redis failure
// denies (fail closed). The verifier is the same one the callback
// middleware builds, so the JWKS endpoint, the recipient identity and the
// T06 ReplayCache configuration stay in one place. A disabled adapter
// reports contracts.ErrGatewayNotConfigured before any I/O. Implements
// contracts.ApprovalClient.
// Parameters: ctx bounds the JWKS fetch and the jti reservation, token the
// compact ticket JWS, expectedScope the exact scope of the operation
// about to be performed.
// Returns: nil once the ticket verifies for the expected scope, or the
// error carrying the SDK sentinel (matchable with errors.Is). Never
// panics.
func (c *Client) VerifyTicket(ctx context.Context, token, expectedScope string) error {
	if !c.enabled {
		return contracts.ErrGatewayNotConfigured
	}
	verifier, err := newCallbackVerifier()
	if err != nil {
		return fmt.Errorf("gateway: verify ticket: %w", err)
	}
	if _, err := verifier.Ticket(ctx, token, wire.TypApproval, expectedScope); err != nil {
		return fmt.Errorf("gateway: verify ticket: %w", err)
	}
	return nil
}
