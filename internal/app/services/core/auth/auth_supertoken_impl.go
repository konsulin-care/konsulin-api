package auth

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/supertokens/supertokens-golang/ingredients/emaildelivery"
	"github.com/supertokens/supertokens-golang/ingredients/smsdelivery"
	"github.com/supertokens/supertokens-golang/recipe/dashboard"
	"github.com/supertokens/supertokens-golang/recipe/dashboard/dashboardmodels"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

const (
	supertokenAccessTokenPayloadRolesKey      = "st-role"
	supertokenAccessTokenPayloadRolesValueKey = "v"
)

func (uc *authUsecase) InitializeSupertoken() error {
	apiBasePath := fmt.Sprintf("%s/%s%s", uc.InternalConfig.App.EndpointPrefix, uc.InternalConfig.App.Version, uc.DriverConfig.Supertoken.ApiBasePath)
	websiteBasePath := uc.DriverConfig.Supertoken.WebsiteBasePath
	cookieSameSite := constvars.CookieSameSiteStrictMode
	cookieSecure := true

	if uc.InternalConfig.App.Env == "local" || uc.InternalConfig.App.Env == "development" {
		cookieSameSite = constvars.CookieSameSiteLaxMode
		cookieSecure = false
	}

	supertokenConnectionInfo := &supertokens.ConnectionInfo{
		ConnectionURI: uc.DriverConfig.Supertoken.ConnectionURI,
		APIKey:        uc.DriverConfig.Supertoken.APIKey,
	}

	supertokenAppInfo := supertokens.AppInfo{
		AppName:         uc.DriverConfig.Supertoken.AppName,
		APIDomain:       uc.DriverConfig.Supertoken.ApiDomain,
		WebsiteDomain:   uc.DriverConfig.Supertoken.WebsiteDomain,
		APIBasePath:     &apiBasePath,
		WebsiteBasePath: &websiteBasePath,
	}

	supertokenRecipeList := []supertokens.Recipe{
		passwordless.Init(uc.buildPasswordlessConfig()),
		userroles.Init(nil),
		session.Init(uc.buildSessionConfig(&cookieSameSite, &cookieSecure)),
		dashboard.Init(uc.buildDashboardConfig()),
	}

	err := supertokens.Init(supertokens.TypeInput{
		OnSuperTokensAPIError: func(err error, req *http.Request, res http.ResponseWriter) {
			log.Println(err.Error())
		},
		Supertokens: supertokenConnectionInfo,
		AppInfo:     supertokenAppInfo,
		RecipeList:  supertokenRecipeList,
	})
	if err != nil {
		return err
	}

	initializeRoles()

	log.Println("Successfully initialized supertokens SDK")
	return nil
}

// buildPasswordlessConfig builds the full passwordless recipe configuration.
func (uc *authUsecase) buildPasswordlessConfig() plessmodels.TypeInput {
	return plessmodels.TypeInput{
		Override: &plessmodels.OverrideStruct{
			Functions: func(originalImplementation plessmodels.RecipeInterface) plessmodels.RecipeInterface {
				originalCreateCode := *originalImplementation.CreateCode
				(*originalImplementation.CreateCode) = uc.buildPasswordlessCreateCodeOverride(originalCreateCode)

				originalConsumeCode := *originalImplementation.ConsumeCode
				(*originalImplementation.ConsumeCode) = uc.buildPasswordlessConsumeCodeOverride(originalConsumeCode)
				return originalImplementation
			},
			APIs: func(originalImplementation plessmodels.APIInterface) plessmodels.APIInterface {
				originalImplementation.EmailExistsGET = nil
				return originalImplementation
			},
		},
		EmailDelivery: uc.buildEmailDeliveryConfig(),
		SmsDelivery:   uc.buildSMSDeliveryConfig(),
		FlowType:      "MAGIC_LINK",
		ContactMethodEmailOrPhone: plessmodels.ContactMethodEmailOrPhoneConfig{
			Enabled: true,
			ValidateEmailAddress: func(email interface{}, tenantId string) *string {
				emailStr, ok := email.(string)
				if !ok {
					msg := "invalid email format"
					return &msg
				}

				matched, err := regexp.MatchString(constvars.RegexEmail, emailStr)
				if err != nil || !matched {
					msg := "invalid email address"
					return &msg
				}

				return nil
			},
			ValidatePhoneNumber: func(phoneNumber interface{}, tenantId string) *string {
				phoneStr, ok := phoneNumber.(string)
				if !ok {
					msg := "invalid phone format"
					return &msg
				}
				phoneDigits := utils.NormalizePhoneDigits(phoneStr)
				if err := utils.ValidateInternationalPhoneDigits(phoneDigits); err != nil {
					msg := err.Error()
					return &msg
				}

				return nil
			},
		},
	}
}

