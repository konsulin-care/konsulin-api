package responses

import "time"

type Appointment struct {
	ID              string    `json:"id,omitempty"`
	Status          string    `json:"status,omitempty"`
	ClinicianID     string    `json:"clinician_id,omitempty"`
	ClinicianName   string    `json:"clinician_name,omitempty"`
	PatientID       string    `json:"patient_id,omitempty"`
	PatientName     string    `json:"patient_name,omitempty"`
	AppointmentTime time.Time `json:"appointment_time,omitempty"`
	Description     string    `json:"description,omitempty"`
	MinutesDuration uint      `json:"minutes_duration,omitempty"`
}

type CreateAppointment struct {
	PaymentLink string `json:"payment_link,omitempty"`
}
