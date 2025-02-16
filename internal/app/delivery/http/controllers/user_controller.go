package controllers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

type UserController struct {
	Log            *zap.Logger
	UserUsecase    contracts.UserUsecase
	InternalConfig *config.InternalConfig
}

var (
	userControllerInstance *UserController
	onceUserController     sync.Once
)

func NewUserController(logger *zap.Logger, userUsecase contracts.UserUsecase, internalConfig *config.InternalConfig) *UserController {
	onceUserController.Do(func() {
		instance := &UserController{
			Log:            logger,
			UserUsecase:    userUsecase,
			InternalConfig: internalConfig,
		}
		userControllerInstance = instance
	})
	return userControllerInstance
}
func (ctrl *UserController) GetUserProfileBySession(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("UserController.GetUserProfileBySession requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("UserController.GetUserProfileBySession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("UserController.GetUserProfileBySession sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.UserUsecase.GetUserProfileBySession(ctx, sessionData)
	if err != nil {
		ctrl.Log.Error("UserController.GetUserProfileBySession error from usecase",
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

	ctrl.Log.Info("UserController.GetUserProfileBySession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetProfileSuccessMessage, result)
}

func (ctrl *UserController) UpdateUserBySession(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("UserController.UpdateUserBySession requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("UserController.UpdateUserBySession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	reqPayload := new(requests.UpdateProfile)
	if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
		ctrl.Log.Error("UserController.UpdateUserBySession error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if reqPayload.ProfilePicture != "" {
		data, ext, err := utils.DecodeBase64Image(reqPayload.ProfilePicture)
		if err != nil {
			ctrl.Log.Error("UserController.UpdateUserBySession error decoding base64 image",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrImageValidation(err))
			return
		}
		if err := utils.ValidateImageFormat(ext, constvars.ImageAllowedProfilePictureFormats); err != nil {
			ctrl.Log.Error("UserController.UpdateUserBySession error validating image format",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrImageValidation(err))
			return
		}
		if err := utils.ValidateImageSize(data, ctrl.InternalConfig.Minio.ProfilePictureMaxUploadSizeInMB); err != nil {
			ctrl.Log.Error("UserController.UpdateUserBySession error validating image size",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrImageValidation(err))
			return
		}
		reqPayload.ProfilePictureData = data
		reqPayload.ProfilePictureExtension = ext
	}

	utils.SanitizeUpdateProfileRequest(reqPayload)

	if err := utils.ValidateStruct(reqPayload); err != nil {
		ctrl.Log.Error("UserController.UpdateUserBySession validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("UserController.UpdateUserBySession sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()

	response, err := ctrl.UserUsecase.UpdateUserProfileBySession(ctx, sessionData, reqPayload)
	if err != nil {
		ctrl.Log.Error("UserController.UpdateUserBySession error from usecase",
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

	ctrl.Log.Info("UserController.UpdateUserBySession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any("response", response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateUserSuccessMessage, response)
}

func (ctrl *UserController) DeactivateUserBySession(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("UserController.DeactivateUserBySession requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("UserController.DeactivateUserBySession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("UserController.DeactivateUserBySession sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.UserUsecase.DeactivateUserBySession(ctx, sessionData)
	if err != nil {
		ctrl.Log.Error("UserController.DeactivateUserBySession error from usecase",
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

	ctrl.Log.Info("UserController.DeactivateUserBySession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteUserSuccessMessage, nil)
}
