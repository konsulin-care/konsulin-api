package controllers

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type AuthController struct {
	Log         *zap.Logger
	AuthUsecase contracts.AuthUsecase
}

var (
	authControllerInstance *AuthController
	onceAuthController     sync.Once
)

func NewAuthController(logger *zap.Logger, authUsecase contracts.AuthUsecase) *AuthController {
	onceAuthController.Do(func() {
		instance := &AuthController{
			Log:         logger,
			AuthUsecase: authUsecase,
		}
		authControllerInstance = instance
	})
	return authControllerInstance
}
func (ctrl *AuthController) RegisterViaWhatsApp(w http.ResponseWriter, r *http.Request) {
	// Check for requestID.
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.RegisterViaWhatsApp requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.RegisterViaWhatsApp called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	// Bind body to request.
	request := new(requests.RegisterViaWhatsApp)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.RegisterViaWhatsApp error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Sanitize and validate request.
	utils.SanitizeRegisterViaWhatsAppRequest(request)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.RegisterViaWhatsApp validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Process registration.
	if err := ctrl.AuthUsecase.RegisterViaWhatsApp(ctx, request); err != nil {
		ctrl.Log.Error("AuthController.RegisterViaWhatsApp error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.RegisterViaWhatsApp succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.WhatsAppOTPSuccessMessage, nil)
}

func (ctrl *AuthController) LoginViaWhatsApp(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.LoginViaWhatsApp  requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.LoginViaWhatsApp called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.LoginViaWhatsApp)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.LoginViaWhatsApp error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.SanitizeLoginViaWhatsAppRequest(request)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.LoginViaWhatsApp validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := ctrl.AuthUsecase.LoginViaWhatsApp(ctx, request); err != nil {
		ctrl.Log.Error("AuthController.LoginViaWhatsApp error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.LoginViaWhatsApp succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.WhatsAppOTPSuccessMessage, nil)
}

func (ctrl *AuthController) VerifyRegisterWhatsAppOTP(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.VerifyRegisterWhatsAppOTP requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.VerifyRegisterWhatsAppOTP called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.VerivyRegisterWhatsAppOTP)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.VerifyRegisterWhatsAppOTP error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.SanitizeVerifyRegisterWhatsAppOTP(request)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.VerifyRegisterWhatsAppOTP validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.VerifyRegisterWhatsAppOTP(ctx, request)
	if err != nil {
		ctrl.Log.Error("AuthController.VerifyRegisterWhatsAppOTP error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.VerifyRegisterWhatsAppOTP succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) VerifyLoginWhatsAppOTP(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.VerifyLoginWhatsAppOTP requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.VerifyLoginWhatsAppOTP called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.VerivyLoginWhatsAppOTP)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.VerifyLoginWhatsAppOTP error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.SanitizeVerifyLoginWhatsAppOTP(request)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.VerifyLoginWhatsAppOTP validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.VerifyLoginWhatsAppOTP(ctx, request)
	if err != nil {
		ctrl.Log.Error("AuthController.VerifyLoginWhatsAppOTP error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.VerifyLoginWhatsAppOTP succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) RegisterClinician(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.RegisterClinician requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.RegisterClinician called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.RegisterUser)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.RegisterClinician error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.SanitizeRegisterUserRequest(request)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.RegisterClinician validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	// var userContext supertokens.UserContext = &map[string]any{
	// 	"username": request.Username,
	// }

	// resp, err := emailpassword.SignUp(constvars.SupertokenKonsulinTenantID, request.Email, request.Password, userContext)
	// if err != nil {
	// 	utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerProcess(err))
	// 	return
	// }

	// fmt.Println(resp)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.RegisterClinician(ctx, request)
	if err != nil {
		ctrl.Log.Error("AuthController.RegisterClinician error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.RegisterClinician succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusCreated, constvars.CreateUserSuccessMessage, response)
}

func (ctrl *AuthController) RegisterPatient(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.RegisterPatient requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.RegisterPatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.RegisterUser)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.RegisterPatient error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.SanitizeRegisterUserRequest(request)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.RegisterPatient validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.RegisterPatient(ctx, request)
	if err != nil {
		ctrl.Log.Error("AuthController.RegisterPatient error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.RegisterPatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusCreated, constvars.CreateUserSuccessMessage, response)
}

func (ctrl *AuthController) LoginPatient(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.LoginPatient requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.LoginPatient called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.LoginUser)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.LoginPatient error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.LoginPatient validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.LoginPatient(ctx, request)
	if err != nil {
		ctrl.Log.Error("AuthController.LoginPatient error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.LoginPatient succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) LoginClinician(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.LoginClinician requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.LoginClinician called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.LoginUser)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.LoginClinician error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.LoginClinician validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AuthUsecase.LoginClinician(ctx, request)
	if err != nil {
		ctrl.Log.Error("AuthController.LoginClinician error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.LoginClinician succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.Logout requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.Logout called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("AuthController.Logout sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := ctrl.AuthUsecase.LogoutUser(ctx, sessionData); err != nil {
		ctrl.Log.Error("AuthController.Logout error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}
	ctrl.Log.Info("AuthController.Logout succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LogoutSuccessMessage, nil)
}

func (ctrl *AuthController) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.ForgotPassword requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.ForgotPassword called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.ForgotPassword)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.ForgotPassword error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.ForgotPassword validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := ctrl.AuthUsecase.ForgotPassword(ctx, request); err != nil {
		ctrl.Log.Error("AuthController.ForgotPassword error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.ForgotPassword succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ForgotPasswordSuccessMessage, nil)
}

func (ctrl *AuthController) ResetPassword(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.ResetPassword requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.ResetPassword called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.ResetPassword)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.ResetPassword error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.ResetPassword validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := ctrl.AuthUsecase.ResetPassword(ctx, request); err != nil {
		ctrl.Log.Error("AuthController.ResetPassword error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.ResetPassword succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResetPasswordSuccessMessage, nil)
}

func (ctrl *AuthController) CreateMagicLink(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.MagicLink requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.MagicLink called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.SupertokenPasswordlessCreateMagicLink)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AuthController.MagicLink error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AuthController.MagicLink validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := ctrl.AuthUsecase.CreateMagicLink(ctx, request); err != nil {
		ctrl.Log.Error("AuthController.MagicLink error from usecase",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	ctrl.Log.Info("AuthController.MagicLink succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.MagicLinkSuccessMessage, nil)
}
