package auth

import (
	"context"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/app/services/core/roles"
	"konsulin-service/internal/app/services/core/session"
	"konsulin-service/internal/app/services/core/users"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/shared/mailer"
	"konsulin-service/internal/app/services/shared/redis"
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
	SessionService         session.SessionService
	RoleRepository         roles.RoleRepository
	PatientFhirClient      patients.PatientFhirClient
	PractitionerFhirClient practitioners.PractitionerFhirClient
	MailerService          mailer.MailerService
	InternalConfig         *config.InternalConfig
	Roles                  map[string]*models.Role
	mu                     sync.RWMutex
}

func NewAuthUsecase(
	userMongoRepository users.UserRepository,
	redisRepository redis.RedisRepository,
	sessionService session.SessionService,
	rolesRepository roles.RoleRepository,
	patientFhirClient patients.PatientFhirClient,
	practitionerFhirClient practitioners.PractitionerFhirClient,
	mailerService mailer.MailerService,
	internalConfig *config.InternalConfig,
) (AuthUsecase, error) {
	authUsecase := &authUsecase{
		UserRepository:         userMongoRepository,
		RedisRepository:        redisRepository,
		SessionService:         sessionService,
		RoleRepository:         rolesRepository,
		PatientFhirClient:      patientFhirClient,
		PractitionerFhirClient: practitionerFhirClient,
		MailerService:          mailerService,
		InternalConfig:         internalConfig,
		Roles:                  make(map[string]*models.Role),
	}

	ctx := context.Background()
	err := authUsecase.loadRoles(ctx)
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

	// Check if email or username already exists
	existingUser, err := uc.UserRepository.FindByEmailOrUsername(ctx, request.Email, request.Username)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, exceptions.ErrEmailAlreadyExist(nil)
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

	// Check if email or username already exists
	existingUser, err := uc.UserRepository.FindByEmailOrUsername(ctx, request.Email, request.Username)
	if err != nil {
		return nil, err
	}
	if existingUser != nil {
		return nil, exceptions.ErrEmailAlreadyExist(nil)
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
	// Retrieve the user by username from the user repository
	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		// Return error if there is an issue with the user retrieval
		return nil, err
	}
	if user == nil {
		// Return error if the user is not found
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Retrieve the user's role by role ID from the role repository
	role, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		// Return error if there is an issue with role retrieval
		return nil, err
	}

	// Check if the role is not of type 'Patient' and return an error if true
	if role.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	// Verify the provided password with the stored hashed password
	if !utils.CheckPasswordHash(request.Password, user.Password) {
		// Return error if the passwords do not match
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Generate a UUID for the session ID
	sessionID := uuid.New().String()

	// Create session data with user, role, and session details
	sessionData := models.Session{
		UserID:    user.ID,
		PatientID: user.PatientID,
		Email:     user.Email,
		RoleID:    role.ID,
		RoleName:  role.Name,
		SessionID: sessionID,
	}

	// Store the session data in Redis with a 1-hour expiration
	err = uc.RedisRepository.Set(ctx, sessionID, sessionData, time.Hour)
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

	// Generate a JWT token using the session ID and secret
	tokenString, err := utils.GenerateSessionJWT(sessionID, uc.InternalConfig.JWT.Secret, uc.InternalConfig.JWT.ExpTimeInHour)
	if err != nil {
		// Return error if there is an issue generating the JWT token
		return nil, exceptions.ErrTokenGenerate(err)
	}

	// Prepare the response with the generated token and user details
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
	// Return the prepared response
	return response, nil
}
func (uc *authUsecase) LoginClinician(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error) {
	// Retrieve the user by username from the user repository
	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		// Return error if there is an issue with the user retrieval
		return nil, err
	}
	if user == nil {
		// Return error if the user is not found
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Retrieve the user's role by role ID from the role repository
	role, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		// Return error if there is an issue with role retrieval
		return nil, err
	}

	// Check if the role is not of type 'Practitioner' and return an error if true
	if role.IsNotPractitioner() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	// Verify the provided password with the stored hashed password
	if !utils.CheckPasswordHash(request.Password, user.Password) {
		// Return error if the passwords do not match
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Generate a UUID for the session ID
	sessionID := uuid.New().String()

	// Create session data with user, role, and session details
	sessionData := models.Session{
		UserID:         user.ID,
		PractitionerID: user.PractitionerID,
		Email:          user.Email,
		Username:       user.Username,
		RoleID:         role.ID,
		RoleName:       role.Name,
		SessionID:      sessionID,
	}

	// Store the session data in Redis with a 1-hour expiration
	err = uc.RedisRepository.Set(ctx, sessionID, sessionData, time.Hour)
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

	// Generate a JWT token using the session ID and secret
	tokenString, err := utils.GenerateSessionJWT(sessionID, uc.InternalConfig.JWT.Secret, uc.InternalConfig.JWT.ExpTimeInHour)
	if err != nil {
		// Return error if there is an issue generating the JWT token
		return nil, exceptions.ErrTokenGenerate(err)
	}

	// Prepare the response with the generated token and user details
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
	// Return the prepared response
	return response, nil
}

func (uc *authUsecase) LogoutUser(ctx context.Context, sessionData string) error {
	// Parse the session data using the SessionService
	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		// Return an error if parsing the session data fails
		return err
	}

	// Delete the session data from Redis using the session ID
	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		// Return an error if there is an issue deleting the session data from Redis
		return err
	}

	// Return nil if the session was successfully deleted
	return nil
}

