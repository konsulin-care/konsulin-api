package requests

type ClinicianCreateClinics struct {
	ClinicIDs []string `json:"clinic_ids"`
}

type CreateClinicsAvailability struct {
	PractitionerID string                            `json:"practitioner_id"`
	ClinicIDs      []string                          `json:"clinic_ids"`
	AvailableTimes map[string][]AvailableTimeRequest `json:"available_times"`
}

type AvailableTimeRequest struct {
	DaysOfWeek         []string `json:"days_of_week"`
	AvailableStartTime string   `json:"available_start_time"`
	AvailableEndTime   string   `json:"available_end_time"`
}
