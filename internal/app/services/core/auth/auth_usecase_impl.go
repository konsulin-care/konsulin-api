package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/app/services/shared/smtp"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"sync"
	"time"

	"github.com/google/uuid"
)

type authUsecase struct {
	UserRepository         users.UserRepository
	RedisRepository        redis.RedisRepository
	RoleRepository         roles.RoleRepository
	PatientFhirClient      patients.PatientFhirClient
	PractitionerFhirClient practitioners.PractitionerFhirClient
	SMTPUsecase            smtp.SMTPUsecase
	InternalConfig         *config.InternalConfig
	Roles                  map[string]*models.Role
	mu                     sync.RWMutex
}

func NewAuthUsecase(
	userMongoRepository users.UserRepository,
	redisRepository redis.RedisRepository,
	rolesRepository roles.RoleRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	smtpUsecase smtp.SMTPUsecase,
	internalConfig *config.InternalConfig,
) (AuthUsecase, error) {
	authUsecase := &authUsecase{
		UserRepository:         userMongoRepository,
		RedisRepository:        redisRepository,
		RoleRepository:         rolesRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		SMTPUsecase:            smtpUsecase,
		InternalConfig:         internalConfig,
		Roles:                  make(map[string]*models.Role),
	}
	err := authUsecase.loadRoles()
	if err != nil {
		return nil, err
	}

	return authUsecase, nil
}

func (uc *authUsecase) RegisterClinician(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
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

	// Build FHIR practitioner request
	fhirPractitionerRequest := utils.BuildFhirPractitionerRequest(request.Username, request.Email)

	// Create FHIR practitioner to Spark and get the response
	fhirPractitioner, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
	if err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		return nil, exceptions.ErrHashPassword(err)
	}

	// Find the practitioner role
	role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePractitioner)
	if err != nil {
		return nil, err
	}

	// Build the user model
	user := &models.User{
		Username:       request.Username,
		Email:          request.Email,
		RoleID:         role.ID,
		PractitionerID: fhirPractitioner.ID,
		Password:       hashedPassword,
		TimeModel: models.TimeModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
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

func (uc *authUsecase) RegisterPatient(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
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

	// Build FHIR patient request
	fhirPatientRequest := utils.BuildFhirPatientRequest(request.Username, request.Email)

	// Create FHIR patient to Spark and get the response
	fhirPatient, err := uc.PatientFhirClient.CreatePatient(ctx, fhirPatientRequest)
	if err != nil {
		return nil, err
	}

	// Hash password
	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		return nil, exceptions.ErrHashPassword(err)
	}

	// Find the patient role
	role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePatient)
	if err != nil {
		return nil, err
	}

	// Build the user model
	user := &models.User{
		Username:  request.Username,
		Email:     request.Email,
		RoleID:    role.ID,
		PatientID: fhirPatient.ID,
		Password:  hashedPassword,
		TimeModel: models.TimeModel{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
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

func (uc *authUsecase) LoginPatient(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error) {
	// Get user by username
	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	role, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	if role.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
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
		RoleID:    role.ID,
		RoleName:  role.Name,
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

	response := &responses.LoginUser{
		Token: tokenString,
		LoginUserData: responses.LoginUserData{
			Name:     user.Name,
			Email:    user.Email,
			UserID:   user.ID,
			RoleID:   role.ID,
			RoleName: role.Name,
		},
	}
	return response, nil
}

func (uc *authUsecase) LoginClinician(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error) {
	// Get user by username
	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	role, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		return nil, err
	}

	if role.IsNotPractitioner() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
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
		PractitionerID: user.PractitionerID,
		RoleID:         role.ID,
		RoleName:       role.Name,
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
		Token: tokenString,
		LoginUserData: responses.LoginUserData{
			Name:     user.Name,
			Email:    user.Email,
			UserID:   user.ID,
			RoleID:   role.ID,
			RoleName: role.Name,
		},
	}
	return response, nil
}

func (uc *authUsecase) LogoutUser(ctx context.Context, sessionData string) error {
	session := new(models.Session)
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

func (uc *authUsecase) IsUserHasPermission(ctx context.Context, request requests.AuthorizeUser) (bool, error) {
	session := new(models.Session)
	err := json.Unmarshal([]byte(request.SessionData), &session)
	if err != nil {
		return false, exceptions.ErrCannotParseJSON(err)
	}

	uc.mu.RLock()
	defer uc.mu.RUnlock()

	role, exists := uc.Roles[session.RoleID]
	if !exists {
		return false, exceptions.ErrAuthInvalidRole(nil)
	}

	for _, permission := range role.Permissions {
		if permission.Resource == request.Resource {
			for _, allowedAction := range permission.Actions {
				if allowedAction == request.RequiredAction {
					return true, nil
				}
			}
		}
	}

	return false, exceptions.ErrAuthInvalidRole(nil)
}

func (uc *authUsecase) loadRoles() error {
	ctx := context.Background()
	roles, err := uc.RoleRepository.GetAllRoles(ctx)
	if err != nil {
		return err
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()

	for _, role := range roles {
		r := role
		uc.Roles[role.ID] = &r
	}

	return nil
}

func (uc *authUsecase) GetSessionData(ctx context.Context, sessionID string) (string, error) {
	sessionData, err := uc.RedisRepository.Get(ctx, sessionID)
	if err != nil {
		return "", exceptions.ErrTokenInvalid(err)
	}
	return sessionData, nil
}

func (uc *authUsecase) ForgotPassword(ctx context.Context, request *requests.ForgotPassword) error {
	user, err := uc.UserRepository.FindByEmail(ctx, request.Email)
	if err != nil {
		return err
	}
	if user == nil {
		return exceptions.ErrUserNotExist(nil)
	}

	token := uuid.New().String()
	user.ResetToken = token

	err = uc.UserRepository.UpdateUser(ctx, user)
	if err != nil {
		return err
	}

	resetLink := uc.InternalConfig.App.ResetPasswordUrl + token
	emailBody := fmt.Sprintf(constvars.EmailBodyResetPassword + resetLink)

	err = uc.SMTPUsecase.SendEmail(user.Email, constvars.EmailForgotPasswordSubjectMessage, emailBody)
	if err != nil {
		return err
	}
	return nil
}

func (uc *authUsecase) ResetPassword(ctx context.Context, request *requests.ResetPassword) error {
	user, err := uc.UserRepository.FindByResetToken(ctx, request.Token)
	if err != nil {
		return err
	}

	hashedNewPassword, err := utils.HashPassword(request.NewPassword)
	if err != nil {
		return err
	}
	request.HashedNewPassword = hashedNewPassword
	user.SetUpdateResetPassword(request)

	err = uc.UserRepository.UpdateUser(ctx, user)
	if err != nil {
		return err
	}

	return nil
}
