package responses

type UserProfile struct {
	Fullname       string `json:"fullname"`
	Email          string `json:"email"`
	Age            int    `json:"age"`
	Gender         string `json:"gender"`
	Education      string `json:"education"`
	WhatsAppNumber string `json:"whatsapp_number"`
	Address        string `json:"address"`
	BirthDate      string `json:"birth_date"`
}

type UpdateUserProfile struct {
	PatientID      string `json:"patient_id,omitempty"`
	PractitionerID string `json:"practitioner_id,omitempty"`
}
