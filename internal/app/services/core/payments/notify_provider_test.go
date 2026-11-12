package payments

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// webhookEnqueueCall captures one contracts.WebhookClient.Enqueue invocation
// so tests can pin the recipient party, listing name, user ref and payload.
type webhookEnqueueCall struct {
	to      string
	service string
	userRef string
	data    []byte
}

// recordingWebhookClient is a contracts.WebhookClient double that records
// every Enqueue into a buffered channel; Sync and Status answer trivially.
type recordingWebhookClient struct {
	enqueued chan webhookEnqueueCall
}

// newRecordingWebhookClient builds the recording double with a buffered
// channel so the settlement's notification goroutine never blocks.
func newRecordingWebhookClient() *recordingWebhookClient {
	return &recordingWebhookClient{enqueued: make(chan webhookEnqueueCall, 8)}
}

var _ contracts.WebhookClient = (*recordingWebhookClient)(nil)

// Enqueue records the invocation for the test to assert on.
// Parameters: ctx bounds the call, to is the recipient party, service the
// listing name, userRef the peppered end-user hash, data the payload bytes.
// Returns nil. Never panics.
func (c *recordingWebhookClient) Enqueue(_ context.Context, to, service, userRef string, data []byte) error {
	c.enqueued <- webhookEnqueueCall{
		to:      to,
		service: service,
		userRef: userRef,
		data:    append([]byte(nil), data...),
	}
	return nil
}

// Sync answers with an empty reply; the payment flows never call it.
// Parameters: ctx bounds the call, to the recipient party, service the listing
// name, userRef the peppered end-user hash, data the payload bytes.
// Returns nil bytes and nil error. Never panics.
func (*recordingWebhookClient) Sync(context.Context, string, string, string, []byte) ([]byte, error) {
	return nil, nil
}

// Status answers "delivered"; the payment flows never call it.
// Parameters: ctx bounds the call, id the enqueued webhook id.
// Returns "delivered" and nil error. Never panics.
func (*recordingWebhookClient) Status(context.Context, string) (string, error) {
	return "delivered", nil
}

// notifyProviderUcase wires the settlement usecase with an enabled gateway,
// the automation party id and the recording webhook client, plus a patient
// carrying the display name and contacts the notification payload repeats.
func notifyProviderUcase(t *testing.T, gatewayEnabled bool, automationPartyID string) (*paymentUsecase, *recordingWebhookClient) {
	t.Helper()
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), &settlementBundleRecorder{})
	uc.InternalConfig.Gateway.Enabled = gatewayEnabled
	uc.InternalConfig.Gateway.AutomationPartyID = automationPartyID
	patient := &fhir_dto.Patient{ID: settlementIDs[2]}
	patient.Name = []fhir_dto.HumanName{{Use: "official", Text: "Jane Doe"}}
	patient.Telecom = []fhir_dto.ContactPoint{
		{System: fhir_dto.ContactPointSystemPhone, Value: "+62811111111"},
		{System: fhir_dto.ContactPointSystemEmail, Value: "jane@example.test"},
	}
	uc.PatientFhirClient = &settlementPatientClient{patient: patient}
	hooks := newRecordingWebhookClient()
	uc.Gateway = GatewayDeps{Webhooks: hooks, AutomationPartyID: automationPartyID}
	return uc, hooks
}

// notifyProviderPaidEvent builds the PAID settlement event for the canonical
// appointment ids.
func notifyProviderPaidEvent(t *testing.T) SettlementEvent {
	t.Helper()
	return SettlementEvent{
		PaymentID:  "pay-notify",
		ExternalID: settlementExternalID(t),
		Status:     settlementStatusPaid,
		Amount:     150000,
		Currency:   "IDR",
	}
}

// requireNoEnqueue asserts that no notification was enqueued within wait.
func requireNoEnqueue(t *testing.T, hooks *recordingWebhookClient, wait time.Duration) {
	t.Helper()
	select {
	case call := <-hooks.enqueued:
		t.Fatalf("expected the notification to be skipped, got enqueue to=%q service=%q userRef=%q",
			call.to, call.service, call.userRef)
	case <-time.After(wait):
	}
}

// TestHandleSettlement_PaidEnqueuesNotifyProviderToAutomationParty pins the
// first contract assertion: after a PAID settlement the practitioner
// notification is enqueued through the gateway WebhookClient to the
// automation party's notify-provider listing with an empty user_ref and the
// practitioner/payment payload fields.
func TestHandleSettlement_PaidEnqueuesNotifyProviderToAutomationParty(t *testing.T) {
	uc, hooks := notifyProviderUcase(t, true, "pty-automation")

	err := uc.HandleSettlement(context.Background(), notifyProviderPaidEvent(t))
	require.NoError(t, err, "a PAID settlement must apply without error")

	select {
	case call := <-hooks.enqueued:
		assert.Equal(t, "pty-automation", call.to, "the notification must target the automation party")
		assert.Equal(t, "notify-provider", call.service, "the notification must list the notify-provider listing")
		assert.Equal(t, "", call.userRef, "the notification is party-scoped, so user_ref stays empty")

		var payload map[string]any
		require.NoError(t, json.Unmarshal(call.data, &payload), "the payload must be valid JSON")
		assert.Equal(t, "Jane Doe", payload["patientName"], "the payload must carry the patient name")
		assert.Equal(t, "IDR 150000", payload["amount"], "the payload must carry the invoiced amount")
		assert.Equal(t, "IDR 150000", payload["amountPaid"], "the payload must carry the paid amount")
		assert.Equal(t, "2026-08-04T09:00:00Z", payload["timeSlotStart"], "the payload must carry the slot start")
		assert.Equal(t, "2026-08-04T10:00:00Z", payload["timeSlotEnd"], "the payload must carry the slot end")

		paymentDate, ok := payload["paymentDate"].(string)
		require.True(t, ok, "paymentDate must be a string")
		_, parseErr := time.Parse(time.RFC3339, paymentDate)
		require.NoError(t, parseErr, "paymentDate must be an RFC3339 timestamp")

		contact, ok := payload["contact"].(map[string]any)
		require.True(t, ok, "contact must be an object")
		assert.Equal(t, "+62811111111", contact["phone"], "contact must carry the patient phone")
		assert.Equal(t, "jane@example.test", contact["email"], "contact must carry the patient email")
	case <-time.After(3 * time.Second):
		t.Fatal("the practitioner notification was never enqueued to the automation party")
	}
}

// TestHandleSettlement_PaidSkipsNotifyProviderWithoutGatewayOrParty pins the
// second contract assertion: the notification is skipped without error when
// the gateway is disabled or APP_AUTOMATION_PARTY_ID is empty.
func TestHandleSettlement_PaidSkipsNotifyProviderWithoutGatewayOrParty(t *testing.T) {
	cases := []struct {
		name            string
		gatewayEnabled  bool
		automationParty string
	}{
		{name: "gateway disabled", gatewayEnabled: false, automationParty: "pty-automation"},
		{name: "automation party unset", gatewayEnabled: true, automationParty: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, hooks := notifyProviderUcase(t, tc.gatewayEnabled, tc.automationParty)

			err := uc.HandleSettlement(context.Background(), notifyProviderPaidEvent(t))
			require.NoError(t, err, "the settlement must succeed when the notification is skipped")

			// The notification runs on a goroutine; give it a chance to fire
			// before asserting that nothing was enqueued.
			requireNoEnqueue(t, hooks, 300*time.Millisecond)
		})
	}
}
