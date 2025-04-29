package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"log"
	"net/http"

	"github.com/supertokens/supertokens-golang/ingredients/emaildelivery"
	"github.com/supertokens/supertokens-golang/ingredients/smsdelivery"
	"github.com/supertokens/supertokens-golang/recipe/dashboard"
	"github.com/supertokens/supertokens-golang/recipe/dashboard/dashboardmodels"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword/epmodels"
	"github.com/supertokens/supertokens-golang/recipe/emailverification"
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
				APIs: func(originalImplementation plessmodels.APIInterface) plessmodels.APIInterface {
					(*originalImplementation.CreateCodePOST) = func(email *string, phoneNumber *string, tenantId string, options plessmodels.APIOptions, userContext supertokens.UserContext) (plessmodels.CreateCodePOSTResponse, error) {
						request := new(requests.SupertokenPasswordlessSigninupCreateCode)
						request.Email = email

						err := utils.ValidateStruct(request)
						if err != nil {
							err = exceptions.ErrInputValidation(err)
							utils.BuildErrorResponse(uc.Log, options.Res, err)
							return plessmodels.CreateCodePOSTResponse{
								OK: &struct {
									DeviceID         string
									PreAuthSessionID string
									FlowType         string
								}{
									DeviceID:         "",
									PreAuthSessionID: "",
									FlowType:         "",
								},
							}, nil
						}

						response, err := (*options.RecipeImplementation.CreateCode)(request.Email, phoneNumber, nil, tenantId, userContext)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error while do func originalCreateCode",
								zap.Error(err),
							)
							err = exceptions.ErrSupertoken(err)
							utils.BuildErrorResponse(uc.Log, options.Res, err)
							return plessmodels.CreateCodePOSTResponse{
								OK: &struct {
									DeviceID         string
									PreAuthSessionID string
									FlowType         string
								}{
									DeviceID:         "",
									PreAuthSessionID: "",
									FlowType:         "",
								},
							}, nil
						}

						magicLink, err := utils.GenerateMagicLink(
							uc.InternalConfig.Supertoken.MagiclinkBaseUrl,
							response.OK.PreAuthSessionID,
							uc.InternalConfig.Supertoken.KonsulinTenantID,
							response.OK.LinkCode,
						)

						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error while do func utils.GenerateMagicLink",
								zap.Error(err),
							)
							err = exceptions.ErrSupertoken(err)
							utils.BuildErrorResponse(uc.Log, options.Res, err)
							return plessmodels.CreateCodePOSTResponse{
								OK: &struct {
									DeviceID         string
									PreAuthSessionID string
									FlowType         string
								}{
									DeviceID:         "",
									PreAuthSessionID: "",
									FlowType:         "",
								},
							}, nil
						}

						emailPayload := utils.BuildPasswordlessMagicLinkEmailPayload(uc.InternalConfig.Mailer.EmailSender, *request.Email, magicLink)

						err = uc.MailerService.SendEmail(context.Background(), emailPayload)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error while do func MailerService.SendEmail",
								zap.Error(err),
							)
							err = exceptions.ErrSupertoken(err)
							utils.BuildErrorResponse(uc.Log, options.Res, err)
							return plessmodels.CreateCodePOSTResponse{
								OK: &struct {
									DeviceID         string
									PreAuthSessionID string
									FlowType         string
								}{
									DeviceID:         "",
									PreAuthSessionID: "",
									FlowType:         "",
								},
							}, nil
						}

						dataResponse := responses.SupertokenCreateCode{
							CodeID:           response.OK.CodeID,
							DeviceID:         response.OK.DeviceID,
							PreAuthSessionID: response.OK.PreAuthSessionID,
						}

						options.Res.Header().Set("Content-Type", "application/json; charset=utf-8")
						options.Res.WriteHeader(200)

						responseJson := responses.ResponseDTO{
							Success: true,
							Message: "successfully create passwordless code",
							Data:    dataResponse,
						}

						bytes, _ := json.Marshal(responseJson)
						options.Res.Write(bytes)

						return plessmodels.CreateCodePOSTResponse{
							OK: &struct {
								DeviceID         string
								PreAuthSessionID string
								FlowType         string
							}{
								DeviceID:         "",
								PreAuthSessionID: "",
								FlowType:         "",
							},
						}, nil
					}

					(*originalImplementation.ConsumeCodePOST) = func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, options plessmodels.APIOptions, userContext supertokens.UserContext) (plessmodels.ConsumeCodePOSTResponse, error) {
						response, err := (*options.RecipeImplementation.ConsumeCode)(userInput, linkCode, preAuthSessionID, tenantId, userContext)
						if err != nil {
							uc.Log.Error("authUsecase.ConsumeCodePOST error while do func originalCreateCode",
								zap.Error(err),
							)
							return plessmodels.ConsumeCodePOSTResponse{}, exceptions.ErrSupertoken(err)
						}

						if response.OK == nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error while do func supertoken options.RecipeImplementation.ConsumeCode")
							err = exceptions.ErrSupertoken(err)
							utils.BuildErrorResponse(uc.Log, options.Res, err)
							return plessmodels.ConsumeCodePOSTResponse{}, nil
						}

						user := response.OK.User

						if user.Email != nil {
							evInstance := emailverification.GetRecipeInstance()
							if evInstance != nil {
								tokenResponse, err := (*evInstance.RecipeImpl.CreateEmailVerificationToken)(user.ID, *user.Email, tenantId, userContext)
								if err != nil {
									uc.Log.Error("authUsecase.SupertokenCreateCode error while do func supertoken evInstance.RecipeImpl.CreateEmailVerificationToken")
									err = exceptions.ErrSupertoken(err)
									utils.BuildErrorResponse(uc.Log, options.Res, err)
									return plessmodels.ConsumeCodePOSTResponse{}, nil
								}
								if tokenResponse.OK != nil {
									_, err := (*evInstance.RecipeImpl.VerifyEmailUsingToken)(tokenResponse.OK.Token, tenantId, userContext)
									if err != nil {
										uc.Log.Error("authUsecase.SupertokenCreateCode error while do func supertoken evInstance.RecipeImpl.VerifyEmailUsingToken")
										err = exceptions.ErrSupertoken(err)
										utils.BuildErrorResponse(uc.Log, options.Res, err)
										return plessmodels.ConsumeCodePOSTResponse{}, nil
									}
								}
							}
						}

						session, err := session.CreateNewSession(options.Req, options.Res, tenantId, user.ID, map[string]interface{}{}, map[string]interface{}{}, userContext)
						if err != nil {
							uc.Log.Error("authUsecase.SupertokenCreateCode error while do func supertoken session.CreateNewSession")
							err = exceptions.ErrSupertoken(err)
							utils.BuildErrorResponse(uc.Log, options.Res, err)
							return plessmodels.ConsumeCodePOSTResponse{}, nil
						}

						options.Res.Header().Set("Content-Type", "application/json; charset=utf-8")
						options.Res.WriteHeader(200)

						dataResponse := responses.SupertokenConsumeCode{
							User: responses.SupertokenPlessUser{
								ID:         response.OK.User.ID,
								Email:      *response.OK.User.Email,
								TimeJoined: response.OK.User.TimeJoined,
								TenantIds:  response.OK.User.TenantIds,
							},
							CreatedNewUser: response.OK.CreatedNewUser,
							Session:        session,
						}

						responseJson := responses.ResponseDTO{
							Success: true,
							Message: "successfully consume code",
							Data:    dataResponse,
						}

						bytes, _ := json.Marshal(responseJson)
						options.Res.Write(bytes)

						return plessmodels.ConsumeCodePOSTResponse{
							OK: &struct {
								CreatedNewUser bool
								User           plessmodels.User
								Session        sessmodels.SessionContainer
							}{},
						}, nil
					}

					return originalImplementation
				},
				Functions: func(originalImplementation plessmodels.RecipeInterface) plessmodels.RecipeInterface {
					originalConsumeCode := *originalImplementation.ConsumeCode
					(*originalImplementation.ConsumeCode) = func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error) {
						response, err := originalConsumeCode(userInput, linkCode, preAuthSessionID, tenantId, userContext)
						if err != nil {
							uc.Log.Error("authUsecase.ConsumeCode error while do func originalConsumeCode",
								zap.Error(err),
							)
							return plessmodels.ConsumeCodeResponse{}, exceptions.ErrSupertoken(err)
						}

						if response.OK != nil {
							user := response.OK.User
							if response.OK.CreatedNewUser {
								uc.Log.Info("authUsecase.SupertokenConsumeCode assigning Patient Role to CreatedNewUser")
								response, err := userroles.AddRoleToUser(uc.InternalConfig.Supertoken.KonsulinTenantID, user.ID, constvars.KonsulinRolePatient, nil)
								if err != nil {
									uc.Log.Error("authUsecase.SupertokenConsumeCode error userroles.AddRoleToUser",
										zap.Error(err),
									)
									return plessmodels.ConsumeCodeResponse{}, exceptions.ErrSupertoken(err)
								}

								if response.UnknownRoleError != nil {
									uc.Log.Error("authUsecase.SupertokenConsumeCode error unknown role",
										zap.Error(err),
									)
									return plessmodels.ConsumeCodeResponse{}, exceptions.ErrUnknownRoleType(nil)
								}

								if response.OK.DidUserAlreadyHaveRole {
									uc.Log.Info("authUsecase.SupertokenConsumeCode user already have role")
								}
							} else {
							}
						}
						return response, nil
					}
					return originalImplementation
				},
			},
			EmailDelivery: &emaildelivery.TypeInput{
				Override: func(originalImplementation emaildelivery.EmailDeliveryInterface) emaildelivery.EmailDeliveryInterface {
					(*originalImplementation.SendEmail) = func(input emaildelivery.EmailType, userContext supertokens.UserContext) error {
						emailPayload := utils.BuildPasswordlessMagicLinkEmailPayload(
							uc.InternalConfig.Mailer.EmailSender,
							input.PasswordlessLogin.Email,
							*input.PasswordlessLogin.UrlWithLinkCode,
						)

						ctx := context.Background()
						err := uc.MailerService.SendEmail(ctx, emailPayload)
						if err != nil {
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
						fmt.Println(*urlWithLinkCode)

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
					return nil
				},
			},
		}),
		emailpassword.Init(&epmodels.TypeInput{
			SignUpFeature: &epmodels.TypeInputSignUp{
				FormFields: []epmodels.TypeInputFormField{
					{
						ID: "username",
					},
				},
			},
			Override: &epmodels.OverrideStruct{
				Functions: func(originalImplementation epmodels.RecipeInterface) epmodels.RecipeInterface {
					originalSignIn := *originalImplementation.SignIn
					originalSignUp := *originalImplementation.SignUp

					(*originalImplementation.SignIn) = func(email, password, tenantId string, userContext supertokens.UserContext) (epmodels.SignInResponse, error) {
						response, err := originalSignIn(email, password, tenantId, userContext)
						if err != nil {
							return epmodels.SignInResponse{}, err
						}

						if response.OK != nil {
							// sign in was successful

							// user object contains the ID and email
							user := response.OK.User

							// TODO: Post sign in logic.
							fmt.Println(user)

						}
						return response, nil
					}

					(*originalImplementation.SignUp) = func(email, password, tenantId string, userContext supertokens.UserContext) (epmodels.SignUpResponse, error) {
						response, err := originalSignUp(email, password, tenantId, userContext)
						if err != nil {
							return epmodels.SignUpResponse{}, err
						}
						return response, nil
					}

					return originalImplementation
				},
			},
		}),
		userroles.Init(nil),
		session.Init(&sessmodels.TypeInput{
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
