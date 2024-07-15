package models

import "time"

type TimeModel struct {
	CreatedAt time.Time  `json:"createdAt" bson:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt" bson:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty" bson:"deletedAt,omitempty"`
}

type User struct {
	ID             string `bson:"_id,omitempty"`
	RoleID         string `bson:"roleId"`
	Email          string `bson:"email"`
	Username       string `bson:"username"`
	Password       string `bson:"password"`
	WhatsAppNumber string `bson:"whatsAppNumber"`
	PatientID      string `bson:"patientId,omitempty"`
	PractitionerID string `bson:"practitionerId,omitempty"`
	Role           `bson:"user_role,omitempty"`
	TimeModel      `bson:",inline"`
}
