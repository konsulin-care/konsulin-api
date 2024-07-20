package models

import "konsulin-service/internal/pkg/dto/requests"

type User struct {
	ID             string `bson:"_id,omitempty"`
	RoleID         string `bson:"roleId"`
	Fullname       string `bson:"fullName"`
	Email          string `bson:"email"`
	Username       string `bson:"username"`
	Password       string `bson:"password"`
	BirthDate      string `bson:"birthDate"`
	Address        string `bson:"address"`
	Gender         string `bson:"gender"`
	Education      string `bson:"education"`
	WhatsAppNumber string `bson:"whatsAppNumber"`
	PatientID      string `bson:"patientId,omitempty"`
	PractitionerID string `bson:"practitionerId,omitempty"`
	ResetToken     string `bson:"resetToken"`
	Role           `bson:"user_role,omitempty"`
	TimeModel      `bson:",inline"`
}

func (u *User) SetUpdateProfileData(request *requests.UpdateProfile) {
	u.Fullname = request.Fullname
	u.Email = request.Email
	u.BirthDate = request.BirthDate
	u.WhatsAppNumber = request.WhatsAppNumber
	u.Address = request.Address
	u.Gender = request.Gender
	u.Education = request.Education
	u.SetUpdatedAt()
}

func (u *User) SetUpdateResetPassword(request *requests.ResetPassword) {
	u.Password = request.HashedNewPassword
	u.ResetToken = ""
	u.SetUpdatedAt()
}
