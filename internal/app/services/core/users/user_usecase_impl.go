package users

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/core/session"
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
	SessionService         session.SessionService
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	redisRepository redis.RedisRepository,
	sessionService session.SessionService,
) UserUsecase {
	return &userUsecase{
		UserRepository:         userMongoRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		RedisRepository:        redisRepository,
		SessionService:         sessionService,
	}
}

func (uc *userUsecase) GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error) {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	// Handle get user profile based on role
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
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	// Handle update user profile based on role
	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.updatePractitionerProfile(ctx, session, request)
	case constvars.RoleTypePatient:
		return uc.updatePatientProfile(ctx, session, request)
	default:
		return nil, exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) DeleteUserBySession(ctx context.Context, sessionData string) error {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return err
	}

	// Delete the user by using his/her 'session.UserID'
	err = uc.UserRepository.DeleteByID(ctx, session.UserID)
	if err != nil {
		return err
	}

	// Delete the session in Redis using 'session.SessionID'
	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		return err
	}

	// Return nil error indicating successful deletion
	return nil
}

func (uc *userUsecase) updatePatientProfile(ctx context.Context, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Build the 'UpdateProfile' request into 'patientFhirRequest'
	patientFhirRequest := utils.BuildFhirPatientUpdateRequest(request, session.PatientID)

	// Send 'patientFhirRequest' to FHIR Spark Patient Client to update the 'patient' resource
	fhirPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		return nil, err
	}

	// Find user by ID by 'session.UserID'
	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	// Throw 'ErrUserNotExist' if existingUser doesn't exist
	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(nil)
	}

	// Set the existingUser data with 'UpdateProfile' request
	existingUser.SetDataForUpdateProfile(request)

	// Update the user using 'existingUser' that already updated with the request
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return nil, err
	}

	// Build the response before sending it back to Controller
	response := &responses.UpdateUserProfile{
		PatientID: fhirPatient.ID,
	}

	// Return the response back to Controller
	return response, nil
}

func (uc *userUsecase) updatePractitionerProfile(ctx context.Context, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Build the 'UpdateProfile' request into 'practitionerFhirRequest'
	practitionerFhirRequest := utils.BuildFhirPractitionerUpdateRequest(request, session.PractitionerID)

	// Send 'practitionerFhirRequest' to FHIR Spark Practitioner Client to update the 'practitioner' resource
	fhirPractitioner, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
	if err != nil {
		return nil, err
	}

	// Find user by ID by 'session.UserID'
	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}
	// Throw error 'userNotExist' if existingUser doesn't exist
	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(err)
	}

	// Set the existingUser data with requests.UpdateProfile
	existingUser.SetDataForUpdateProfile(request)

	// Update the user using 'existingUser' that already updated with the request
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return nil, err
	}

	// Build the response before sending it back to Controller
	response := &responses.UpdateUserProfile{
		PractitionerID: fhirPractitioner.ID,
	}

	// Return the response back to Controller
	return response, nil
}

func (uc *userUsecase) getPatientProfile(ctx context.Context, session *models.Session) (*responses.UserProfile, error) {
	// Get patient data from FHIR Spark Patient Client
	patientFhir, err := uc.PatientFhirClient.GetPatientByID(ctx, session.PatientID)
	if err != nil {
		return nil, err
	}

	// Build patient profile response
	response := utils.BuildPatientProfileResponse(patientFhir)

	// Return the response to Controller
	return response, nil
}

func (uc *userUsecase) getPractitionerProfile(ctx context.Context, session *models.Session) (*responses.UserProfile, error) {
	// Get practitioner data from FHIR Spark Practitioner Client
	practitionerFhir, err := uc.PractitionerFhirClient.GetPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	// Build patient profile response
	response := utils.BuildPractitionerProfileResponse(practitionerFhir)

	// Return the response to Controller
	return response, nil
}
