package payments

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// settlementParty is the own party id the settlement external IDs embed.
const settlementParty = "pty-settlement-own"

// settlementBundleRecorder records every posted transaction bundle so tests
// can pin how many bundles a settlement wrote and what they contain.
type settlementBundleRecorder struct {
	bundles []map[string]any
}

// PostTransactionBundle captures the bundle and answers with an empty response.
// Parameters: ctx is the call context, bundle is the posted transaction bundle.
// Returns an empty response bundle and nil error. Never panics.
func (r *settlementBundleRecorder) PostTransactionBundle(_ context.Context, bundle map[string]any) (*fhir_dto.FHIRBundle, error) {
	r.bundles = append(r.bundles, bundle)
	return &fhir_dto.FHIRBundle{}, nil
}

// settlementPatientClient is a minimal contracts.PatientFhirClient stub that
// only answers FindPatientByID; every other method reports unsupported.
type settlementPatientClient struct {
	patient *fhir_dto.Patient
}

func (c *settlementPatientClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	return c.patient, nil
}

func (*settlementPatientClient) CreatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

func (*settlementPatientClient) UpdatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

func (*settlementPatientClient) PatchPatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

func (*settlementPatientClient) FindPatientByIdentifier(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}

func (*settlementPatientClient) FindPatientByEmail(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}

func (*settlementPatientClient) FindPatientByPhone(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}

// settlementIDs are the six canonical appointment ids the settlement tests use.
var settlementIDs = []string{"slot-1", "pr-2", "pat-3", "inv-4", "hs-5", "appt-6"}

// settlementExternalID builds the own-party appointment external ID used by
// the PAID/EXPIRED/CANCELED settlement tests.
func settlementExternalID(t *testing.T) string {
	t.Helper()
	opaque, err := BuildAppointmentOpaque(settlementIDs)
	require.NoError(t, err)
	return settlementParty + ":" + string(constvars.AppointmentPaymentService) + ":" + opaque
}

// newSettlementUcase assembles a paymentUsecase wired with the stubs the
// settlement flow touches: FHIR resource fetches answer the appointment and
// healthcare service, the bundle recorder captures writes, and the gateway
// party id matches settlementExternalID.
func newSettlementUcase(t *testing.T, slot *fhir_dto.Slot, recorder *settlementBundleRecorder) *paymentUsecase {
	t.Helper()
	appointment := &fhir_dto.Appointment{
		ResourceType: constvars.ResourceAppointment,
		ID:           settlementIDs[5],
		Status:       constvars.FhirAppointmentStatusPending,
	}
	appointmentJSON, err := json.Marshal(appointment)
	require.NoError(t, err)
	hsJSON, err := json.Marshal(&fhir_dto.HealthcareService{
		ResourceType: constvars.ResourceHealthcareService,
		ID:           settlementIDs[4],
	})
	require.NoError(t, err)

	return &paymentUsecase{
		InternalConfig: &config.InternalConfig{
			FHIR:    config.AppFHIR{BaseUrl: "http://fhir.test/"},
			App:     config.App{BaseUrl: ""},
			Gateway: config.AppGateway{PartyID: settlementParty},
		},
		FHIRClient: &mockAppointmentResourceClient{
			getResponses: map[string][]byte{
				"http://fhir.test/Appointment/appt-6":     appointmentJSON,
				"http://fhir.test/HealthcareService/hs-5": hsJSON,
			},
		},
		PatientFhirClient: &settlementPatientClient{patient: &fhir_dto.Patient{ID: settlementIDs[2]}},
		PractitionerFhirClient: &mockPractitionerFhirClient{
			practitioner: &fhir_dto.Practitioner{ID: "prac-1"},
		},
		PractitionerRoleFhirClient: &mockPractitionerRoleFhirClient{
			role: &fhir_dto.PractitionerRole{
				ID:           settlementIDs[1],
				Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-1"},
			},
		},
		InvoiceFhirClient: &mockInvoiceFhirClient{
			invoices: []fhir_dto.Invoice{{
				ID:       settlementIDs[3],
				TotalNet: &fhir_dto.Money{Value: 150000, Currency: "IDR"},
			}},
		},
		SlotFhirClient:     &mockSlotFhirClient{slot: slot},
		ScheduleFhirClient: &mockScheduleFhirClient{},
		BundleFhirClient:   recorder,
		Log:                zap.NewNop(),
	}
}

