package controllers

import (
	"context"
	"encoding/json"
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

type AssessmentResponseController struct {
	Log                       *zap.Logger
	AssessmentResponseUsecase contracts.AssessmentResponseUsecase
}

var (
	assessmentResponseControllerInstance *AssessmentResponseController
	onceAssessmentResponseController     sync.Once
)

func NewAssessmentResponseController(logger *zap.Logger, assessmentResponseUsecase contracts.AssessmentResponseUsecase) *AssessmentResponseController {
	onceAssessmentResponseController.Do(func() {
		instance := &AssessmentResponseController{
			Log:                       logger,
			AssessmentResponseUsecase: assessmentResponseUsecase,
		}
		assessmentResponseControllerInstance = instance
	})
	return assessmentResponseControllerInstance
}

func (ctrl *AssessmentResponseController) CreateAssesmentResponse(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AssessmentResponseController.CreateAssessmentResponse requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	sessionData, _ := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	ctrl.Log.Info("AssessmentResponseController.CreateAssessmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(requests.CreateAssesmentResponse)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AssessmentResponseController.CreateAssessmentResponse error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.QuestionnaireResponse["resourceType"] = constvars.ResourceQuestionnaireResponse
	request.SessionData = sessionData

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.CreateAssessmentResponse(ctx, request)
	if err != nil {
		ctrl.Log.Error("AssessmentResponseController.CreateAssessmentResponse error from usecase",
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

	ctrl.Log.Info("AssessmentResponseController.CreateAssessmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingPaymentResponseKey, response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateAssessmentResponseSuccessMessage, response)
}

func (ctrl *AssessmentResponseController) UpdateAssessmentResponse(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AssessmentResponseController.UpdateAssessmentResponse error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AssessmentResponseController.UpdateAssessmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	request := new(fhir_dto.QuestionnaireResponse)
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		ctrl.Log.Error("AssessmentResponseController.UpdateAssessmentResponse error decoding JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaireResponse
	request.ID = chi.URLParam(r, constvars.URLParamAssessmentResponseID)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.UpdateAssessmentResponse(ctx, request)
	if err != nil {
		ctrl.Log.Error("AssessmentResponseController.UpdateAssessmentResponse error from usecase",
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

	ctrl.Log.Info("AssessmentResponseController.UpdateAssessmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingPaymentResponseKey, response),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateAssessmentResponseSuccessMessage, response)
}

func (ctrl *AssessmentResponseController) FindAll(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AssessmentResponseController.FindAll error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	ctrl.Log.Info("AssessmentResponseController.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	sessionData, ok := r.Context().Value(constvars.CONTEXT_SESSION_DATA_KEY).(string)
	if !ok || sessionData == "" {
		ctrl.Log.Error("AssessmentResponseController.FindAll error: sessionData not found in context",
			zap.String(constvars.LoggingRequestIDKey, requestID),
		)
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingSessionData(nil))
		return
	}

	request := &requests.FindAllAssessmentResponse{
		SessionData:  sessionData,
		AssessmentID: r.URL.Query().Get(constvars.URLQueryParamAssessmentID),
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.FindAll(ctx, request)
	if err != nil {
		ctrl.Log.Error("AssessmentResponseController.FindAll error from usecase",
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

	ctrl.Log.Info("AssessmentResponseController.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(response)),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.GetAssessmentsSuccessMessage, response)
}

func (ctrl *AssessmentResponseController) FindQuestionnaireResponseByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AssessmentResponseController.FindQuestionnaireResponseByID error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	questionnaireResponseID := chi.URLParam(r, constvars.URLParamAssessmentResponseID)
	ctrl.Log.Info("AssessmentResponseController.FindQuestionnaireResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.FindAssessmentResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		ctrl.Log.Error("AssessmentResponseController.FindQuestionnaireResponseByID error from usecase",
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

	ctrl.Log.Info("AssessmentResponseController.FindQuestionnaireResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.FindAssessmentResponseSuccessMessage, response)
}

func (ctrl *AssessmentResponseController) DeleteQuestionnaireResponseByID(w http.ResponseWriter, r *http.Request) {
	requestID, ok := r.Context().Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	if !ok || requestID == "" {
		ctrl.Log.Error("AssessmentResponseController.DeleteQuestionnaireResponseByID error: requestID not found in context")
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrMissingRequestID(nil))
		return
	}
	questionnaireResponseID := chi.URLParam(r, constvars.URLParamAssessmentResponseID)
	ctrl.Log.Info("AssessmentResponseController.DeleteQuestionnaireResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := ctrl.AssessmentResponseUsecase.DeleteAssessmentResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		ctrl.Log.Error("AssessmentResponseController.DeleteQuestionnaireResponseByID error from usecase",
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

	ctrl.Log.Info("AssessmentResponseController.DeleteQuestionnaireResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)
	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteAssessmentResponseSuccessMessage, nil)
}
