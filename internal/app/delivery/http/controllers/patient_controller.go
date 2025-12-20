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

	ctrl.Log.Debug("Appointment creation started",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingEndpointKey, r.URL.Path),
		zap.String(constvars.LoggingMethodKey, r.Method),
	)

	clinicianID := chi.URLParam(r, constvars.URLParamClinicianID)
	ctrl.Log.Debug("Retrieved clinician ID from URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
	)

	request := new(requests.CreateAppointmentRequest)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("Failed to parse request body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingErrorTypeKey, "JSON parsing"),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	request.ClinicianID = clinicianID
	ctrl.Log.Debug("Request body parsed successfully",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
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

	response, err := ctrl.PatientUsecase.CreateAppointment(ctx, sessionData, request)
	if err != nil {
		ctrl.Log.Error("Failed to create appointment",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingClinicianIDKey, clinicianID),
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

	utils.LogBusinessEvent(ctrl.Log, "appointment_created", requestID,
		zap.String(constvars.LoggingClinicianIDKey, clinicianID),
		zap.String("appointment_id", response.ID),
		zap.Duration(constvars.LoggingDurationKey, time.Since(start)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreatePatientAppointmentSuccessMessage, response)
}
