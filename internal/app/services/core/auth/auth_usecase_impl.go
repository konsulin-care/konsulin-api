package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type authUsecase struct {
	UserRepository                  contracts.UserRepository
	RedisRepository                 contracts.RedisRepository
	SessionService                  contracts.SessionService
	RoleRepository                  contracts.RoleRepository
	PatientFhirClient               contracts.PatientFhirClient
	PractitionerFhirClient          contracts.PractitionerFhirClient
	PractitionerRoleFhirClient      contracts.PractitionerRoleFhirClient
	QuestionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient
	MailerService                   contracts.MailerService
	WhatsAppService                 contracts.WhatsAppService
	MinioStorage                    contracts.Storage
	InternalConfig                  *config.InternalConfig
	Roles                           map[string]*models.Role
	mu                              sync.RWMutex
	Log                             *zap.Logger
}

var (
	authUsecaseInstance contracts.AuthUsecase
	onceAuthUsecase     sync.Once
	authUsecaseError    error
)

func NewAuthUsecase(
	userRepository contracts.UserRepository,
	redisRepository contracts.RedisRepository,
	sessionService contracts.SessionService,
	rolesRepository contracts.RoleRepository,
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	practitionerRoleFhirClient contracts.PractitionerRoleFhirClient,
	questionnaireResponsesFhirClient contracts.QuestionnaireResponseFhirClient,
	mailerService contracts.MailerService,
	whatsAppService contracts.WhatsAppService,
	minioStorage contracts.Storage,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) (contracts.AuthUsecase, error) {
	onceAuthUsecase.Do(func() {
		instance := &authUsecase{
			UserRepository:                  userRepository,
			RedisRepository:                 redisRepository,
			SessionService:                  sessionService,
			RoleRepository:                  rolesRepository,
			PatientFhirClient:               patientFhirClient,
			PractitionerFhirClient:          practitionerFhirClient,
			PractitionerRoleFhirClient:      practitionerRoleFhirClient,
			QuestionnaireResponseFhirClient: questionnaireResponsesFhirClient,
			MailerService:                   mailerService,
			MinioStorage:                    minioStorage,
			WhatsAppService:                 whatsAppService,
			InternalConfig:                  internalConfig,
			Roles:                           make(map[string]*models.Role),
			Log:                             logger,
		}

		ctx := context.Background()
		err := instance.loadRoles(ctx)
		if err != nil {
			authUsecaseError = err
			return
		}
		authUsecaseInstance = instance
	})

	return authUsecaseInstance, authUsecaseError
}

func (uc *authUsecase) RegisterViaWhatsApp(ctx context.Context, request *requests.RegisterViaWhatsApp) error {
	whatsAppOTP, err := utils.GenerateOTP(constvars.WHATSAPP_OTP_LENGTH)
	if err != nil {
		return exceptions.ErrClientCustomMessage(err)
	}

	err = uc.checkExistingUserByWhatsAppNumber(ctx, request.To)
	if err != nil {
		return err
	}

	err = uc.createWhatsAppUser(ctx, request.To, whatsAppOTP)
	if err != nil {
		return err
	}

	err = uc.sendWhatsAppOTP(ctx, request.To, whatsAppOTP)
	if err != nil {
		return err
	}

	return nil
}

func (uc *authUsecase) LoginViaWhatsApp(ctx context.Context, request *requests.LoginViaWhatsApp) error {
	whatsAppOTP, err := utils.GenerateOTP(constvars.WHATSAPP_OTP_LENGTH)
	if err != nil {
		return exceptions.ErrClientCustomMessage(err)
	}

	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, request.To)
	if err != nil {
		return err
	}

	if existingUser == nil {
		return exceptions.ErrUserNotExist(nil)
	}

	existingUser.WhatsAppOTP = whatsAppOTP
	existingUser.WhatsAppNumber = request.To
	existingUser.SetWhatsAppOTPExpiryTime(uc.InternalConfig.App.WhatsAppOTPExpiredTimeInMinutes)
	existingUser.SetUpdatedAt()
	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		return err
	}

	whatsAppMessage := &requests.WhatsAppMessage{
		To:        request.To,
		Message:   whatsAppOTP,
		WithImage: false,
	}

	err = uc.WhatsAppService.SendWhatsAppMessage(ctx, whatsAppMessage)
	if err != nil {
		return err
	}

	return nil
}

