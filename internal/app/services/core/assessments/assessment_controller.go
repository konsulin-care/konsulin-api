package assessments

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type AssessmentController struct {
	Log               *zap.Logger
	AssessmentUsecase AssessmentUsecase
}

func NewAssessmentController(logger *zap.Logger, assessmentUsecase AssessmentUsecase) *AssessmentController {
	return &AssessmentController{
		Log:               logger,
		AssessmentUsecase: assessmentUsecase,
	}
}

func (ctrl *AssessmentController) CreateAssessment(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(fhir_dto.Questionnaire)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaire

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentUsecase.CreateAssessment(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.CreateQuestionnaireSuccessMessage, response)
}

func (ctrl *AssessmentController) UpdateAssessment(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(fhir_dto.Questionnaire)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaire
	request.ID = chi.URLParam(r, constvars.URLParamAssessmentID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentUsecase.UpdateAssessment(ctx, request)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.UpdateQuestionnaireSuccessMessage, response)
}

func (ctrl *AssessmentController) FindAssessmentByID(w http.ResponseWriter, r *http.Request) {
	questionnaireID := chi.URLParam(r, constvars.URLParamAssessmentID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.AssessmentUsecase.FindAssessmentByID(ctx, questionnaireID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.FindQuestionnaireSuccessMessage, response)
}
func (ctrl *AssessmentController) DeleteAssessmentByID(w http.ResponseWriter, r *http.Request) {
	questionnaireID := chi.URLParam(r, constvars.URLParamAssessmentID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ctrl.AssessmentUsecase.DeleteAssessmentByID(ctx, questionnaireID)
	if err != nil {
		if err == context.DeadlineExceeded {
			utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrServerDeadlineExceeded(err))
			return
		}
		utils.BuildErrorResponse(ctrl.Log, w, err)
		return
	}

	utils.BuildSuccessResponse(w, constvars.StatusOK, constvars.DeleteQuestionnaireSuccessMessage, nil)
}
