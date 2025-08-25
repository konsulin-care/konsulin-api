package requests

type PaymentRequestDTO struct {
	PartnerUserID           string           `json:"partner_user_id" validate:"required"`
	UseLinkedAccount        bool             `json:"use_linked_account" validate:"required"`
	PartnerTransactionID    string           `json:"partner_trx_id" validate:"required"`
	FullName                string           `json:"full_name"`
	NeedFrontend            bool             `json:"need_frontend" validate:"required"`
	SenderEmail             string           `json:"sender_email" validate:"required,email"`
	PaymentExpirationTime   string           `json:"trx_expiration_time"`
	ReceiveAmount           int              `json:"receive_amount" validate:"required"`
	ListEnablePaymentMethod string           `json:"list_enable_payment_method" validate:"required"`
	ListEnableSOF           string           `json:"list_enable_sof" validate:"required"`
	VADisplayName           string           `json:"va_display_name" validate:"required"`
	PaymentRouting          []PaymentRouting `json:"payment_routing" validate:"required,dive"`
}

type PaymentRoutingStatus struct {
	PartnerTransactionID string `json:"partner_trx_id"`
}

type PaymentRequest struct {
	PartnerUserID           string           `json:"partner_user_id" validate:"required"`
	UseLinkedAccount        bool             `json:"use_linked_account" validate:"required"`
	PartnerTransactionID    string           `json:"partner_trx_id" validate:"required"`
	NeedFrontend            bool             `json:"need_frontend" validate:"required"`
	SenderEmail             string           `json:"sender_email" validate:"required,email"`
	PaymentExpirationTime   string           `json:"trx_expiration_time"`
	ReceiveAmount           int              `json:"receive_amount" validate:"required"`
	ListEnablePaymentMethod []string         `json:"list_enable_payment_method" validate:"required"`
	ListEnableSOF           []string         `json:"list_enable_sof" validate:"required"`
	VADisplayName           string           `json:"va_display_name" validate:"required"`
	PaymentRouting          []PaymentRouting `json:"payment_routing" validate:"required,dive"`
}

type PaymentRouting struct {
	RecipientBank    string `json:"recipient_bank" validate:"required"`
	RecipientAccount string `json:"recipient_account" validate:"required"`
	RecipientAmount  int    `json:"recipient_amount" validate:"required"`
	RecipientEmail   string `json:"recipient_email" validate:"required,email"`
}

type PaymentInfo struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	BankCode      string `json:"bank_code"`
}

type PaymentRoutingToCallback struct {
	RecipientBank        string  `json:"recipient_bank"`
	RecipientAccount     string  `json:"recipient_account"`
	RecipientAccountName string  `json:"recipient_account_name"`
	RecipientAmount      float64 `json:"recipient_amount"`
	DisbursementTrxID    string  `json:"disbursement_trx_id"`
	TrxStatus            string  `json:"trx_status"`
}

// Transaction represents the overall structure of the JSON response.
type PaymentRoutingCallback struct {
	TrxID          string                     `json:"trx_id"`
	PartnerUserID  string                     `json:"partner_user_id"`
	PartnerTrxID   string                     `json:"partner_trx_id"`
	ReceiveAmount  int                        `json:"receive_amount"`
	PaymentStatus  string                     `json:"payment_status"`
	NeedFrontend   bool                       `json:"need_frontend"`
	PaymentMethod  string                     `json:"payment_method"`
	SenderBank     string                     `json:"sender_bank"`
	PaymentInfo    PaymentInfo                `json:"payment_info"`
	PaymentRouting []PaymentRoutingToCallback `json:"payment_routing"`
}
