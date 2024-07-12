package users

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"
)

type UserController struct {
	UserUsecase UserUsecase
}

func NewUserController(userUsecase UserUsecase) *UserController {
	return &UserController{
		UserUsecase: userUsecase,
	}
}

func (ctrl *UserController) GetUserProfileBySession(w http.ResponseWriter, r *http.Request) {
	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.UserUsecase.GetUserProfileBySession(ctx, sessionData)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.ProfileGetSuccess, result)
}

func (ctrl *UserController) UpdateUserBySession(w http.ResponseWriter, r *http.Request) {
	request := new(requests.UpdateProfile)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Validate request
	err = utils.ValidateStruct(request)
	if err != nil {
		utils.BuildErrorResponse(w, exceptions.ErrInputValidation(err))
		return
	}

	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.UserUsecase.UpdateUserProfileBySession(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UserUpdatedSuccess, response)
}
