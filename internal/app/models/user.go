package models

import (
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"strings"
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
		"createdAt":          u.TimeModel.CreatedAt,
		"updatedAt":          u.TimeModel.UpdatedAt,
		"deletedAt":          u.TimeModel.DeletedAt,
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

func (u *User) ConvertToPatientFhirDeactivationRequest() *fhir_dto.Patient {
	var extensions []fhir_dto.Extension
	for _, education := range u.Educations {
		extensions = append(extensions, fhir_dto.Extension{
			Url:         "http://example.org/fhir/StructureDefinition/education",
			ValueString: education,
		})
	}

	return &fhir_dto.Patient{
		ResourceType: constvars.ResourcePatient,
		ID:           u.PatientID,
		Active:       false,
		Name: []fhir_dto.HumanName{
			{
				Use:    "official",
				Family: u.Fullname,
				Given:  []string{u.Fullname},
			},
		},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: "email",
				Value:  u.Email,
				Use:    "home",
			},
			{
				System: "phone",
				Value:  u.WhatsAppNumber,
				Use:    "mobile",
			},
		},
		Gender:    u.Gender,
		BirthDate: u.BirthDate,
		Address: []fhir_dto.Address{
			{
				Use:  "home",
				Line: strings.Split(u.Address, ", "),
			},
		},
		Extension: extensions,
	}
}

func (u *User) ConvertToPractitionerFhirDeactivationRequest() *fhir_dto.Practitioner {
	var extensions []fhir_dto.Extension
	for _, education := range u.Educations {
		extensions = append(extensions, fhir_dto.Extension{
			Url:         "http://example.org/fhir/StructureDefinition/education",
			ValueString: education,
		})
	}

	return &fhir_dto.Practitioner{
		ResourceType: constvars.ResourcePatient,
		ID:           u.PractitionerID,
		Active:       false,
		Name: []fhir_dto.HumanName{
			{
				Use:    "official",
				Family: u.Fullname,
				Given:  []string{u.Fullname},
			},
		},
		Telecom: []fhir_dto.ContactPoint{
			{
				System: "email",
				Value:  u.Email,
				Use:    "home",
			},
			{
				System: "phone",
				Value:  u.WhatsAppNumber,
				Use:    "mobile",
			},
		},
		Gender:    u.Gender,
		BirthDate: u.BirthDate,
		Address: []fhir_dto.Address{
			{
				Use:  "home",
				Line: strings.Split(u.Address, ", "),
			},
		},
		Extension: extensions,
	}
}
