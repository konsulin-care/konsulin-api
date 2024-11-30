package responses

type Status struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// PaymentInfo represents the payment information in the JSON response.
type PaymentInfo struct {
	PaymentCheckoutURL string `json:"payment_checkout_url"`
}

// Response represents the overall JSON response.
type PaymentResponse struct {
	Status        Status `json:"status"`
	TrxID         string `json:"trx_id"`
	PartnerTrxID  string `json:"partner_trx_id"`
	ReceiveAmount int    `json:"receive_amount"`
	// TrxExpirationTime time.Time   `json:"trx_expiration_time"`
	PaymentInfo PaymentInfo `json:"payment_info"`
}
