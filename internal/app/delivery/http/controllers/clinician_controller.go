package controllers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/services/core/clinicians"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type ClinicianController struct {
	Log              *zap.Logger
	ClinicianUsecase clinicians.ClinicianUsecase
}

func NewClinicianController(logger *zap.Logger, clinicianUsecase clinicians.ClinicianUsecase) *ClinicianController {
	return &ClinicianController{
		Log:              logger,
		ClinicianUsecase: clinicianUsecase,
	}
}

func (ctrl *ClinicianController) CreatePracticeInformation(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.CreatePracticeInformation)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.ClinicianUsecase.CreatePracticeInformation(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateClinicianClinicsSuccessMessage, response)
}

func (ctrl *ClinicianController) FindClinicsByClinicianID(w http.ResponseWriter, r *http.Request) {
	request := &requests.FindClinicianByClinicianID{
		PractitionerID:   chi.URLParam(r, constvars.URLParamClinicianID),
		OrganizationName: r.URL.Query().Get("name"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicianUsecase.FindClinicsByClinicianID(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, result)
}

func (ctrl *ClinicianController) FindAvailability(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.FindAvailability)

	request.Year = r.URL.Query().Get("year")
	request.Month = r.URL.Query().Get("month")
	request.PractitionerRoleID = r.URL.Query().Get("practitioner_role_id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicianUsecase.FindAvailability(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetClinicsSuccessfully, result)
}

func (ctrl *ClinicianController) CreatePracticeAvailability(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.CreatePracticeAvailability)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.ClinicianUsecase.CreatePracticeAvailability(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateClinicianPracticeAvailabilitySuccessMessage, response)
}

func (ctrl *ClinicianController) DeleteClinicByID(w http.ResponseWriter, r *http.Request) {
	clinicID := chi.URLParam(r, constvars.URLParamClinicID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	err := ctrl.ClinicianUsecase.DeleteClinicByID(ctx, sessionData, clinicID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteClinicianClinicSuccessMessage, nil)
}
