package responses

type RegisterPatient struct {
	UserID    string `json:"user_id"`
	PatientID string `json:"patient_id"`
}

type LoginPatient struct {
	Token string `json:"token"`
}