// initializeRoles creates SuperTokens roles if they do not already exist.
func initializeRoles() {
	roleNames := []string{
		constvars.KonsulinRolePatient,
		constvars.KonsulinRoleGuest,
		constvars.KonsulinRoleClinicAdmin,
		constvars.KonsulinRolePractitioner,
		constvars.KonsulinRoleResearcher,
		constvars.KonsulinRoleSuperadmin,
	}
	for _, name := range roleNames {
		resp, err := userroles.CreateNewRoleOrAddPermissions(name, []string{}, nil)
		if err != nil {
			log.Println("Error creating", name, "role", zap.Error(err))
			continue
		}
		if !resp.OK.CreatedNewRole {
			log.Println("'" + name + "' role already exists")
		}
	}
}

// lookupUserForCreateCode resolves user details and roles during the create-code flow.
func (uc *authUsecase) lookupUserForCreateCode(email *string, phoneNumber *string, normalizedPhoneNumber string) (userEmail, userPhoneNumber, userID string, userRoles []string, err error) {
	userRecord := &plessmodels.User{}

	if email != nil {
		userEmail = *email

		userRecord, err = passwordless.GetUserByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, userEmail)
		if err != nil {
			uc.Log.Error("authUsecase.SupertokenCreateCode failed to fetch user by email",
				zap.String("email", userEmail),
				zap.Error(err),
			)
			return
		}
	} else if phoneNumber != nil {
		userPhoneNumber = normalizedPhoneNumber

		userRecord, err = passwordless.GetUserByPhoneNumber(uc.InternalConfig.Supertoken.KonsulinTenantID, userPhoneNumber)
		if err != nil {
			uc.Log.Error("authUsecase.SupertokenCreateCode failed to fetch user by phone number",
				zap.String("phone_number", userPhoneNumber),
				zap.Error(err),
			)
			return
		}
	} else {
		err = errors.New("either email or phone number is required")
		return
	}

	userRoles = []string{constvars.KonsulinRolePatient}
	userID = ""

	if userRecord != nil {
		userID = userRecord.ID
		userRolesResp, rErr := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, userRecord.ID)
		if rErr != nil {
			uc.Log.Error("authUsecase.SupertokenCreateCode failed to fetch user roles by user ID",
				zap.String("user_id", userRecord.ID),
				zap.Error(rErr),
			)
			err = rErr
			return
		}

		if userRolesResp.OK != nil {
			userRoles = append(userRoles, userRolesResp.OK.Roles...)
		}
	}

	return
}