func (uc *authUsecase) IsUserHasPermission(ctx context.Context, request requests.AuthorizeUser) (bool, error) {
	// Parse the session data using the SessionService
	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		// Return false and an error if parsing the session data fails
		return false, err
	}

	// Acquire a read lock on the mutex to safely access the roles map
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	// Check if the role associated with the session exists in the roles map
	role, exists := uc.Roles[session.RoleID]
	if !exists {
		// Return false and an error if the role does not exist
		return false, exceptions.ErrAuthInvalidRole(nil)
	}

	// Check if the user role has the required permission for the requested resource
	if uc.isRoleHasPermission(role, request.Resource, request.RequiredAction) {
		return true, nil
	}

	// Return false and an error if the user does not have the required permission
	return false, exceptions.ErrAuthInvalidRole(nil)
}

func (uc *authUsecase) ForgotPassword(ctx context.Context, request *requests.ForgotPassword) error {
	user, err := uc.UserRepository.FindByEmail(ctx, request.Email)
	if err != nil {
		return err
	}
	if user == nil {
		return exceptions.ErrUserNotExist(nil)
	}

	uuid := uuid.New().String()
	user.ResetToken, err = utils.GenerateResetPasswordJWT(uuid, uc.InternalConfig.JWT.Secret, uc.InternalConfig.App.ForgotPasswordTokenExpTimeInMinute)
	if err != nil {
		return exceptions.ErrTokenGenerate(err)
	}
	user.ResetTokenExpiry = time.Now().Add(time.Duration(uc.InternalConfig.App.ForgotPasswordTokenExpTimeInMinute) * time.Minute)
	user.SetUpdatedAt()

	err = uc.UserRepository.UpdateUser(ctx, user)
	if err != nil {
		return err
	}

	expiryTimeString := user.ResetTokenExpiry.Format("02 January 2006, 15:04 MST")
	resetLink := uc.InternalConfig.App.ResetPasswordUrl + user.ResetToken
	emailPayload := utils.BuildForgotPasswordEmailPayload(
		uc.InternalConfig.Mailer.EmailSender,
		request.Email,
		resetLink,
		user.Fullname,
		expiryTimeString,
	)

	err = uc.MailerService.SendEmail(ctx, emailPayload)
	if err != nil {
		return err
	}
	return nil
}

func (uc *authUsecase) ResetPassword(ctx context.Context, request *requests.ResetPassword) error {
	// Check if passwords match
	if request.NewPassword != request.RetypeNewPassword {
		return exceptions.ErrPasswordDoNotMatch(nil)
	}

	user, err := uc.UserRepository.FindByResetToken(ctx, request.Token)
	if err != nil {
		return err
	}

	// Check if the reset token is expired
	if time.Now().After(user.ResetTokenExpiry) {
		return exceptions.ErrTokenResetPasswordExpired(nil)
	}

	hashedNewPassword, err := utils.HashPassword(request.NewPassword)
	if err != nil {
		return exceptions.ErrHashPassword(err)
	}

	request.HashedNewPassword = hashedNewPassword
	user.SetDataForUpdateResetPassword(request)

	err = uc.UserRepository.UpdateUser(ctx, user)
	if err != nil {
		return err
	}

	return nil
}

func (uc *authUsecase) isRoleHasPermission(role *models.Role, resource, requiredAction string) bool {
	for _, permission := range role.Permissions {
		if permission.Resource == resource {
			for _, action := range permission.Actions {
				if action == requiredAction {
					return true
				}
			}
		}
	}
	return false
}

func (uc *authUsecase) loadRoles(ctx context.Context) error {
	roles, err := uc.RoleRepository.FindAll(ctx)
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
