package contracts

import (
	"context"
	"errors"
)

// ErrGatewayNotConfigured indicates a gateway client or dependency is missing
// from the deployment configuration. Callers should surface it as a
// configuration problem, not a client error.
var ErrGatewayNotConfigured = errors.New("gateway not configured")

// ErrGatewayNotEnrolled indicates the authenticated subject has not completed
// gateway enrollment. Callers should surface it as an onboarding requirement.
var ErrGatewayNotEnrolled = errors.New("gateway not enrolled")

// GatewayStatus reports the availability of a gateway integration.
type GatewayStatus interface {
	// Enabled reports whether the gateway integration is configured and active.
	Enabled() bool
	// Enrolled reports whether the current subject is enrolled with the gateway.
	Enrolled() bool
}

// PartyRegistry exposes the gateway party's enrollment lifecycle as the
// gateway itself reports it (BK-PARTYID-002): the gateway is the only
// enrollment source of truth, never a local flag.
type PartyRegistry interface {
	// IsEnrolled fetches GET /v1/parties/{id}/jwks for the adapter's own
	// party and reports whether the served JWKS contains the adapter's own
	// kid (RFC 7638 thumbprint). It fails closed: a missing configuration
	// reports ErrGatewayNotConfigured; transport or decode failures return
	// false with the error; a JWKS without the kid returns (false, nil).
	// Parameters: ctx bounds the JWKS fetch.
	// Returns: whether the party JWKS serves the adapter's own kid, plus the
	// error explaining a failed probe.
	IsEnrolled(ctx context.Context) (bool, error)
	// Enroll completes first-startup enrollment (BK-PARTYID-002): it
	// submits the single-use enrollment token, the adapter's signing
	// public key and the enc_public_key derived from
	// APP_PARTY_ENC_PRIVATE_KEY through the SDK parties.Enroll route
	// (POST /v1/parties/{id}/enroll). It fails closed: a disabled adapter
	// or missing key material reports ErrGatewayNotConfigured before any
	// I/O.
	// Parameters: ctx bounds the enrollment call, token is the single-use
	// enrollment token delivered out of band by the gateway admin.
	// Returns: nil once the gateway accepts the enrollment, or the error
	// explaining the failure. Never logs key values.
	Enroll(ctx context.Context, token string) error
}
