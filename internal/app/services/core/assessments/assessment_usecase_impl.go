package assessments

import (
	"context"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
	"sync"

	"go.uber.org/zap"
)

type assessmentUsecase struct {
	QuestionnaireFhirClient contracts.QuestionnaireFhirClient
	Log                     *zap.Logger
}

var (
	assessmentUsecaseInstance contracts.AssessmentUsecase
	onceAssessmentUsecase     sync.Once
)

func NewAssessmentUsecase(
	questionnaireFhirClient contracts.QuestionnaireFhirClient,
	logger *zap.Logger,
) contracts.AssessmentUsecase {
	onceAssessmentUsecase.Do(func() {
		instance := &assessmentUsecase{
			QuestionnaireFhirClient: questionnaireFhirClient,
			Log:                     logger,
		}
		assessmentUsecaseInstance = instance
	})
	return assessmentUsecaseInstance
}

func (uc *assessmentUsecase) FindAll(ctx context.Context) ([]responses.Assessment, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	questionnaires, err := uc.QuestionnaireFhirClient.FindQuestionnaires(ctx)
	if err != nil {
		uc.Log.Error("assessmentUsecase.FindAll error fetching questionnaires",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("assessmentUsecase.FindAll fetched questionnaires",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingQuestionnaireCountKey, len(questionnaires)),
	)

	assessments := uc.mapFHIRQuestionnaireToAssessment(ctx, questionnaires)
	uc.Log.Info("assessmentUsecase.FindAll mapped assessments",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingAssessmentCountKey, len(assessments)),
	)

	return assessments, nil
}

func (uc *assessmentUsecase) CreateAssessment(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentUsecase.CreateAssessment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	questionnaire, err := uc.QuestionnaireFhirClient.CreateQuestionnaire(ctx, request)
	if err != nil {
		uc.Log.Error("assessmentUsecase.CreateAssessment error creating questionnaire",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("assessmentUsecase.CreateAssessment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return questionnaire, nil
}

func (uc *assessmentUsecase) UpdateAssessment(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentUsecase.UpdateAssessment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	questionnaire, err := uc.QuestionnaireFhirClient.UpdateQuestionnaire(ctx, request)
	if err != nil {
		uc.Log.Error("assessmentUsecase.UpdateAssessment error updating questionnaire",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("assessmentUsecase.UpdateAssessment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return questionnaire, nil
}

func (uc *assessmentUsecase) FindAssessmentByID(ctx context.Context, questionnaireID string) (map[string]interface{}, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentUsecase.FindAssessmentByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)

	questionnaire, err := uc.QuestionnaireFhirClient.FindRawQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		uc.Log.Error("assessmentUsecase.FindAssessmentByID error fetching questionnaire",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("assessmentUsecase.FindAssessmentByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)
	return questionnaire, nil
}

func (uc *assessmentUsecase) DeleteAssessmentByID(ctx context.Context, questionnaireID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentUsecase.DeleteAssessmentByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)

	err := uc.QuestionnaireFhirClient.DeleteQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		uc.Log.Error("assessmentUsecase.DeleteAssessmentByID error deleting questionnaire",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("assessmentUsecase.DeleteAssessmentByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)
	return nil
}

func (uc *assessmentUsecase) mapFHIRQuestionnaireToAssessment(ctx context.Context, questionnaires []fhir_dto.Questionnaire) []responses.Assessment {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentUsecase.mapFHIRQuestionnaireToAssessment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	assessments := make([]responses.Assessment, 0, len(questionnaires))
	for _, eachQuestionnaire := range questionnaires {
		assessment := responses.Assessment{
			AssessmentID: eachQuestionnaire.ID,
			Title:        eachQuestionnaire.Title,
		}
		assessments = append(assessments, assessment)
	}
	uc.Log.Info("assessmentUsecase.mapFHIRQuestionnaireToAssessment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingAssessmentCountKey, len(assessments)),
	)
	return assessments
}
