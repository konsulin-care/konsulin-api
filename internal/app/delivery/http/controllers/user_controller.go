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
	start := time.Now()
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("Request ID missing from context",
			zap.String(constvars.LoggingEndpointKey, r.URL.Path),
			zap.String(constvars.LoggingMethodKey, r.Method),
			zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	ctrl.Log.Debug("User profile retrieval started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEndpointKey, r.URL.Path),
		zap.String(constvars.LoggingMethodKey, r.Method),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("Session data missing from context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "authentication"),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := ctrl.UserUsecase.GetUserProfileBySession(ctx, sessionData)
	if err != nil {
		ctrl.Log.Error("Failed to retrieve user profile",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "usecase error"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.LogBusinessEvent(ctrl.Log, "user_profile_retrieved", requestID,
		zap.String(constvars.LoggingEmailKey, result.Email),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetProfileSuccessMessage, result)
}

func (ctrl *UserController) UpdateUserBySession(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("Request ID missing from context",
			zap.String(constvars.LoggingEndpointKey, r.URL.Path),
			zap.String(constvars.LoggingMethodKey, r.Method),
			zap.String(constvars.LoggingRemoteAddrKey, r.RemoteAddr),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	ctrl.Log.Debug("User profile update started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEndpointKey, r.URL.Path),
		zap.String(constvars.LoggingMethodKey, r.Method),
	)

	reqPayload := new(requests.UpdateProfile)
	if err := json.NewDecoder(r.Body).Decode(&reqPayload); err != nil {
		ctrl.Log.Error("Failed to parse request body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	

	utils.SanitizeUpdateProfileRequest(reqPayload)

	if err := utils.ValidateStruct(reqPayload); err != nil {
		ctrl.Log.Error("Request validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "input validation"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("Session data missing from context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "authentication"),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()

	response, err := ctrl.UserUsecase.UpdateUserProfileBySession(ctx, sessionData, reqPayload)
	if err != nil {
		ctrl.Log.Error("Failed to update user profile",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "usecase error"),
			zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.LogBusinessEvent(ctrl.Log, "user_profile_updated", requestID,
		zap.String("patient_id", response.PatientID),
		zap.String("practitioner_id", response.PractitionerID),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
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
