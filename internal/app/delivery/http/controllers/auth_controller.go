package controllers

import (
	"context"
	"fmt"
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

func (ctrl *AuthController) CreateMagicLink(w http.ResponseWriter, r *http.Request) {
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

	ctrl.Log.Debug("Magic link creation started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEndpointKey, r.URL.Path),
		zap.String(constvars.LoggingMethodKey, r.Method),
	)

	request := new(requests.SupertokenPasswordlessCreateMagicLink)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("Failed to parse request body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	utils.SanitizeCreateMagicLinkRequest(request)

	// Basic validation (email format)
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("Request validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String(constvars.LoggingErrorTypeKey, "input validation"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	// Check if user exists to determine if roles are required
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	userExists, err := ctrl.AuthUsecase.CheckUserExists(ctx, request.Email)
	if err != nil {
		ctrl.Log.Error("AuthController.MagicLink error checking user existence",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	// If user doesn't exist, roles are mandatory
	if !userExists && (len(request.Roles) == 0) {
		ctrl.Log.Error("AuthController.MagicLink roles required for new user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("email", request.Email),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrRolesRequired(nil))
		return
	}

	// If roles are provided, validate them
	if len(request.Roles) > 0 {
		// Validate each role individually
		for _, role := range request.Roles {
			if role != "Patient" && role != "Practitioner" && role != "Clinic Admin" && role != "Researcher" {
				ctrl.Log.Error("Invalid role provided",
					zap.String(constvars.LoggingRequestIDKey, requestID),
					zap.String(constvars.LoggingEmailKey, request.Email),
					zap.String("invalid_role", role),
					zap.String(constvars.LoggingErrorTypeKey, "role validation"),
				)
				utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(fmt.Errorf("invalid role: %s", role)))
				return
			}
		}
	}

	ctx, cancel = context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := ctrl.AuthUsecase.CreateMagicLink(ctx, request); err != nil {
		ctrl.Log.Error("Failed to create magic link",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
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

	// Log business event
	utils.LogBusinessEvent(ctrl.Log, "magic_link_created", requestID,
		zap.String(constvars.LoggingEmailKey, request.Email),
		zap.Strings(constvars.LoggingRolesKey, request.Roles),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.MagicLinkSuccessMessage, nil)
}

func (ctrl *AuthController) CreateAnonymousSession(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.CreateAnonymousSession requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.CreateAnonymousSession called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	sessionHandle, err := ctrl.AuthUsecase.CreateAnonymousSession(ctx)
	if err != nil {
		ctrl.Log.Error("AuthController.CreateAnonymousSession error from usecase",
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

	response := map[string]interface{}{
		"session_handle": sessionHandle,
		"role":           "guest",
	}

	ctrl.Log.Info("AuthController.CreateAnonymousSession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("session_handle", sessionHandle),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, "Anonymous session created successfully", response)
}
