package responses

type AppointmentPaymentResponse struct {
	Status          int    `json:"status"`
	Message         string `json:"message"`
	AppointmentID   string `json:"appointment,omitempty"`
	SlotID          string `json:"slot"`
	PaymentNoticeID string `json:"paymentNotice,omitempty"`
	PaymentURL      string `json:"paymentUrl,omitempty"`
	ExpiresAt       string `json:"expiresAt,omitempty"`
}
