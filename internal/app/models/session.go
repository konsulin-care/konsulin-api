package models

import (
	"konsulin-service/internal/pkg/constvars"
	"time"
)

type Session struct {
	SessionID      string
	UserID         string
	Email          string
	Username       string
	PatientID      string
	PractitionerID string
	RoleID         string
	RoleName       string
	ClinicIDs      []string
	ExpiresAt      time.Time
}

func (s *Session) IsNotPatient() bool {
	return s.RoleName != constvars.RoleTypePatient
}

func (s *Session) IsNotPractitioner() bool {
	return s.RoleName != constvars.RoleTypePractitioner
}

func (s *Session) IsPatient() bool {
	return s.RoleName == constvars.RoleTypePatient
}
func (s *Session) IsPractitioner() bool {
	return s.RoleName == constvars.RoleTypePractitioner
}
