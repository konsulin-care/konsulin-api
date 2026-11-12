package auth

import (
	"context"
	"encoding/json"
	"testing"

	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatewayEnqueueCall records one Enqueue invocation on the fake client.
type gatewayEnqueueCall struct {
	to      string
	service string
	userRef string
	payload []byte
}

// fakeGatewayWebhookClient is an in-memory GatewayWebhookClient: it records
// Enqueue calls and reports the enabled flag it was built with.
type fakeGatewayWebhookClient struct {
	enabled bool
	calls   []gatewayEnqueueCall
	err     error
}

// Enqueue records the call and returns the configured error.
func (f *fakeGatewayWebhookClient) Enqueue(_ context.Context, to, service, userRef string, data []byte) error {
	f.calls = append(f.calls, gatewayEnqueueCall{
		to:      to,
		service: service,
		userRef: userRef,
		payload: append([]byte(nil), data...),
	})
	return f.err
}

// Sync is unused by the sender; it satisfies contracts.WebhookClient.
func (f *fakeGatewayWebhookClient) Sync(context.Context, string, string, string, []byte) ([]byte, error) {
	return nil, nil
}

// Status is unused by the sender; it satisfies contracts.WebhookClient.
func (f *fakeGatewayWebhookClient) Status(context.Context, string) (string, error) {
	return "", nil
}

// Enabled reports the configured gateway state.
func (f *fakeGatewayWebhookClient) Enabled() bool { return f.enabled }

// TestGatewayMagicLinkSender_EnqueuesToAutomationParty verifies the first
// contract line: SendMagicLink enqueues send-magiclink to the automation
// party with the link and either the email or the phone number.
func TestGatewayMagicLinkSender_EnqueuesToAutomationParty(t *testing.T) {
	tests := []struct {
		name    string
		in      contracts.SendMagicLinkInput
		wantKey string
		wantVal string
		// otherKey is the contact key that must be absent from the payload.
		otherKey string
	}{
		{
			name: "email destination",
			in: contracts.SendMagicLinkInput{
				URL:   "https://app.example/magic?token=abc",
				Email: "user@example.com",
			},
			wantKey:  "email",
			wantVal:  "user@example.com",
			otherKey: "phoneNumber",
		},
		{
			name: "phone destination",
			in: contracts.SendMagicLinkInput{
				URL:   "https://app.example/magic?token=abc",
				Phone: "628111111111",
			},
			wantKey:  "phoneNumber",
			wantVal:  "628111111111",
			otherKey: "email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeGatewayWebhookClient{enabled: true}
			sender := NewGatewayMagicLinkSender(client, config.AppGateway{
				Enabled:           true,
				AutomationPartyID: "01HZAUTOMATIONPARTY0000000",
			})

			err := sender.SendMagicLink(context.Background(), tt.in)
			require.NoError(t, err)

			require.Len(t, client.calls, 1, "exactly one enqueue expected")
			call := client.calls[0]
			assert.Equal(t, "01HZAUTOMATIONPARTY0000000", call.to, "enqueue must target the automation party")
			assert.Equal(t, "send-magiclink", call.service, "enqueue must use the send-magiclink listing")
			assert.Empty(t, call.userRef, "magiclink listing is party-scoped, user_ref stays empty")

			var payload map[string]any
			require.NoError(t, json.Unmarshal(call.payload, &payload))
			assert.Equal(t, tt.in.URL, payload["url"], "payload must carry the link")
			assert.Equal(t, tt.wantVal, payload[tt.wantKey], "payload must carry the destination")
			_, hasOther := payload[tt.otherKey]
			assert.False(t, hasOther, "payload must not carry both contact fields")
		})
	}
}

// TestGatewayMagicLinkSender_UnavailableWhenDisabledOrAutomationUnset
// verifies the second contract line: SendMagicLink returns
// contracts.ErrMagicLinkUnavailable (and enqueues nothing) when the gateway
// is disabled or APP_AUTOMATION_PARTY_ID is unset.
func TestGatewayMagicLinkSender_UnavailableWhenDisabledOrAutomationUnset(t *testing.T) {
	tests := []struct {
		name    string
		client  *fakeGatewayWebhookClient
		gateway config.AppGateway
	}{
		{
			name:   "gateway disabled",
			client: &fakeGatewayWebhookClient{enabled: false},
			gateway: config.AppGateway{
				Enabled:           false,
				AutomationPartyID: "01HZAUTOMATIONPARTY0000000",
			},
		},
		{
			name:   "automation party unset",
			client: &fakeGatewayWebhookClient{enabled: true},
			gateway: config.AppGateway{
				Enabled:           true,
				AutomationPartyID: "",
			},
		},
		{
			name:   "automation party whitespace only",
			client: &fakeGatewayWebhookClient{enabled: true},
			gateway: config.AppGateway{
				Enabled:           true,
				AutomationPartyID: "   ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := NewGatewayMagicLinkSender(tt.client, tt.gateway)

			err := sender.SendMagicLink(context.Background(), contracts.SendMagicLinkInput{
				URL:   "https://app.example/magic?token=abc",
				Email: "user@example.com",
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, contracts.ErrMagicLinkUnavailable)
			assert.Empty(t, tt.client.calls, "no enqueue may happen when delivery is unavailable")
		})
	}
}
