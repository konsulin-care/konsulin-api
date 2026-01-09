package auth

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/contracts"
	webhooksvc "konsulin-service/internal/app/services/core/webhook"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"log"
	"net/http"
	"regexp"
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
		passwordless.Init(plessmodels.TypeInput{
			Override: &plessmodels.OverrideStruct{
				Functions: func(originalImplementation plessmodels.RecipeInterface) plessmodels.RecipeInterface {
					originalCreateCode := *originalImplementation.CreateCode
					(*originalImplementation.CreateCode) = func(email *string, phoneNumber *string, userInputCode *string, tenantId string, userContext supertokens.UserContext) (plessmodels.CreateCodeResponse, error) {
						response, err := originalCreateCode(email, phoneNumber, userInputCode, tenantId, userContext)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error while calling originalCreateCode",
								zap.Error(err),
							)
							return response, err
						}

						userEmail := ""
						if email == nil {
							uc.Log.Warn("authUsecase.SupertokenCreateCode email is nil, nothing can be done")
							return response, nil
						}

						userEmail = *email

						userRecord, err := passwordless.GetUserByEmail(uc.InternalConfig.Supertoken.KonsulinTenantID, userEmail)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode failed to fetch user by email",
								zap.String("email", userEmail),
								zap.Error(err),
							)
							return response, err
						}

						// by default, always assumes the user roles is Patient
						// because if the user is the first time user, supertokens is not yet assigns any roles to the user.
						// We're also assumes the registered user is always have a Patient role.
						userRoles := []string{
							constvars.KonsulinRolePatient,
						}
						userID := ""

						if userRecord != nil {
							userID = userRecord.ID
							userRolesResp, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, userRecord.ID)
							if err != nil {
								uc.Log.Error("authUsecase.SupertokenCreateCode failed to fetch user roles by user ID",
									zap.String("user_id", userRecord.ID),
									zap.Error(err),
								)
								return response, err
							}

							if userRolesResp.OK != nil {
								// override the default roles with the user roles from supertokens
								userRoles = append(userRoles, userRolesResp.OK.Roles...)
							}
						}

						initFHIRResourcesInput := &contracts.InitializeNewUserFHIRResourcesInput{
							Email:            userEmail,
							SuperTokenUserID: userID,
						}
						initFHIRResourcesInput.ToogleByRoles(userRoles)

						initializeResourceCtx, initializeResourceCtxCancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))

						defer initializeResourceCtxCancel()

						initializedResources, err := uc.UserUsecase.InitializeNewUserFHIRResources(initializeResourceCtx, initFHIRResourcesInput)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error initializing new user FHIR resources",
								zap.Error(err),
							)
							return response, err
						}

						uc.Log.Info("authUsecase.SupertokenCreateCode fetched user by email",
							zap.String("email", userEmail),
							zap.String("initialized_resources_patient_id", initializedResources.PatientID),
							zap.String("initialized_resources_practitioner_id", initializedResources.PractitionerID),
							zap.String("initialized_resources_person_id", initializedResources.PersonID),
						)

						return response, nil
					}

					originalConsumeCode := *originalImplementation.ConsumeCode
					(*originalImplementation.ConsumeCode) = func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error) {
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
							return plessmodels.ConsumeCodeResponse{}, err
						}

						userRoles := rolesResp.OK.Roles

						// if the user roles is empty, it means that the user
						// registered using create code flow and thus no roles
						// assigned to the user. We will assign the default role
						// to the user.
						if len(userRoles) == 0 {
							roleResp, err := userroles.AddRoleToUser(
								uc.InternalConfig.Supertoken.KonsulinTenantID,
								user.ID,
								constvars.KonsulinRolePatient,
								nil,
							)

							if err != nil {
								uc.Log.Error("authUsecase.SupertokenConsumeCode error adding role to user",
									zap.Error(err),
									zap.String("user_id", user.ID),
								)
								return plessmodels.ConsumeCodeResponse{}, err
							}

							if roleResp.OK == nil {
								uc.Log.Error(
									"unexpected nil response when initializing user roles after consume code",
									zap.String("user_id", user.ID),
								)
								return plessmodels.ConsumeCodeResponse{}, errors.New("unexpected nil response when initializing user roles after consume code")
							}

							newUserRolesResp, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID)

							if err != nil {
								uc.Log.Error("authUsecase.SupertokenConsumeCode error getting roles for user",
									zap.Error(err),
									zap.String("user_id", user.ID),
								)
								return plessmodels.ConsumeCodeResponse{}, err
							}

							if newUserRolesResp.OK == nil {
								uc.Log.Error("authUsecase.SupertokenConsumeCode unexpected nil response when getting roles for user",
									zap.String("user_id", user.ID),
								)
								return plessmodels.ConsumeCodeResponse{}, errors.New("unexpected nil response when getting roles for user")
							}

							userRoles = newUserRolesResp.OK.Roles
						}

						initializeFHIRResourcesInput := &contracts.InitializeNewUserFHIRResourcesInput{
							Email:            *user.Email,
							SuperTokenUserID: user.ID,
						}
						initializeFHIRResourcesInput.ToogleByRoles(userRoles)

						initializeResourceCtx, initializeResourceCtxCancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
						defer initializeResourceCtxCancel()

						initializedResources, err := uc.UserUsecase.InitializeNewUserFHIRResources(initializeResourceCtx, initializeFHIRResourcesInput)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenConsumeCode error initializing new user FHIR resources",
								zap.Error(err),
							)
							return plessmodels.ConsumeCodeResponse{}, err
						}

						uc.Log.Info("consumeCode: login OK",
							zap.String("uid", user.ID),
							zap.String("initialized_resources_patient_id", initializedResources.PatientID),
							zap.String("initialized_resources_practitioner_id", initializedResources.PractitionerID),
							zap.String("initialized_resources_person_id", initializedResources.PersonID),
						)
						return response, nil
					}
					return originalImplementation
				},
				APIs: func(originalImplementation plessmodels.APIInterface) plessmodels.APIInterface {
					// Disable SuperTokens' built-in email existence endpoint so our chi handler can take over.
					originalImplementation.EmailExistsGET = nil
					return originalImplementation
				},
			},
			EmailDelivery: &emaildelivery.TypeInput{
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
							timeoutSeconds = 15
						}
						ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)
						defer cancel()

						err := webhooksvc.SendMagicLink(ctx, uc.JWTManager, webhooksvc.SendMagicLinkInput{
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
			},
			SmsDelivery: &smsdelivery.TypeInput{
				Override: func(originalImplementation smsdelivery.SmsDeliveryInterface) smsdelivery.SmsDeliveryInterface {
					(*originalImplementation.SendSms) = func(input smsdelivery.SmsType, userContext supertokens.UserContext) error {
						phoneNumber := input.PasswordlessLogin.PhoneNumber
						urlWithLinkCode := input.PasswordlessLogin.UrlWithLinkCode

						whatsappRequest := &requests.WhatsAppMessage{
							To:        phoneNumber,
							Message:   *urlWithLinkCode,
							WithImage: false,
						}

						ctx := context.Background()
						err := uc.WhatsAppService.SendWhatsAppMessage(ctx, whatsappRequest)
						if err != nil {
							return err
						}

						return nil
					}
					return originalImplementation
				},
			},
			FlowType: "MAGIC_LINK",
			ContactMethodEmail: plessmodels.ContactMethodEmailConfig{
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
			},
		}),
		userroles.Init(nil),
		session.Init(&sessmodels.TypeInput{
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
			CookieSameSite: &cookieSameSite,
			CookieSecure:   &cookieSecure,
		}),
		dashboard.Init(&dashboardmodels.TypeInput{
			Admins: &[]string{
				uc.InternalConfig.Supertoken.KonsulinDasboardAdminEmail,
			},
		}),
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

	resp, err := userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRolePatient, []string{}, nil)
	if err != nil {
		log.Println("Error creating 'patient' role", zap.Error(err))
	}
	if !resp.OK.CreatedNewRole {
		log.Println("'patient' role already exists")
	}

	resp, err = userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRoleGuest, []string{}, nil)
	if err != nil {
		log.Println("Error creating 'guest' role", zap.Error(err))
	}
	if !resp.OK.CreatedNewRole {
		log.Println("'guest' role already exists")
	}

	resp, err = userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRoleClinicAdmin, []string{}, nil)
	if err != nil {
		log.Println("Error creating 'clinic_admin' role", zap.Error(err))
	}
	if !resp.OK.CreatedNewRole {
		log.Println("'clinic_admin' role already exists")
	}

	resp, err = userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRolePractitioner, []string{}, nil)
	if err != nil {
		log.Println("Error creating 'practitioner' role", zap.Error(err))
	}
	if !resp.OK.CreatedNewRole {
		log.Println("'practitioner' role already exists")
	}

	resp, err = userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRoleResearcher, []string{}, nil)
	if err != nil {
		log.Println("Error creating 'researcher' role", zap.Error(err))
	}
	if !resp.OK.CreatedNewRole {
		log.Println("'researcher' role already exists")
	}

	resp, err = userroles.CreateNewRoleOrAddPermissions(constvars.KonsulinRoleSuperadmin, []string{}, nil)
	if err != nil {
		log.Println("Error creating 'superadmin' role", zap.Error(err))
	}
	if !resp.OK.CreatedNewRole {
		log.Println("'superadmin' role already exists")
	}

	log.Println("Successfully initialized supertokens SDK")
	return nil
}
