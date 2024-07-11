package auth

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/goccy/go-json"
)

type AuthController struct {
	AuthUsecase AuthUsecase
}

func NewAuthController(authUsecase AuthUsecase) *AuthController {
	return &AuthController{
		AuthUsecase: authUsecase,
	}
}

func (ctrl *AuthController) RegisterUser(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.RegisterUser)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get query params and attach to request struct
	queryParams := r.URL.Query()
	request.UserType = queryParams.Get(constvars.QueryParamsUserType)

	// Sanitize request
	utils.SanitizeRegisterUserRequest(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send it to be processed by usecase
	response, err := ctrl.AuthUsecase.RegisterUser(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusCreated, constvars.UserCreatedSuccess, response)
}

func (ctrl *AuthController) LoginUser(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.LoginUser)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get query params and attach to request struct
	queryParams := r.URL.Query()
	request.UserType = queryParams.Get(constvars.QueryParamsUserType)

	utils.SanitizeLoginUserRequest(request)

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrInputValidation(err))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send request to be processed by usecase
	response, err := ctrl.AuthUsecase.LoginUser(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}

	// Send response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LoginSuccess, response)
}

func (ctrl *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	// Get session
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ctrl.AuthUsecase.LogoutUser(ctx, sessionData)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.LogoutSuccess, nil)
}
