package auth

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"sync"

	"github.com/supertokens/supertokens-golang/ingredients/emaildelivery"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"go.uber.org/zap"
)

type authUsecase struct {
	RedisRepository        contracts.RedisRepository
	SessionService         contracts.SessionService
	RoleRepository         contracts.RoleRepository
	PatientFhirClient      contracts.PatientFhirClient
	PractitionerFhirClient contracts.PractitionerFhirClient
	MailerService          contracts.MailerService
	WhatsAppService        contracts.WhatsAppService
	MinioStorage           contracts.Storage
	InternalConfig         *config.InternalConfig
	DriverConfig           *config.DriverConfig
	Roles                  map[string]*models.Role
	mu                     sync.RWMutex
	Log                    *zap.Logger
}

var (
	authUsecaseInstance contracts.AuthUsecase
	onceAuthUsecase     sync.Once
	authUsecaseError    error
)

func NewAuthUsecase(
	redisRepository contracts.RedisRepository,
	sessionService contracts.SessionService,
	patientFhirClient contracts.PatientFhirClient,
	practitionerFhirClient contracts.PractitionerFhirClient,
	mailerService contracts.MailerService,
	whatsAppService contracts.WhatsAppService,
	minioStorage contracts.Storage,
	internalConfig *config.InternalConfig,
	driverConfig *config.DriverConfig,
	logger *zap.Logger,
) (contracts.AuthUsecase, error) {
	onceAuthUsecase.Do(func() {
		instance := &authUsecase{
			RedisRepository:        redisRepository,
			SessionService:         sessionService,
			PatientFhirClient:      patientFhirClient,
			PractitionerFhirClient: practitionerFhirClient,
			MailerService:          mailerService,
			MinioStorage:           minioStorage,
			WhatsAppService:        whatsAppService,
			InternalConfig:         internalConfig,
			DriverConfig:           driverConfig,
			Roles:                  make(map[string]*models.Role),
			Log:                    logger,
		}

		authUsecaseInstance = instance
	})

	return authUsecaseInstance, authUsecaseError
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

func (uc *authUsecase) CreateMagicLink(ctx context.Context, request *requests.SupertokenPasswordlessCreateMagicLink) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.CreateMagicLink called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	user, err := passwordless.GetUserByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, request.Email)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink supertokens error get user by tenantID & Email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if user != nil {
		userRoles, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID)
		if err != nil {
			uc.Log.Error("authUsecase.CreateMagicLink supertokens error get roles for user by tenantID & UserID",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return err
		}

		if len(userRoles.OK.Roles) == 1 && userRoles.OK.Roles[0] == constvars.KonsulinRolePatient {
			uc.Log.Error("authUsecase.CreateMagicLink supertokens error while check user eligibility",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return errors.New("user is registered as patient first. You can't invite this user via magic link")
		}
	}

	plessResponse, err := passwordless.SignInUpByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, request.Email)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink supertokens error create user by tenantID & Email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	inviteLink, err := passwordless.CreateMagicLinkByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, request.Email)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink supertokens error create magic link by email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.InitializeSupertoken assigning Patient Practitioner roles to CreatedNewUser")
	response, err := userroles.AddRoleToUser(uc.InternalConfig.Supertoken.KonsulinTenantID, plessResponse.User.ID, constvars.KonsulinRolePractitioner, nil)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink error userroles.AddRoleToUser",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	if response.UnknownRoleError != nil {
		uc.Log.Error("authUsecase.CreateMagicLink error unknown role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return fmt.Errorf("unknown role found when assigning role: %v", response.UnknownRoleError)
	}

	if response.OK.DidUserAlreadyHaveRole {
		uc.Log.Info("authUsecase.CreateMagicLink user already have role",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
	}

	emailData := emaildelivery.EmailType{
		PasswordlessLogin: &emaildelivery.PasswordlessLoginType{
			Email:           request.Email,
			UrlWithLinkCode: &inviteLink,
		},
	}

	err = passwordless.SendEmail(emailData)
	if err != nil {
		uc.Log.Error("authUsecase.CreateMagicLink supertokens error send email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("authUsecase.CreateMagicLink succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}
