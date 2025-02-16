package controllers

import (
	"context"
	"encoding/json"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type AssessmentController struct {
	Log               *zap.Logger
	AssessmentUsecase contracts.AssessmentUsecase
}

var (
	assessmentControllerInstance *AssessmentController
	onceAssessmentController     sync.Once
)

func NewAssessmentController(logger *zap.Logger, assessmentUsecase contracts.AssessmentUsecase) *AssessmentController {
	onceAssessmentController.Do(func() {
		instance := &AssessmentController{
			Log:               logger,
			AssessmentUsecase: assessmentUsecase,
		}
		assessmentControllerInstance = instance
	})
	return assessmentControllerInstance
}

func (ctrl *AssessmentController) FindAll(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AssessmentController.FindAll requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	sessionData, _ := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	ctrl.Log.Info("AssessmentController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	request := &requests.FindAllAssessment{
		AssessmentType: r.URL.Query().Get("assessment_type"),
	}

	if err := utils.ValidateStruct(request); err != nil {
		ctrl.Log.Error("AssessmentController.FindAll validation error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrInputValidation(err))
		return
	}

	response, err := ctrl.AssessmentUsecase.FindAll(ctx, request, sessionData)
	if err != nil {
		ctrl.Log.Error("AssessmentController.FindAll error in AssessmentUsecase.FindAll",
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

	ctrl.Log.Info("AssessmentController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(response)),
	)

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetAssessmentsSuccessMessage, response)
}

func (ctrl *AssessmentController) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AssessmentController.CreateAssessment requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	ctrl.Log.Info("AssessmentController.CreateAssessment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ctrl.Log.Error("AssessmentController.CreateAssessment error reading request body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	data, err := utils.ParseJSONBody(body)
	if err != nil {
		ctrl.Log.Error("AssessmentController.CreateAssessment error parsing JSON body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentUsecase.CreateAssessment(ctx, data)
	if err != nil {
		ctrl.Log.Error("AssessmentController.CreateAssessment error in AssessmentUsecase.CreateAssessment",
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

	ctrl.Log.Info("AssessmentController.CreateAssessment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateAssessmentSuccessMessage, response)
}

func (ctrl *AssessmentController) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AssessmentController.UpdateAssessment requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	ctrl.Log.Info("AssessmentController.UpdateAssessment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(fhir_dto.Questionnaire)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		ctrl.Log.Error("AssessmentController.UpdateAssessment error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaire
	request.ID = chi.URLParam(r, constvars.URLParamAssessmentID)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentUsecase.UpdateAssessment(ctx, request)
	if err != nil {
		ctrl.Log.Error("AssessmentController.UpdateAssessment error in AssessmentUsecase.UpdateAssessment",
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

	ctrl.Log.Info("AssessmentController.UpdateAssessment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateAssessmentSuccessMessage, response)
}

func (ctrl *AssessmentController) FindAssessmentByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AssessmentController.FindAssessmentByID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	questionnaireID := chi.URLParam(r, constvars.URLParamAssessmentID)
	ctrl.Log.Info("AssessmentController.FindAssessmentByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("questionnaire_id", questionnaireID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentUsecase.FindAssessmentByID(ctx, questionnaireID)
	if err != nil {
		ctrl.Log.Error("AssessmentController.FindAssessmentByID error in AssessmentUsecase.FindAssessmentByID",
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

	ctrl.Log.Info("AssessmentController.FindAssessmentByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingResponseKey, response),
	)

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.FindAssessmentSuccessMessage, response)
}

func (ctrl *AssessmentController) DeleteAssessmentByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok {
		ctrl.Log.Error("AssessmentController.DeleteAssessmentByID requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}

	questionnaireID := chi.URLParam(r, constvars.URLParamAssessmentID)
	ctrl.Log.Info("AssessmentController.DeleteAssessmentByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("questionnaire_id", questionnaireID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.AssessmentUsecase.DeleteAssessmentByID(ctx, questionnaireID)
	if err != nil {
		ctrl.Log.Error("AssessmentController.DeleteAssessmentByID error in AssessmentUsecase.DeleteAssessmentByID",
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

	ctrl.Log.Info("AssessmentController.DeleteAssessmentByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteAssessmentSuccessMessage, nil)
}
