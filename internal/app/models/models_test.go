package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
)

func TestUserProfileUpdatesPreserveIdentity(t *testing.T) {
	u := &User{ID: "user1", RoleID: "role1", PatientID: "patient1", PractitionerID: "pr1", Username: "username"}
	input := &requests.UpdateProfile{Fullname: "Test User", Email: "test@example.invalid", BirthDate: "1990-01-02", WhatsAppNumber: "+628123456789", Address: "Jakarta", Gender: "female", Educations: []string{"Degree"}, ProfilePictureObjectName: "picture.png"}
	before := time.Now()
	u.SetDataForUpdateProfile(input)
	require.Equal(t, "user1", u.ID)
	require.Equal(t, "role1", u.RoleID)
	require.Equal(t, "patient1", u.PatientID)
	require.Equal(t, "pr1", u.PractitionerID)
	require.Equal(t, "username", u.Username)
	bson := u.ConvertToBsonM()
	for key, value := range map[string]interface{}{"fullName": input.Fullname, "email": input.Email, "birthDate": input.BirthDate, "whatsAppNumber": input.WhatsAppNumber, "address": input.Address, "gender": input.Gender, "educations": input.Educations, "profilePictureName": input.ProfilePictureObjectName, "roleId": "role1", "patientId": "patient1", "practitionerId": "pr1"} {
		require.Equal(t, value, bson[key], key)
	}
	require.Len(t, bson, 21)
	require.False(t, u.UpdatedAt.Before(before))
	require.False(t, u.UpdatedAt.After(time.Now()))
}

func TestUserCredentialAndExpiryUpdates(t *testing.T) {
	u := &User{ResetToken: "test-placeholder", Username: "username"}
	u.SetDataForUpdateResetPassword(&requests.ResetPassword{HashedNewPassword: "test-hash"})
	require.Equal(t, "test-hash", u.Password)
	require.Empty(t, u.ResetToken)
	require.Equal(t, "username", u.Username)
	before := time.Now()
	u.SetResetTokenExpiryTime(5)
	u.SetWhatsAppOTPExpiryTime(3)
	after := time.Now()
	require.NotNil(t, u.ResetTokenExpiry)
	require.NotNil(t, u.WhatsAppOTPExpiry)
	require.False(t, u.ResetTokenExpiry.Before(before.Add(5*time.Minute)))
	require.False(t, u.ResetTokenExpiry.After(after.Add(5*time.Minute)))
	require.False(t, u.WhatsAppOTPExpiry.Before(before.Add(3*time.Minute)))
	require.False(t, u.WhatsAppOTPExpiry.After(after.Add(3*time.Minute)))
}

func TestSoftDeletionAndTimestamps(t *testing.T) {
	u := &User{}
	before := time.Now()
	u.SetCreatedAtUpdatedAt()
	require.Equal(t, u.CreatedAt, u.UpdatedAt)
	require.False(t, u.CreatedAt.Before(before))
	require.False(t, u.CreatedAt.After(time.Now()))
	require.False(t, u.IsDeactivated())
	u.SetDeletedAt()
	require.True(t, u.IsDeactivated())
	require.False(t, u.IsDeactivationDeadlineExpired(1))
	require.False(t, u.UpdatedAt.Before(*u.DeletedAt))
	past := time.Now().AddDate(0, 0, -3)
	u.DeletedAt = &past
	require.True(t, u.IsDeactivationDeadlineExpired(1))
	u.SetEmptyDeletedAt()
	require.False(t, u.IsDeactivated())
}

func TestRoleAndGenderContracts(t *testing.T) {
	for _, name := range []string{constvars.RoleTypePatient, constvars.RoleTypePractitioner, "other"} {
		role := &Role{Name: name}
		require.Equal(t, name != constvars.RoleTypePatient, role.IsNotPatient())
		require.Equal(t, name != constvars.RoleTypePractitioner, role.IsNotPractitioner())
	}
	gender := Gender{Code: "female", Display: "Female", CustomDisplay: "Perempuan"}
	require.Equal(t, "Perempuan", gender.ConvertIntoResponse().Name)
	require.Equal(t, "Jakarta", (City{Name: "Jakarta"}).ConvertIntoResponse().Name)
	require.Equal(t, "Sarjana", (EducationLevel{Display: "Bachelor", CustomDisplay: "Sarjana"}).ConvertIntoResponse().Name)
}
