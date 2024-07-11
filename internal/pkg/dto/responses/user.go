package responses

type UserProfile struct {
	Fullname       string `json:"fullname"`
	Email          string `json:"email"`
	Age            int    `json:"age"`
	Sex            string `json:"sex"`
	Education      string `json:"education"`
	WhatsAppNumber string `json:"whatsapp_number"`
	HomeAddress    string `json:"home_address"`
	BirthDate      string `json:"birth_date"`
}

type UpdateUserProfile struct {
	PatientID string
}
