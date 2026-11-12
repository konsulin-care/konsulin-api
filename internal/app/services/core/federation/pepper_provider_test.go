package federation

import (
	"context"
	"crypto/ecdh"
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