// settlementFindEntry returns the first bundle entry whose request URL matches.
func settlementFindEntry(bundle map[string]any, url string) map[string]any {
	entries, ok := bundle[constvars.FhirFieldEntry].([]map[string]any)
	if !ok {
		return nil
	}
	for _, entry := range entries {
		req, ok := entry[constvars.FhirFieldRequest].(map[string]any)
		if !ok {
			continue
		}
		if entryURL, _ := req[constvars.FhirFieldURL].(string); entryURL == url {
			return entry
		}
	}
	return nil
}

// settlementFindResourceEntry returns the first bundle entry carrying a typed
// resource of the given FHIR resource type (e.g. PaymentNotice).
func settlementFindResourceEntry(bundle map[string]any, resourceType string) map[string]any {
	entries, ok := bundle[constvars.FhirFieldEntry].([]map[string]any)
	if !ok {
		return nil
	}
	for _, entry := range entries {
		switch resource := entry[constvars.FhirFieldResource].(type) {
		case *fhir_dto.PaymentNotice:
			if resourceType == constvars.ResourcePaymentNotice {
				return entry
			}
		case fhir_dto.PaymentNotice:
			if resourceType == constvars.ResourcePaymentNotice {
				return entry
			}
		case *fhir_dto.PaymentReconciliation:
			if resourceType == constvars.ResourcePaymentReconciliation {
				return entry
			}
		case fhir_dto.PaymentReconciliation:
			if resourceType == constvars.ResourcePaymentReconciliation {
				return entry
			}
		case map[string]any:
			if rt, _ := resource[constvars.FhirFieldResourceType].(string); rt == resourceType {
				return entry
			}
		}
	}
	return nil
}

// settlementSlot builds the tentative slot the booking flow left behind.
func settlementSlot(status fhir_dto.SlotStatus) *fhir_dto.Slot {
	return &fhir_dto.Slot{
		ID:       settlementIDs[0],
		Status:   status,
		Start:    time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC),
		Schedule: fhir_dto.Reference{Reference: "Schedule/sched-1"},
	}
}

// TestHandleSettlement_PaidAppointmentBooksSlotAndWritesPaymentArtifacts pins
// the first contract assertion: a PAID appointment settlement sets the slot
// busy, books the appointment and creates PaymentNotice and
// PaymentReconciliation in one transaction bundle.
func TestHandleSettlement_PaidAppointmentBooksSlotAndWritesPaymentArtifacts(t *testing.T) {
	recorder := &settlementBundleRecorder{}
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), recorder)

	err := uc.HandleSettlement(context.Background(), SettlementEvent{
		PaymentID:  "pay-1",
		ExternalID: settlementExternalID(t),
		Status:     settlementStatusPaid,
		Amount:     150000,
		Currency:   "IDR",
	})
	require.NoError(t, err)

	require.Len(t, recorder.bundles, 1, "the settlement must post exactly one bundle")
	bundle := recorder.bundles[0]
	assert.Equal(t, "transaction", bundle["type"], "the bundle must be a transaction bundle")

	slotEntry := settlementFindEntry(bundle, constvars.ResourceSlot+"/"+settlementIDs[0])
	require.NotNil(t, slotEntry, "the bundle must update the slot")
	slotResource, ok := slotEntry[constvars.FhirFieldResource].(*fhir_dto.Slot)
	require.True(t, ok, "the slot entry must carry a Slot resource")
	assert.Equal(t, fhir_dto.SlotStatusBusy, slotResource.Status,
		"the slot must be set to busy, not busy-unavailable")

	apptEntry := settlementFindEntry(bundle, constvars.ResourceAppointment+"/"+settlementIDs[5])
	require.NotNil(t, apptEntry, "the bundle must update the appointment")
	apptResource, ok := apptEntry[constvars.FhirFieldResource].(*fhir_dto.Appointment)
	require.True(t, ok, "the appointment entry must carry an Appointment resource")
	assert.Equal(t, constvars.FhirAppointmentStatusBooked, apptResource.Status,
		"the appointment must be booked")

	noticeEntry := settlementFindResourceEntry(bundle, constvars.ResourcePaymentNotice)
	require.NotNil(t, noticeEntry, "the same bundle must carry a PaymentNotice")
	_, ok = noticeEntry[constvars.FhirFieldResource].(fhir_dto.PaymentNotice)
	require.True(t, ok, "the payment notice entry must carry a PaymentNotice resource")

	reconEntry := settlementFindResourceEntry(bundle, constvars.ResourcePaymentReconciliation)
	require.NotNil(t, reconEntry, "the same bundle must carry a PaymentReconciliation")
	_, ok = reconEntry[constvars.FhirFieldResource].(fhir_dto.PaymentReconciliation)
	require.True(t, ok, "the reconciliation entry must carry a PaymentReconciliation resource")
}

