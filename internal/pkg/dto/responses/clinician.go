package responses

type ClinicianSummary struct {
	PractitionerID      string              `json:"practitioner_id,omitempty"`
	Name                string              `json:"name,omitempty"`
	Affiliation         string              `json:"affiliation,omitempty"`
	PracticeInformation PracticeInformation `json:"practice_information,omitempty"`
	Specialties         []string            `json:"specialties,omitempty"`
	ScheduleID          string              `json:"schedule_id,omitempty"`
	PractitionerRoleID  string              `json:"practitioner_role_id,omitempty"`
}
type ClinicianClinic struct {
	ClinicID   string `json:"clinic_id,omitempty"`
	ClinicName string `json:"clinic_name,omitempty"`
}

type PracticeInformation struct {
	ClinicID    string `json:"clinic_id,omitempty"`
	ClinicName  string `json:"clinic_name,omitempty"`
	Affiliation string `json:"affiliation,omitempty"`
	Experience  string `json:"experience,omitempty"`
	Fee         string `json:"fee,omitempty"`
}

type PracticeAvailability struct {
	OrganizationID string                  `json:"organization_id"`
	AvailableTimes []AvailableTimeResponse `json:"available_time"`
}

type MonthlyAvailabilityResponse struct {
	Year  int               `json:"year"`
	Month int               `json:"month"`
	Days  []DayAvailability `json:"days"`
}

// DayAvailability represents the availability for a single day
type DayAvailability struct {
	Date             string   `json:"date"`
	AvailableTimes   []string `json:"available_times"`
	UnavailableTimes []string `json:"unavailable_times"`
}
