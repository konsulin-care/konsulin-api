package questionnaireResponses

import (
	"context"
	"konsulin-service/internal/app/config"
	questionnaireResponses "konsulin-service/internal/app/services/fhir_spark/questionnaires_responses"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	fhir_dto "konsulin-service/internal/pkg/dto/fhir"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"time"

	"github.com/google/uuid"
)

type questionnaireResponseUsecase struct {
	QuestionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient
	RedisRepository                 redis.RedisRepository
	InternalConfig                  *config.InternalConfig
}

func NewQuestionnaireResponseUsecase(
	questionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient,
	redisRepository redis.RedisRepository,
	internalConfig *config.InternalConfig,
) QuestionnaireResponseUsecase {
	return &questionnaireResponseUsecase{
		QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
		RedisRepository:                 redisRepository,
		InternalConfig:                  internalConfig,
	}
}

func (uc *questionnaireResponseUsecase) CreateQuestionnaireResponse(ctx context.Context, request *requests.CreateQuestionnaireResponse) (*responses.CreateQuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.CreateQuestionnaireResponse(ctx, request.QuestionnaireResponse)
	if err != nil {
		return nil, err
	}

	response := &responses.CreateQuestionnaireResponse{
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
func (uc *questionnaireResponseUsecase) UpdateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.UpdateQuestionnaireResponse(ctx, request)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}

func (uc *questionnaireResponseUsecase) FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error) {
	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		return nil, err
	}

	return questionnaireResponse, nil
}
func (uc *questionnaireResponseUsecase) DeleteQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) error {
	err := uc.QuestionnaireResponseFhirClient.DeleteQuestionnaireResponseByID(ctx, questionnaireResponseID)
	if err != nil {
		return err
	}

	return nil
}
