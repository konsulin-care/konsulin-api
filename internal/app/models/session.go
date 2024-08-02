package models

import "time"

type Session struct {
	SessionID      string
	UserID         string
	Email          string
	Username       string
	PatientID      string
	PractitionerID string
	RoleID         string
	RoleName       string
	ExpiresAt      time.Time
}
