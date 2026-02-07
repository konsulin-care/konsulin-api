package auth

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	bundleSvc "konsulin-service/internal/app/services/fhir_spark/bundle"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/supertokens/supertokens-golang/ingredients/emaildelivery"
	"github.com/supertokens/supertokens-golang/ingredients/smsdelivery"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"go.uber.org/zap"
)

type authUsecase struct {
	RedisRepository                 contracts.RedisRepository
	SessionService                  contracts.SessionService
	RoleRepository                  contracts.RoleRepository
	UserUsecase                     contracts.UserUsecase
	PatientFhirClient               contracts.PatientFhirClient
	PractitionerFhirClient          contracts.PractitionerFhirClient
	QuestionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient
	BundleFhirClient                bundleSvc.BundleFhirClient
	MailerService                   contracts.MailerService
	WhatsAppService                 contracts.WhatsAppService
	MinioStorage                    contracts.Storage
	MagicLinkDelivery               contracts.MagicLinkDeliveryService
	InternalConfig                  *config.InternalConfig
	DriverConfig                    *config.DriverConfig
	Roles                           map[string]*models.Role
	Log                             *zap.Logger
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
	questionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient,
	bundleFhirClient bundleSvc.BundleFhirClient,
	userUsecase contracts.UserUsecase,
	mailerService contracts.MailerService,
	magicLinkDelivery contracts.MagicLinkDeliveryService,
	internalConfig *config.InternalConfig,
	driverConfig *config.DriverConfig,
	logger *zap.Logger,
) (contracts.AuthUsecase, error) {
	onceAuthUsecase.Do(func() {
		instance := &authUsecase{
			RedisRepository:                 redisRepository,
			SessionService:                  sessionService,
			PatientFhirClient:               patientFhirClient,
			PractitionerFhirClient:          practitionerFhirClient,
			QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
			BundleFhirClient:                bundleFhirClient,
			UserUsecase:                     userUsecase,
			MailerService:                   mailerService,
			MagicLinkDelivery:               magicLinkDelivery,
			InternalConfig:                  internalConfig,
			DriverConfig:                    driverConfig,
			Roles:                           make(map[string]*models.Role),
			Log:                             logger,
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
	hasPhone := strings.TrimSpace(request.Phone) != ""
	hasEmail := strings.TrimSpace(request.Email) != ""

	uc.Log.Debug("Starting magic link creation",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEmailKey, request.Email),
		zap.String("phone", request.Phone),
		zap.Strings(constvars.LoggingRolesKey, request.Roles),
	)

	// Phone flow (WhatsApp): generate magic link in SuperTokens and deliver via internal webhook.
	if hasPhone && !hasEmail {
		phoneDigits := utils.NormalizePhoneDigits(request.Phone)
		if err := utils.ValidateInternationalPhoneDigits(phoneDigits); err != nil {
			return err
		}

		// SuperTokens can accept digits-only phone numbers; we intentionally never include a '+'.
		plessResponse, err := passwordless.SignInUpByPhoneNumber(uc.InternalConfig.Supertoken.KonsulinTenantID, phoneDigits)
		if err != nil {
			uc.Log.Error("Failed to create user account (phone)",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("phone", phoneDigits),
				zap.String(constvars.LoggingErrorTypeKey, "SuperTokens API"),
				zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
				zap.Error(err),
			)
			return err
		}

		inviteLink, err := passwordless.CreateMagicLinkByPhoneNumber(uc.InternalConfig.Supertoken.KonsulinTenantID, phoneDigits)
		if err != nil {
			uc.Log.Error("Failed to generate magic link (phone)",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("phone", phoneDigits),
				zap.String(constvars.LoggingErrorTypeKey, "SuperTokens API"),
				zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
				zap.Error(err),
			)
			return err
		}

		if len(request.Roles) > 0 {
			uc.Log.Info("Assigning roles to user (phone)",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("phone", phoneDigits),
				zap.Strings(constvars.LoggingRolesKey, request.Roles),
			)

			for _, role := range request.Roles {
				response, err := userroles.AddRoleToUser(uc.InternalConfig.Supertoken.KonsulinTenantID, plessResponse.User.ID, role, nil)
				if err != nil {
					uc.Log.Error("Failed to assign role to user (phone)",
						zap.String(constvars.LoggingRequestIDKey, requestID),
						zap.String("phone", phoneDigits),
						zap.String("role", role),
						zap.String(constvars.LoggingErrorTypeKey, "role assignment"),
						zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
						zap.Error(err),
					)
					return err
				}
				if response.UnknownRoleError != nil {
					return fmt.Errorf("unknown role found when assigning role %s: %v", role, response.UnknownRoleError)
				}
			}
		}

		// Initialize FHIR resources similarly to email flow.
		initializeResourcesInput := &contracts.InitializeNewUserFHIRResourcesInput{
			Phone:            phoneDigits,
			SuperTokenUserID: plessResponse.User.ID,
		}
		initializeResourcesInput.ToogleByRoles(request.Roles)
		initializeResourceCtx, initializeResourceCtxCancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
		defer initializeResourceCtxCancel()
		if _, err := uc.UserUsecase.InitializeNewUserFHIRResources(initializeResourceCtx, initializeResourcesInput); err != nil {
			uc.Log.Error("Failed to initialize new user FHIR resources (phone)",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("phone", phoneDigits),
				zap.String(constvars.LoggingErrorTypeKey, "FHIR resources initialization"),
				zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
				zap.Error(err),
			)
			return err
		}

		err = passwordless.SendSms(smsdelivery.SmsType{
			PasswordlessLogin: &smsdelivery.PasswordlessLoginType{
				UrlWithLinkCode: &inviteLink,
				PhoneNumber:     phoneDigits,
			},
		})

		if err != nil {
			uc.Log.Error("Failed to send magic link (phone)",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("phone", phoneDigits),
				zap.String(constvars.LoggingErrorTypeKey, "SuperTokens API"),
				zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
				zap.Error(err),
			)
			return err
		}

		uc.Log.Info("Magic link creation completed successfully (phone)",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("phone", phoneDigits),
			zap.Strings(constvars.LoggingRolesKey, request.Roles),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Bool(constvars.LoggingSuccessKey, true),
		)
		return nil
	}

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

	initializeResourcesInput := &contracts.InitializeNewUserFHIRResourcesInput{
		Email:            request.Email,
		SuperTokenUserID: plessResponse.User.ID,
	}
	initializeResourcesInput.ToogleByRoles(request.Roles)
	initializeResourceCtx, initializeResourceCtxCancel := context.WithDeadline(ctx, time.Now().Add(10*time.Second))
	defer initializeResourceCtxCancel()
	initializeResources, err := uc.UserUsecase.InitializeNewUserFHIRResources(initializeResourceCtx, initializeResourcesInput)
	if err != nil {
		uc.Log.Error("Failed to initialize new user FHIR resources",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String(constvars.LoggingErrorTypeKey, "FHIR resources initialization"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		return err
	}

	// NOTE: this must run after we call webhook [base]/api/v1/hook/synchronous/modify-profile
	// because the underlying email delivery service relies on the result
	// of profile synchronization (omnichannel service).
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
		zap.String("initialized_resources_patient_id", initializeResources.PatientID),
		zap.String("initialized_resources_practitioner_id", initializeResources.PractitionerID),
		zap.String("initialized_resources_person_id", initializeResources.PersonID),
	)
	return nil
}

func (uc *authUsecase) CreateAnonymousSession(ctx context.Context, existingToken string, forceNew bool) (*contracts.AnonymousSessionResult, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.CreateAnonymousSession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if !forceNew && strings.TrimSpace(existingToken) != "" {
		guestID, err := uc.parseAnonymousSessionToken(existingToken)
		if err == nil && guestID != "" {
			uc.Log.Info("authUsecase.CreateAnonymousSession reused existing token",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("guest_id", guestID),
			)
			return &contracts.AnonymousSessionResult{
				Token:   existingToken,
				GuestID: guestID,
				IsNew:   false,
			}, nil
		}
	}

	guestID := uuid.New().String()

	token, err := uc.createAnonymousSessionToken(guestID)
	if err != nil {
		uc.Log.Error("authUsecase.CreateAnonymousSession failed to create token",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("authUsecase.CreateAnonymousSession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("guest_id", guestID),
		zap.String("role", constvars.KonsulinRoleGuest),
	)

	return &contracts.AnonymousSessionResult{
		Token:   token,
		GuestID: guestID,
		IsNew:   true,
	}, nil
}

func (uc *authUsecase) ClaimAnonymousResources(ctx context.Context, supertokensUserID string, roles []string, anonToken string) (*contracts.ClaimAnonymousResourcesOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("authUsecase.ClaimAnonymousResources called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if strings.TrimSpace(supertokensUserID) == "" {
		return nil, exceptions.ErrSupertokensSessionMissing(nil)
	}

	if strings.TrimSpace(anonToken) == "" {
		return nil, exceptions.ErrAnonymousTokenMissing(nil)
	}

	guestID, err := uc.parseAnonymousSessionToken(anonToken)
	if err != nil || guestID == "" {
		uc.Log.Info("authUsecase.ClaimAnonymousResources invalid anon token",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		return &contracts.ClaimAnonymousResourcesOutput{}, nil
	}

	ownerRef, err := uc.resolveOwnerReferenceBySupertokensID(ctx, supertokensUserID, roles)
	if err != nil {
		uc.Log.Error("authUsecase.ClaimAnonymousResources error resolving owner reference",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	responses, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponsesByIdentifier(ctx, constvars.AnonymousSessionIdentifierSystem, guestID)
	if err != nil {
		uc.Log.Error("authUsecase.ClaimAnonymousResources error fetching questionnaire responses",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	entries := make([]map[string]any, 0, len(responses))
	refs := make([]string, 0, len(responses))
	for _, response := range responses {
		if !canClaimQuestionnaireResponse(response, ownerRef) {
			continue
		}

		updated := response
		changed := false

		if updated.Author.Reference != ownerRef {
			updated.Author.Reference = ownerRef
			changed = true
		}
		if updated.Subject.Reference != ownerRef {
			updated.Subject.Reference = ownerRef
			changed = true
		}

		if updated.Identifier != nil && updated.Identifier.System == constvars.AnonymousSessionIdentifierSystem && updated.Identifier.Value == guestID {
			updated.Identifier = nil
			changed = true
		}

		if !changed {
			continue
		}

		entry := map[string]any{
			"resource": updated,
			"request": map[string]any{
				"method": http.MethodPut,
				"url":    fmt.Sprintf("%s/%s", constvars.ResourceQuestionnaireResponse, updated.ID),
			},
		}
		entries = append(entries, entry)
		refs = append(refs, fmt.Sprintf("%s/%s", constvars.ResourceQuestionnaireResponse, updated.ID))
	}

	if len(entries) == 0 {
		return &contracts.ClaimAnonymousResourcesOutput{}, nil
	}

	bundle := map[string]any{
		"resourceType": "Bundle",
		"type":         "transaction",
		"entry":        entries,
	}

	if _, err := uc.BundleFhirClient.PostTransactionBundle(ctx, bundle); err != nil {
		uc.Log.Error("authUsecase.ClaimAnonymousResources error posting transaction bundle",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	sort.Strings(refs)
	return &contracts.ClaimAnonymousResourcesOutput{
		Count:         len(entries),
		ReferenceList: refs,
	}, nil
}

func (uc *authUsecase) createAnonymousSessionToken(guestID string) (string, error) {
	ttl := time.Duration(constvars.AnonymousSessionTokenTTLDays) * 24 * time.Hour
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		constvars.AnonymousSessionGuestIDClaimKey: guestID,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(uc.InternalConfig.JWT.Secret))
}

func (uc *authUsecase) parseAnonymousSessionToken(tokenString string) (string, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected jwt alg: %s", t.Header["alg"])
		}
		return []byte(uc.InternalConfig.JWT.Secret), nil
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return "", fmt.Errorf("invalid anonymous token")
	}

	rawGuestID, ok := claims[constvars.AnonymousSessionGuestIDClaimKey]
	if !ok {
		return "", fmt.Errorf("guest_id missing in token")
	}
	guestID, ok := rawGuestID.(string)
	if !ok || strings.TrimSpace(guestID) == "" {
		return "", fmt.Errorf("guest_id invalid in token")
	}
	return guestID, nil
}

func (uc *authUsecase) resolveOwnerReferenceBySupertokensID(ctx context.Context, supertokensUserID string, roles []string) (string, error) {
	// Determine target type from roles. Default to Patient if ambiguous.
	isPractitioner := false
	for _, r := range roles {
		if r == constvars.KonsulinRolePractitioner || r == constvars.RoleTypePractitioner {
			isPractitioner = true
			break
		}
	}

	if isPractitioner {
		practitioners, err := uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, supertokensUserID)
		if err != nil {
			return "", err
		}
		if len(practitioners) < 1 || strings.TrimSpace(practitioners[0].ID) == "" {
			return "", fmt.Errorf("practitioner not found for supertokens user id")
		}
		return fmt.Sprintf("Practitioner/%s", practitioners[0].ID), nil
	}

	identifier := fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, supertokensUserID)
	patients, err := uc.PatientFhirClient.FindPatientByIdentifier(ctx, identifier)
	if err != nil {
		return "", err
	}
	if len(patients) < 1 || strings.TrimSpace(patients[0].ID) == "" {
		return "", fmt.Errorf("patient not found for supertokens user id")
	}
	return fmt.Sprintf("Patient/%s", patients[0].ID), nil
}

func canClaimQuestionnaireResponse(response fhir_dto.QuestionnaireResponse, ownerRef string) bool {
	if strings.TrimSpace(response.Subject.Reference) != "" && response.Subject.Reference != ownerRef {
		return false
	}
	if strings.TrimSpace(response.Author.Reference) != "" && response.Author.Reference != ownerRef {
		return false
	}
	return true
}

func (uc *authUsecase) CheckUserExists(ctx context.Context, email string) (*contracts.CheckUserExistsOutput, error) {
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
		return nil, err
	}

	exists := user != nil
	output := &contracts.CheckUserExistsOutput{
		SupertokenUser:  user,
		PatientIds:      []string{},
		PractitionerIds: []string{},
	}

	if user != nil {
		identifier := fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, user.ID)

		patients, err := uc.PatientFhirClient.FindPatientByIdentifier(ctx, identifier)
		if err != nil {
			uc.Log.Error("authUsecase.CheckUserExists error finding patient by identifier",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("identifier", identifier),
				zap.Error(err),
			)
			return nil, err
		}
		for _, p := range patients {
			if p.ID != "" {
				output.PatientIds = append(output.PatientIds, p.ID)
			}
		}

		practitioners, err := uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, user.ID)
		if err != nil {
			uc.Log.Error("authUsecase.CheckUserExists error finding practitioner by identifier",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("identifier", user.ID),
				zap.Error(err),
			)
			return nil, err
		}
		for _, prac := range practitioners {
			if prac.ID != "" {
				output.PractitionerIds = append(output.PractitionerIds, prac.ID)
			}
		}
	}
	uc.Log.Info("authUsecase.CheckUserExists completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("email", email),
		zap.Bool("exists", exists),
	)

	return output, nil
}

func (uc *authUsecase) CheckUserExistsByPhone(ctx context.Context, phone string) (*contracts.CheckUserExistsOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	phoneDigits := utils.NormalizePhoneDigits(phone)

	uc.Log.Info("authUsecase.CheckUserExistsByPhone called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("phone", phoneDigits),
	)

	if err := utils.ValidateInternationalPhoneDigits(phoneDigits); err != nil {
		return nil, err
	}

	user, err := passwordless.GetUserByPhoneNumber(uc.InternalConfig.Supertoken.KonsulinTenantID, phoneDigits)
	if err != nil {
		uc.Log.Error("authUsecase.CheckUserExistsByPhone supertokens error get user by phone",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("phone", phoneDigits),
			zap.Error(err),
		)
		return nil, err
	}

	exists := user != nil
	output := &contracts.CheckUserExistsOutput{
		SupertokenUser:  user,
		PatientIds:      []string{},
		PractitionerIds: []string{},
	}

	if user != nil {
		identifier := fmt.Sprintf("%s|%s", constvars.FhirSupertokenSystemIdentifier, user.ID)

		patients, err := uc.PatientFhirClient.FindPatientByIdentifier(ctx, identifier)
		if err != nil {
			uc.Log.Error("authUsecase.CheckUserExistsByPhone error finding patient by identifier",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("identifier", identifier),
				zap.Error(err),
			)
			return nil, err
		}
		for _, p := range patients {
			if p.ID != "" {
				output.PatientIds = append(output.PatientIds, p.ID)
			}
		}

		practitioners, err := uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, user.ID)
		if err != nil {
			uc.Log.Error("authUsecase.CheckUserExistsByPhone error finding practitioner by identifier",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("identifier", user.ID),
				zap.Error(err),
			)
			return nil, err
		}
		for _, prac := range practitioners {
			if prac.ID != "" {
				output.PractitionerIds = append(output.PractitionerIds, prac.ID)
			}
		}
	}

	uc.Log.Info("authUsecase.CheckUserExistsByPhone completed",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("phone", phoneDigits),
		zap.Bool("exists", exists),
	)

	return output, nil
}
