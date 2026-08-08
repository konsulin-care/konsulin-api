package controllers

import (
	"errors"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/delivery/http/middlewares"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"

	"go.uber.org/zap"
)

// PurgeController exposes the erasure (purge) endpoint. It resolves the session
// identity the same way the Auth middleware does and delegates to the purge
// usecase, which owns the FHIR deletion + account-deletion ordering.
type PurgeController struct {
	Usecase     contracts.PatientDataPurger
	Middlewares *middlewares.Middlewares
	Log         *zap.Logger
}

// NewPurgeController returns a PurgeController with the purge usecase and the
// middlewares (for session identity resolution).
func NewPurgeController(usecase contracts.PatientDataPurger, mw *middlewares.Middlewares, log *zap.Logger) *PurgeController {
	return &PurgeController{
		Usecase:     usecase,
		Middlewares: mw,
		Log:         log,
	}
}

// PurgeData handles DELETE /privacy/purge. It erases all FHIR data linked to
// the session patient and, on success, deletes the associated SuperTokens
// account. Only the caller's own identity is ever purged.
func (c *PurgeController) PurgeData(w http.ResponseWriter, r *http.Request) {
	roles, _ := r.Context().Value(constvars.CONTEXT_FHIR_ROLE).([]string)
	uid, _ := r.Context().Value(constvars.CONTEXT_UID).(string)

	fhirRole, fhirID, err := c.Middlewares.ResolveUserRoles(r.Context(), roles, uid)
	if err != nil {
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(err, constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "cannot resolve session identity"))
		return
	}
	if fhirRole != constvars.KonsulinRolePatient || fhirID == "" {
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(errors.New("patient identity required"), constvars.StatusForbidden, constvars.ErrClientNotAuthorized, "purge requires a patient session"))
		return
	}

	if err := c.Usecase.PurgePatientData(r.Context(), fhirID, uid); err != nil {
		utils.BuildErrorResponse(c.Log, w, exceptions.BuildNewCustomError(err, constvars.StatusInternalServerError, constvars.ErrClientSomethingWrongWithApplication, "purge failed"))
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, "data purged successfully", nil)
}
