package auth

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"log"
	"net/http"
	"regexp"
	"slices"

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

func (uc *authUsecase) InitializeSupertoken() error {
	apiBasePath := fmt.Sprintf("%s/%s%s", uc.InternalConfig.App.EndpointPrefix, uc.InternalConfig.App.Version, uc.DriverConfig.Supertoken.ApiBasePath)
	websiteBasePath := uc.DriverConfig.Supertoken.WebsiteBasePath
	cookieSameSite := constvars.CookieSameSiteStrictMode

	if uc.InternalConfig.App.Env == "local" || uc.InternalConfig.App.Env == "development" {
		cookieSameSite = constvars.CookieSameSiteNoneMode
	}

	supertokenConnectionInfo := &supertokens.ConnectionInfo{
		ConnectionURI: uc.DriverConfig.Supertoken.ConnectionURI,
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
					originalConsumeCode := *originalImplementation.ConsumeCode
					(*originalImplementation.ConsumeCode) = func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error) {
						response, err := originalConsumeCode(userInput, linkCode, preAuthSessionID, tenantId, userContext)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenConsumeCode error while do func originalConsumeCode",
								zap.Error(err),
							)
							return plessmodels.ConsumeCodeResponse{}, err
						}

						if response.OK != nil {
							user := response.OK.User
							userRoles, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID)
							if err != nil {
								uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID",
									zap.Error(err),
								)
								return plessmodels.ConsumeCodeResponse{}, err
							}

							if slices.Contains(userRoles.OK.Roles, constvars.KonsulinRolePractitioner) {
								ctx := context.Background()
								fhirPractitioners, err := uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, user.ID)
								if len(fhirPractitioners) > 1 {
									uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID",
										zap.Error(err),
									)
									return plessmodels.ConsumeCodeResponse{}, errors.New(constvars.ErrClientCannotProcessRequest)
								}
								if len(fhirPractitioners) == 0 {
									fhirPractitionerRequest := &fhir_dto.Practitioner{
										ResourceType: constvars.ResourcePractitioner,
										Active:       true,
										Identifier: []fhir_dto.Identifier{
											{
												System: "https://login.konsulin.care/userid",
												Value:  user.ID,
											},
										},
										Telecom: []fhir_dto.ContactPoint{
											{
												System: "email",
												Value:  *user.Email,
												Use:    "work",
											},
										},
									}
									_, err = uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
									if err != nil {
										uc.Log.Error("authUsecase.SupertokenConsumeCode supertokens error create practitioner for user by UserID & email",
											zap.Error(err),
										)
										return plessmodels.ConsumeCodeResponse{}, err
									}
								}
							}

							uc.Log.Info("authUsecase.SupertokenConsumeCode assigning Patient Role to User")
							response, err := userroles.AddRoleToUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID, constvars.KonsulinRolePatient, nil)
							if err != nil {
								uc.Log.Error("authUsecase.SupertokenConsumeCode error userroles.AddRoleToUser",
									zap.Error(err),
								)
								return plessmodels.ConsumeCodeResponse{}, err
							}

							if response.UnknownRoleError != nil {
								uc.Log.Error("authUsecase.SupertokenConsumeCode error unknown role",
									zap.Error(err),
								)
								return plessmodels.ConsumeCodeResponse{
									RestartFlowError: &struct{}{},
								}, nil
							}

							if response.OK.DidUserAlreadyHaveRole {
								uc.Log.Info("authUsecase.SupertokenConsumeCode user already have role")
							}
						}
						return response, nil
					}
					return originalImplementation
				},
			},
			EmailDelivery: &emaildelivery.TypeInput{
				// Override: func(originalImplementation emaildelivery.EmailDeliveryInterface) emaildelivery.EmailDeliveryInterface {
				// 	(*originalImplementation.SendEmail) = func(input emaildelivery.EmailType, userContext supertokens.UserContext) error {
				// 		emailPayload := utils.BuildPasswordlessMagicLinkEmailPayload(
				// 			uc.InternalConfig.Mailer.EmailSender,
				// 			input.PasswordlessLogin.Email,
				// 			*input.PasswordlessLogin.UrlWithLinkCode,
				// 		)

				// 		ctx := context.Background()
				// 		err := uc.MailerService.SendEmail(ctx, emailPayload)
				// 		if err != nil {
				// 			return err
				// 		}

				// 		return nil
				// 	}

				// 	return originalImplementation
				// },
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
						fmt.Println(userID, userContext)
						// or call the default behaviour as show below
						return originalCreateNewSession(userID, accessTokenPayload, sessionDataInDatabase, disableAntiCsrf, tenantId, userContext)
					}

					return originalImplementation
				},
			},
			CookieSameSite: &cookieSameSite,
		}),
		dashboard.Init(&dashboardmodels.TypeInput{
			Admins: &[]string{
				uc.InternalConfig.Supertoken.KonsulinDasboardAdminEmail,
			},
		}),
	}

	err := supertokens.Init(supertokens.TypeInput{
		OnSuperTokensAPIError: func(err error, req *http.Request, res http.ResponseWriter) {
			uc.Log.Error(err.Error())
		},
		Supertokens: supertokenConnectionInfo,
		AppInfo:     supertokenAppInfo,
		RecipeList:  supertokenRecipeList,
	})
	if err != nil {
		return err
	}
	log.Println("Successfully initialized supertokens SDK")
	return nil
}
