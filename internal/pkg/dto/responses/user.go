package responses

type UserProfile struct {
	Fullname               string                 `json:"fullname,omitempty"`
	Email                  string                 `json:"email,omitempty"`
	Age                    int                    `json:"age,omitempty"`
	Gender                 string                 `json:"gender,omitempty"`
	Educations             []string               `json:"educations,omitempty"`
	WhatsAppNumber         string                 `json:"whatsapp_number,omitempty"`
	Address                string                 `json:"address,omitempty"`
	BirthDate              string                 `json:"birth_date,omitempty"`
	PracticeInformations   []PracticeInformation  `json:"practice_informations,omitempty"`
	PracticeAvailabilities []PracticeAvailability `json:"practice_availabilities,omitempty"`
	ProfilePicture         *string                `json:"profile_picture,omitempty"`
}

type UpdateUserProfile struct {
	PatientID      string `json:"patient_id,omitempty"`
	PractitionerID string `json:"practitioner_id,omitempty"`
}
