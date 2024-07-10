package responses

type RegisterUser struct {
	UserID         string `json:"user_id"`
	PatientID      string `json:"patient_id,omitempty"`
	PractitionerID string `json:"clinician_id,omitempty"`
}

type LoginUser struct {
	Token string `json:"token"`
}
