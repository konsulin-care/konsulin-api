package requests

type LoginViaWhatsApp struct {
	To string `json:"to" validate:"required,phone_number"`
}
type VerivyWhatsAppOTP struct {
	To   string `json:"to" validate:"required,phone_number"`
	OTP  string `json:"otp" validate:"required,len=6,numeric"`
	Role string `json:"role" validate:"required,oneof=Patient Practitioner"`
}

type WhatsAppMessage struct {
	To        string `json:"to"`
	Message   string `json:"message"`
	WithImage bool   `json:"with_image"`
}