func (uc *authUsecase) VerifyRegisterWhatsAppOTP(ctx context.Context, request *requests.VerivyRegisterWhatsAppOTP) (*responses.RegisterUserWhatsApp, error) {
	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, request.To)
	if err != nil {
		return nil, err
	}

	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(nil)
	}

	// Check if the whatsapp otp is expired
	if time.Now().After(*existingUser.WhatsAppOTPExpiry) {
		return nil, exceptions.ErrWhatsAppOTPExpired(nil)
	}

	if existingUser.WhatsAppOTP != request.OTP {
		return nil, exceptions.ErrWhatsAppOTPInvalid(nil)
	}

	if request.Role == constvars.ResourcePatient {
		// Build FHIR patient request
		fhirPatientRequest := utils.BuildFhirPatientWhatsAppRegistrationRequest(request.To)

		// Create FHIR patient to Spark and get the response
		fhirPatient, err := uc.PatientFhirClient.CreatePatient(ctx, fhirPatientRequest)
		if err != nil {
			return nil, err
		}

		role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePatient)
		if err != nil {
			return nil, err
		}

		existingUser.RoleID = role.ID
		existingUser.PatientID = fhirPatient.ID
		existingUser.SetUpdatedAt()

		err = uc.UserRepository.UpdateUser(ctx, existingUser)
		if err != nil {
			return nil, err
		}

		// Prepare the response with the generated token and user details
		response := &responses.RegisterUserWhatsApp{
			UserID:    existingUser.ID,
			PatientID: existingUser.PatientID,
		}

		return response, nil

	} else if request.Role == constvars.ResourcePractitioner {
		// Build FHIR practitioner request
		fhirPractitionerRequest := utils.BuildFhirPractitionerWhatsAppRegistrationRequest(request.To)

		// Create FHIR practitioner to Spark and get the response
		fhirPractitioner, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
		if err != nil {
			return nil, err
		}

		role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePractitioner)
		if err != nil {
			return nil, err
		}

		existingUser.RoleID = role.ID
		existingUser.PractitionerID = fhirPractitioner.ID
		existingUser.SetUpdatedAt()

		err = uc.UserRepository.UpdateUser(ctx, existingUser)
		if err != nil {
			return nil, err
		}

		// Prepare the response with the generated token and user details
		response := &responses.RegisterUserWhatsApp{
			UserID:         existingUser.ID,
			PractitionerID: existingUser.PractitionerID,
		}

		return response, nil
	}

	// Return the prepared response
	return nil, exceptions.ErrAuthInvalidRole(nil)
}

