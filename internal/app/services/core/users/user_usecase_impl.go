package users

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/core/session"
	patientsFhir "konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"time"
)

type userUsecase struct {
	UserRepository         UserRepository
	PatientFhirClient      patientsFhir.PatientFhirClient
	PractitionerFhirClient practitioners.PractitionerFhirClient
	RedisRepository        redis.RedisRepository
	SessionService         session.SessionService
	MinioStorage           storage.Storage
	InternalConfig         *config.InternalConfig
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	patientFhirClient patientsFhir.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	redisRepository redis.RedisRepository,
	sessionService session.SessionService,
	minioStorage storage.Storage,
	internalConfig *config.InternalConfig,
) UserUsecase {
	return &userUsecase{
		UserRepository:         userMongoRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		RedisRepository:        redisRepository,
		SessionService:         sessionService,
		MinioStorage:           minioStorage,
		InternalConfig:         internalConfig,
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

	// Make sure to allow the changes if current (session) email is the same as the requested email
	if session.Email != request.Email {
		// Check if email already exists
		existingUser, err := uc.UserRepository.FindByEmail(ctx, request.Email)
		if err != nil {
			return nil, err
		}
		if existingUser != nil {
			return nil, exceptions.ErrEmailAlreadyExist(nil)
		}

		if request.ProfilePicture != "" {
			request.ProfilePictureMinioUrl, err = uc.uploadProfilePicture(ctx, session.Username, request)
			if err != nil {
				return nil, err
			}
		}
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

	// Create session data with updated user, existing role, and session details
	sessionModel := models.Session{
		UserID:    existingUser.ID,
		PatientID: existingUser.PatientID,
		Email:     existingUser.Email,
		Username:  existingUser.Username,
		RoleID:    session.RoleID,
		RoleName:  session.RoleName,
		SessionID: session.SessionID,
	}

	// Store the session data in Redis with a 1-hour expiration
	err = uc.RedisRepository.Set(ctx, session.SessionID, sessionModel, time.Hour)
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

	// Handle update user profile based on role
	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.updatePractitionerFhirProfile(ctx, session, request)
	case constvars.RoleTypePatient:
		return uc.updatePatientFhirProfile(ctx, session, request)
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

func (uc *userUsecase) DeactivateUserBySession(ctx context.Context, sessionData string) error {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return err
	}

	// Delete the user by using his/her 'session.UserID'
	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		return err
	}

	// Set deleted at for existing user
	existingUser.SetDeletedAt()

	// Update the user using 'existingUser' that already updated with the request
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return err
	}

	// Delete the session in Redis using 'session.SessionID'
	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		return err
	}

	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.deactivatePractitionerFhirData(ctx, existingUser)
	case constvars.RoleTypePatient:
		return uc.deactivatePatientFhirData(ctx, existingUser)
	default:
		return exceptions.ErrInvalidRoleType(nil)
	}
}

func (uc *userUsecase) deactivatePractitionerFhirData(ctx context.Context, user *models.User) error {
	// Set deactivate account request for User's fhir resource
	practitionerFhirRequest := user.ConvertToPractitionerFhirDeactivationRequest()

	// Send 'patientFhirRequest' to FHIR Spark Patient Client to update the 'patient' resource
	_, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
	if err != nil {
		return err
	}

	// Return the response back to Controller
	return nil
}

func (uc *userUsecase) deactivatePatientFhirData(ctx context.Context, user *models.User) error {
	// Set deactivate account request for User's fhir resource
	patientFhirRequest := user.ConvertToPatientFhirDeactivationRequest()

	// Send 'patientFhirRequest' to FHIR Spark Patient Client to update the 'patient' resource
	_, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
	if err != nil {
		return err
	}

	// Return the response back to Controller
	return nil
}

func (uc *userUsecase) updatePatientFhirProfile(ctx context.Context, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Build the 'UpdateProfile' request into 'patientFhirRequest'
	patientFhirRequest := utils.BuildFhirPatientUpdateProfileRequest(request, session.PatientID)

	// Send 'patientFhirRequest' to FHIR Spark Patient Client to update the 'patient' resource
	fhirPatient, err := uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
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

func (uc *userUsecase) updatePractitionerFhirProfile(ctx context.Context, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Build the 'UpdateProfile' request into 'practitionerFhirRequest'
	practitionerFhirRequest := utils.BuildFhirPractitionerUpdateProfileRequest(request, session.PractitionerID)

	// Send 'practitionerFhirRequest' to FHIR Spark Practitioner Client to update the 'practitioner' resource
	fhirPractitioner, err := uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
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
	patientFhir, err := uc.PatientFhirClient.FindPatientByID(ctx, session.PatientID)
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
	practitionerFhir, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	// Build patient profile response
	response := utils.BuildPractitionerProfileResponse(practitionerFhir)

	// Return the response to Controller
	return response, nil
}

func (uc *userUsecase) uploadProfilePicture(ctx context.Context, username string, request *requests.UpdateProfile) (string, error) {
	fileName := utils.GenerateFileName(constvars.ImageProfilePicturePrefix, username, request.ProfilePictureExtension)

	profilePictureURL, err := uc.MinioStorage.UploadBase64Image(
		ctx,
		request.ProfilePictureData,
		uc.InternalConfig.Minio.BucketName,
		fileName,
		request.ProfilePictureExtension,
	)

	if err != nil {
		return "", err
	}

	return profilePictureURL, nil
}
