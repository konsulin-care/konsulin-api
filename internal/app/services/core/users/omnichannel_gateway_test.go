package users

import (
	"context"
	"encoding/json"
	"testing"

	"konsulin-service/internal/app/services/core/federation"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// gatewaySyncCall records one gateway Sync invocation for assertions.
type gatewaySyncCall struct {
	to      string
	service string
	userRef string
	data    []byte
}

// fakeGatewayWebhookClient is an in-memory GatewayWebhookClient: it records
// Sync calls and answers with the configured reply, so no gateway I/O runs.
type fakeGatewayWebhookClient struct {
	enabled bool
	syncs   []gatewaySyncCall
	reply   []byte
}

// Enqueue records nothing and succeeds; the profile sync path never enqueues.
func (f *fakeGatewayWebhookClient) Enqueue(context.Context, string, string, string, []byte) error {
	return nil
}

// Sync records the invocation and returns the configured reply.
func (f *fakeGatewayWebhookClient) Sync(_ context.Context, to, service, userRef string, data []byte) ([]byte, error) {
	f.syncs = append(f.syncs, gatewaySyncCall{to: to, service: service, userRef: userRef, data: data})
	return f.reply, nil
}

// Status is unused by the profile sync; it answers an empty state.
func (f *fakeGatewayWebhookClient) Status(context.Context, string) (string, error) {
	return "", nil
}

// Enabled reports the configured gateway availability.
func (f *fakeGatewayWebhookClient) Enabled() bool { return f.enabled }

// TestCallWebhookSvcKonsulinOmnichannel_GatewaySync pins the first contract
// line: profile sync performs a sync automation/modify-profile invocation
// through the gateway WebhookClient (data = the existing body, user_ref =
// the user's peppered hash) and parses the chatwoot id from the reply.
func TestCallWebhookSvcKonsulinOmnichannel_GatewaySync(t *testing.T) {
	// Isolate the pepper loader: no sealed artifact, APP_PEPPER fallback only.
	t.Setenv("APP_DATA_DIR", t.TempDir())
	t.Setenv("APP_PEPPER", "0123456789abcdef0123456789abcdef")

	fake := &fakeGatewayWebhookClient{
		enabled: true,
		reply:   []byte(`[{"chatwoot_id":42,"email":"person@example.com","phoneNumber":"+6281234567890"}]`),
	}
	uc := &userUsecase{
		Log:                      zap.NewNop(),
		gatewayWebhooks:          fake,
		gatewayAutomationPartyID: "pty-automation",
	}

	out, err := uc.callWebhookSvcKonsulinOmnichannel(context.Background(), callWebhookSvcKonsulinOmnichannelInput{
		Email:    "person@example.com",
		Username: "person",
		Phone:    "6281234567890",
	})
	require.NoError(t, err)
	assert.Equal(t, 42, out.ChatwootID, "the chatwoot id must come from the gateway reply")
	assert.Equal(t, "person@example.com", out.Email)
	assert.Equal(t, "+6281234567890", out.PhoneNumber)

	require.Len(t, fake.syncs, 1, "exactly one sync invocation must happen")
	call := fake.syncs[0]
	assert.Equal(t, "pty-automation", call.to, "the sync must target the automation party")
	assert.Equal(t, "modify-profile", call.service, "the sync must list the automation modify-profile listing")

	expectedRef, err := federation.UserRef("person@example.com", "6281234567890")
	require.NoError(t, err)
	assert.NotEmpty(t, call.userRef, "user_ref must be the peppered end-user hash")
	assert.Equal(t, expectedRef, call.userRef)

	var payload struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Phone string `json:"phoneNumber"`
	}
	require.NoError(t, json.Unmarshal(call.data, &payload), "data must be the existing body")
	assert.Equal(t, "person@example.com", payload.Email)
	assert.Equal(t, "person", payload.Name)
	assert.Equal(t, "+6281234567890", payload.Phone)
}

// TestCallWebhookSvcKonsulinOmnichannel_SkippedWhenGatewayUnavailable pins
// the second contract line: profile sync is skipped without error when the
// gateway is disabled or the automation party is unset — no gateway Sync and
// no legacy forwarder call happens either.
func TestCallWebhookSvcKonsulinOmnichannel_SkippedWhenGatewayUnavailable(t *testing.T) {
	cases := []struct {
		name   string
		client *fakeGatewayWebhookClient
		party  string
	}{
		{name: "gateway disabled", client: &fakeGatewayWebhookClient{enabled: false}, party: "pty-automation"},
		{name: "automation party unset", client: &fakeGatewayWebhookClient{enabled: true}, party: ""},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			uc := &userUsecase{
				Log:                      zap.NewNop(),
				gatewayWebhooks:          tt.client,
				gatewayAutomationPartyID: tt.party,
				webhookForwardFn: func(context.Context, string, string, []byte, string) (int, []byte, error) {
					t.Fatal("the legacy forwarder must not run while the gateway path is wired")
					return 0, nil, nil
				},
			}

			out, err := uc.callWebhookSvcKonsulinOmnichannel(context.Background(), callWebhookSvcKonsulinOmnichannelInput{
				Email: "person@example.com",
			})
			require.NoError(t, err, "a skipped profile sync must not surface an error")
			assert.Zero(t, out.ChatwootID, "a skipped profile sync returns the zero output")
			assert.Empty(t, tt.client.syncs, "no gateway sync may happen")
		})
	}
}
