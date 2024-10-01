package users

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/fhir_spark/organizations"
	patientsFhir "konsulin-service/internal/app/services/fhir_spark/patients"
	practitionerRoles "konsulin-service/internal/app/services/fhir_spark/practitioner_role"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/shared/storage"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"strings"
	"time"
)

type userUsecase struct {
	UserRepository             UserRepository
	PatientFhirClient          patientsFhir.PatientFhirClient
	PractitionerFhirClient     practitioners.PractitionerFhirClient
	PractitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient
	OrganizationFhirClient     organizations.OrganizationFhirClient
	RedisRepository            redis.RedisRepository
	SessionService             session.SessionService
	MinioStorage               storage.Storage
	InternalConfig             *config.InternalConfig
}

func NewUserUsecase(
	userMongoRepository UserRepository,
	patientFhirClient patientsFhir.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	practitionerRoleFhirClient practitionerRoles.PractitionerRoleFhirClient,
	organizationFhirClient organizations.OrganizationFhirClient,
	redisRepository redis.RedisRepository,
	sessionService session.SessionService,
	minioStorage storage.Storage,
	internalConfig *config.InternalConfig,
) UserUsecase {
	return &userUsecase{
		UserRepository:             userMongoRepository,
		PatientFhirClient:          patientFhirClient,
		PractitionerFhirClient:     practitionerFhirClient,
		PractitionerRoleFhirClient: practitionerRoleFhirClient,
		OrganizationFhirClient:     organizationFhirClient,
		RedisRepository:            redisRepository,
		SessionService:             sessionService,
		MinioStorage:               minioStorage,
		InternalConfig:             internalConfig,
	}
}

func (uc *userUsecase) GetUserProfileBySession(ctx context.Context, sessionData string) (*responses.UserProfile, error) {
	// Parse session data
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		return nil, err
	}

	existingUser, err := uc.UserRepository.FindByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(nil)
	}

	var preSignedUrl string
	if existingUser.ProfilePictureName != "" {
		objectUrlExpiryTime := time.Duration(uc.InternalConfig.App.MinioPreSignedUrlObjectExpiryTimeInHours) * time.Hour
		preSignedUrl, err = uc.MinioStorage.GetObjectUrlWithExpiryTime(ctx, uc.InternalConfig.Minio.BucketName, existingUser.ProfilePictureName, objectUrlExpiryTime)
		if err != nil {
			return nil, err
		}
	}
	fmt.Println(preSignedUrl)

	// Handle get user profile based on role
	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.getPractitionerProfile(ctx, session, preSignedUrl)
	case constvars.RoleTypePatient:
		return uc.getPatientProfile(ctx, session, preSignedUrl)
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
	}

	if request.ProfilePicture != "" {
		request.ProfilePictureObjectName, err = uc.uploadProfilePicture(ctx, session.Username, request)
		if err != nil {
			return nil, err
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

	// Handle update user profile based on role
	switch session.RoleName {
	case constvars.RoleTypePractitioner:
		return uc.updatePractitionerFhirProfile(ctx, existingUser, session, request)
	case constvars.RoleTypePatient:
		return uc.updatePatientFhirProfile(ctx, existingUser, session, request)
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

func (uc *userUsecase) updatePatientFhirProfile(ctx context.Context, user *models.User, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Create session data with updated user, existing role, and session details
	sessionModel := models.Session{
		UserID:    user.ID,
		PatientID: user.PatientID,
		Email:     user.Email,
		Username:  user.Username,
		RoleID:    session.RoleID,
		RoleName:  session.RoleName,
		SessionID: session.SessionID,
	}

	// Store the session data in Redis with a 1-hour expiration
	err := uc.RedisRepository.Set(ctx, session.SessionID, sessionModel, time.Hour)
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

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

func (uc *userUsecase) updatePractitionerFhirProfile(ctx context.Context, user *models.User, session *models.Session, request *requests.UpdateProfile) (*responses.UpdateUserProfile, error) {
	// Create session data with updated user, existing role, and session details
	sessionModel := models.Session{
		UserID:         user.ID,
		PractitionerID: user.PractitionerID,
		Email:          user.Email,
		Username:       user.Username,
		RoleID:         session.RoleID,
		RoleName:       session.RoleName,
		SessionID:      session.SessionID,
	}

	// Store the session data in Redis with a 1-hour expiration
	err := uc.RedisRepository.Set(ctx, session.SessionID, sessionModel, time.Hour)
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

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

func (uc *userUsecase) getPatientProfile(ctx context.Context, session *models.Session, preSignedUrl string) (*responses.UserProfile, error) {
	// Get patient data from FHIR Spark Patient Client
	patientFhir, err := uc.PatientFhirClient.FindPatientByID(ctx, session.PatientID)
	if err != nil {
		return nil, err
	}

	// Build patient profile response
	response := utils.BuildPatientProfileResponse(patientFhir)
	response.ProfilePicture = preSignedUrl

	// Return the response to Controller
	return response, nil
}

func (uc *userUsecase) getPractitionerProfile(ctx context.Context, session *models.Session, preSignedUrl string) (*responses.UserProfile, error) {
	// Get practitioner data from FHIR Spark Practitioner Client
	practitionerFhir, err := uc.PractitionerFhirClient.FindPractitionerByID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	// Build patient profile response
	response := utils.BuildPractitionerProfileResponse(practitionerFhir)
	response.ProfilePicture = preSignedUrl

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, session.PractitionerID)
	if err != nil {
		return nil, err
	}

	practiceInformations := make([]responses.PracticeInformation, 0, len(practitionerRoles))
	practiceAvailabilities := make([]responses.PracticeAvailability, 0, len(practitionerRoles))

	for _, practitionerRole := range practitionerRoles {
		organizationID := strings.Split(practitionerRole.Organization.Reference, "/")[1]
		organization, err := uc.OrganizationFhirClient.FindOrganizationByID(ctx, organizationID)
		if err != nil {
			return nil, err
		}
		practiceInformations = append(practiceInformations, responses.PracticeInformation{
			ClinicID:    organization.ID,
			ClinicName:  organization.Name,
			Affiliation: organization.Name,
			Specialties: utils.ExtractSpecialties(practitionerRole.Specialty),
			PricePerSession: responses.PricePerSession{
				Value:    practitionerRole.Extension[0].ValueMoney.Value,
				Currency: practitionerRole.Extension[0].ValueMoney.Currency,
			},
		})

		if len(practitionerRole.AvailableTime) > 0 {
			practiceAvailabilities = append(practiceAvailabilities, responses.PracticeAvailability{
				ClinicID:       organization.ID,
				AvailableTimes: utils.ConvertToAvailableTimesResponse(practitionerRole.AvailableTime),
			})
		}
	}

	response.PracticeInformations = practiceInformations
	response.PracticeAvailabilities = practiceAvailabilities

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
