package responses

// CreatePayResponse represents the minimal data returned by /pay.
type CreatePayResponse struct {
	PaymentCheckoutURL string `json:"payment_checkout_url"`
	PartnerTrxID       string `json:"partner_trx_id"`
	TrxID              string `json:"trx_id"`
	Amount             int    `json:"amount"`
}
