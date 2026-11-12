package payments

import (
	"context"
	"testing"
	"time"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// reconcilerPaymentClient is a stub contracts.PaymentClient that answers
// GetPayment with a fixed status and records every payment id read.
type reconcilerPaymentClient struct {
	status string
	err    error
	reads  []string
}

// CreateSellerPayment is never called by the reconciler and fails the test
// run if it ever is.
// Parameters: none reach the test — the call is unexpected.
// Returns empty strings and an error. Never panics.
func (*reconcilerPaymentClient) CreateSellerPayment(context.Context, string, string, int64, string, string, string) (string, string, error) {
	return "", "", assert.AnError
}

// CreateListingPayment is never called by the reconciler and fails the test
// run if it ever is.
// Parameters: none reach the test — the call is unexpected.
// Returns empty strings and an error. Never panics.
func (*reconcilerPaymentClient) CreateListingPayment(context.Context, string, string) (string, string, error) {
	return "", "", assert.AnError
}

// GetPayment records the payment id and answers the configured status.
// Parameters: _ is the call context, paymentID is the payment to read.
// Returns the configured status and error. Never panics.
func (f *reconcilerPaymentClient) GetPayment(_ context.Context, paymentID string) (string, error) {
	f.reads = append(f.reads, paymentID)
	return f.status, f.err
}

// reconcilerScannerStub returns a fixed candidate list and records how many
// times the scan seam was invoked.
type reconcilerScannerStub struct {
	candidates []SettlementCandidate
	err        error
	calls      int
}

// StaleSettlementCandidates records the call and answers the fixed list.
// Parameters: _ is the call context.
// Returns the configured candidates and error. Never panics.
func (s *reconcilerScannerStub) StaleSettlementCandidates(_ context.Context) ([]SettlementCandidate, error) {
	s.calls++
	return s.candidates, s.err
}

// newReconcilerCandidate builds the one stale busy-tentative candidate the
// settlement tests use: the canonical appointment external ID the real
// settlement sink routes on, plus a gateway payment id and the IDR amount.
func newReconcilerCandidate(t *testing.T) SettlementCandidate {
	t.Helper()
	return SettlementCandidate{
		SlotID:     settlementIDs[0],
		PaymentID:  "pay-reconcile-1",
		ExternalID: settlementExternalID(t),
		Amount:     150000,
		Currency:   "IDR",
	}
}

// TestSettlementReconciler_PaidCandidateSettlesSlotToBusy covers the first
// contract assertion: a stale busy-tentative slot whose gateway payment reads
// PAID is dispatched to HandleSettlement, which settles the slot to busy in
// one transaction bundle.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestSettlementReconciler_PaidCandidateSettlesSlotToBusy(t *testing.T) {
	recorder := &settlementBundleRecorder{}
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), recorder)
	payments := &reconcilerPaymentClient{status: settlementStatusPaid}
	scanner := &reconcilerScannerStub{candidates: []SettlementCandidate{newReconcilerCandidate(t)}}

	reconciler := NewSettlementReconciler(payments, uc, scanner, time.Minute, zap.NewNop())
	reconciler.ReconcileOnce(context.Background())

	require.Equal(t, []string{"pay-reconcile-1"}, payments.reads,
		"the reconciler must read the candidate's gateway payment")
	require.Len(t, recorder.bundles, 1, "the PAID settlement must post exactly one bundle")

	slotEntry := settlementFindEntry(recorder.bundles[0], "Slot/"+settlementIDs[0])
	require.NotNil(t, slotEntry, "the bundle must update the slot")
	slotResource, ok := slotEntry[constvars.FhirFieldResource].(*fhir_dto.Slot)
	require.True(t, ok, "the slot entry must carry a Slot resource")
	assert.Equal(t, fhir_dto.SlotStatusBusy, slotResource.Status,
		"a PAID gateway payment must settle the stale slot to busy")
}

// TestSettlementReconciler_PendingCandidateLeavesSlotUnchanged covers the
// second contract assertion: a slot whose gateway payment is still PENDING
// is left unchanged — the payment is read but no settlement is dispatched
// and no FHIR write happens.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestSettlementReconciler_PendingCandidateLeavesSlotUnchanged(t *testing.T) {
	recorder := &settlementBundleRecorder{}
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), recorder)
	payments := &reconcilerPaymentClient{status: "PENDING"}
	scanner := &reconcilerScannerStub{candidates: []SettlementCandidate{newReconcilerCandidate(t)}}

	reconciler := NewSettlementReconciler(payments, uc, scanner, time.Minute, zap.NewNop())
	reconciler.ReconcileOnce(context.Background())

	require.Equal(t, []string{"pay-reconcile-1"}, payments.reads,
		"the reconciler must read the candidate's gateway payment")
	assert.Empty(t, recorder.bundles,
		"a PENDING gateway payment must leave the slot unchanged (no FHIR writes)")
}

// TestStartSettlementReconciler_NotStartedWhenGatewayDisabled covers the
// third contract assertion: with the gateway disabled nothing starts — no
// goroutine, no scan, no gateway read — and nil is returned.
// Parameters: t is the test harness.
// Returns nothing; fails on any deviation.
func TestStartSettlementReconciler_NotStartedWhenGatewayDisabled(t *testing.T) {
	payments := &reconcilerPaymentClient{status: settlementStatusPaid}
	scanner := &reconcilerScannerStub{candidates: []SettlementCandidate{newReconcilerCandidate(t)}}
	uc := newSettlementUcase(t, settlementSlot(fhir_dto.SlotStatusBusyTentative), &settlementBundleRecorder{})

	started := StartSettlementReconciler(context.Background(), false, payments, uc, scanner, time.Millisecond, zap.NewNop())
	require.Nil(t, started, "a disabled gateway must not start the reconciler")

	// Grace period: a wrongly started loop would scan and read now.
	time.Sleep(50 * time.Millisecond)
	assert.Zero(t, scanner.calls, "a disabled gateway must never scan for candidates")
	assert.Empty(t, payments.reads, "a disabled gateway must never read payments")
}

// compile-time check: the stub satisfies the payment client contract the
// reconciler reads through.
var _ contracts.PaymentClient = (*reconcilerPaymentClient)(nil)
