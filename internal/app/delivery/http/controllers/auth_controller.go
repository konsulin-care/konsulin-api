package controllers

import (
	"context"
	"errors"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/supertokens/supertokens-golang/recipe/session"
	"github.com/supertokens/supertokens-golang/recipe/session/sessmodels"
	"go.uber.org/zap"
)

type AuthController struct {
	Log            *zap.Logger
	AuthUsecase    contracts.AuthUsecase
	InternalConfig *config.InternalConfig
}

var (
	authControllerInstance *AuthController
	onceAuthController     sync.Once
)

func NewAuthController(logger *zap.Logger, authUsecase contracts.AuthUsecase, internalConfig *config.InternalConfig) *AuthController {
	onceAuthController.Do(func() {
		instance := &AuthController{
			Log:            logger,
			AuthUsecase:    authUsecase,
			InternalConfig: internalConfig,
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

	if request.RedirectToPath != "" {
		if err := utils.ValidateRedirectPath(request.RedirectToPath); err != nil {
			ctrl.Log.Warn("magic link creation rejected: invalid redirectToPath",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String("redirect_to_path", request.RedirectToPath),
				zap.Error(err),
			)
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
			return
		}
		ctrl.Log.Info("magic link redirect path requested",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("redirect_to_path", request.RedirectToPath),
		)
	}

	// Enforce mutually-exclusive email or phone (exactly one must be set).
	hasEmail := strings.TrimSpace(request.Email) != ""
	phoneDigits := ""
	if strings.TrimSpace(request.Phone) != "" {
		// Normalize phone to digits-only format (remove all non-digit characters).
		phoneDigits = utils.NormalizePhoneDigits(request.Phone)
	}
	hasPhone := strings.TrimSpace(phoneDigits) != ""
	if hasEmail && hasPhone {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(fmt.Errorf("email and phone are mutually exclusive")))
		return
	}
	if !hasEmail && !hasPhone {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(fmt.Errorf("either email or phone is required")))
		return
	}
	if hasPhone {
		if err := utils.ValidateInternationalPhoneDigits(phoneDigits); err != nil {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
			return
		}
	}

	// Persist normalized phone for downstream use (repo expects digits-only without '+').
	request.Phone = phoneDigits

	// Struct tag validation (email format, roles).
	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("Request validation failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String("phone", request.Phone),
			zap.String(constvars.LoggingErrorTypeKey, "input validation"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	// Check if user exists to determine if roles are required
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var userExistsOutput *contracts.CheckUserExistsOutput
	var err error
	if hasEmail {
		userExistsOutput, err = ctrl.AuthUsecase.CheckUserExists(ctx, request.Email)
	} else {
		userExistsOutput, err = ctrl.AuthUsecase.CheckUserExistsByPhone(ctx, request.Phone)
	}
	if err != nil {
		ctrl.Log.Error("AuthController.MagicLink error checking user existence",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String("phone", request.Phone),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	userExists := userExistsOutput != nil && userExistsOutput.SupertokenUser != nil

	// If user doesn't exist, roles are mandatory
	if !userExists && (len(request.Roles) == 0) {
		ctrl.Log.Error("AuthController.MagicLink roles required for new user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, request.Email),
			zap.String("phone", request.Phone),
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
			zap.String("phone", request.Phone),
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
		zap.String("phone", request.Phone),
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

	existingToken := ""
	if cookie, err := r.Cookie(constvars.AnonymousSessionCookieName); err == nil {
		existingToken = cookie.Value
	}

	forceNew := r.URL.Query().Has(constvars.AnonymousSessionForceNewQueryKey)

	result, err := ctrl.AuthUsecase.CreateAnonymousSession(ctx, existingToken, forceNew)
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

	if result != nil && result.IsNew {
		// Secure flag defaults to true for security. Only disable for local development
		// where HTTPS may not be available. In production/staging, cookies must be secure.
		secure := true
		if strings.EqualFold(ctrl.InternalConfig.App.Env, "local") {
			secure = false
		}
		// cookie here is intended to be saved as a session cookie.
		http.SetCookie(w, &http.Cookie{
			Name:     constvars.AnonymousSessionCookieName,
			Value:    result.Token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
		})
	}

	response := map[string]interface{}{
		"token":    result.Token,
		"guest_id": result.GuestID,
		"is_new":   result.IsNew,
		"role":     "guest",
	}

	ctrl.Log.Info("AuthController.CreateAnonymousSession succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("guest_id", result.GuestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, "Anonymous session created successfully", response)
}

func (ctrl *AuthController) ClaimAnonymousResources(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.ClaimAnonymousResources requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AuthController.ClaimAnonymousResources called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	// Read identity and roles from context (set by SessionOptional middleware)
	ctxIface := r.Context()
	supertokensUserID, _ := ctxIface.Value(constvars.CONTEXT_UID).(string)
	roles, _ := ctxIface.Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	if strings.TrimSpace(supertokensUserID) == "" || supertokensUserID == "anonymous" || len(roles) == 0 {
		ctrl.Log.Error("AuthController.ClaimAnonymousResources missing or anonymous user in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrSupertokensSessionMissing(nil))
		return
	}

	anonToken := ""
	if cookie, err := r.Cookie(constvars.AnonymousSessionCookieName); err == nil {
		anonToken = cookie.Value
	}

	if strings.TrimSpace(anonToken) == "" {
		w.WriteHeader(constvars.StatusNoContent)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	result, err := ctrl.AuthUsecase.ClaimAnonymousResources(ctx, supertokensUserID, roles, anonToken)
	if err != nil {
		ctrl.Log.Error("AuthController.ClaimAnonymousResources error from usecase",
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

	// Secure flag defaults to true for security. Only disable for local development
	// where HTTPS may not be available. In production/staging, cookies must be secure.
	secure := true
	if strings.EqualFold(ctrl.InternalConfig.App.Env, "local") {
		secure = false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     constvars.AnonymousSessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	utils.BuildSuccessResponse(w, constvars.StatusOK, "Anonymous resources claimed successfully", map[string]interface{}{
		"claimed":       true,
		"count":         result.Count,
		"referenceList": result.ReferenceList,
	})
}

// setRoleBody holds the role value from the request body.
type setRoleBody struct {
	Role string `json:"role"`
}

// parseSetRoleBody decodes and validates the role from the request body.
func (ctrl *AuthController) parseSetRoleBody(r *http.Request) (string, error) {
	var body setRoleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to parse body: %w", err)
	}
	if body.Role == "" {
		return "", fmt.Errorf("role is required")
	}
	return body.Role, nil
}

// validateUserRole checks that the requested role is in the user's assigned roles.
func validateUserRole(role string, userRoles []string) error {
	if !slices.Contains(userRoles, role) {
		return fmt.Errorf("role %s is not assigned to user", role)
	}
	return nil
}

// SetActiveRole sets the active role in the SuperTokens session's access token
// payload so downstream middleware reads it instead of iterating all roles.
func (ctrl *AuthController) SetActiveRole(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.SetActiveRole requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	role, err := ctrl.parseSetRoleBody(r)
	if err != nil {
		ctrl.Log.Error("AuthController.SetActiveRole failed to parse body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	sessRequired := true
	sess, err := session.GetSession(r, w, &sessmodels.VerifySessionOptions{SessionRequired: &sessRequired})
	if err != nil {
		ctrl.Log.Error("AuthController.SetActiveRole session not found",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrSupertokensSessionMissing(err))
		return
	}

	userRoles, roleErr := getUserRolesFromSession(sess)
	if roleErr != nil {
		ctrl.Log.Error("AuthController.SetActiveRole failed to parse user roles",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(roleErr),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(fmt.Errorf("could not verify user roles")))
		return
	}
	if err := validateUserRole(role, userRoles); err != nil {
		ctrl.Log.Warn("AuthController.SetActiveRole role not assigned",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String("role", role),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	if err := sess.MergeIntoAccessTokenPayload(map[string]interface{}{
		constvars.SupertokenPayloadActiveRoleKey: role,
	}); err != nil {
		ctrl.Log.Error("AuthController.SetActiveRole merge payload failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.BuildNewCustomError(err, http.StatusInternalServerError, "internal server error", "merge into access token payload failed"))
		return
	}

	ctrl.Log.Info("AuthController.SetActiveRole succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("role", role),
	)
	utils.BuildSuccessResponse(w, http.StatusOK, "active role set", nil)
}

// getUserRolesFromSession extracts the list of role strings from the SuperTokens
// access token payload. Returns an empty slice if no roles are found.
func getUserRolesFromSession(sess sessmodels.SessionContainer) ([]string, error) {
	raw := sess.GetAccessTokenPayload()
	if raw == nil {
		return nil, errors.New("empty access token payload")
	}
	rolesData, exists := raw[constvars.SupertokenPayloadRolesKey]
	if !exists {
		return nil, nil
	}
	rolesMap, ok := rolesData.(map[string]interface{})
	if !ok {
		return nil, errors.New("roles not a map")
	}
	rolesValue, ok := rolesMap[constvars.SupertokenPayloadRolesValueKey]
	if !ok {
		return nil, nil
	}
	rolesList, ok := rolesValue.([]interface{})
	if !ok {
		return nil, errors.New("roles value not a list")
	}
	result := make([]string, 0, len(rolesList))
	for _, item := range rolesList {
		if role, ok := item.(string); ok {
			result = append(result, role)
		}
	}
	return result, nil
}

// PasswordlessEmailExists exposes the SuperTokens passwordless email lookup
// endpoint so we can extend it with custom logic later.
func (ctrl *AuthController) PasswordlessEmailExists(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AuthController.PasswordlessEmailExists requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	email := strings.TrimSpace(r.URL.Query().Get("email"))
	if email == "" {
		ctrl.Log.Error("AuthController.PasswordlessEmailExists missing email query",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(fmt.Errorf("email is required")))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	output, err := ctrl.AuthUsecase.CheckUserExists(ctx, email)
	if err != nil {
		ctrl.Log.Error("AuthController.PasswordlessEmailExists error checking user",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingEmailKey, email),
			zap.Error(err),
		)
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	exists := output != nil && output.SupertokenUser != nil

	patientIds := []string{}
	practitionerIds := []string{}

	if output != nil {
		patientIds = output.PatientIds
		practitionerIds = output.PractitionerIds
	}

	response := map[string]interface{}{
		"exists":          exists,
		"status":          "OK",
		"patientIds":      patientIds,
		"practitionerIds": practitionerIds,
	}

	ctrl.Log.Info("AuthController.PasswordlessEmailExists succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEmailKey, email),
		zap.Bool("exists", exists),
	)

	w.Header().Set(constvars.HeaderContentType, constvars.MIMEApplicationJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
