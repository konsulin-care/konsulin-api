package requests

// XenditInvoiceStatus is a typed invoice status returned by Xendit
type XenditInvoiceStatus string

const (
	XenditInvoiceStatusPending XenditInvoiceStatus = "PENDING"
	XenditInvoiceStatusPaid    XenditInvoiceStatus = "PAID"
	XenditInvoiceStatusExpired XenditInvoiceStatus = "EXPIRED"
	XenditInvoiceStatusSettled XenditInvoiceStatus = "SETTLED"
)

// HeaderKey is a special HTTP Headers set by Xendit that
// are useful for us for a range of operations.
type HeaderKey string

// list of known xendit header keys
const (
	HeaderKeyCallbackToken HeaderKey = "X-Callback-Token"
	HeaderKeyWebhookID     HeaderKey = "Webhook-Id"
)

// XenditInvoiceCallbackHeader represents the HTTP headers sent by Xendit in webhook callbacks
type XenditInvoiceCallbackHeader struct {
	CallbackToken string // x-callback-token header
	WebhookID     string // x-webhook-id header (optional but recommended)
}

// XenditInvoiceCallbackBody represents the JSON body sent by Xendit in invoice webhook callbacks
type XenditInvoiceCallbackBody struct {
	ID         string              `json:"id"`          // Invoice ID (required)
	ExternalID string              `json:"external_id"` // Partner transaction ID (required)
	Status     XenditInvoiceStatus `json:"status"`      // Invoice status: PENDING, PAID, or EXPIRED (required)
	Amount     *float64            `json:"amount,omitempty"`
	Currency   *string             `json:"currency,omitempty"`
	Created    *string             `json:"created,omitempty"`
	Updated    *string             `json:"updated,omitempty"`
}
