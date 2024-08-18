package responses

type RegisterUser struct {
	UserID         string `json:"user_id"`
	PatientID      string `json:"patient_id,omitempty"`
	PractitionerID string `json:"clinician_id,omitempty"`
}

type LoginUser struct {
	Token         string        `json:"token"`
	LoginUserData LoginUserData `json:"user"`
}

type LoginUserData struct {
	Name           string   `json:"name"`
	Email          string   `json:"email"`
	UserID         string   `json:"user_id"`
	RoleID         string   `json:"role_id"`
	RoleName       string   `json:"role_name"`
	PatientID      string   `json:"patient_id,omitempty"`
	PractitionerID string   `json:"practitioner_id,omitempty"`
	ProfilePicture string   `json:"profile_picture,omitempty"`
	ClinicIDs      []string `json:"clinic_ids,omitempty"`
}
