package federation

import (
	"context"
	"crypto/ecdh"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/konsulin-care/gateway-sdk-go/envelope"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPepperDeliverySealsPepperToCallerEncKey covers T46A assertion 1: a
// federation-pepper delivery is answered with the pepper sealed to the
// caller's use:enc key as application/jose. The handler runs with injected
// fakes: GetSub returns the caller party, ResolveKey records the party id
// it was asked for and returns the caller key, Pepper returns the
// plaintext.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestPepperDeliverySealsPepperToCallerEncKey(t *testing.T) {
	callerKey, _ := partyEncKey(t)
	pepper := []byte("pepper-secret")

	var gotPartyID string
	handler := NewPepperDelivery(PepperDeliveryDeps{
		GetSub: func(*http.Request) string { return "pty-caller" },
		ResolveKey: func(_ context.Context, partyID string) (*ecdh.PublicKey, string, error) {
			gotPartyID = partyID
			return callerKey.PublicKey(), "test-enc-kid", nil
		},
		Pepper: func() ([]byte, error) { return pepper, nil },
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost,
		"/api/v1/federation/deliveries/"+pepperListingName, nil))

	require.Equal(t, http.StatusOK, rec.Code,
		"a resolvable caller key and an available pepper must be answered 200")
	assert.Equal(t, "pty-caller", gotPartyID,
		"the resolver must be asked for the party id GetSub returned")
	assert.Equal(t, "application/jose", rec.Header().Get("Content-Type"),
		"the sealed pepper must be answered as application/jose")
	opened, err := envelope.Open(rec.Body.String(), callerKey)
	require.NoError(t, err,
		"the body must open with the caller's use:enc private key")
	assert.Equal(t, pepper, opened,
		"the sealed body must carry the pepper")
}

// TestPepperDeliveryDeniesCallerWithoutEncKey covers T46B assertion 1: a
// caller whose key cannot be resolved (a party JWKS without a use:enc key)
// is denied with 403 — the delivery fails closed before any pepper is
// loaded or sealed. The handler runs with injected fakes: ResolveKey
// returns an error, Pepper would return a plaintext but must never be
// reached.
//
// Parameters: t is the test harness.
// Returns: nothing; fails on any deviation.
func TestPepperDeliveryDeniesCallerWithoutEncKey(t *testing.T) {
	pepper := []byte("pepper-secret")
	pepperCalled := false
	handler := NewPepperDelivery(PepperDeliveryDeps{
		GetSub: func(*http.Request) string { return "pty-caller" },
		ResolveKey: func(context.Context, string) (*ecdh.PublicKey, string, error) {
			return nil, "", errors.New("party JWKS has no use:enc key")
		},
		Pepper: func() ([]byte, error) {
			pepperCalled = true
			return pepper, nil
		},
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost,
		"/api/v1/federation/deliveries/"+pepperListingName, nil))

	require.Equal(t, http.StatusForbidden, rec.Code,
		"a caller without a use:enc key must be denied with 403")
	assert.False(t, pepperCalled,
		"the pepper must not be loaded when key resolution fails")
}

// TestPepperDeliveryAnswers503WithoutAppPepper covers T46B assertion 2: an
// instance with APP_PEPPER unset answers 503 when the caller's key
// resolves. The handler runs with the real envPepper seam (EnvPepper)
// under an empty APP_PEPPER, plus a fake ResolveKey that succeeds.
//
// Parameters: t is the test harness; it sets APP_PEPPER to empty for the
// duration of the test.
// Returns: nothing; fails on any deviation.
func TestPepperDeliveryAnswers503WithoutAppPepper(t *testing.T) {
	t.Setenv("APP_PEPPER", "")
	callerKey, _ := partyEncKey(t)
	handler := NewPepperDelivery(PepperDeliveryDeps{
		GetSub: func(*http.Request) string { return "pty-caller" },
		ResolveKey: func(context.Context, string) (*ecdh.PublicKey, string, error) {
			return callerKey.PublicKey(), "test-enc-kid", nil
		},
		Pepper: EnvPepper,
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost,
		"/api/v1/federation/deliveries/"+pepperListingName, nil))

	require.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"an instance without APP_PEPPER must answer 503")
}
