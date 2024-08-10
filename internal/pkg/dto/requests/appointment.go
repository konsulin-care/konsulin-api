package requests

type CreateAppointmentRequest struct {
	ClinicianId      string `json:"clinicianId"`
	Date             string `json:"date"`
	Time             string `json:"time"`
	SessionType      string `json:"session_type"`
	NumberOfSessions int    `json:"number_of_sessions"`
	ProblemBrief     string `json:"problem_brief"`
}
