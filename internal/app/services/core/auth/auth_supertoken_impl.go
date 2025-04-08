package auth

import (
	"context"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"log"
	"regexp"
	"strings"

	"github.com/supertokens/supertokens-golang/ingredients/smsdelivery"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword"
	"github.com/supertokens/supertokens-golang/recipe/emailpassword/epmodels"
	"github.com/supertokens/supertokens-golang/recipe/passwordless"
	"github.com/supertokens/supertokens-golang/recipe/passwordless/plessmodels"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty"
	"github.com/supertokens/supertokens-golang/recipe/thirdparty/tpmodels"
	"github.com/supertokens/supertokens-golang/recipe/userroles"
	"github.com/supertokens/supertokens-golang/supertokens"
	"go.uber.org/zap"
)

func (uc *authUsecase) InitializeSupertoken() error {
	apiBasePath := fmt.Sprintf("%s/%s%s", uc.InternalConfig.App.EndpointPrefix, uc.InternalConfig.App.Version, uc.DriverConfig.Supertoken.ApiBasePath)
	websiteBasePath := uc.DriverConfig.Supertoken.WebsiteBasePath

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
		thirdparty.Init(&tpmodels.TypeInput{
			SignInAndUpFeature: tpmodels.TypeInputSignInAndUp{
				Providers: []tpmodels.ProviderInput{
					{
						Config: tpmodels.ProviderConfig{
							ThirdPartyId: "google",
							Clients: []tpmodels.ProviderClientConfig{
								{
									ClientID:     "1060725074195-kmeum4crr01uirfl2op9kd5acmi9jutn.apps.googleusercontent.com",
									ClientSecret: "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",
								},
							},
						},
					},
					{
						Config: tpmodels.ProviderConfig{
							ThirdPartyId: "github",
							Clients: []tpmodels.ProviderClientConfig{
								{
									ClientID:     "467101b197249757c71f",
									ClientSecret: "e97051221f4b6426e8fe8d51486396703012f5bd",
								},
							},
						},
					},
				},
			},
		}),
		passwordless.Init(plessmodels.TypeInput{
			Override: &plessmodels.OverrideStruct{
				Functions: func(originalImplementation plessmodels.RecipeInterface) plessmodels.RecipeInterface {
					originalConsumeCode := *originalImplementation.ConsumeCode

					(*originalImplementation.ConsumeCode) = func(userInput *plessmodels.UserInputCodeWithDeviceID, linkCode *string, preAuthSessionID string, tenantId string, userContext supertokens.UserContext) (plessmodels.ConsumeCodeResponse, error) {

						response, err := originalConsumeCode(userInput, linkCode, preAuthSessionID, tenantId, userContext)
						if err != nil {
							return plessmodels.ConsumeCodeResponse{}, err
						}

						if response.OK != nil {
							user := response.OK.User
							if response.OK.CreatedNewUser {
								uc.Log.Info("authUsecase.InitializeSupertoken assigning Patient Role to CreatedNewUser")
								response, err := userroles.AddRoleToUser("public", user.ID, constvars.KonsulinRolePatient, nil)
								if err != nil {
									uc.Log.Error("authUsecase.InitializeSupertoken error userroles.AddRoleToUser",
										zap.Error(err),
									)
									return plessmodels.ConsumeCodeResponse{}, err
								}

								if response.UnknownRoleError != nil {
									uc.Log.Error("authUsecase.InitializeSupertoken error unknown role",
										zap.Error(err),
									)
									return plessmodels.ConsumeCodeResponse{}, fmt.Errorf("unknown role")
								}

								if response.OK.DidUserAlreadyHaveRole {
									uc.Log.Info("authUsecase.InitializeSupertoken user already have role")
								}
							} else {
								// TODO: Post sign in logic
							}

						}
						return response, nil
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
			ContactMethodPhone: plessmodels.ContactMethodPhoneConfig{
				Enabled: true,
				ValidatePhoneNumber: func(phoneNumber interface{}, tenantId string) *string {
					number := strings.TrimPrefix(phoneNumber.(string), "+")
					match, err := regexp.MatchString(`^[1-9][0-9]{7,14}$`, number)
					if err != nil {
						message := "invalid phone number"
						return &message
					}

					if match {
						return nil
					}
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
					// create a copy of the originalImplementation func
					originalSignIn := *originalImplementation.SignIn
					originalSignUp := *originalImplementation.SignUp

					// override the sign in up function
					(*originalImplementation.SignIn) = func(email, password, tenantId string, userContext supertokens.UserContext) (epmodels.SignInResponse, error) {
						// First we call the original implementation of SignIn.
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
		session.Init(nil),
	}

	err := supertokens.Init(supertokens.TypeInput{
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
