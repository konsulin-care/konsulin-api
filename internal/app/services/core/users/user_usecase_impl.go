package users

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
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
	RedisRepository        redis.RedisRepository
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	redisRepository redis.RedisRepository,
) UserUsecase {
	return &userUsecase{
		UserRepository:         userMongoRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		RedisRepository:        redisRepository,
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
	// Get patient data from FHIR Spark Patient Client
	patient, err := uc.PatientFhirClient.GetPatientByID(ctx, session.PatientID)
	if err != nil {
		return nil, err
	}

	// The 'patient' data that we get from FHIR Spark,
	// then built into suitable format to be shown to our end-users
	fullname := utils.GetFullName(patient.Name)
	email, whatsAppNumber := utils.GetEmailAndWhatsapp(patient.Telecom)
	age := utils.CalculateAge(patient.BirthDate)
	education := utils.GetEducationFromExtensions(patient.Extension)
	homeAddress := utils.GetHomeAddress(patient.Address)
	formattedBirthDate := utils.FormatBirthDate(patient.BirthDate)

	// After changing the 'patient' data into the suitable format,
	// build it into 'UserProfile' response before sending it back to Controller
	response := &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Gender:         patient.Gender,
		Education:      education,
		WhatsAppNumber: whatsAppNumber,
		Address:        homeAddress,
		BirthDate:      formattedBirthDate,
	}

	// Return the response to Controller
	return response, nil
}

func (uc *userUsecase) getPractitionerProfile(ctx context.Context, session *models.Session) (*responses.UserProfile, error) {
	// Get practitioner data from FHIR Spark Practitioner Client
	practitioner, err := uc.PractitionerFhirClient.GetPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	// The 'practitioner' data that we get from FHIR Spark,
	// then built into suitable format to be shown to our end-users
	fullname := utils.GetFullName(practitioner.Name)
	email, whatsAppNumber := utils.GetEmailAndWhatsapp(practitioner.Telecom)
	age := utils.CalculateAge(practitioner.BirthDate)
	education := utils.GetEducationFromExtensions(practitioner.Extension)
	workAddress := utils.GetWorkAddress(practitioner.Address)
	formattedBirthDate := utils.FormatBirthDate(practitioner.BirthDate)

	// After changing the 'practitioner' data into the suitable format,
	// build it into 'UserProfile' response before sending it back to Controller
	response := &responses.UserProfile{
		Fullname:       fullname,
		Email:          email,
		Age:            age,
		Gender:         practitioner.Gender,
		Education:      education,
		WhatsAppNumber: whatsAppNumber,
		Address:        workAddress,
		BirthDate:      formattedBirthDate,
	}

	// Send the response to the Controller
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

	// Find user by ID
	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	// Throw error userNotExist if existingUser doesn't exist
	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(err)
	}
	// Set the existingUser data with requests.UpdateProfile
	existingUser.SetUpdateProfileData(request)

	// Update the user
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return nil, err
	}

	// Build the response before sending it back to Controller
	response := &responses.UpdateUserProfile{
		PatientID: fhirPatient.ID,
	}

	// Return the response
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

	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(err)
	}

	existingUser.SetUpdateProfileData(request)

	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return nil, err
	}

	response := &responses.UpdateUserProfile{
		PractitionerID: fhirPractitioner.ID,
	}

	return response, nil
}

func (uc *userUsecase) DeleteUserBySession(ctx context.Context, sessionData string) error {
	session := new(models.Session)
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return exceptions.ErrCannotParseJSON(err)
	}

	err = uc.UserRepository.DeleteByID(ctx, session.UserID)
	if err != nil {
		return err
	}

	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		return err
	}

	return nil
}
