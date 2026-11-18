package payments

import (
	"context"
	"strings"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/contracts"

	"go.uber.org/zap"
)

// settlementReconcileInterval is the default period between reconciliation
// passes of the settlement reconciler.
const settlementReconcileInterval = time.Minute

// SettlementCandidate is one stale busy-tentative slot whose invoice carries
// a gateway payment id: SlotID identifies the FHIR slot, PaymentID is the
// gateway payment_id recorded on the invoice (system
// urn:konsulin:gateway-payment), ExternalID is the own-party appointment
// external ID HandleSettlement routes on, and Amount/Currency describe the
// settled money (informational).
type SettlementCandidate struct {
	SlotID     string
	PaymentID  string
	ExternalID string
	Amount     float64
	Currency   string
}

// SettlementScanner supplies the stale busy-tentative candidates for one
// reconciliation pass. It is the injectable scan seam: production wires a
// FHIR-backed scan, tests supply a stub.
type SettlementScanner interface {
	// StaleSettlementCandidates returns the busy-tentative slots whose
	// invoice carries a gateway payment id and whose payment window has
	// expired.
	// Parameters: ctx bounds the scan.
	// Returns the candidates and the error explaining a failed scan.
	StaleSettlementCandidates(ctx context.Context) ([]SettlementCandidate, error)
}

// SettlementReconciler is the periodic job that settles stale
// busy-tentative slots: each pass scans candidates, reads the gateway
// payment status through PaymentClient.GetPayment and dispatches terminal
// statuses (PAID, EXPIRED, CANCELED) to the payment usecase's
// HandleSettlement. PENDING (or any non-terminal status) leaves the slot
// unchanged.
type SettlementReconciler struct {
	payments contracts.PaymentClient
	sink     SettlementEventSink
	scanner  SettlementScanner
	interval time.Duration
	logger   *zap.Logger
}

// NewSettlementReconciler builds the periodic settlement reconciler.
//
// Parameters:
//   - payments reads the gateway payment status (GetPayment).
//   - sink applies terminal settlements (HandleSettlement).
//   - scanner supplies the stale busy-tentative candidates.
//   - interval is the period between passes; <= 0 selects the default.
//   - logger reports pass outcomes; nil selects a no-op logger.
//
// Returns the reconciler. Never panics.
func NewSettlementReconciler(
	payments contracts.PaymentClient,
	sink SettlementEventSink,
	scanner SettlementScanner,
	interval time.Duration,
	logger *zap.Logger,
) *SettlementReconciler {
	if interval <= 0 {
		interval = settlementReconcileInterval
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SettlementReconciler{
		payments: payments,
		sink:     sink,
		scanner:  scanner,
		interval: interval,
		logger:   logger,
	}
}

// StartSettlementReconciler starts the periodic settlement reconciler only
// when the gateway is enabled: with gatewayEnabled false — or a missing
// payment client, sink or scanner — nothing is started and nil is returned,
// so no scan and no gateway call ever happens. When enabled it launches the
// background loop (first pass immediately, then every interval) and returns
// the running reconciler.
//
// Parameters:
//   - ctx bounds the background loop.
//   - gatewayEnabled mirrors the gateway adapter's enabled state.
//   - payments reads the gateway payment status.
//   - sink applies terminal settlements.
//   - scanner supplies the stale candidates.
//   - interval is the period between passes; <= 0 selects the default.
//   - logger reports outcomes; nil selects a no-op logger.
//
// Returns the started reconciler, or nil when nothing was started.
func StartSettlementReconciler(
	ctx context.Context,
	gatewayEnabled bool,
	payments contracts.PaymentClient,
	sink SettlementEventSink,
	scanner SettlementScanner,
	interval time.Duration,
	logger *zap.Logger,
) *SettlementReconciler {
	if !gatewayEnabled || payments == nil || sink == nil || scanner == nil {
		return nil
	}
	reconciler := NewSettlementReconciler(payments, sink, scanner, interval, logger)
	go reconciler.Start(ctx)
	return reconciler
}

// Start runs the reconciliation loop until ctx is done: one pass
// immediately, then one pass every interval. It blocks; callers run it in a
// goroutine. Never panics.
//
// Parameters: ctx bounds the loop.
// Returns nothing.
func (r *SettlementReconciler) Start(ctx context.Context) {
	r.ReconcileOnce(ctx)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.ReconcileOnce(ctx)
		}
	}
}

// ReconcileOnce performs one reconciliation pass: it scans the stale
// busy-tentative candidates, reads each gateway payment status and dispatches
// terminal statuses (PAID, EXPIRED, CANCELED) to HandleSettlement. A PENDING
// payment — or a failed GetPayment — leaves the slot unchanged; failures are
// logged and never abort the pass. Never panics.
//
// Parameters: ctx bounds the scan, gateway reads and settlement dispatch.
// Returns nothing.
func (r *SettlementReconciler) ReconcileOnce(ctx context.Context) {
	if r == nil || r.payments == nil || r.sink == nil || r.scanner == nil {
		return
	}
	candidates, err := r.scanner.StaleSettlementCandidates(ctx)
	if err != nil {
		r.logger.Error("settlement reconciler: scan failed", zap.Error(err))
		return
	}
	for _, candidate := range candidates {
		r.reconcileCandidate(ctx, candidate)
	}
}

// reconcileCandidate reads one candidate's gateway payment status and
// dispatches terminal statuses to HandleSettlement; any other status
// (PENDING) leaves the slot unchanged.
//
// Parameters: ctx bounds the calls, candidate is one stale slot with its
// gateway payment id.
// Returns nothing; failures are logged.
func (r *SettlementReconciler) reconcileCandidate(ctx context.Context, candidate SettlementCandidate) {
	status, err := r.payments.GetPayment(ctx, candidate.PaymentID)
	if err != nil {
		r.logger.Error("settlement reconciler: get payment failed",
			zap.String("payment_id", candidate.PaymentID),
			zap.Error(err),
		)
		return
	}
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case settlementStatusPaid, settlementStatusExpired, settlementStatusCanceled:
		event := SettlementEvent{
			PaymentID:  candidate.PaymentID,
			ExternalID: candidate.ExternalID,
			Status:     status,
			Amount:     candidate.Amount,
			Currency:   candidate.Currency,
		}
		if err := r.sink.HandleSettlement(ctx, event); err != nil {
			r.logger.Error("settlement reconciler: handle settlement failed",
				zap.String("payment_id", candidate.PaymentID),
				zap.String("status", status),
				zap.Error(err),
			)
		}
	default:
		r.logger.Info("settlement reconciler: payment not terminal, slot left unchanged",
			zap.String("payment_id", candidate.PaymentID),
			zap.String("status", status),
		)
	}
}