// buildPasswordlessCreateCodeOverride returns a CreateCode override that handles
// phone normalization, user lookup, role fetching, and FHIR resource initialization.
func (uc *authUsecase) buildPasswordlessCreateCodeOverride(originalCreateCode func(email *string, phoneNumber *string, userInputCode *string, tenantId string, userContext supertokens.UserContext) (plessmodels.CreateCodeResponse, error)) func(email *string, phoneNumber *string, userInputCode *string, tenantId string, userContext supertokens.UserContext) (plessmodels.CreateCodeResponse, error) {
	return func(email *string, phoneNumber *string, userInputCode *string, tenantId string, userContext supertokens.UserContext) (plessmodels.CreateCodeResponse, error) {
		var userPhoneNumberPtr *string
		normalizedPhoneNumber := ""
		if phoneNumber != nil {
			normalizedPhoneNumber = utils.NormalizePhoneDigits(*phoneNumber)
			userPhoneNumberPtr = &normalizedPhoneNumber
		}

		response, err := originalCreateCode(email, userPhoneNumberPtr, userInputCode, tenantId, userContext)
		if err != nil {
			uc.Log.Error("authUsecase.SupertokenCreateCode error while calling originalCreateCode",
				zap.Error(err),
			)
			return response, err
		}

		userEmail, userPhoneNumber, userID, userRoles, err := uc.lookupUserForCreateCode(email, phoneNumber, normalizedPhoneNumber)
		if err != nil {
			return response, err
		}

		if err := uc.initializeFHIRForUser(userID, &userEmail, &userPhoneNumber, userRoles); err != nil {
			return response, err
		}

		return response, nil
	}
}

// resolveRolesForConsumeCode fetches user roles, adding the Patient role if none exist.
func (uc *authUsecase) resolveRolesForConsumeCode(userID string) ([]string, error) {
	rolesResp, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, userID)
	if err != nil {
		uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID",
			zap.Error(err),
		)
		return nil, err
	}

	if rolesResp.OK == nil {
		uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID is nil",
			zap.String("user_id", userID),
		)
		return nil, errors.New("unexpected nil response when getting roles for user")
	}

	userRoles := rolesResp.OK.Roles

	if len(userRoles) == 0 {
		roleResp, err := userroles.AddRoleToUser(
			uc.InternalConfig.Supertoken.KonsulinTenantID,
			userID,
			constvars.KonsulinRolePatient,
			nil,
		)

		if err != nil {
			uc.Log.Error("authUsecase.SupertokenConsumeCode error adding role to user",
				zap.Error(err),
				zap.String("user_id", userID),
			)
			return nil, err
		}

		if roleResp.OK == nil {
			uc.Log.Error(
				"unexpected nil response when initializing user roles after consume code",
				zap.String("user_id", userID),
			)
			return nil, errors.New("unexpected nil response when initializing user roles after consume code")
		}

		newUserRolesResp, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, userID)
		if err != nil {
			uc.Log.Error("authUsecase.SupertokenConsumeCode error getting roles for user",
				zap.Error(err),
				zap.String("user_id", userID),
			)
			return nil, err
		}

		if newUserRolesResp.OK == nil {
			uc.Log.Error("authUsecase.SupertokenConsumeCode unexpected nil response when getting roles for user",
				zap.String("user_id", userID),
			)
			return nil, errors.New("unexpected nil response when getting roles for user")
		}

		userRoles = newUserRolesResp.OK.Roles
	}

	return userRoles, nil
}

// initializeFHIRForUser initializes FHIR resources (Patient/Practitioner/Person) for a user.
func (uc *authUsecase) initializeFHIRForUser(userID string, email *string, phoneNumber *string, userRoles []string) error {
	userEmail := ""
	userPhoneNumber := ""

	if email != nil {
		userEmail = *email
	}
	if phoneNumber != nil {
		userPhoneNumber = *phoneNumber
	}

	initInput := &contracts.InitializeNewUserFHIRResourcesInput{
		Email:            userEmail,
		Phone:            userPhoneNumber,
		SuperTokenUserID: userID,
	}
	initInput.ToogleByRoles(userRoles)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	result, err := uc.UserUsecase.InitializeNewUserFHIRResources(ctx, initInput)
	if err != nil {
		uc.Log.Error("authUsecase failed to initialize FHIR resources",
			zap.Error(err),
			zap.String("user_id", userID),
		)
		return err
	}

	uc.Log.Info("authUsecase initialized FHIR resources",
		zap.String("user_id", userID),
		zap.String("patient_id", result.PatientID),
		zap.String("practitioner_id", result.PractitionerID),
		zap.String("person_id", result.PersonID),
	)
	return nil
}

