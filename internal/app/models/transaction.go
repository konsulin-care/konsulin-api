package models

import (
	"time"
)

type TransactionStatusPayment string

const (
	Pending       TransactionStatusPayment = "pending"
	Completed     TransactionStatusPayment = "completed"
	Failed        TransactionStatusPayment = "failed"
	Refunded      TransactionStatusPayment = "refunded"
	PartialRefund TransactionStatusPayment = "partial_refund"
)

type TransactionRefundStatus string

const (
	None          TransactionRefundStatus = "none"
	RefundPending TransactionRefundStatus = "pending"
	Processing    TransactionRefundStatus = "processing"
	RefundedFull  TransactionRefundStatus = "refunded"
	Partial       TransactionRefundStatus = "partial_refund"
	FailedRefund  TransactionRefundStatus = "failed"
	Cancelled     TransactionRefundStatus = "cancelled"
)

type TransactionSessionType string

const (
	Online  TransactionSessionType = "online"
	Offline TransactionSessionType = "offline"
)

type Transaction struct {
	ID                      string                   `json:"id"`
	PatientID               string                   `json:"patient_id"`
	PractitionerID          string                   `json:"practitioner_id"`
	PaymentLink             string                   `json:"payment_link"`
	Amount                  float64                  `json:"amount"`
	Currency                string                   `json:"currency"`
	Notes                   string                   `json:"notes,omitempty"`
	CreatedAt               time.Time                `json:"created_at"`
	UpdatedAt               time.Time                `json:"updated_at"`
	SessionTotal            int                      `json:"session_total"`
	LengthMinutesPerSession int                      `json:"length_minutes_per_session"`
	RefundAmount            float64                  `json:"refund_amount"`
	StatusPayment           TransactionStatusPayment `json:"status_payment"`
	SessionType             TransactionSessionType   `json:"session_type"`
	RefundStatus            TransactionRefundStatus  `json:"refund_status"`
	AuditLog                interface{}              `json:"audit_log,omitempty"`
}
