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

type PaymentRoutingStatus struct {
	Status struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
	TrxID               string `json:"trx_id"`
	PartnerTrxID        string `json:"partner_trx_id"`
	RequestAmount       int    `json:"request_amount"`
	ReceivedAmount      int    `json:"received_amount"`
	PaymentStatus       string `json:"payment_status"`
	PaymentReceivedTime string `json:"payment_received_time"`
	SettlementTime      string `json:"settlement_time"`
	SettlementType      string `json:"settlement_type"`
	SettlementStatus    string `json:"settlement_status"`
	TrxExpirationTime   string `json:"trx_expiration_time"`
	NeedFrontend        bool   `json:"need_frontend"`
	PaymentInfo         struct {
		PaymentCheckoutURL string `json:"payment_checkout_url"`
	} `json:"payment_info"`
	PaymentRouting []struct {
		RecipientBank        string   `json:"recipient_bank"`
		RecipientAccount     string   `json:"recipient_account"`
		RecipientAccountName string   `json:"recipient_account_name"`
		RecipientAmount      float64  `json:"recipient_amount"`
		TrxNotes             *string  `json:"trx_notes"`
		DisbursementTrxID    string   `json:"disbursement_trx_id"`
		TrxStatus            string   `json:"trx_status"`
		EmailStatus          *string  `json:"email_status"`
		RecipientEmail       *string  `json:"recipient_email"`
		DisbursementAdminFee *float64 `json:"disbursement_admin_fee"`
	} `json:"payment_routing"`
	PaymentMethod    string `json:"payment_method"`
	SenderBank       string `json:"sender_bank"`
	UseLinkedAccount bool   `json:"use_linked_account"`
}
