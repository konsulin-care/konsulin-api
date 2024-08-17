package users

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type UserController struct {
	Log            *zap.Logger
	UserUsecase    UserUsecase
	InternalConfig *config.InternalConfig
}

func NewUserController(logger *zap.Logger, userUsecase UserUsecase, internalConfig *config.InternalConfig) *UserController {
	return &UserController{
		Log:            logger,
		UserUsecase:    userUsecase,
		InternalConfig: internalConfig,
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
	// Bind body to request
	request := new(requests.UpdateProfile)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if request.ProfilePicture != "" {
		data, ext, err := utils.DecodeBase64Image(request.ProfilePicture)
		if err != nil {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrImageValidation(err))
			return
		}

		err = utils.ValidateImageFormat(ext, constvars.ImageAllowedProfilePictureFormats)
		if err != nil {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrImageValidation(err))
			return
		}

		err = utils.ValidateImageSize(data, ctrl.InternalConfig.Minio.ProfilePictureMaxUploadSizeInMB)
		if err != nil {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrImageValidation(err))
			return
		}

		request.ProfilePictureData = data
		request.ProfilePictureExtension = ext
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
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
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
