package patients

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

type PatientController struct {
	Log            *zap.Logger
	PatientUsecase PatientUsecase
}

func NewPatientController(logger *zap.Logger, patientUsecase PatientUsecase) *PatientController {
	return &PatientController{
		Log:            logger,
		PatientUsecase: patientUsecase,
	}
}

func (ctrl *PatientController) CreateAppointment(w http.ResponseWriter, r *http.Request) {
	clinicianID := chi.URLParam(r, constvars.URLParamClinicianID)

	// Bind body to request
	request := new(requests.CreateAppointmentRequest)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}
	request.ClinicianID = clinicianID

	// Get session data from context
	sessionData := r.Context().Value("sessionData").(string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.PatientUsecase.CreateAppointment(ctx, sessionData, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreatePatientAppointmentSuccessMessage, response)
}
