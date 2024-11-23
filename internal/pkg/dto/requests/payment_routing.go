package requests

type PaymentRequestDTO struct {
	PartnerUserID           string           `json:"partner_user_id" validate:"required"`
	UseLinkedAccount        bool             `json:"use_linked_account" validate:"required"`
	PartnerTransactionID    string           `json:"partner_trx_id" validate:"required"`
	NeedFrontend            bool             `json:"need_frontend" validate:"required"`
	SenderEmail             string           `json:"sender_email" validate:"required,email"`
	ReceiveAmount           int              `json:"receive_amount" validate:"required"`
	ListEnablePaymentMethod string           `json:"list_enable_payment_method" validate:"required"`
	ListEnableSOF           string           `json:"list_enable_sof" validate:"required"`
	VADisplayName           string           `json:"va_display_name" validate:"required"`
	PaymentRouting          []PaymentRouting `json:"payment_routing" validate:"required,dive"`
}

type PaymentRequest struct {
	PartnerUserID           string           `json:"partner_user_id" validate:"required"`
	UseLinkedAccount        bool             `json:"use_linked_account" validate:"required"`
	PartnerTransactionID    string           `json:"partner_trx_id" validate:"required"`
	NeedFrontend            bool             `json:"need_frontend" validate:"required"`
	SenderEmail             string           `json:"sender_email" validate:"required,email"`
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