// buildPasswordlessConsumeCodeOverride returns a ConsumeCode override that handles
// role assignment on consume and FHIR resource initialization.
// buildEmailDeliveryConfig constructs the email delivery config for passwordless login.
func (uc *authUsecase) buildEmailDeliveryConfig() *emaildelivery.TypeInput {
	return &emaildelivery.TypeInput{
		Override: func(originalImplementation emaildelivery.EmailDeliveryInterface) emaildelivery.EmailDeliveryInterface {
			originalSendEmail := *originalImplementation.SendEmail
			(*originalImplementation.SendEmail) = func(input emaildelivery.EmailType, userContext supertokens.UserContext) error {
				// Only intercept passwordless magic-link emails; for anything else, fall back to default.
				if input.PasswordlessLogin == nil {
					return originalSendEmail(input, userContext)
				}

				if input.PasswordlessLogin.UrlWithLinkCode == nil {
					return errors.New("passwordless email delivery: missing UrlWithLinkCode")
				}

				// NOTE: SuperTokens' email delivery interface does not provide request context.
				// Use Background context with timeout (from InternalConfig) for now.
				timeoutSeconds := uc.InternalConfig.Webhook.HTTPTimeoutInSeconds
				if timeoutSeconds <= 0 {
					timeoutSeconds = 10
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
				defer cancel()

				err := uc.MagicLinkDelivery.SendMagicLink(ctx, contracts.SendMagicLinkInput{
					URL:   *input.PasswordlessLogin.UrlWithLinkCode,
					Email: input.PasswordlessLogin.Email,
				})
				if err != nil {
					uc.Log.Error("authUsecase.EmailDelivery.SendEmail error calling magiclink webhook",
						zap.Error(err),
					)
					return err
				}
				return nil
			}
			return originalImplementation
		},
	}
}

// buildSessionConfig constructs the session recipe configuration.
func (uc *authUsecase) buildSessionConfig(cookieSameSite *string, cookieSecure *bool) *sessmodels.TypeInput {
	return &sessmodels.TypeInput{
		Override: &sessmodels.OverrideStruct{
			Functions: func(originalImplementation sessmodels.RecipeInterface) sessmodels.RecipeInterface {
				originalCreateNewSession := *originalImplementation.CreateNewSession

				(*originalImplementation.CreateNewSession) = func(userID string, accessTokenPayload, sessionDataInDatabase map[string]interface{}, disableAntiCsrf *bool, tenantId string, userContext supertokens.UserContext) (sessmodels.SessionContainer, error) {
					if accessTokenPayload == nil {
						accessTokenPayload = make(map[string]interface{})
					}

					if userID == "" {
						accessTokenPayload[supertokenAccessTokenPayloadRolesKey] = map[string]interface{}{
							supertokenAccessTokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRoleGuest},
						}
					} else {
						rolesResp, err := userroles.GetRolesForUser(tenantId, userID)
						if err == nil && rolesResp.OK != nil {
							roles := make([]interface{}, len(rolesResp.OK.Roles))
							for i, role := range rolesResp.OK.Roles {
								roles[i] = role
							}
							accessTokenPayload[supertokenAccessTokenPayloadRolesKey] = map[string]interface{}{
								supertokenAccessTokenPayloadRolesValueKey: roles,
							}
						} else {
							accessTokenPayload[supertokenAccessTokenPayloadRolesKey] = map[string]interface{}{
								supertokenAccessTokenPayloadRolesValueKey: []interface{}{constvars.KonsulinRoleGuest},
							}
						}
					}

					return originalCreateNewSession(userID, accessTokenPayload, sessionDataInDatabase, disableAntiCsrf, tenantId, userContext)
				}

				return originalImplementation
			},
		},
		CookieSameSite: cookieSameSite,
		CookieSecure:   cookieSecure,
	}
}

