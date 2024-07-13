package models

import "time"

type Session struct {
	SessionID      string
	UserID         string
	UserType       string
	PatientID      string
	PractitionerID string
	RoleID         string
	ExpiresAt      time.Time
}
