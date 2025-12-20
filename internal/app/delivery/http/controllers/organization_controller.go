package controllers

import (
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type OrganizationController struct {
	Log     *zap.Logger
	Usecase contracts.OrganizationUsecase
}

var (
	organizationControllerInstance *OrganizationController
	onceOrganizationController     sync.Once
)

func NewOrganizationController(logger *zap.Logger, uc contracts.OrganizationUsecase) *OrganizationController {
	onceOrganizationController.Do(func() {
		organizationControllerInstance = &OrganizationController{
			Log:     logger,
			Usecase: uc,
		}
	})
	return organizationControllerInstance
}

type registerPractitionerRoleRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func (r *registerPractitionerRoleRequest) validate() error {
	if strings.TrimSpace(r.Email) == "" {
		return exceptions.BuildNewCustomError(nil, constvars.StatusBadRequest, constvars.ErrClientCannotProcessRequest, "email is required")
	}
	return utils.ValidateStruct(r)
}

func (ctrl *OrganizationController) RegisterPractitionerRole(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("OrganizationController.RegisterPractitionerRole requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	orgID := chi.URLParam(r, "organizationId")
	if strings.TrimSpace(orgID) == "" {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrURLParamIDValidation(nil, "organizationId"))
		return
	}

	var req registerPractitionerRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctrl.Log.Error("OrganizationController.RegisterPractitionerRole error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	if err := req.validate(); err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	out, err := ctrl.Usecase.RegisterPractitionerRoleAndSchedule(r.Context(), contracts.RegisterPractitionerRoleInput{
		OrganizationID: orgID,
		Email:          req.Email,
	})
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	payload := map[string]any{
		"practitionerId":     out.PractitionerID,
		"practitionerRoleId": out.PractitionerRoleID,
		"scheduleId":         out.ScheduleID,
	}

	utils.BuildSuccessResponse(w, constvars.StatusCreated, "Created", payload)
}
