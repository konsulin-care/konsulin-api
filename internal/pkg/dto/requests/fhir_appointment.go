package requests

type Appointment struct {
	ID               string        `json:"id,omitempty"`
	PatientId        string        `json:"patientId"`
	ClinicianId      string        `json:"clinicianId"`
	Appointments     []Appointment `json:"appointments,omitempty"`
	Status           string        `json:"status,omitempty"`
	NumberOfSessions int           `json:"number_of_sessions"`
	SessionType      string        `json:"session_type"`
	ProblemBrief     string        `json:"problem_brief"`
}
