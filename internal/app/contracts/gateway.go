package contracts

import "errors"

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
