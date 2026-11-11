package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/exceptions"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubGatewayStatus is a configurable contracts.GatewayStatus for tests.
type stubGatewayStatus struct {
	enabled  bool
	enrolled bool
}

// Enabled reports whether the stub gateway integration is active.
// Returns the configured enabled flag.
func (s *stubGatewayStatus) Enabled() bool { return s.enabled }

// Enrolled reports whether the stub subject is enrolled with the gateway.
// Returns the configured enrolled flag.
func (s *stubGatewayStatus) Enrolled() bool { return s.enrolled }

// runRequireGateway serves one request through RequireGateway(status) and
// reports the recorded response plus whether the next handler ran.
//
// Parameters: t is the test harness; status is the GatewayStatus to guard with.
// Returns: the recorder and whether the next handler was called.
func runRequireGateway(t *testing.T, status contracts.GatewayStatus) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	// BuildErrorResponse only exposes dev_message outside production.
	t.Setenv("APP_ENV", "development")

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("next-handler"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/federation", nil)
	rr := httptest.NewRecorder()
	RequireGateway(status)(next).ServeHTTP(rr, req)
	return rr, called
}

// decodeCustomError unmarshals a CustomError JSON envelope from the body.
//
// Parameters: t is the test harness; body is the raw response body.
// Returns: the decoded exceptions.CustomError.
func decodeCustomError(t *testing.T, body []byte) exceptions.CustomError {
	t.Helper()
	var envelope exceptions.CustomError
	require.NoError(t, json.Unmarshal(body, &envelope),
		"the error response must be a CustomError JSON envelope")
	return envelope
}

// TestRequireGatewayDisabled covers assertion 1: with the gateway disabled
// (nil status or Enabled() false) the middleware answers 503 with the
// configure-the-gateway message and dev_message GATEWAY_NOT_CONFIGURED
// without calling the next handler.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRequireGatewayDisabled(t *testing.T) {
	const configureMsg = "This endpoint requires the federation gateway. Configure APP_GATEWAY_URL and enroll this party to use it."

	cases := map[string]contracts.GatewayStatus{
		"nil status counts as disabled": nil,
		"disabled status":               &stubGatewayStatus{enabled: false, enrolled: true},
	}

	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			rr, called := runRequireGateway(t, status)

			assert.Equal(t, http.StatusServiceUnavailable, rr.Code,
				"a disabled gateway must answer 503")
			envelope := decodeCustomError(t, rr.Body.Bytes())
			assert.Equal(t, configureMsg, envelope.ClientMessage,
				"the client message must tell the operator to configure the gateway")
			assert.Equal(t, "GATEWAY_NOT_CONFIGURED", envelope.DevMessage,
				"dev_message must be the GATEWAY_NOT_CONFIGURED code")
			assert.False(t, called,
				"the next handler must not run while the gateway is disabled")
		})
	}
}

// TestRequireGatewayNotEnrolled covers assertion 2: with the gateway enabled
// but the subject not enrolled the middleware answers 503 with dev_message
// GATEWAY_NOT_ENROLLED.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRequireGatewayNotEnrolled(t *testing.T) {
	rr, called := runRequireGateway(t, &stubGatewayStatus{enabled: true, enrolled: false})

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code,
		"missing enrollment must answer 503")
	envelope := decodeCustomError(t, rr.Body.Bytes())
	assert.Equal(t, "GATEWAY_NOT_ENROLLED", envelope.DevMessage,
		"dev_message must be the GATEWAY_NOT_ENROLLED code")
	assert.False(t, called,
		"the next handler must not run while the subject is not enrolled")
}

// TestRequireGatewayEnrolled covers assertion 3: with the gateway enabled and
// the subject enrolled the middleware calls the next handler.
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestRequireGatewayEnrolled(t *testing.T) {
	rr, called := runRequireGateway(t, &stubGatewayStatus{enabled: true, enrolled: true})

	assert.True(t, called, "the next handler must run when enrolled")
	assert.Equal(t, http.StatusOK, rr.Code, "the next handler must own the response")
	assert.Equal(t, "next-handler", rr.Body.String(),
		"the next handler must write the response body")
}
