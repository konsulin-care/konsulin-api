package models

import (
	"konsulin-service/internal/pkg/dto/requests"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               string    `bson:"_id,omitempty"`
	RoleID           string    `bson:"roleId"`
	Fullname         string    `bson:"fullName"`
	Email            string    `bson:"email"`
	Username         string    `bson:"username"`
	Password         string    `bson:"password"`
	BirthDate        string    `bson:"birthDate"`
	Address          string    `bson:"address"`
	Gender           string    `bson:"gender"`
	Educations       []string  `bson:"educations"`
	WhatsAppNumber   string    `bson:"whatsAppNumber"`
	PatientID        string    `bson:"patientId,omitempty"`
	PractitionerID   string    `bson:"practitionerId,omitempty"`
	ResetToken       string    `bson:"resetToken, omitempty"`
	ResetTokenExpiry time.Time `bson:"resetTokenExpiry, omitempty"`
	Role             `bson:"user_role,omitempty"`
	TimeModel        `bson:",inline"`
}

func (u *User) SetDataForUpdateProfileData(request *requests.UpdateProfile) {
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

func (u *User) SetDataForUpdateForgotPassword(tokenExpiryTime int) {
	u.ResetToken = uuid.New().String()
	u.ResetTokenExpiry = time.Now().Add(time.Duration(tokenExpiryTime) * time.Minute)
	u.SetUpdatedAt()
}