// TestHandleSettlement_PaidAlreadyBusySlotIsIdempotent pins the second
// contract assertion: a PAID settlement for a slot that is already busy
// succeeds without writing.
func TestHandleSettlement_PaidAlreadyBusySlotIsIdempotent(t *testing.T) {
	recorder := &settlementBundleRecorder{}
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusy), recorder)

	err := uc.HandleSettlement(context.Background(), SettlementEvent{
		PaymentID:  "pay-2",
		ExternalID: settlementExternalID(t),
		Status:     settlementStatusPaid,
		Amount:     150000,
		Currency:   "IDR",
	})
	require.NoError(t, err, "an already-settled slot must acknowledge without error")
	assert.Empty(t, recorder.bundles, "an already-busy slot must produce no writes")
}

// TestHandleSettlement_ExpiredAndCanceledDeleteBookingResources pins the third
// contract assertion: an EXPIRED or CANCELED settlement deletes the slot,
// invoice and appointment in one transaction bundle.
func TestHandleSettlement_ExpiredAndCanceledDeleteBookingResources(t *testing.T) {
	for _, status := range []string{settlementStatusExpired, settlementStatusCanceled} {
		t.Run(status, func(t *testing.T) {
			recorder := &settlementBundleRecorder{}
			uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), recorder)

			err := uc.HandleSettlement(context.Background(), SettlementEvent{
				PaymentID:  "pay-3",
				ExternalID: settlementExternalID(t),
				Status:     status,
				Amount:     150000,
				Currency:   "IDR",
			})
			require.NoError(t, err)

			require.Len(t, recorder.bundles, 1, "the settlement must post exactly one bundle")
			entries, ok := recorder.bundles[0][constvars.FhirFieldEntry].([]map[string]any)
			require.True(t, ok, "the bundle must carry an entry list")
			require.Len(t, entries, 3, "the delete bundle must carry exactly three entries")

			wantURLs := []string{
				constvars.ResourceSlot + "/" + settlementIDs[0],
				constvars.ResourceInvoice + "/" + settlementIDs[3],
				constvars.ResourceAppointment + "/" + settlementIDs[5],
			}
			for i, wantURL := range wantURLs {
				req, ok := entries[i][constvars.FhirFieldRequest].(map[string]any)
				require.True(t, ok, "entry %d must carry a request", i)
				assert.Equal(t, constvars.MethodDelete, req[constvars.FhirFieldMethod],
					"entry %d must be a DELETE", i)
				assert.Equal(t, wantURL, req[constvars.FhirFieldURL],
					"entry %d must target the right resource", i)
			}
		})
	}
}

// TestHandleSettlement_ListingDomainAcknowledgesWithoutWrites pins that a
// non-appointment (listing) settlement is acknowledged with no FHIR writes.
func TestHandleSettlement_ListingDomainAcknowledgesWithoutWrites(t *testing.T) {
	recorder := &settlementBundleRecorder{}
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), recorder)

	err := uc.HandleSettlement(context.Background(), SettlementEvent{
		PaymentID:  "pay-4",
		ExternalID: settlementParty + ":listing:lst-1",
		Status:     settlementStatusPaid,
		Amount:     50000,
		Currency:   "IDR",
	})
	require.NoError(t, err, "listing settlements must be acknowledged")
	assert.Empty(t, recorder.bundles, "listing settlements must perform no FHIR writes")
}
