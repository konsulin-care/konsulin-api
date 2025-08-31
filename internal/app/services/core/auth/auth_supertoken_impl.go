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

const (
	supertokenAccessTokenPayloadRolesKey      = "st-role"
	supertokenAccessTokenPayloadRolesValueKey = "v"
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
							log.Println("authUsecase.SupertokenConsumeCode error while do func originalConsumeCode",
								zap.Error(err),
							)
							return plessmodels.ConsumeCodeResponse{}, err
						}

						if response.OK != nil {
							user := response.OK.User
							rolesResp, err := userroles.GetRolesForUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID)
							if err != nil {
								log.Println("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID",
									zap.Error(err),
								)
								return plessmodels.ConsumeCodeResponse{}, err
							}

							userRoles := rolesResp.OK.Roles
							hasPract := slices.Contains(userRoles, constvars.KonsulinRolePractitioner)
							hasPat := slices.Contains(userRoles, constvars.KonsulinRolePatient)

							mainRole := ""
							fhirID := ""

							if hasPract {
								ctx := context.Background()
								fhirPractitioners, err := uc.PractitionerFhirClient.FindPractitionerByIdentifier(ctx, constvars.FhirSupertokenSystemIdentifier, user.ID)
								if len(fhirPractitioners) > 1 {
									log.Println("authUsecase.SupertokenConsumeCode supertokens error get roles for user by tenantID & UserID",
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
												System: constvars.FhirSupertokenSystemIdentifier,
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
									created, err := uc.PractitionerFhirClient.CreatePractitioner(ctx, fhirPractitionerRequest)
									if err != nil {
										log.Println("authUsecase.SupertokenConsumeCode supertokens error create practitioner for user by UserID & email",
											zap.Error(err),
										)
										return plessmodels.ConsumeCodeResponse{}, err
									}
									fhirID = created.ID
								} else {
									fhirID = fhirPractitioners[0].ID
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
									uc.Log.Error("authUsecase.SupertokenConsumeCode user already have role")
								}
							} else {
								if !hasPat {
									addResp, err := userroles.AddRoleToUser(
										uc.InternalConfig.Supertoken.KonsulinTenantID,
										user.ID,
										constvars.KonsulinRolePatient,
										nil,
									)
									if err != nil {
										log.Println("consumeCode: add Patient role", zap.Error(err))
										return plessmodels.ConsumeCodeResponse{}, err
									}
									if addResp.UnknownRoleError != nil {
										log.Println("consumeCode: unknown Patient role")
										return plessmodels.ConsumeCodeResponse{
											RestartFlowError: &struct{}{},
										}, nil
									}
									if addResp.OK.DidUserAlreadyHaveRole {
										log.Println("consumeCode: user already Patient")
									}
								}

								mainRole = constvars.KonsulinRolePatient

								ctx := context.Background()
								list, err := uc.PatientFhirClient.FindPatientByIdentifier(
									ctx, constvars.FhirSystemSupertokenIdentifier, user.ID)
								if err != nil {
									log.Println("consumeCode: find patient", zap.Error(err))
									return plessmodels.ConsumeCodeResponse{}, err
								}
								if len(list) > 1 {
									log.Println("consumeCode: more than 1 patient for uid",
										zap.String("uid", user.ID))
									return plessmodels.ConsumeCodeResponse{}, errors.New(constvars.ErrClientCannotProcessRequest)
								}
								if len(list) == 0 {
									newPatient := &fhir_dto.Patient{
										ResourceType: constvars.ResourcePatient,
										Active:       true,
										Identifier: []fhir_dto.Identifier{{
											System: constvars.FhirSystemSupertokenIdentifier,
											Value:  user.ID,
										}},
										Telecom: []fhir_dto.ContactPoint{{
											System: "email",
											Value:  *user.Email,
											Use:    "home",
										}},
									}
									created, err := uc.PatientFhirClient.CreatePatient(ctx, newPatient)
									if err != nil {
										uc.Log.Error("consumeCode: create patient", zap.Error(err))
										return plessmodels.ConsumeCodeResponse{}, err
									}
									fhirID = created.ID
								} else {
									fhirID = list[0].ID
								}
							}

							uc.Log.Info("consumeCode: login OK",
								zap.String("uid", user.ID),
								zap.String("role", mainRole),
								zap.String("fhir_id", fhirID),
							)
							return response, nil
						}
						return response, nil
					}
					return originalImplementation
				},
			},
			EmailDelivery: &emaildelivery.TypeInput{},
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
