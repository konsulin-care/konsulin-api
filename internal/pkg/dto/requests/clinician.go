package requests

type CreatePracticeInformation struct {
	PracticeInformation []PracticeInformation `json:"practice_informations"`
}

type PracticeInformation struct {
	ClinicID        string          `json:"clinic_id"`
	PricePerSession PricePerSession `json:"price_per_session"`
}

type PricePerSession struct {
	Value    float64 `json:"value"`
	Currency string  `json:"currency"`
}

type CreatePracticeAvailability struct {
	PractitionerID string                            `json:"practitioner_id"`
	ClinicIDs      []string                          `json:"clinic_ids"`
	AvailableTimes map[string][]AvailableTimeRequest `json:"available_times"`
}

type AvailableTimeRequest struct {
	DaysOfWeek         []string `json:"days_of_week"`
	AvailableStartTime string   `json:"available_start_time"`
	AvailableEndTime   string   `json:"available_end_time"`
}

type FindAvailability struct {
	Year               string `json:"year"`
	Month              string `json:"month"`
	PractitionerRoleID string `json:"practitioner_role_id"`
}
