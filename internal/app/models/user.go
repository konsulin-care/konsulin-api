package models

import (
	"konsulin-service/internal/pkg/dto/requests"
	"time"
)

type User struct {
	ID                string `bson:"_id,omitempty"`
	Email             string `bson:"email"`
	Gender            string `bson:"gender"`
	RoleID            string `bson:"roleId"`
	Address           string `bson:"address"`
	Fullname          string `bson:"fullName"`
	Username          string `bson:"username"`
	Password          string `bson:"password"`
	BirthDate         string `bson:"birthDate"`
	PatientID         string `bson:"patientId,omitempty"`
	ResetToken        string `bson:"resetToken, omitempty"`
	WhatsAppNumber    string `bson:"whatsAppNumber"`
	PractitionerID    string `bson:"practitionerId,omitempty"`
	ProfilePictureUrl string `bson:"profilePictureUrl"`

	Educations       []string  `bson:"educations"`
	ResetTokenExpiry time.Time `bson:"resetTokenExpiry, omitempty"`

	Role      `bson:"user_role,omitempty"`
	TimeModel `bson:",inline"`
}

func (u *User) SetDataForUpdateProfile(request *requests.UpdateProfile) {
	u.Fullname = request.Fullname
	u.Email = request.Email
	u.BirthDate = request.BirthDate
	u.WhatsAppNumber = request.WhatsAppNumber
	u.Address = request.Address
	u.Gender = request.Gender
	u.Educations = request.Educations
	u.SetUpdatedAt()
}

func (u *User) SetDataForUpdateResetPassword(request *requests.ResetPassword) {
	u.Password = request.HashedNewPassword
	u.ResetToken = ""
	u.SetUpdatedAt()
}
