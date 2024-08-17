package clinicians

import (
	"context"
	"encoding/json"
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
	ClinicianUsecase ClinicianUsecase
}

func NewClinicianController(logger *zap.Logger, clinicianUsecase ClinicianUsecase) *ClinicianController {
	return &ClinicianController{
		Log:              logger,
		ClinicianUsecase: clinicianUsecase,
	}
}

func (ctrl *ClinicianController) CreateClinics(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.ClinicianCreateClinics)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = ctrl.ClinicianUsecase.CreateClinics(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateClinicianClinicsSuccessMessage, nil)
}

func (ctrl *ClinicianController) FindClinicsByClinicianID(w http.ResponseWriter, r *http.Request) {
	clinicianID := chi.URLParam(r, constvars.URLParamClinicianID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := ctrl.ClinicianUsecase.FindClinicsByClinicianID(ctx, clinicianID)
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

func (ctrl *ClinicianController) CreateClinicsAvailability(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.CreateClinicsAvailability)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = ctrl.ClinicianUsecase.CreateClinicsAvailability(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateClinicianClinicsSuccessMessage, nil)
}

func (ctrl *ClinicianController) DeleteClinicByID(w http.ResponseWriter, r *http.Request) {
	clinicID := chi.URLParam(r, constvars.URLParamClinicID)

	// err := utils.ValidateUrlParamID(clinicID)
	// if err != nil {
	// 	utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrURLParamIDValidation(err, constvars.URLParamPractitionerID))
	// 	return
	// }

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

func (ctrl *ClinicianController) CreateAvailibilityTime(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.AvailableTime)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = ctrl.ClinicianUsecase.CreateAvailibilityTime(ctx, sessionData, request)
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