func (uc *authUsecase) VerifyLoginWhatsAppOTP(ctx context.Context, request *requests.VerivyLoginWhatsAppOTP) (*responses.LoginUser, error) {
	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, request.To)
	if err != nil {
		return nil, err
	}

	if existingUser == nil {
		return nil, exceptions.ErrUserNotExist(nil)
	}

	// Check if the whatsapp otp is expired
	if time.Now().After(*existingUser.WhatsAppOTPExpiry) {
		return nil, exceptions.ErrWhatsAppOTPExpired(nil)
	}

	if existingUser.WhatsAppOTP != request.OTP {
		return nil, exceptions.ErrWhatsAppOTPInvalid(nil)
	}

	existingUser.Role, err = uc.RoleRepository.FindRoleByID(ctx, existingUser.RoleID)
	if err != nil {
		return nil, err
	}

	if existingUser.Role.Name == constvars.ResourcePatient {
		sessionID := uuid.New().String()

		sessionModel := models.Session{
			UserID:    existingUser.ID,
			PatientID: existingUser.PatientID,
			RoleID:    existingUser.Role.ID,
			RoleName:  existingUser.Role.Name,
			SessionID: sessionID,
		}

		err = uc.RedisRepository.Set(
			ctx,
			sessionID,
			sessionModel,
			time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours),
		)
		if err != nil {
			return nil, err
		}

		tokenString, err := utils.GenerateSessionJWT(
			sessionID,
			uc.InternalConfig.JWT.Secret,
			uc.InternalConfig.App.LoginSessionExpiredTimeInHours,
		)
		if err != nil {
			return nil, exceptions.ErrTokenGenerate(err)
		}

		response := &responses.LoginUser{
			Token: tokenString,
			LoginUserData: responses.LoginUserData{
				Fullname:  existingUser.Fullname,
				UserID:    existingUser.ID,
				RoleID:    existingUser.Role.ID,
				RoleName:  existingUser.Role.Name,
				PatientID: existingUser.PatientID,
			},
		}

		return response, nil
	} else if existingUser.Role.Name == constvars.ResourcePractitioner {
		sessionID := uuid.New().String()

		sessionModel := models.Session{
			UserID:         existingUser.ID,
			PractitionerID: existingUser.PractitionerID,
			RoleID:         existingUser.Role.ID,
			RoleName:       existingUser.Role.Name,
			SessionID:      sessionID,
		}

		err = uc.RedisRepository.Set(
			ctx,
			sessionID,
			sessionModel,
			time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours),
		)
		if err != nil {
			return nil, err
		}

		tokenString, err := utils.GenerateSessionJWT(
			sessionID,
			uc.InternalConfig.JWT.Secret,
			uc.InternalConfig.App.LoginSessionExpiredTimeInHours,
		)
		if err != nil {
			return nil, exceptions.ErrTokenGenerate(err)
		}

		response := &responses.LoginUser{
			Token: tokenString,
			LoginUserData: responses.LoginUserData{
				Fullname:       existingUser.Fullname,
				UserID:         existingUser.ID,
				RoleID:         existingUser.Role.ID,
				RoleName:       existingUser.Role.Name,
				PractitionerID: existingUser.PractitionerID,
			},
		}

		return response, nil
	}

	return nil, exceptions.ErrAuthInvalidRole(nil)
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
	fhirPractitionerRequest := utils.BuildFhirPractitionerRegistrationRequest(request.Username, request.Email)

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
	fhirPatientRequest := utils.BuildFhirPatientRegistrationRequest(request.Username, request.Email)

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

	err = uc.checkQuestionnaireResponseAndAttachWithPatientData(ctx, request.ResponseID, user)
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

	if user.IsDeactivated() {
		if user.IsDeactivationDeadlineExpired(uc.InternalConfig.App.AccountDeactivationAgeInDays) {
			return nil, exceptions.ErrAccountDeactivationAgeExpired(nil)
		}

		user.SetEmptyDeletedAt()
		err = uc.UserRepository.UpdateUser(ctx, user)
		if err != nil {
			return nil, err
		}

		patientFhirRequest := utils.BuildFhirPatientReactivateRequest(user.PatientID)
		_, err = uc.PatientFhirClient.UpdatePatient(ctx, patientFhirRequest)
		if err != nil {
			return nil, err
		}
	}

	// Retrieve the user's role by role ID from the role repository
	userRole, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		// Return error if there is an issue with role retrieval
		return nil, err
	}

	// Check if the role is not of type 'Patient' and return an error if true
	if userRole.IsNotPatient() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	// Verify the provided password with the stored hashed password
	if !utils.CheckPasswordHash(request.Password, user.Password) {
		// Return error if the passwords do not match
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	err = uc.checkQuestionnaireResponseAndAttachWithPatientData(ctx, request.ResponseID, user)
	if err != nil {
		return nil, err
	}

	// Generate a UUID for the session ID
	sessionID := uuid.New().String()

	// Create session data with user, role, and session details
	sessionModel := models.Session{
		UserID:    user.ID,
		PatientID: user.PatientID,
		Email:     user.Email,
		RoleID:    userRole.ID,
		RoleName:  userRole.Name,
		SessionID: sessionID,
	}

	// Store the session data in Redis with a 2-hour expirations
	err = uc.RedisRepository.Set(
		ctx,
		sessionID,
		sessionModel,
		time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours),
	)
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

	// Generate a JWT token using the session ID and secret
	tokenString, err := utils.GenerateSessionJWT(
		sessionID,
		uc.InternalConfig.JWT.Secret,
		uc.InternalConfig.App.LoginSessionExpiredTimeInHours,
	)
	if err != nil {
		// Return error if there is an issue generating the JWT token
		return nil, exceptions.ErrTokenGenerate(err)
	}

	preSignedProfilePictureUrl, err := uc.getPresignedUrl(ctx, user)
	if err != nil {
		return nil, err
	}

	// Prepare the response with the generated token and user details
	response := &responses.LoginUser{
		Token: tokenString,
		LoginUserData: responses.LoginUserData{
			Fullname:       user.Fullname,
			Email:          user.Email,
			UserID:         user.ID,
			RoleID:         userRole.ID,
			RoleName:       userRole.Name,
			PatientID:      user.PatientID,
			ProfilePicture: preSignedProfilePictureUrl,
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

	if user.IsDeactivated() {
		if user.IsDeactivationDeadlineExpired(uc.InternalConfig.App.AccountDeactivationAgeInDays) {
			return nil, exceptions.ErrAccountDeactivationAgeExpired(nil)
		}

		user.SetEmptyDeletedAt()
		err = uc.UserRepository.UpdateUser(ctx, user)
		if err != nil {
			return nil, err
		}

		practitionerFhirRequest := utils.BuildFhirPractitionerReactivateRequest(user.PractitionerID)
		_, err = uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
		if err != nil {
			return nil, err
		}
	}

	// Retrieve the user's role by role ID from the role repository
	userRole, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		// Return error if there is an issue with role retrieval
		return nil, err
	}

	// Check if the role is not of type 'Practitioner' and return an error if true
	if userRole.IsNotPractitioner() {
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	// Verify the provided password with the stored hashed password
	if !utils.CheckPasswordHash(request.Password, user.Password) {
		// Return error if the passwords do not match
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	// Get the organizations linked to this practitioner
	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, user.PractitionerID)
	if err != nil {
		return nil, err
	}

	practitionerOrganizationIDs := utils.ExtractOrganizationIDsFromPractitionerRoles(practitionerRoles)

	// Generate a UUID for the session ID
	sessionID := uuid.New().String()

	// Create session data with user, role, and session details
	sessionData := models.Session{
		UserID:         user.ID,
		PractitionerID: user.PractitionerID,
		Email:          user.Email,
		Username:       user.Username,
		RoleID:         userRole.ID,
		RoleName:       userRole.Name,
		SessionID:      sessionID,
		ClinicIDs:      practitionerOrganizationIDs,
	}

	// Store the session data in Redis with a 1-hour expiration
	err = uc.RedisRepository.Set(ctx, sessionID, sessionData, time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours))
	if err != nil {
		// Return error if there is an issue storing the session data
		return nil, err
	}

	// Generate a JWT token using the session ID and secret
	tokenString, err := utils.GenerateSessionJWT(sessionID, uc.InternalConfig.JWT.Secret, uc.InternalConfig.App.LoginSessionExpiredTimeInHours)
	if err != nil {
		// Return error if there is an issue generating the JWT token
		return nil, exceptions.ErrTokenGenerate(err)
	}

	preSignedProfilePictureUrl, err := uc.getPresignedUrl(ctx, user)
	if err != nil {
		return nil, err
	}

	// Prepare the response with the generated token and user details
	response := &responses.LoginUser{
		Token: tokenString,
		LoginUserData: responses.LoginUserData{
			Fullname:       user.Fullname,
			Email:          user.Email,
			UserID:         user.ID,
			RoleID:         userRole.ID,
			RoleName:       userRole.Name,
			PractitionerID: user.PractitionerID,
			ClinicIDs:      practitionerOrganizationIDs,
			ProfilePicture: preSignedProfilePictureUrl,
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
		return nil
	}

	uuid := uuid.New().String()
	user.ResetToken, err = utils.GenerateResetPasswordJWT(uuid, uc.InternalConfig.JWT.Secret, uc.InternalConfig.App.ForgotPasswordTokenExpiredTimeInMinutes)
	if err != nil {
		return exceptions.ErrTokenGenerate(err)
	}
	user.SetResetTokenExpiryTime(uc.InternalConfig.App.ForgotPasswordTokenExpiredTimeInMinutes)

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
	if time.Now().After(*user.ResetTokenExpiry) {
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

func (uc *authUsecase) checkQuestionnaireResponseAndAttachWithPatientData(ctx context.Context, responseID string, user *models.User) error {
	if responseID != "" {
		questionnaireResponseID := new(string)
		redisRawQuestionnaireResponseID, err := uc.RedisRepository.Get(
			ctx,
			responseID,
		)
		if err != nil {
			return err
		}

		err = json.Unmarshal([]byte(redisRawQuestionnaireResponseID), questionnaireResponseID)
		if err != nil {
			return exceptions.ErrCannotParseJSON(err)
		}

		questionnaireResponseFhir, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, *questionnaireResponseID)
		if err != nil {
			return err
		}
		questionnaireResponseFhir.Subject.Reference = fmt.Sprintf("%s/%s", constvars.ResourcePatient, user.PatientID)
		_, err = uc.QuestionnaireResponseFhirClient.UpdateQuestionnaireResponse(ctx, questionnaireResponseFhir)
		if err != nil {
			return err
		}
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

// 'checkExistingUserByWhatsAppNumber' usage flow is:
//
// 1. find user from UserRepository by phoneNumber
//
// 2. check whether the user is exist or not
func (uc *authUsecase) checkExistingUserByWhatsAppNumber(ctx context.Context, phoneNumber string) error {
	// find user from UserRepository by phoneNumber
	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, phoneNumber)
	if err != nil {
		return err
	}

	// check whether the user is exist or not
	if existingUser != nil {
		return exceptions.ErrPhoneNumberAlreadyRegistered(nil)
	}
	return nil
}

// 'createWhatsAppUser' flow usage is:
//
// 1. initiate new user entity
//
// 2. set required user attributes the user entity
//
// 3. send the entity to UserRepository to be created
func (uc *authUsecase) createWhatsAppUser(ctx context.Context, phoneNumber string, whatsAppOTP string) error {
	// initiate new user entity
	user := new(models.User)

	// set required user attributes the user entity
	user.WhatsAppOTP = whatsAppOTP
	user.WhatsAppNumber = phoneNumber
	user.SetWhatsAppOTPExpiryTime(uc.InternalConfig.App.WhatsAppOTPExpiredTimeInMinutes)
	user.SetCreatedAtUpdatedAt()

	// send the entity to userRepository to be created
	_, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

// 'sendWhatsAppOTP' flow usage is:
//
// 1. create a new WhatsAppMessage DTO request
//
// 2. send the request DTO to WhatsAppService to be sent
func (uc *authUsecase) sendWhatsAppOTP(ctx context.Context, phoneNumber string, whatsAppOTP string) error {
	// create a new WhatsAppMessage DTO request
	whatsAppMessage := &requests.WhatsAppMessage{
		To:        phoneNumber,
		Message:   whatsAppOTP,
		WithImage: false,
	}

	// send the request DTO to WhatsAppService to be sent
	err := uc.WhatsAppService.SendWhatsAppMessage(ctx, whatsAppMessage)
	if err != nil {
		return err
	}
	return nil
}

func (uc *authUsecase) getPresignedUrl(ctx context.Context, user *models.User) (preSignedUrl string, err error) {
	if user.ProfilePictureName != "" {
		objectUrlExpiryTime := time.Duration(uc.InternalConfig.App.MinioPreSignedUrlObjectExpiryTimeInHours) * time.Hour
		preSignedUrl, err = uc.MinioStorage.GetObjectUrlWithExpiryTime(ctx, uc.InternalConfig.Minio.BucketName, user.ProfilePictureName, objectUrlExpiryTime)
		if err != nil {
			return "", err
		}
	}
	return preSignedUrl, err
}
