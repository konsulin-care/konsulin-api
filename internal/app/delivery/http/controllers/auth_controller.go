package controllers

import (
	"context"
	"konsulin-service/internal/app/services/core/auth"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/zap"
)

type AuthController struct {
	Log         *zap.Logger
	AuthUsecase auth.AuthUsecase
}

func NewAuthController(logger *zap.Logger, authUsecase auth.AuthUsecase) *AuthController {
	return &AuthController{
		Log:         logger,
		AuthUsecase: authUsecase,
	}
}

func (ctrl *AuthController) RegisterViaWhatsApp(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.RegisterViaWhatsApp)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	// Sanitize request
	utils.SanitizeRegisterViaWhatsAppRequest(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	err = ctrl.AuthUsecase.RegisterViaWhatsApp(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.WhatsAppOTPSuccessMessage, nil)
}

func (ctrl *AuthController) LoginViaWhatsApp(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.LoginViaWhatsApp)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	// Sanitize request
	utils.SanitizeLoginViaWhatsAppRequest(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	err = ctrl.AuthUsecase.LoginViaWhatsApp(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.WhatsAppOTPSuccessMessage, nil)
}

func (ctrl *AuthController) VerifyRegisterWhatsAppOTP(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.VerivyRegisterWhatsAppOTP)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	// Sanitize request
	utils.SanitizeVerifyRegisterWhatsAppOTP(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	response, err := ctrl.AuthUsecase.VerifyRegisterWhatsAppOTP(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) VerifyLoginWhatsAppOTP(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.VerivyLoginWhatsAppOTP)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	// Sanitize request
	utils.SanitizeVerifyLoginWhatsAppOTP(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	response, err := ctrl.AuthUsecase.VerifyLoginWhatsAppOTP(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) RegisterClinician(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.RegisterUser)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Sanitize request
	utils.SanitizeRegisterUserRequest(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	response, err := ctrl.AuthUsecase.RegisterClinician(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusCreated, constvars.CreateUserSuccessMessage, response)
}

func (ctrl *AuthController) RegisterPatient(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.RegisterUser)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	// Sanitize request
	utils.SanitizeRegisterUserRequest(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	response, err := ctrl.AuthUsecase.RegisterPatient(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusCreated, constvars.CreateUserSuccessMessage, response)
}

func (ctrl *AuthController) LoginPatient(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.LoginUser)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send request to be processed by usecase
	response, err := ctrl.AuthUsecase.LoginPatient(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) LoginClinician(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.LoginUser)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send request to be processed by usecase
	response, err := ctrl.AuthUsecase.LoginClinician(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccessMessage, response)
}

func (ctrl *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	// Get session
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ctrl.AuthUsecase.LogoutUser(ctx, sessionData)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LogoutSuccessMessage, nil)
}

func (ctrl *AuthController) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.ForgotPassword)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send request to be processed by usecase
	err = ctrl.AuthUsecase.ForgotPassword(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ForgotPasswordSuccessMessage, nil)
}

func (ctrl *AuthController) ResetPassword(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.ResetPassword)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send request to be processed by usecase
	err = ctrl.AuthUsecase.ResetPassword(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ResetPasswordSuccessMessage, nil)
}