// buildDashboardConfig constructs the dashboard recipe config.
func (uc *authUsecase) buildDashboardConfig() *dashboardmodels.TypeInput {
	return &dashboardmodels.TypeInput{
		Admins: &[]string{
			uc.InternalConfig.Supertoken.KonsulinDasboardAdminEmail,
		},
	}
}

// buildSMSDeliveryConfig constructs the SMS delivery config for passwordless login.
func (uc *authUsecase) buildSMSDeliveryConfig() *smsdelivery.TypeInput {
	return &smsdelivery.TypeInput{
		Override: func(originalImplementation smsdelivery.SmsDeliveryInterface) smsdelivery.SmsDeliveryInterface {
			(*originalImplementation.SendSms) = func(input smsdelivery.SmsType, userContext supertokens.UserContext) error {
				if input.PasswordlessLogin == nil {
					return errors.New("passwordless sms delivery: missing PasswordlessLogin payload")
				}
				if input.PasswordlessLogin.UrlWithLinkCode == nil {
					return errors.New("passwordless sms delivery: missing UrlWithLinkCode")
				}

				phoneDigits := strings.TrimSpace(input.PasswordlessLogin.PhoneNumber)
				if phoneDigits == "" {
					return errors.New("passwordless sms delivery: missing PhoneNumber")
				}

				phoneDigitsNormalized := utils.NormalizePhoneDigits(phoneDigits)
				if err := utils.ValidateInternationalPhoneDigits(phoneDigitsNormalized); err != nil {
					return err
				}

				timeoutSeconds := uc.InternalConfig.Webhook.HTTPTimeoutInSeconds
				if timeoutSeconds <= 0 {
					timeoutSeconds = 10
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
				defer cancel()

				err := uc.MagicLinkDelivery.SendMagicLink(ctx, contracts.SendMagicLinkInput{
					URL:   *input.PasswordlessLogin.UrlWithLinkCode,
					Phone: phoneDigitsNormalized,
				})
				if err != nil {
					uc.Log.Error("authUsecase.SmsDelivery.SendSms error calling magiclink webhook",
						zap.Error(err),
					)
					return err
				}

				return nil
			}
			return originalImplementation
		},
	}
}

func (uc *authUsecase) buildPasswordlessConsumeCodeOverride(originalConsumeCode func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error)) func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error) {
	return func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error) {
		response, err := originalConsumeCode(userInput, linkCode, preAuthSessionID, tenantId, userContext)
		if err != nil {
			uc.Log.Error("authUsecase.SupertokenConsumeCode error while do func originalConsumeCode",
				zap.Error(err),
			)
			return plessmodels.ConsumeCodeResponse{}, err
		}

		// early return to avoid nested if statements
		if response.OK == nil {
			return response, nil
		}

		user := response.OK.User

		rolesResp, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID)
		if err != nil {
			uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID",
				zap.Error(err),
			)
			return plessmodels.ConsumeCodeResponse{}, err
		}

		if rolesResp.OK == nil {
			uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID is nil",
				zap.String("user_id", user.ID),
			)
			return plessmodels.ConsumeCodeResponse{}, errors.New("unexpected nil response when getting roles for user")
		}

		userRoles, err := uc.resolveRolesForConsumeCode(user.ID)
		if err != nil {
			return plessmodels.ConsumeCodeResponse{}, err
		}

		if err := uc.initializeFHIRForUser(user.ID, user.Email, user.PhoneNumber, userRoles); err != nil {
			return plessmodels.ConsumeCodeResponse{}, err
		}

		return response, nil
	}
}
