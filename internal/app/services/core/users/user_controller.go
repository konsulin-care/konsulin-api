package users

import (
	"context"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type UserController struct {
	Log         *zap.Logger
	UserUsecase UserUsecase
}

func NewUserController(logger *zap.Logger, userUsecase UserUsecase) *UserController {
	return &UserController{
		Log:         logger,
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
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetProfileSuccessMessage, result)
}
func (ctrl *UserController) UpdateUserBySession(w http.ResponseWriter, r *http.Request) {
	// Parse the multipart form from the request with a maximum memory of 10 MB
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		// If there is an error parsing the form, build and send an error response
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Build the update user profile request from the form data
	request, err := utils.BuildUpdateUserProfileRequest(r)
	if err != nil {
		// If there is an error building the request, build and send an error response
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrBuildRequest(err))
		return
	}

	// Sanitize the request data to remove any unwanted characters or spaces
	utils.SanitizeUpdateProfileRequest(request)

	// Validate the sanitized request data to ensure it meets all requirements
	err = utils.ValidateStruct(request)
	if err != nil {
		// If the validation fails, build and send an error response
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	// Retrieve the session data from the request context
	sessionData := r.Context().Value("sessionData").(string)

	// Create a new context with a timeout of 20 seconds for the update operation
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Call the usecase to update the user profile based on the session data and request
	response, err := ctrl.UserUsecase.UpdateUserProfileBySession(ctx, sessionData, request)
	if err != nil {
		// Check if the error is due to the context deadline being exceeded
		if err == context.DeadlineExceeded {
			// If the context deadline is exceeded, build and send a deadline exceeded error response
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		// For any other error, build and send a generic error response
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	// If the update is successful, build and send a success response
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateUserSuccessMessage, response)
}

func (ctrl *UserController) DeactivateUserBySession(w http.ResponseWriter, r *http.Request) {
	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ctrl.UserUsecase.DeactivateUserBySession(ctx, sessionData)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteUserSuccessMessage, nil)
}
