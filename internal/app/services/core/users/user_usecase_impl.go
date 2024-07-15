package users

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
)

type userUsecase struct {
	UserRepository         UserRepository
	PatientFhirClient      patients.PatientFhirClient
	PractitionerFhirClient practitioners.PractitionerFhirClient
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
) UserUsecase {
	return &userUsecase{
		UserRepository:         userMongoRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
	}
}

func (uc *userUsecase) GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error) {
	session := new(models.Session)
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.getPractitionerProfile(ctx, session)
	case constvars.RoleTypePatient:
		return uc.getPatientProfile(ctx, session)
	default:
		return nil, exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) UpdateUserProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	session := new(models.Session)
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.updatePractitionerProfile(ctx, session, request)
	case constvars.RoleTypePatient:
		return uc.updatePatientProfile(ctx, session, request)
	default:
		return nil, exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) getPatientProfile(ctx context.Context, session *models.Session) (*responses.UserProfile, error) {
	patient, err := uc.PatientFhirClient.GetPatientByID(ctx, session.PatientID)
	if err != nil {
		return nil, err
	}

	fullname := utils.GetFullName(patient.Name)
	email, whatsAppNumber := utils.GetEmailAndWhatsapp(patient.Telecom)
	age := utils.CalculateAge(patient.BirthDate)
	education := utils.GetEducationFromExtensions(patient.Extension)
	homeAddress := utils.GetHomeAddress(patient.Address)
	formattedBirthDate := utils.FormatBirthDate(patient.BirthDate)

	response := &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Sex:            patient.Gender,
		Education:      education,
		WhatsAppNumber: whatsAppNumber,
		HomeAddress:    homeAddress,
		BirthDate:      formattedBirthDate,
	}

	return response, nil
}

func (uc *userUsecase) getPractitionerProfile(ctx context.Context, session *models.Session) (*responses.UserProfile, error) {
	practitioner, err := uc.PractitionerFhirClient.GetPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	fullname := utils.GetFullName(practitioner.Name)
	email, whatsAppNumber := utils.GetEmailAndWhatsapp(practitioner.Telecom)
	age := utils.CalculateAge(practitioner.BirthDate)
	education := utils.GetEducationFromExtensions(practitioner.Extension)
	homeAddress := utils.GetHomeAddress(practitioner.Address)
	formattedBirthDate := utils.FormatBirthDate(practitioner.BirthDate)

	response := &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Sex:            practitioner.Gender,
		Education:      education,
		WhatsAppNumber: whatsAppNumber,
		HomeAddress:    homeAddress,
		BirthDate:      formattedBirthDate,
	}

	return response, nil
}

func (uc *userUsecase) updatePatientProfile(ctx context.Context, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Build the update request
	patientFhirRequest := utils.BuildFhirPatientUpdateRequest(request, session.PatientID)

	// Send PUT request to FHIR server to update the user resource
	fhirPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		return nil, err
	}

	response := &responses.UpdateUserProfile{
		PatientID: fhirPatient.ID,
	}

	return response, nil
}

func (uc *userUsecase) updatePractitionerProfile(ctx context.Context, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Build the update request
	practitionerFhirRequest := utils.BuildFhirPractitionerUpdateRequest(request, session.PractitionerID)

	// Send PUT request to FHIR server to update the user resource
	fhirPractitioner, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
	if err != nil {
		return nil, err
	}

	response := &responses.UpdateUserProfile{
		PractitionerID: fhirPractitioner.ID,
	}

	return response, nil
}
