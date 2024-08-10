package responses

type ClinicianSummary struct {
	PractitionerID      string              `json:"practitioner_id,omitempty"`
	Name                string              `json:"name,omitempty"`
	Affiliation         string              `json:"affiliation,omitempty"`
	PracticeInformation PracticeInformation `json:"practice_information,omitempty"`
	Specialties         []string            `json:"specialties,omitempty"`
	Availability        []AvailableTime     `json:"availability,omitempty"`
}

type PracticeInformation struct {
	Affiliation string `json:"affiliation,omitempty"`
	Experience  string `json:"experience,omitempty"`
	Fee         string `json:"fee,omitempty"`
}
