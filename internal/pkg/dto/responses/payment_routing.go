package responses

type Status struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PaymentInfo struct {
	PaymentCheckoutURL string `json:"payment_checkout_url"`
}

type PaymentResponse struct {
	Status                    Status      `json:"status"`
	TransactionID             string      `json:"trx_id"`
	PartnerTransactionID      string      `json:"partner_trx_id"`
	ReceiveAmount             int         `json:"receive_amount"`
	TransactionExpirationTime string      `json:"trx_expiration_time"`
	PaymentInfo               PaymentInfo `json:"payment_info"`
}
