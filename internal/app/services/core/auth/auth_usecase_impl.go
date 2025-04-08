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
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
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
	DriverConfig                    *config.DriverConfig
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
	driverConfig *config.DriverConfig,
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
			DriverConfig:                    driverConfig,
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
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.RegisterViaWhatsApp called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	whatsAppOTP, err := utils.GenerateOTP(constvars.WHATSAPP_OTP_LENGTH)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterViaWhatsApp error generating OTP",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrClientCustomMessage(err)
	}

	uc.Log.Info("authUsecase.RegisterViaWhatsApp generated OTP",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	err = uc.checkExistingUserByWhatsAppNumber(ctx, request.To)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterViaWhatsApp WhatsApp number already registered",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	err = uc.createWhatsAppUser(ctx, request.To, whatsAppOTP)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterViaWhatsApp error creating WhatsApp user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	err = uc.sendWhatsAppOTP(ctx, request.To, whatsAppOTP)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterViaWhatsApp error sending WhatsApp OTP",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.RegisterViaWhatsApp succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) LoginViaWhatsApp(ctx context.Context, request *requests.LoginViaWhatsApp) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.LoginViaWhatsApp called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	whatsAppOTP, err := utils.GenerateOTP(constvars.WHATSAPP_OTP_LENGTH)
	if err != nil {
		uc.Log.Error("authUsecase.LoginViaWhatsApp error generating OTP",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrClientCustomMessage(err)
	}

	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, request.To)
	if err != nil {
		uc.Log.Error("authUsecase.LoginViaWhatsApp error finding user by WhatsApp number",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if existingUser == nil {
		uc.Log.Error("authUsecase.LoginViaWhatsApp user not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrUserNotExist(nil)
	}

	existingUser.WhatsAppOTP = whatsAppOTP
	existingUser.WhatsAppNumber = request.To
	existingUser.SetWhatsAppOTPExpiryTime(uc.InternalConfig.App.WhatsAppOTPExpiredTimeInMinutes)
	existingUser.SetUpdatedAt()

	err = uc.UserRepository.UpdateUser(ctx, existingUser)
	if err != nil {
		uc.Log.Error("authUsecase.LoginViaWhatsApp error updating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	whatsAppMessage := &requests.WhatsAppMessage{
		To:        request.To,
		Message:   whatsAppOTP,
		WithImage: false,
	}
	uc.Log.Info("authUsecase.LoginViaWhatsApp sending WhatsApp OTP",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	err = uc.WhatsAppService.SendWhatsAppMessage(ctx, whatsAppMessage)
	if err != nil {
		uc.Log.Error("authUsecase.LoginViaWhatsApp error sending WhatsApp message",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.LoginViaWhatsApp succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) VerifyRegisterWhatsAppOTP(ctx context.Context, request *requests.VerivyRegisterWhatsAppOTP) (*responses.RegisterUserWhatsApp, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.VerifyRegisterWhatsAppOTP called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, request.To)
	if err != nil {
		uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error finding user by WhatsApp number",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if existingUser == nil {
		uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP user not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrUserNotExist(nil)
	}

	if time.Now().After(*existingUser.WhatsAppOTPExpiry) {
		uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP WhatsApp OTP expired",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrWhatsAppOTPExpired(nil)
	}

	if existingUser.WhatsAppOTP != request.OTP {
		uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP invalid OTP",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrWhatsAppOTPInvalid(nil)
	}

	if request.Role == constvars.ResourcePatient {
		fhirPatientRequest := utils.BuildFhirPatientWhatsAppRegistrationRequest(request.To)
		uc.Log.Info("authUsecase.VerifyRegisterWhatsAppOTP creating FHIR patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		fhirPatient, err := uc.PatientFhirClient.CreatePatient(ctx, fhirPatientRequest)
		if err != nil {
			uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error creating FHIR patient",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePatient)
		if err != nil {
			uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error finding patient role",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		existingUser.RoleID = role.ID
		existingUser.PatientID = fhirPatient.ID
		existingUser.SetUpdatedAt()

		err = uc.UserRepository.UpdateUser(ctx, existingUser)
		if err != nil {
			uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error updating user after patient registration",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		response := &responses.RegisterUserWhatsApp{
			UserID:    existingUser.ID,
			PatientID: existingUser.PatientID,
		}
		uc.Log.Info("authUsecase.VerifyRegisterWhatsAppOTP succeeded for patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return response, nil

	} else if request.Role == constvars.ResourcePractitioner {
		fhirPractitionerRequest := utils.BuildFhirPractitionerWhatsAppRegistrationRequest(request.To)
		uc.Log.Info("authUsecase.VerifyRegisterWhatsAppOTP creating FHIR practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		fhirPractitioner, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
		if err != nil {
			uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error creating FHIR practitioner",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePractitioner)
		if err != nil {
			uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error finding practitioner role",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		existingUser.RoleID = role.ID
		existingUser.PractitionerID = fhirPractitioner.ID
		existingUser.SetUpdatedAt()

		err = uc.UserRepository.UpdateUser(ctx, existingUser)
		if err != nil {
			uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP error updating user after practitioner registration",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		response := &responses.RegisterUserWhatsApp{
			UserID:         existingUser.ID,
			PractitionerID: existingUser.PractitionerID,
		}
		uc.Log.Info("authUsecase.VerifyRegisterWhatsAppOTP succeeded for practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return response, nil
	}

	uc.Log.Error("authUsecase.VerifyRegisterWhatsAppOTP invalid role",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil, exceptions.ErrAuthInvalidRole(nil)
}

func (uc *authUsecase) VerifyLoginWhatsAppOTP(ctx context.Context, request *requests.VerivyLoginWhatsAppOTP) (*responses.LoginUser, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.VerifyLoginWhatsAppOTP called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, request.To)
	if err != nil {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP error finding user by WhatsApp number",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if existingUser == nil {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP user not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrUserNotExist(nil)
	}

	if time.Now().After(*existingUser.WhatsAppOTPExpiry) {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP WhatsApp OTP expired",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrWhatsAppOTPExpired(nil)
	}

	if existingUser.WhatsAppOTP != request.OTP {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP invalid OTP",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrWhatsAppOTPInvalid(nil)
	}

	userRole, err := uc.RoleRepository.FindRoleByID(ctx, existingUser.RoleID)
	if err != nil {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP error retrieving role by ID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if userRole.IsNotPatient() {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP role mismatch, not a patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrWhatsAppOTPInvalid(nil)
	}

	sessionID := uuid.New().String()
	sessionModel := models.Session{
		UserID:    existingUser.ID,
		PatientID: existingUser.PatientID,
		RoleID:    userRole.ID,
		RoleName:  userRole.Name,
		SessionID: sessionID,
	}
	err = uc.RedisRepository.Set(
		ctx,
		sessionID,
		sessionModel,
		time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours),
	)
	if err != nil {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP error setting session in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	tokenString, err := utils.GenerateSessionJWT(
		sessionID,
		uc.InternalConfig.JWT.Secret,
		uc.InternalConfig.App.LoginSessionExpiredTimeInHours,
	)
	if err != nil {
		uc.Log.Error("authUsecase.VerifyLoginWhatsAppOTP error generating JWT",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrTokenGenerate(err)
	}

	response := &responses.LoginUser{
		Token: tokenString,
		LoginUserData: responses.LoginUserData{
			Fullname:  existingUser.Fullname,
			UserID:    existingUser.ID,
			RoleID:    userRole.ID,
			RoleName:  userRole.Name,
			PatientID: existingUser.PatientID,
		},
	}
	uc.Log.Info("authUsecase.VerifyLoginWhatsAppOTP succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *authUsecase) RegisterClinician(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.RegisterClinician called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if request.Password != request.RetypePassword {
		uc.Log.Error("authUsecase.RegisterClinician passwords do not match",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrPasswordDoNotMatch(nil)
	}

	existingUser, err := uc.UserRepository.FindByEmailOrUsername(ctx, request.Email, request.Username)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterClinician error finding user by email/username",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if existingUser != nil {
		uc.Log.Error("authUsecase.RegisterClinician email already exists",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrEmailAlreadyExist(nil)
	}

	fhirPractitionerRequest := utils.BuildFhirPractitionerRegistrationRequest(request.Username, request.Email)
	uc.Log.Info("authUsecase.RegisterClinician creating FHIR practitioner",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	fhirPractitioner, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterClinician error creating FHIR practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterClinician error hashing password",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrHashPassword(err)
	}

	role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePractitioner)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterClinician error finding practitioner role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

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

	userID, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterClinician error creating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response := &responses.RegisterUser{
		UserID:         userID,
		PractitionerID: fhirPractitioner.ID,
	}
	uc.Log.Info("authUsecase.RegisterClinician succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *authUsecase) RegisterPatient(ctx context.Context, request *requests.RegisterUser) (*responses.RegisterUser, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.RegisterPatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if request.Password != request.RetypePassword {
		uc.Log.Error("authUsecase.RegisterPatient passwords do not match",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrPasswordDoNotMatch(nil)
	}

	existingUser, err := uc.UserRepository.FindByEmailOrUsername(ctx, request.Email, request.Username)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterPatient error finding user by email/username",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if existingUser != nil {
		uc.Log.Error("authUsecase.RegisterPatient email already exists",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrEmailAlreadyExist(nil)
	}

	fhirPatientRequest := utils.BuildFhirPatientRegistrationRequest(request.Username, request.Email)
	uc.Log.Info("authUsecase.RegisterPatient creating FHIR patient",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	fhirPatient, err := uc.PatientFhirClient.CreatePatient(ctx, fhirPatientRequest)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterPatient error creating FHIR patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterPatient error hashing password",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrHashPassword(err)
	}

	role, err := uc.RoleRepository.FindByName(ctx, constvars.RoleTypePatient)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterPatient error finding patient role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

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

	userID, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterPatient error creating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	err = uc.checkQuestionnaireResponseAndAttachWithPatientData(ctx, request.ResponseID, user)
	if err != nil {
		uc.Log.Error("authUsecase.RegisterPatient error attaching questionnaire response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response := &responses.RegisterUser{
		UserID:    userID,
		PatientID: fhirPatient.ID,
	}
	uc.Log.Info("authUsecase.RegisterPatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *authUsecase) LoginPatient(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.LoginPatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		uc.Log.Error("authUsecase.LoginPatient error retrieving user by username",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if user == nil {
		uc.Log.Error("authUsecase.LoginPatient user not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	if user.IsDeactivated() {
		if user.IsDeactivationDeadlineExpired(uc.InternalConfig.App.AccountDeactivationAgeInDays) {
			uc.Log.Error("authUsecase.LoginPatient deactivation deadline expired",
				zap.String(constvars.LoggingRequestIDKey, requestID),
			)
			return nil, exceptions.ErrAccountDeactivationAgeExpired(nil)
		}

		user.SetEmptyDeletedAt()
		err = uc.UserRepository.UpdateUser(ctx, user)
		if err != nil {
			uc.Log.Error("authUsecase.LoginPatient error updating reactivated user",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		fhirPatientRequest := utils.BuildFhirPatientReactivateRequest(user.PatientID)
		_, err = uc.PatientFhirClient.UpdatePatient(ctx, fhirPatientRequest)
		if err != nil {
			uc.Log.Error("authUsecase.LoginPatient error reactivating FHIR patient",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
	}

	userRole, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		uc.Log.Error("authUsecase.LoginPatient error retrieving role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if userRole.IsNotPatient() {
		uc.Log.Error("authUsecase.LoginPatient role mismatch, not a patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	err = uc.checkQuestionnaireResponseAndAttachWithPatientData(ctx, request.ResponseID, user)
	if err != nil {
		uc.Log.Error("authUsecase.LoginPatient error attaching questionnaire response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	sessionID := uuid.New().String()
	sessionModel := models.Session{
		UserID:    user.ID,
		PatientID: user.PatientID,
		RoleID:    userRole.ID,
		RoleName:  userRole.Name,
		SessionID: sessionID,
	}
	err = uc.RedisRepository.Set(
		ctx,
		sessionID,
		sessionModel,
		time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours),
	)
	if err != nil {
		uc.Log.Error("authUsecase.LoginPatient error setting session in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	tokenString, err := utils.GenerateSessionJWT(
		sessionID,
		uc.InternalConfig.JWT.Secret,
		uc.InternalConfig.App.LoginSessionExpiredTimeInHours,
	)
	if err != nil {
		uc.Log.Error("authUsecase.LoginPatient error generating JWT",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrTokenGenerate(err)
	}

	preSignedProfilePictureUrl, err := uc.getPresignedUrl(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.LoginPatient error getting presigned URL",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

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
	uc.Log.Info("authUsecase.LoginPatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *authUsecase) LoginClinician(ctx context.Context, request *requests.LoginUser) (*responses.LoginUser, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.LoginClinician called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	user, err := uc.UserRepository.FindByUsername(ctx, request.Username)
	if err != nil {
		uc.Log.Error("authUsecase.LoginClinician error retrieving user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	if user == nil {
		uc.Log.Error("authUsecase.LoginClinician user not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrInvalidUsernameOrPassword(nil)
	}

	if user.IsDeactivated() {
		if user.IsDeactivationDeadlineExpired(uc.InternalConfig.App.AccountDeactivationAgeInDays) {
			uc.Log.Error("authUsecase.LoginClinician deactivation deadline expired",
				zap.String(constvars.LoggingRequestIDKey, requestID),
			)
			return nil, exceptions.ErrAccountDeactivationAgeExpired(nil)
		}
		user.SetEmptyDeletedAt()
		err = uc.UserRepository.UpdateUser(ctx, user)
		if err != nil {
			uc.Log.Error("authUsecase.LoginClinician error updating reactivated user",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		practitionerFhirRequest := utils.BuildFhirPractitionerReactivateRequest(user.PractitionerID)
		_, err = uc.PractitionerFhirClient.UpdatePractitioner(ctx, practitionerFhirRequest)
		if err != nil {
			uc.Log.Error("authUsecase.LoginClinician error reactivating practitioner",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
	}

	userRole, err := uc.RoleRepository.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		uc.Log.Error("authUsecase.LoginClinician error retrieving role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	if userRole.IsNotPractitioner() {
		uc.Log.Error("authUsecase.LoginClinician role mismatch, not a practitioner",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil, exceptions.ErrNotMatchRoleType(nil)
	}

	practitionerRoles, err := uc.PractitionerRoleFhirClient.FindPractitionerRoleByPractitionerID(ctx, user.PractitionerID)
	if err != nil {
		uc.Log.Error("authUsecase.LoginClinician error fetching practitioner roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	practitionerOrganizationIDs := utils.ExtractOrganizationIDsFromPractitionerRoles(practitionerRoles)
	sessionID := uuid.New().String()
	sessionModel := models.Session{
		UserID:         user.ID,
		PractitionerID: user.PractitionerID,
		RoleID:         userRole.ID,
		RoleName:       userRole.Name,
		SessionID:      sessionID,
		ClinicIDs:      practitionerOrganizationIDs,
	}
	err = uc.RedisRepository.Set(ctx, sessionID, sessionModel,
		time.Hour*time.Duration(uc.InternalConfig.App.LoginSessionExpiredTimeInHours))
	if err != nil {
		uc.Log.Error("authUsecase.LoginClinician error setting session in Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	tokenString, err := utils.GenerateSessionJWT(
		sessionID,
		uc.InternalConfig.JWT.Secret,
		uc.InternalConfig.App.LoginSessionExpiredTimeInHours,
	)
	if err != nil {
		uc.Log.Error("authUsecase.LoginClinician error generating JWT",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrTokenGenerate(err)
	}

	preSignedProfilePictureUrl, err := uc.getPresignedUrl(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.LoginClinician error getting presigned URL",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

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
	uc.Log.Info("authUsecase.LoginClinician succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *authUsecase) LogoutUser(ctx context.Context, sessionData string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.LogoutUser called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, sessionData)
	if err != nil {
		uc.Log.Error("authUsecase.LogoutUser error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	err = uc.RedisRepository.Delete(ctx, session.SessionID)
	if err != nil {
		uc.Log.Error("authUsecase.LogoutUser error deleting session from Redis",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.LogoutUser succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) IsUserHasPermission(ctx context.Context, request requests.AuthorizeUser) (bool, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.IsUserHasPermission called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		uc.Log.Error("authUsecase.IsUserHasPermission error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return false, err
	}

	uc.mu.RLock()
	role, exists := uc.Roles[session.RoleID]
	uc.mu.RUnlock()
	if !exists {
		uc.Log.Error("authUsecase.IsUserHasPermission role not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return false, exceptions.ErrAuthInvalidRole(nil)
	}

	permission := uc.isRoleHasPermission(role, request.Resource, request.RequiredAction)
	uc.Log.Info("authUsecase.IsUserHasPermission result",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return permission, nil
}

func (uc *authUsecase) ForgotPassword(ctx context.Context, request *requests.ForgotPassword) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.ForgotPassword called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	user, err := uc.UserRepository.FindByEmail(ctx, request.Email)
	if err != nil {
		uc.Log.Error("authUsecase.ForgotPassword error finding user by email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}
	if user == nil {
		uc.Log.Info("authUsecase.ForgotPassword: no user found for email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return nil
	}

	uuidStr := uuid.New().String()
	resetToken, err := utils.GenerateResetPasswordJWT(uuidStr, uc.InternalConfig.JWT.Secret, uc.InternalConfig.App.ForgotPasswordTokenExpiredTimeInMinutes)
	if err != nil {
		uc.Log.Error("authUsecase.ForgotPassword error generating reset token",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrTokenGenerate(err)
	}
	user.ResetToken = resetToken
	user.SetResetTokenExpiryTime(uc.InternalConfig.App.ForgotPasswordTokenExpiredTimeInMinutes)

	err = uc.UserRepository.UpdateUser(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.ForgotPassword error updating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
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
	uc.Log.Info("authUsecase.ForgotPassword sending email",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("email_payload", emailPayload),
	)

	err = uc.MailerService.SendEmail(ctx, emailPayload)
	if err != nil {
		uc.Log.Error("authUsecase.ForgotPassword error sending email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.ForgotPassword succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) ResetPassword(ctx context.Context, request *requests.ResetPassword) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.ResetPassword called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if request.NewPassword != request.RetypeNewPassword {
		uc.Log.Error("authUsecase.ResetPassword passwords do not match",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrPasswordDoNotMatch(nil)
	}

	user, err := uc.UserRepository.FindByResetToken(ctx, request.Token)
	if err != nil {
		uc.Log.Error("authUsecase.ResetPassword error finding user by reset token",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if time.Now().After(*user.ResetTokenExpiry) {
		uc.Log.Error("authUsecase.ResetPassword reset token expired",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrTokenResetPasswordExpired(nil)
	}

	hashedNewPassword, err := utils.HashPassword(request.NewPassword)
	if err != nil {
		uc.Log.Error("authUsecase.ResetPassword error hashing new password",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrHashPassword(err)
	}
	request.HashedNewPassword = hashedNewPassword
	user.SetDataForUpdateResetPassword(request)

	err = uc.UserRepository.UpdateUser(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.ResetPassword error updating user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.ResetPassword succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) CreateMagicLink(ctx context.Context, request *requests.CreateMagicLink) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.CreateMagicLink called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	tenantID := "public"
	plessResponse, err := passwordless.SignInUpByPhoneNumber(tenantID, request.PhoneNumber)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink supertokens error create user by tenantID & phoneNumber",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	inviteLink, err := passwordless.CreateMagicLinkByPhoneNumber(tenantID, request.PhoneNumber)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink supertokens error create magic link by phone number",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.InitializeSupertoken assigning Patient Clinician to CreatedNewUser")
	response, err := userroles.AddRoleToUser("public", plessResponse.User.ID, constvars.KonsulinRoleClinician, nil)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink error userroles.AddRoleToUser",
			zap.Error(err),
		)
		return err
	}

	if response.UnknownRoleError != nil {
		uc.Log.Error("authUsecase.CreateMagicLink error unknown role",
			zap.Error(err),
		)
		return fmt.Errorf("unknown role found when assign role: %v", response.UnknownRoleError)
	}

	if response.OK.DidUserAlreadyHaveRole {
		uc.Log.Info("authUsecase.CreateMagicLink user already have role")
	}

	whatsappRequest := &requests.WhatsAppMessage{
		To:        request.PhoneNumber,
		Message:   inviteLink,
		WithImage: false,
	}

	err = uc.WhatsAppService.SendWhatsAppMessage(ctx, whatsappRequest)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink error sending the magic link via whatsapp",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	fmt.Println(inviteLink)

	uc.Log.Info("authUsecase.CreateMagicLink succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) checkQuestionnaireResponseAndAttachWithPatientData(ctx context.Context, responseID string, user *models.User) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.checkQuestionnaireResponseAndAttachWithPatientData called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, responseID),
	)

	if responseID != "" {
		questionnaireResponseID := new(string)
		redisRaw, err := uc.RedisRepository.Get(ctx, responseID)
		if err != nil {
			uc.Log.Error("authUsecase.checkQuestionnaireResponseAndAttachWithPatientData error fetching Redis key",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}

		err = json.Unmarshal([]byte(redisRaw), questionnaireResponseID)
		if err != nil {
			uc.Log.Error("authUsecase.checkQuestionnaireResponseAndAttachWithPatientData error unmarshaling Redis value",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return exceptions.ErrCannotParseJSON(err)
		}

		questionnaireResponseFhir, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, *questionnaireResponseID)
		if err != nil {
			uc.Log.Error("authUsecase.checkQuestionnaireResponseAndAttachWithPatientData error fetching questionnaire response",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}

		questionnaireResponseFhir.Subject.Reference = fmt.Sprintf("%s/%s", constvars.ResourcePatient, user.PatientID)
		_, err = uc.QuestionnaireResponseFhirClient.UpdateQuestionnaireResponse(ctx, questionnaireResponseFhir)
		if err != nil {
			uc.Log.Error("authUsecase.checkQuestionnaireResponseAndAttachWithPatientData error updating questionnaire response",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}
		uc.Log.Info("authUsecase.checkQuestionnaireResponseAndAttachWithPatientData succeeded",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
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
	uc.Log.Info("authUsecase.loadRoles called")
	roles, err := uc.RoleRepository.FindAll(ctx)
	if err != nil {
		uc.Log.Error("authUsecase.loadRoles error fetching roles",
			zap.Error(err),
		)
		return err
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()
	for _, role := range roles {
		r := role
		uc.Roles[role.ID] = &r
	}
	uc.Log.Info("authUsecase.loadRoles succeeded",
		zap.Int("roles_count", len(roles)),
	)
	return nil
}

func (uc *authUsecase) checkExistingUserByWhatsAppNumber(ctx context.Context, phoneNumber string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.checkExistingUserByWhatsAppNumber called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	existingUser, err := uc.UserRepository.FindByWhatsAppNumber(ctx, phoneNumber)
	if err != nil {
		uc.Log.Error("authUsecase.checkExistingUserByWhatsAppNumber error fetching user",
			zap.Error(err),
		)
		return err
	}
	if existingUser != nil {
		uc.Log.Error("authUsecase.checkExistingUserByWhatsAppNumber phone number already registered",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return exceptions.ErrPhoneNumberAlreadyRegistered(nil)
	}
	uc.Log.Info("authUsecase.checkExistingUserByWhatsAppNumber succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) createWhatsAppUser(ctx context.Context, phoneNumber string, whatsAppOTP string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.createWhatsAppUser called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	user := new(models.User)
	user.WhatsAppOTP = whatsAppOTP
	user.WhatsAppNumber = phoneNumber
	user.SetWhatsAppOTPExpiryTime(uc.InternalConfig.App.WhatsAppOTPExpiredTimeInMinutes)
	user.SetCreatedAtUpdatedAt()

	_, err := uc.UserRepository.CreateUser(ctx, user)
	if err != nil {
		uc.Log.Error("authUsecase.createWhatsAppUser error creating user",
			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("authUsecase.createWhatsAppUser succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) sendWhatsAppOTP(ctx context.Context, phoneNumber string, whatsAppOTP string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.sendWhatsAppOTP called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	whatsAppMessage := &requests.WhatsAppMessage{
		To:        phoneNumber,
		Message:   whatsAppOTP,
		WithImage: false,
	}
	err := uc.WhatsAppService.SendWhatsAppMessage(ctx, whatsAppMessage)
	if err != nil {
		uc.Log.Error("authUsecase.sendWhatsAppOTP error sending WhatsApp message",
			zap.String(constvars.LoggingRequestIDKey, requestID),

			zap.Error(err),
		)
		return err
	}
	uc.Log.Info("authUsecase.sendWhatsAppOTP succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (uc *authUsecase) getPresignedUrl(ctx context.Context, user *models.User) (string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.getPresignedUrl called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)

	if user.ProfilePictureName != "" {
		objectUrlExpiryTime := time.Duration(uc.InternalConfig.App.MinioPreSignedUrlObjectExpiryTimeInHours) * time.Hour
		preSignedUrl, err := uc.MinioStorage.GetObjectUrlWithExpiryTime(
			ctx,
			uc.InternalConfig.Minio.BucketName,
			user.ProfilePictureName,
			objectUrlExpiryTime,
		)
		if err != nil {
			uc.Log.Error("authUsecase.getPresignedUrl error generating presigned URL",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingUserIDKey, user.ID),
				zap.Error(err),
			)
			return "", err
		}
		uc.Log.Info("authUsecase.getPresignedUrl succeeded",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingUserIDKey, user.ID),
		)
		return preSignedUrl, nil
	}
	uc.Log.Info("authUsecase.getPresignedUrl no profile picture found",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingUserIDKey, user.ID),
	)
	return "", nil
}
