package assessmentResponses

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type AssessmentResponseController struct {
	Log                       *zap.Logger
	AssessmentResponseUsecase AssessmentResponseUsecase
}

func NewAssessmentResponseController(logger *zap.Logger, assessmentResponseUsecase AssessmentResponseUsecase) *AssessmentResponseController {
	return &AssessmentResponseController{
		Log:                       logger,
		AssessmentResponseUsecase: assessmentResponseUsecase,
	}
}

func (ctrl *AssessmentResponseController) CreateAssesmentResponse(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(requests.CreateAssesmentResponse)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.QuestionnaireResponse.ResourceType = constvars.ResourceQuestionnaireResponse
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.CreateAssessmentResponse(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateQuestionnaireResponseSuccessMessage, response)
}

func (ctrl *AssessmentResponseController) UpdateAssessmentResponse(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(fhir_dto.QuestionnaireResponse)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaireResponse
	request.ID = chi.URLParam(r, constvars.URLParamAssessmentResponseID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.UpdateAssessmentResponse(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateQuestionnaireResponseSuccessMessage, response)
}

func (ctrl *AssessmentResponseController) FindQuestionnaireResponseByID(w http.ResponseWriter, r *http.Request) {
	questionnaireResponseID := chi.URLParam(r, constvars.URLParamAssessmentResponseID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentResponseUsecase.FindAssessmentResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.FindQuestionnaireResponseSuccessMessage, response)
}
func (ctrl *AssessmentResponseController) DeleteQuestionnaireResponseByID(w http.ResponseWriter, r *http.Request) {
	questionnaireResponseID := chi.URLParam(r, constvars.URLParamAssessmentResponseID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ctrl.AssessmentResponseUsecase.DeleteAssessmentResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteQuestionnaireResponseSuccessMessage, nil)
}
