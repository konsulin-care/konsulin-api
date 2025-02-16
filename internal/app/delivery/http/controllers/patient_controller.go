package controllers

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type PatientController struct {
	Log            *zap.Logger
	PatientUsecase contracts.PatientUsecase
}

var (
	patientControllerInstance *PatientController
	oncePatientController     sync.Once
)

func NewPatientController(logger *zap.Logger, patientUsecase contracts.PatientUsecase) *PatientController {
	oncePatientController.Do(func() {
		instance := &PatientController{
			Log:            logger,
			PatientUsecase: patientUsecase,
		}
		patientControllerInstance = instance
	})
	return patientControllerInstance
}
func (ctrl *PatientController) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("PatientController.CreateAppointment requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("PatientController.CreateAppointment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	clinicianID := chi.URLParam(r, constvars.URLParamClinicianID)
	ctrl.Log.Info("PatientController.CreateAppointment retrieved clinicianID",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
	)

	request := new(requests.CreateAppointmentRequest)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("PatientController.CreateAppointment error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	request.ClinicianID = clinicianID
	ctrl.Log.Info("PatientController.CreateAppointment request decoded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("PatientController.CreateAppointment error: sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.PatientUsecase.CreateAppointment(ctx, sessionData, request)
	if err != nil {
		ctrl.Log.Error("PatientController.CreateAppointment error from usecase",
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

	ctrl.Log.Info("PatientController.CreateAppointment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreatePatientAppointmentSuccessMessage, response)
}
