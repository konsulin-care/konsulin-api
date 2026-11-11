package gateway

import (
	"errors"
	"fmt"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/konsulin-care/gateway-sdk-go/wire"
)

// Client-facing and developer-facing texts for the gateway error mapping.
const (
	// clientMsgGatewayUnavailable answers every gateway-dependent call while
	// APP_GATEWAY_URL is unset (standalone mode, PROPOSAL discrepancy 2).
	clientMsgGatewayUnavailable = "This endpoint requires the federation gateway. Configure APP_GATEWAY_URL and enroll this party to use it."
	// clientMsgGatewayNotEnrolled answers a gateway call whose party has no
	// enrollment yet.
	clientMsgGatewayNotEnrolled = "This endpoint requires the federation gateway. Enroll this party to use it."
	// devGatewayNotConfigured is the dev_message code for a missing gateway.
	devGatewayNotConfigured = "GATEWAY_NOT_CONFIGURED"
	// devGatewayNotEnrolled is the dev_message code for a missing enrollment.
	devGatewayNotEnrolled = "GATEWAY_NOT_ENROLLED"
	// devGatewayTransport prefixes dev_message for transport failures.
	devGatewayTransport = "GATEWAY_TRANSPORT_ERROR"
	// devGatewayError stands in for a wire error without a catalog code.
	devGatewayError = "GATEWAY_ERROR"
)

// MapError converts an adapter error into an exceptions.CustomError the
// HTTP layer can render:
//   - contracts.ErrGatewayNotConfigured answers 503 with the
//     configure-the-gateway client message and dev_message
//     GATEWAY_NOT_CONFIGURED;
//   - contracts.ErrGatewayNotEnrolled answers 503 with dev_message
//     GATEWAY_NOT_ENROLLED;
//   - a *wire.Error keeps the gateway's HTTP status, with the GW-* code
//     and the gateway request_id in dev_message;
//   - any other (transport) error answers 502 with the cause in
//     dev_message.
//
// Parameters: err is the error surfaced by Client.Do (may be nil).
// Returns: the mapped *exceptions.CustomError, or nil for a nil error.
// Never panics.
func MapError(err error) *exceptions.CustomError {
	if err == nil {
		return nil
	}
	if errors.Is(err, contracts.ErrGatewayNotConfigured) {
		return exceptions.BuildNewCustomError(nil, constvars.StatusServiceUnavailable,
			clientMsgGatewayUnavailable, devGatewayNotConfigured)
	}
	if errors.Is(err, contracts.ErrGatewayNotEnrolled) {
		return exceptions.BuildNewCustomError(nil, constvars.StatusServiceUnavailable,
			clientMsgGatewayNotEnrolled, devGatewayNotEnrolled)
	}
	var wireErr *wire.Error
	if errors.As(err, &wireErr) && wireErr != nil {
		return mapWireError(wireErr)
	}
	return exceptions.BuildNewCustomError(nil, constvars.StatusBadGateway,
		constvars.ErrClientCannotProcessRequest,
		fmt.Sprintf("%s: %v", devGatewayTransport, err))
}

// mapWireError maps an SDK *wire.Error to a CustomError keeping the
// gateway's HTTP status (falling back to 502 when the status is unset)
// and putting the GW-* code plus the gateway request_id in dev_message.
// Parameters: wireErr is the decoded gateway error envelope.
// Returns: the mapped *exceptions.CustomError. Never panics.
func mapWireError(wireErr *wire.Error) *exceptions.CustomError {
	status := wireErr.Status
	if status <= 0 {
		status = constvars.StatusBadGateway
	}
	clientMsg := wireErr.Message
	if clientMsg == "" {
		clientMsg = constvars.ErrClientCannotProcessRequest
	}
	devMsg := wireErr.Code
	if devMsg == "" {
		devMsg = devGatewayError
	}
	if wireErr.RequestID != "" {
		devMsg = fmt.Sprintf("%s request_id=%s", devMsg, wireErr.RequestID)
	}
	return exceptions.BuildNewCustomError(nil, status, clientMsg, devMsg)
}
