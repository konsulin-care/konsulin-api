package requests

type RegisterViaWhatsApp struct {
	To string `json:"to" validate:"required,phone_number"`
}
type VerivyRegisterWhatsAppOTP struct {
	To   string `json:"to" validate:"required,phone_number"`
	OTP  string `json:"otp" validate:"required,len=6,numeric"`
	Role string `json:"role" validate:"required,oneof=Patient Practitioner"`
}
type LoginViaWhatsApp struct {
	To string `json:"to" validate:"required,phone_number"`
}
type VerivyLoginWhatsAppOTP struct {
	To  string `json:"to" validate:"required,phone_number"`
	OTP string `json:"otp" validate:"required,len=6,numeric"`
}

type WhatsAppMessage struct {
	To        string `json:"to"`
	Message   string `json:"message"`
	WithImage bool   `json:"with_image"`
}
