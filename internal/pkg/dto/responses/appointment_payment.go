package responses

type AppointmentPaymentResponse struct {
	Status          int    `json:"status"`
	Message         string `json:"message"`
	AppointmentID   string `json:"appointment"`
	SlotID          string `json:"slot"`
	PaymentNoticeID string `json:"paymentNotice"`
}

