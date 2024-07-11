package auth

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/patients"
	"konsulin-service/internal/app/services/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/users"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"time"

	"github.com/google/uuid"
)

type authUsecase struct {
	UserRepository         users.UserRepository
	RedisRepository        redis.RedisRepository
	PatientFhirClient      patients.PatientFhirClient
	PractitionerFhirClient practitioners.PractitionerFhirClient
	InternalConfig         *config.InternalConfig
}

func NewAuthUsecase(
	userMongoRepository users.UserRepository,
	redisRepository redis.RedisRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	internalConfig *config.InternalConfig,
) AuthUsecase {
	return &authUsecase{
		UserRepository:         userMongoRepository,
		RedisRepository:        redisRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		InternalConfig:         internalConfig,
	}
}

func (uc *authUsecase) RegisterUser(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
	// Check if passwords match
	if request.Password != request.RetypePassword {
		return nil, exceptions.ErrPasswordDoNotMatch(nil)
	}

	// Check if email already exists
	existingUser, err := uc.UserRepository.FindByEmail(ctx, request.Email)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, exceptions.ErrEmailAlreadyExist(nil)
	}

	// Check if username already exists
	existingUser, err = uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, exceptions.ErrUsernameAlreadyExist(nil)
	}

	switch request.UserType {
	case constvars.UserTypePractitioner:
		return uc.registerPatient(ctx, request)
	case constvars.UserTypePatient:
		return uc.registerClinician(ctx, request)
	default:
		return nil, exceptions.ErrInvalidUserType(nil)
	}
}

func (uc *authUsecase) LoginUser(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error) {
	// Get user by username
	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}
	if user.UserType != request.UserType {
		return nil, exceptions.ErrNotMatchUserType(nil)
	}

	// Check password
	if !utils.CheckPasswordHash(request.Password, user.Password) {
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Generate a UUID for the session key
	sessionID := uuid.New().String()

	// Create session data
	sessionData := models.Session{
		UserID:         user.ID,
		PatientID:      user.PatientID,
		PractitionerID: user.PractitionerID,
		UserType:       user.UserType,
		SessionID:      sessionID,
	}

	// Store session data in Redis
	err = uc.RedisRepository.Set(ctx, sessionID, sessionData, time.Hour)
	if err != nil {
		return nil, err
	}

	// Create a JWT token
	tokenString, err := utils.GenerateJWT(sessionID, uc.InternalConfig.JWT.Secret)
	if err != nil {
		return nil, err
	}

	response := &responses.LoginUser{
		Token:    tokenString,
		UserID:   sessionData.UserID,
		UserType: sessionData.UserType,
	}
	return response, nil
}

func (uc *authUsecase) LogoutUser(ctx context.Context, sessionData string) error {
	var session models.Session
	err := json.Unmarshal([]byte(sessionData), &session)
	if err != nil {
		return exceptions.ErrCannotParseJSON(err)
	}

	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		return err
	}

	return nil
}

func (uc *authUsecase) registerPatient(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
	// Build FHIR patient request
	fhirPatientRequest := utils.BuildFhirPatientRequest(request.Username, request.Email)

	// Create FHIR patient to Spark and get the model
	fhirPatient, err := uc.PatientFhirClient.CreatePatient(ctx, fhirPatientRequest)
	if err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		return nil, exceptions.ErrHashPassword(err)
	}

	// Build the user model
	user := &models.User{
		Username:  request.Username,
		Email:     request.Email,
		UserType:  request.UserType,
		PatientID: fhirPatient.ID,
		Password:  hashedPassword,
	}

	// Create user
	userID, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	// Map the data into response output ready to be used by controller
	response := &responses.RegisterUser{
		UserID:    userID,
		PatientID: fhirPatient.ID,
	}

	return response, nil
}

func (uc *authUsecase) registerClinician(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
	// Build FHIR practitioner request
	fhirPractitionerRequest := utils.BuildFhirPractitionerRequest(request.Username, request.Email)

	// Create FHIR practitioner to Spark and get the model
	fhirPractitioner, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
	if err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		return nil, exceptions.ErrHashPassword(err)
	}

	// Build the user model
	user := &models.User{
		Username:       request.Username,
		Email:          request.Email,
		UserType:       request.UserType,
		PractitionerID: fhirPractitioner.ID,
		Password:       hashedPassword,
		// Add ClinicianID if applicable
	}

	// Create user
	userID, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	// Map the data into response output ready to be used by controller
	response := &responses.RegisterUser{
		UserID:         userID,
		PractitionerID: fhirPractitioner.ID, // Add ClinicianID if applicable
	}

	return response, nil
}
