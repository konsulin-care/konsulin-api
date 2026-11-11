package gateway

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"konsulin-service/internal/app/contracts"

	"github.com/konsulin-care/gateway-sdk-go/wire"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMapErrorWireError covers assertion 1: a *wire.Error maps to a
// CustomError keeping the gateway's HTTP status and carrying the GW-*
// code plus the gateway request_id in dev_message.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestMapErrorWireError(t *testing.T) {
	wireErr := &wire.Error{
		Code:      "GW-PAY-004",
		Message:   "payment method not found",
		RequestID: "gw-req-42",
		Status:    http.StatusNotFound,
	}

	ce := MapError(fmt.Errorf("do payment: %w", wireErr))
	require.NotNil(t, ce, "a *wire.Error must map to a CustomError")
	assert.Equal(t, http.StatusNotFound, ce.StatusCode,
		"the gateway HTTP status must be kept")
	assert.Contains(t, ce.DevMessage, "GW-PAY-004",
		"dev_message must carry the GW-* code")
	assert.Contains(t, ce.DevMessage, "gw-req-42",
		"dev_message must carry the gateway request_id")
}

// TestMapErrorNotConfigured covers assertion 2: contracts.ErrGatewayNotConfigured
// maps to status 503 with the configure-the-gateway client message and
// dev_message GATEWAY_NOT_CONFIGURED.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestMapErrorNotConfigured(t *testing.T) {
	ce := MapError(contracts.ErrGatewayNotConfigured)
	require.NotNil(t, ce, "ErrGatewayNotConfigured must map to a CustomError")
	assert.Equal(t, http.StatusServiceUnavailable, ce.StatusCode,
		"missing gateway configuration must answer 503")
	assert.Equal(t,
		"This endpoint requires the federation gateway. Configure APP_GATEWAY_URL and enroll this party to use it.",
		ce.ClientMessage,
		"the client message must tell the operator to configure the gateway")
	assert.Equal(t, "GATEWAY_NOT_CONFIGURED", ce.DevMessage,
		"dev_message must be the GATEWAY_NOT_CONFIGURED code")
}

// TestMapErrorNotEnrolled covers assertion 3: contracts.ErrGatewayNotEnrolled
// maps to status 503 with dev_message GATEWAY_NOT_ENROLLED.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestMapErrorNotEnrolled(t *testing.T) {
	ce := MapError(contracts.ErrGatewayNotEnrolled)
	require.NotNil(t, ce, "ErrGatewayNotEnrolled must map to a CustomError")
	assert.Equal(t, http.StatusServiceUnavailable, ce.StatusCode,
		"missing enrollment must answer 503")
	assert.Equal(t, "GATEWAY_NOT_ENROLLED", ce.DevMessage,
		"dev_message must be the GATEWAY_NOT_ENROLLED code")
}

// TestMapErrorTransport maps a plain transport failure to 502 with the
// underlying cause preserved in dev_message.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestMapErrorTransport(t *testing.T) {
	ce := MapError(errors.New("dial tcp: connection refused"))
	require.NotNil(t, ce, "a transport error must map to a CustomError")
	assert.Equal(t, http.StatusBadGateway, ce.StatusCode,
		"a transport failure must answer 502")
	assert.Contains(t, ce.DevMessage, "connection refused",
		"dev_message must preserve the transport cause")
}

// TestMapErrorNil proves MapError(nil) returns nil instead of a fabricated
// error, so callers can pass an error straight through.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestMapErrorNil(t *testing.T) {
	assert.Nil(t, MapError(nil), "nil error must map to nil")
}
