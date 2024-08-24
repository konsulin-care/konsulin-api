package questionnaires

import (
	"context"
	"encoding/json"
	"konsulin-service/internal/pkg/constvars"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type QuestionnaireController struct {
	Log                  *zap.Logger
	QuestionnaireUsecase QuestionnaireUsecase
}

func NewQuestionnaireController(logger *zap.Logger, questionnaireUsecase QuestionnaireUsecase) *QuestionnaireController {
	return &QuestionnaireController{
		Log:                  logger,
		QuestionnaireUsecase: questionnaireUsecase,
	}
}

func (ctrl *QuestionnaireController) CreateQuestionnaire(w http.ResponseWriter, r *http.Request) {
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

	response, err := ctrl.QuestionnaireUsecase.CreateQuestionnaire(ctx, request)
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

func (ctrl *QuestionnaireController) UpdateQuestionnaire(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(fhir_dto.Questionnaire)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaire
	request.ID = chi.URLParam(r, constvars.URLParamQuestionnaireID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.QuestionnaireUsecase.UpdateQuestionnaire(ctx, request)
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

func (ctrl *QuestionnaireController) FindQuestionnaireByID(w http.ResponseWriter, r *http.Request) {
	questionnaireID := chi.URLParam(r, constvars.URLParamQuestionnaireID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.QuestionnaireUsecase.FindQuestionnaireByID(ctx, questionnaireID)
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
func (ctrl *QuestionnaireController) DeleteQuestionnaireByID(w http.ResponseWriter, r *http.Request) {
	questionnaireID := chi.URLParam(r, constvars.URLParamQuestionnaireID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ctrl.QuestionnaireUsecase.DeleteQuestionnaireByID(ctx, questionnaireID)
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
