package questionnaireResponses

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

type QuestionnaireResponseController struct {
	Log                          *zap.Logger
	QuestionnaireResponseUsecase QuestionnaireResponseUsecase
}

func NewQuestionnaireResponseController(logger *zap.Logger, questionnaireResponseUsecase QuestionnaireResponseUsecase) *QuestionnaireResponseController {
	return &QuestionnaireResponseController{
		Log:                          logger,
		QuestionnaireResponseUsecase: questionnaireResponseUsecase,
	}
}

func (ctrl *QuestionnaireResponseController) CreateQuestionnaireResponse(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(fhir_dto.QuestionnaireResponse)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaireResponse

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.QuestionnaireResponseUsecase.CreateQuestionnaireResponse(ctx, request)
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

func (ctrl *QuestionnaireResponseController) UpdateQuestionnaireResponse(w http.ResponseWriter, r *http.Request) {
	// Bind body to request
	request := new(fhir_dto.QuestionnaireResponse)
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		utils.BuildErrorResponse(ctrl.Log, w, exceptions.ErrCannotParseJSON(err))
		return
	}

	request.ResourceType = constvars.ResourceQuestionnaireResponse
	request.ID = chi.URLParam(r, constvars.URLParamQuestionnaireResponseID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.QuestionnaireResponseUsecase.UpdateQuestionnaireResponse(ctx, request)
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

func (ctrl *QuestionnaireResponseController) FindQuestionnaireResponseByID(w http.ResponseWriter, r *http.Request) {
	questionnaireResponseID := chi.URLParam(r, constvars.URLParamQuestionnaireResponseID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := ctrl.QuestionnaireResponseUsecase.FindQuestionnaireResponseByID(ctx, questionnaireResponseID)
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
func (ctrl *QuestionnaireResponseController) DeleteQuestionnaireResponseByID(w http.ResponseWriter, r *http.Request) {
	questionnaireResponseID := chi.URLParam(r, constvars.URLParamQuestionnaireResponseID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := ctrl.QuestionnaireResponseUsecase.DeleteQuestionnaireResponseByID(ctx, questionnaireResponseID)
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
