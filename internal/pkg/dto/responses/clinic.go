package responses

type ClinicClinician struct {
	PractitionerID string   `json:"practitioner_id,omitempty"`
	Name           string   `json:"name,omitempty"`
	ClinicName     string   `json:"clinic_name,omitempty"`
	Affiliation    string   `json:"affiliation,omitempty"`
	Specialties    []string `json:"specialties,omitempty"`
}

type Clinic struct {
	ID          string   `json:"clinic_id,omitempty"`
	ClinicName  string   `json:"clinic_name,omitempty"`
	Affiliation string   `json:"affiliation,omitempty"`
	Address     string   `json:"address,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}
