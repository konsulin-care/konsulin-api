package requests

type CreateJournal struct {
	Title       string   `json:"title" validate:"required"`
	JournalBody []string `json:"journal_body,omitempty"`
	JournalDate string   `json:"journal_date" validate:"required,datetime=2006-01-02,not_future_date"`
	PatientID   string
	SessionData string
}

type UpdateJournal struct {
	Title       string   `json:"title" validate:"required"`
	JournalBody []string `json:"journal_body,omitempty"`
	JournalDate string   `json:"journal_date" validate:"required,datetime=2006-01-02,not_future_date"`
	JournalID   string
	PatientID   string
	SessionData string
}

type FindJournalByID struct {
	JournalID   string
	SessionData string
}

type DeleteJournalByID struct {
	JournalID   string
	SessionData string
}
