package assessmentResponses

import (
	"context"
	"konsulin-service/internal/app/config"
	patientFhir "konsulin-service/internal/app/services/fhir_spark/patients"
	questionnaireResponses "konsulin-service/internal/app/services/fhir_spark/questionnaires_responses"
	"konsulin-service/internal/app/services/shared/redis"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"time"

	"github.com/google/uuid"
)

type assessmentResponseUsecase struct {
	QuestionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient
	PatientFhirClient               patientFhir.PatientFhirClient
	RedisRepository                 redis.RedisRepository
	InternalConfig                  *config.InternalConfig
}

func NewAssessmentResponseUsecase(
	questionnaireResponseFhirClient questionnaireResponses.QuestionnaireResponseFhirClient,
	patientFhirClient patientFhir.PatientFhirClient,
	redisRepository redis.RedisRepository,
	internalConfig *config.InternalConfig,
) AssessmentResponseUsecase {
	return &assessmentResponseUsecase{
		QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
		PatientFhirClient:               patientFhirClient,
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

func (uc *assessmentResponseUsecase) FindAssessmentResponseByID(ctx context.Context, assessmentResponseID string) (*responses.AssessmentResponse, error) {
	questionnaireResponseFhir, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, assessmentResponseID)
	if err != nil {
		return nil, err
	}

	patientID, err := utils.ParseIDFromReference(questionnaireResponseFhir.Subject)
	if err != nil {
		return nil, exceptions.ErrServerProcess(err)
	}

	patientResponseFhir, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if err != nil {
		return nil, err
	}

	response := uc.mapFHIRQuestionnaireResponseToAssessment(questionnaireResponseFhir, patientResponseFhir)
	return response, nil
}

func (uc *assessmentResponseUsecase) DeleteAssessmentResponseByID(ctx context.Context, assessmentResponseID string) error {
	err := uc.QuestionnaireResponseFhirClient.DeleteQuestionnaireResponseByID(ctx, assessmentResponseID)
	if err != nil {
		return err
	}

	return nil
}

func (uc *assessmentResponseUsecase) mapFHIRQuestionnaireResponseToAssessment(questionnaireResponse *fhir_dto.QuestionnaireResponse, patientResponse *fhir_dto.Patient) *responses.AssessmentResponse {
	participantName := utils.GetFullName(patientResponse.Name)
	assessmentTitle := "BIG 5 Personality Test (Hardcoded)"
	resultBrief := "Result brief (Hardcoded)"

	var resultTables []responses.VariableResult
	for _, item := range questionnaireResponse.Item {
		var score float64
		if item.LinkID == "score" && len(item.Answer) > 0 && item.Answer[0].ValueDecimal != nil {
			score = *item.Answer[0].ValueDecimal
		} else if item.Answer[0].ValueCoding != nil {
			switch item.Answer[0].ValueCoding.Code {
			case "0":
				score = 0
			case "1":
				score = 1
			case "2":
				score = 2
			case "3":
				score = 3
			}
		}
		resultTables = append(resultTables, responses.VariableResult{
			VariableName: item.Text,
			Score:        score,
		})
	}

	// qrCodeURL := fmt.Sprintf("https://example.com/qr-code/%s", questionnaireResponse.ID)

	return &responses.AssessmentResponse{
		ParticipantName: participantName,
		AssessmentTitle: assessmentTitle,
		ResultBrief:     resultBrief,
		ResultTables:    resultTables,
	}
}
