package requests

type CreateAppointmentRequest struct {
	ClinicianID      string `json:"clinician_id"`
	ScheduleID       string `json:"schedule_id"`
	Date             string `json:"date" validate:"not_past_date"`
	Time             string `json:"time" validate:"not_past_time"`
	SessionType      string `json:"session_type"`
	NumberOfSessions int    `json:"number_of_sessions"`
	PricePerSession  int    `json:"price_per_session"`
	ProblemBrief     string `json:"problem_brief"`
}
