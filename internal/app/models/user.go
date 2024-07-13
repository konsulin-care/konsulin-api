package models

type User struct {
	ID             string `bson:"_id"`
	RoleID         string `bson:"role_id"`
	Email          string `bson:"email"`
	Username       string `bson:"username"`
	Password       string `bson:"password"`
	WhatsAppNumber string `bson:"whatsAppNumber"`
	OTP            string `bson:"otp"`
	UserType       string `bson:"userType"`
	PatientID      string `bson:"patientId"`
	PractitionerID string `bson:"practitionerId"`
	TimeModel
}
