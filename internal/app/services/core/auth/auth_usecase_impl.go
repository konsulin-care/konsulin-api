package auth

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"sync"
	"time"

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
	start := time.Now()
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Debug("Starting magic link creation",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEmailKey, request.Email),
		zap.Strings(constvars.LoggingRolesKey, request.Roles),
	)

	plessResponse, err := passwordless.SignInUpByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, request.Email)
	if err != nil {
		uc.Log.Error("Failed to create user account",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String(constvars.LoggingErrorTypeKey, "SuperTokens API"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		return err
	}

	inviteLink, err := passwordless.CreateMagicLinkByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, request.Email)
	if err != nil {
		uc.Log.Error("Failed to generate magic link",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String(constvars.LoggingErrorTypeKey, "SuperTokens API"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		return err
	}

	if len(request.Roles) > 0 {
		uc.Log.Info("Assigning roles to user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.Strings(constvars.LoggingRolesKey, request.Roles),
		)

		for _, role := range request.Roles {
			response, err := userroles.AddRoleToUser(uc.InternalConfig.Supertoken.KonsulinTenantID, plessResponse.User.ID, role, nil)
			if err != nil {
				uc.Log.Error("Failed to assign role to user",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingEmailKey, request.Email),
					zap.String("role", role),
					zap.String(constvars.LoggingErrorTypeKey, "role assignment"),
					zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
					zap.Error(err),
				)
				return err
			}

			if response.UnknownRoleError != nil {
				uc.Log.Error("Unknown role provided",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingEmailKey, request.Email),
					zap.String("role", role),
					zap.String(constvars.LoggingErrorTypeKey, "unknown role"),
					zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
				)
				return fmt.Errorf("unknown role found when assigning role %s: %v", role, response.UnknownRoleError)
			}

			if response.OK.DidUserAlreadyHaveRole {
				uc.Log.Debug("User already has role",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingEmailKey, request.Email),
					zap.String("role", role),
				)
			} else {
				uc.Log.Info("Role assigned successfully",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingEmailKey, request.Email),
					zap.String("role", role),
				)
			}
		}
	} else {
		uc.Log.Debug("No roles to assign - existing user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
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
		uc.Log.Error("Failed to send magic link email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String(constvars.LoggingErrorTypeKey, "email delivery"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("Magic link creation completed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEmailKey, request.Email),
		zap.Strings(constvars.LoggingRolesKey, request.Roles),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
		zap.Bool(constvars.LoggingSuccessKey, true),
	)
	return nil
}

func (uc *authUsecase) CreateAnonymousSession(ctx context.Context) (string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.CreateAnonymousSession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	sessionID := fmt.Sprintf("anonymous_%s_%d", requestID, time.Now().UnixNano())

	uc.Log.Info("authUsecase.CreateAnonymousSession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("session_id", sessionID),
		zap.String("role", constvars.KonsulinRoleGuest),
	)

	return sessionID, nil
}

func (uc *authUsecase) CheckUserExists(ctx context.Context, email string) (bool, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.CheckUserExists called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("email", email),
	)

	user, err := passwordless.GetUserByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, email)
	if err != nil {
		uc.Log.Error("authUsecase.CheckUserExists supertokens error get user by email",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("email", email),
			zap.Error(err),
		)
		return false, err
	}

	exists := user != nil
	uc.Log.Info("authUsecase.CheckUserExists completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("email", email),
		zap.Bool("exists", exists),
	)

	return exists, nil
}
