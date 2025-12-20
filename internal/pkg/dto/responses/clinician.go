package responses

type ClinicianSummary struct {
	ClinicianID         string              `json:"clinician_id,omitempty"`
	Name                string              `json:"name,omitempty"`
	PracticeInformation PracticeInformation `json:"practice_information,omitempty"`
	ScheduleID          string              `json:"schedule_id,omitempty"`
	PractitionerRoleID  string              `json:"practitioner_role_id,omitempty"`
}
type ClinicianClinic struct {
	ClinicID        string          `json:"clinic_id,omitempty"`
	ClinicName      string          `json:"clinic_name,omitempty"`
	Specialties     []string        `json:"specialties,omitempty"`
	PricePerSession PricePerSession `json:"price_per_session,omitempty"`
}

type PracticeInformation struct {
	ClinicID        string          `json:"clinic_id,omitempty"`
	ClinicName      string          `json:"clinic_name,omitempty"`
	Affiliation     string          `json:"affiliation,omitempty"`
	Experience      string          `json:"experience,omitempty"`
	Specialties     []string        `json:"specialties,omitempty"`
	PricePerSession PricePerSession `json:"price_per_session,omitempty"`
}

type PracticeAvailability struct {
	ClinicID       string                  `json:"clinic_id"`
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

type PricePerSession struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type AvailableTimeResponse struct {
	DaysOfWeek         []string `json:"days_of_Week"`
	AvailableStartTime string   `json:"available_start_time"`
	AvailableEndTime   string   `json:"available_end_time"`
}
