package auth

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/patients"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/users"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"time"

	"github.com/google/uuid"
)

type authUsecase struct {
	PatientRepository patients.PatientRepository
	UserRepository    users.UserRepository
	RedisRepository   redis.RedisRepository
	PatientFhirClient patients.PatientFhirClient
	InternalConfig    *config.InternalConfig
}

func NewAuthUsecase(
	patientMongoRepository patients.PatientRepository,
	userMongoRepository users.UserRepository,
	redisRepository redis.RedisRepository,
	patientFhirClient patients.PatientFhirClient,
	internalConfig *config.InternalConfig,
) AuthUsecase {
	return &authUsecase{
		PatientRepository: patientMongoRepository,
		UserRepository:    userMongoRepository,
		RedisRepository:   redisRepository,
		PatientFhirClient: patientFhirClient,
		InternalConfig:    internalConfig,
	}
}

func (uc *authUsecase) RegisterPatient(ctx context.Context, request *requests.RegisterPatient) (*responses.RegisterPatient, error) {
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

	// Build fhir patient request
	fhirPatientRequest := utils.BuildFhirPatientRequest(request.Username, request.Email)

	// Create fhir patient to spark and get the model
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
		Password:  hashedPassword,
		PatientID: fhirPatient.ID,
	}

	// Create user
	userID, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	// Map the data into response output ready to be used by controller
	response := &responses.RegisterPatient{
		UserID:    userID,
		PatientID: fhirPatient.ID,
	}

	return response, nil
}

func (uc *authUsecase) LoginPatient(ctx context.Context, request *requests.LoginPatient) (*responses.LoginPatient, error) {
	// Get user by username
	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Check password
	if !utils.CheckPasswordHash(request.Password, user.Password) {
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Generate a UUID for the session key
	sessionID := uuid.New().String()

	// Create session data
	sessionData := models.Session{
		UserID:    user.ID,
		PatientID: user.PatientID,
		SessionID: sessionID,
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

	response := &responses.LoginPatient{
		Token: tokenString,
	}
	return response, nil
}

func (uc *authUsecase) LogoutPatient(ctx context.Context, sessionData string) error {
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
