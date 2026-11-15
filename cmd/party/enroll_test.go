package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/konsulin-care/gateway-sdk-go/gatewaytest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnrollRedeemsTokenOnceAndPrintsPartyID covers the success path.
func TestEnrollRedeemsTokenOnceAndPrintsPartyID(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	enlistEnv(t, gw)
	t.Setenv("APP_PARTY_ENROLLMENT_TOKEN", "tok-secret-123")

	var stdout, stderr bytes.Buffer
	require.NoError(t, run([]string{"enroll"}, &stdout, &stderr))
	assert.Contains(t, stdout.String(), "party-test-001")
	assert.NotContains(t, stdout.String(), "tok-secret-123")
	recs := gw.Recorded()
	require.Len(t, recs, 1)
	assert.Equal(t, "/v1/parties/party-test-001/enroll", recs[0].Path)
}

// TestEnrollBlankTokenFailsWithoutRequest covers the missing-token path.
func TestEnrollBlankTokenFailsWithoutRequest(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	enlistEnv(t, gw)
	t.Setenv("APP_PARTY_ENROLLMENT_TOKEN", "   ")

	var stdout, stderr bytes.Buffer
	err := run([]string{"enroll"}, &stdout, &stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "APP_PARTY_ENROLLMENT_TOKEN")
	assert.Empty(t, gw.Recorded())
}

// TestEnrollGatewayRejectionReturnsMappedError covers a rejected token.
func TestEnrollGatewayRejectionReturnsMappedError(t *testing.T) {
	gw := gatewaytest.NewGateway()
	t.Cleanup(gw.Close)
	enlistEnv(t, gw)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":{"code":"GW-PARTY-009",` +
			`"message":"enrollment token already used","request_id":"req-1"}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("APP_GATEWAY_URL", srv.URL)
	t.Setenv("APP_PARTY_ENROLLMENT_TOKEN", "tok-used")

	var stdout, stderr bytes.Buffer
	err := run([]string{"enroll"}, &stdout, &stderr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GW-PARTY-009")
	assert.Empty(t, stdout.String())
}
