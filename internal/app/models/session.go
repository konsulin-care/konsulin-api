package models

import "time"

type Session struct {
	SessionID string
	UserID    string
	PatientID string
	ExpiresAt time.Time
}
