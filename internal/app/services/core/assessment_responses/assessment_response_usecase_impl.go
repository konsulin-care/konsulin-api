package assessmentResponses

import (
	"context"
	"konsulin-service/internal/app/config"
	questionnaireResponses "konsulin-service/internal/app/services/fhir_spark/questionnaires_responses"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
	"time"

	"github.com/google/uuid"
)

type assessmentResponseUsecase struct {
	QuestionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient
	RedisRepository                 redis.RedisRepository
	InternalConfig                  *config.InternalConfig
}

func NewAssessmentResponseUsecase(
	questionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient,
	redisRepository redis.RedisRepository,
	internalConfig *config.InternalConfig,
) AssessmentResponseUsecase {
	return &assessmentResponseUsecase{
		QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
		RedisRepository:                 redisRepository,
		InternalConfig:                  internalConfig,
	}
}

func (uc *assessmentResponseUsecase) CreateAssessmentResponse(ctx context.Context, request *requests.CreateAssesmentResponse) (*responses.CreateAssessmentResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.CreateQuestionnaireResponse(ctx, request.QuestionnaireResponse)
	if err != nil {
		return nil, err
	}

	response := &responses.CreateAssessmentResponse{
		QuestionnaireResponse: questionnaireResponse,
	}
	if request.RespondentType == constvars.RespondentTypeGuest {
		responseID := uuid.New().String()
		responseExpiryTime := time.Minute * time.Duration(uc.InternalConfig.App.QuestionnaireGuestResponseExpiredTimeInMinutes)
		err := uc.RedisRepository.Set(ctx, responseID, questionnaireResponse.ID, responseExpiryTime)
		if err != nil {
			return nil, err
		}
		response.ResponseID = responseID
	}

	return response, nil
}
func (uc *assessmentResponseUsecase) UpdateAssessmentResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.UpdateQuestionnaireResponse(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}

func (uc *assessmentResponseUsecase) FindAssessmentResponseByID(ctx context.Context, assessmentResponseID string) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, assessmentResponseID)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}
func (uc *assessmentResponseUsecase) DeleteAssessmentResponseByID(ctx context.Context, assessmentResponseID string) error {
	err := uc.QuestionnaireResponseFhirClient.DeleteQuestionnaireResponseByID(ctx, assessmentResponseID)
	if err != nil {
		return err
	}

	return nil
}
