package responses

import "time"

type Journal struct {
	JournalID   string    `json:"journal_id"`
	PatientID   string    `json:"patient_id"`
	Title       string    `json:"title"`
	JournalBody []string  `json:"journal_body,omitempty"`
	JournalDate time.Time `json:"journal_date"`
}
