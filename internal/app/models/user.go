package models

import (
	"konsulin-service/internal/pkg/dto/requests"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

type User struct {
	ID                 string `bson:"_id,omitempty"`
	Email              string `bson:"email,omitempty"`
	Gender             string `bson:"gender,omitempty"`
	RoleID             string `bson:"roleId,omitempty"`
	Address            string `bson:"address,omitempty"`
	Fullname           string `bson:"fullName,omitempty"`
	Username           string `bson:"username,omitempty"`
	Password           string `bson:"password,omitempty"`
	BirthDate          string `bson:"birthDate,omitempty"`
	PatientID          string `bson:"patientId,omitempty"`
	ResetToken         string `bson:"resetToken,omitempty"`
	WhatsAppOTP        string `bson:"whatsAppOtp,omitempty"`
	WhatsAppNumber     string `bson:"whatsAppNumber,omitempty"`
	PractitionerID     string `bson:"practitionerId,omitempty"`
	ProfilePictureName string `bson:"profilePictureName,omitempty"`

	Educations        []string   `bson:"educations,omitempty"`
	ResetTokenExpiry  *time.Time `bson:"resetTokenExpiry,omitempty"`
	WhatsAppOTPExpiry *time.Time `bson:"whatsAppOtpExpiry,omitempty"`

	Role      *Role `bson:"user_role,omitempty"`
	TimeModel `bson:",inline"`
}

func (u *User) ConvertToBsonM() bson.M {
	return bson.M{
		"email":              u.Email,
		"gender":             u.Gender,
		"roleId":             u.RoleID,
		"address":            u.Address,
		"fullName":           u.Fullname,
		"username":           u.Username,
		"password":           u.Password,
		"birthDate":          u.BirthDate,
		"patientId":          u.PatientID,
		"resetToken":         u.ResetToken,
		"whatsAppOtp":        u.WhatsAppOTP,
		"whatsAppNumber":     u.WhatsAppNumber,
		"practitionerId":     u.PractitionerID,
		"profilePictureName": u.ProfilePictureName,
		"educations":         u.Educations,
		"resetTokenExpiry":   u.ResetTokenExpiry,
		"whatsAppOtpExpiry":  u.WhatsAppOTPExpiry,
		"user_role":          u.Role,
		"createdAt":          u.CreatedAt,
		"updatedAt":          u.UpdatedAt,
		"deletedAt":          u.DeletedAt,
	}
}

func (u *User) SetDataForUpdateProfile(request *requests.UpdateProfile) {
	u.Fullname = request.Fullname
	u.Email = request.Email
	u.BirthDate = request.BirthDate
	u.WhatsAppNumber = request.WhatsAppNumber
	u.Address = request.Address
	u.Gender = request.Gender
	u.Educations = request.Educations
	u.ProfilePictureName = request.ProfilePictureObjectName
	u.SetUpdatedAt()
}

func (u *User) IsDeactivationDeadlineExpired(deactivationAgeInDays int) bool {
	deactivationDeadline := u.DeletedAt.AddDate(0, 0, deactivationAgeInDays)
	return time.Now().After(deactivationDeadline)
}

func (u *User) SetDataForUpdateResetPassword(request *requests.ResetPassword) {
	u.Password = request.HashedNewPassword
	u.ResetToken = ""
	u.SetUpdatedAt()
}

func (u *User) SetResetTokenExpiryTime(durationInMinutes int) {
	resetTokenExpiryTime := time.Now().Add(time.Duration(durationInMinutes) * time.Minute)
	u.ResetTokenExpiry = &resetTokenExpiryTime
	u.SetUpdatedAt()
}

func (u *User) SetWhatsAppOTPExpiryTime(durationInMinutes int) {
	whatsAppOTPExpiryTime := time.Now().Add(time.Duration(durationInMinutes) * time.Minute)
	u.WhatsAppOTPExpiry = &whatsAppOTPExpiryTime
	u.SetUpdatedAt()
}

func (u *User) IsDeactivated() bool {
	return u.DeletedAt != nil
}
