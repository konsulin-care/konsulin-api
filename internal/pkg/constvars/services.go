package constvars

// PaymentServiceType represents the type of payment service for settlement callbacks.
// These variable will be used to indicate which service the payment
// is for.
type PaymentServiceType string

// list of known paid services
const (
	AppointmentPaymentService PaymentServiceType = "appointment"
	WebhookPaymentService     PaymentServiceType = "webhook"
)
