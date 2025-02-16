package assessmentResponses

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/config"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type assessmentResponseUsecase struct {
	QuestionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient
	QuestionnaireFhirClient         contracts.QuestionnaireFhirClient
	PatientFhirClient               contracts.PatientFhirClient
	SessionService                  contracts.SessionService
	RedisRepository                 contracts.RedisRepository
	InternalConfig                  *config.InternalConfig
	Log                             *zap.Logger
}

var (
	assessmentResponseUsecaseInstance contracts.AssessmentResponseUsecase
	onceAssessmentResponseUsecase     sync.Once
)

func NewAssessmentResponseUsecase(
	questionnaireResponseFhirClient contracts.QuestionnaireResponseFhirClient,
	questionnaireFhirClient contracts.QuestionnaireFhirClient,
	patientFhirClient contracts.PatientFhirClient,
	sessionService contracts.SessionService,
	redisRepository contracts.RedisRepository,
	internalConfig *config.InternalConfig,
	logger *zap.Logger,
) contracts.AssessmentResponseUsecase {
	onceAssessmentResponseUsecase.Do(func() {
		instance := &assessmentResponseUsecase{
			QuestionnaireResponseFhirClient: questionnaireResponseFhirClient,
			QuestionnaireFhirClient:         questionnaireFhirClient,
			PatientFhirClient:               patientFhirClient,
			SessionService:                  sessionService,
			RedisRepository:                 redisRepository,
			InternalConfig:                  internalConfig,
			Log:                             logger,
		}
		assessmentResponseUsecaseInstance = instance
	})
	return assessmentResponseUsecaseInstance
}

func (uc *assessmentResponseUsecase) CreateAssessmentResponse(ctx context.Context, request *requests.CreateAssesmentResponse) (*responses.CreateAssessmentResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.CreateAssessmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	if request.SessionData != "" {
		session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
		if err != nil {
			uc.Log.Error("assessmentResponseUsecase.CreateAssessmentResponse error parse session data",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}

		if session.IsPatient() {
			request.QuestionnaireResponse["subject"] = map[string]interface{}{
				"reference": fmt.Sprintf("%s/%s", constvars.ResourcePatient, session.PatientID),
			}
		} else if session.IsPractitioner() {
			request.QuestionnaireResponse["subject"] = map[string]interface{}{
				"reference": fmt.Sprintf("%s/%s", constvars.ResourcePractitioner, session.PractitionerID),
			}
		} else {
			uc.Log.Error("assessmentResponseUsecase.CreateAssessmentResponse role is not allowed to create resposne",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.String(constvars.LoggingRoleNameKey, session.RoleName),
				zap.Error(err),
			)
			return nil, exceptions.ErrInvalidRoleType(nil)
		}
	}

	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.CreateQuestionnaireResponse(ctx, request.QuestionnaireResponse)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.CreateAssessmentResponse error creating questionnaire response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response := &responses.CreateAssessmentResponse{
		QuestionnaireResponse: questionnaireResponse,
	}
	if request.RespondentType == constvars.RespondentTypeGuest {
		responseID := uuid.New().String()
		responseExpiryTime := time.Minute * time.Duration(uc.InternalConfig.App.QuestionnaireGuestResponseExpiredTimeInMinutes)
		err := uc.RedisRepository.Set(ctx, responseID, questionnaireResponse["id"], responseExpiryTime)
		if err != nil {
			uc.Log.Error("assessmentResponseUsecase.CreateAssessmentResponse error setting Redis key",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, err
		}
		response.ResponseID = responseID
	}

	uc.Log.Info("assessmentResponseUsecase.CreateAssessmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return response, nil
}

func (uc *assessmentResponseUsecase) UpdateAssessmentResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.UpdateAssessmentResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	questionnaireResponse, err := uc.QuestionnaireResponseFhirClient.UpdateQuestionnaireResponse(ctx, request)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.UpdateAssessmentResponse error updating questionnaire response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("assessmentResponseUsecase.UpdateAssessmentResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return questionnaireResponse, nil
}

func (uc *assessmentResponseUsecase) FindAll(ctx context.Context, request *requests.FindAllAssessmentResponse) ([]fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	session, err := uc.SessionService.ParseSessionData(ctx, request.SessionData)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAll error parsing session data",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}
	request.PatientID = session.PatientID

	assessmentResponses, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponses(ctx, request)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAll error fetching questionnaire responses",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	uc.Log.Info("assessmentResponseUsecase.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(assessmentResponses)),
	)
	return assessmentResponses, nil
}

func (uc *assessmentResponseUsecase) FindAssessmentResponseByID(ctx context.Context, assessmentResponseID string) (*responses.AssessmentResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.FindAssessmentResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAssessmentResponseIDKey, assessmentResponseID),
	)

	questionnaireResponseFhir, err := uc.QuestionnaireResponseFhirClient.FindQuestionnaireResponseByID(ctx, assessmentResponseID)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAssessmentResponseByID error fetching questionnaire response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	patientID, err := utils.ParseIDFromReference(questionnaireResponseFhir.Subject)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAssessmentResponseByID error parsing patient ID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrServerProcess(err)
	}

	patientResponseFhir, err := uc.PatientFhirClient.FindPatientByID(ctx, patientID)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAssessmentResponseByID error fetching patient",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.String(constvars.LoggingPatientIDKey, patientID),
			zap.Error(err),
		)
		return nil, err
	}

	questionnaireID, err := uc.getQuestionnaireIDFromQuestionnaireResponseDTO(ctx, questionnaireResponseFhir)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAssessmentResponseByID error extracting questionnaire ID",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	questionnaireFhir, err := uc.QuestionnaireFhirClient.FindQuestionnaireByID(ctx, questionnaireID)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.FindAssessmentResponseByID error fetching questionnaire",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	response := uc.mapFHIRQuestionnaireResponseToAssessment(ctx, questionnaireResponseFhir, questionnaireFhir, patientResponseFhir)
	uc.Log.Info("assessmentResponseUsecase.FindAssessmentResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAssessmentResponseIDKey, questionnaireResponseFhir.ID),
	)
	return response, nil
}

func (uc *assessmentResponseUsecase) DeleteAssessmentResponseByID(ctx context.Context, assessmentResponseID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.DeleteAssessmentResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAssessmentResponseIDKey, assessmentResponseID),
	)

	err := uc.QuestionnaireResponseFhirClient.DeleteQuestionnaireResponseByID(ctx, assessmentResponseID)
	if err != nil {
		uc.Log.Error("assessmentResponseUsecase.DeleteAssessmentResponseByID error from FHIR client",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return err
	}

	uc.Log.Info("assessmentResponseUsecase.DeleteAssessmentResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAssessmentResponseIDKey, assessmentResponseID),
	)
	return nil
}

func (uc *assessmentResponseUsecase) mapFHIRQuestionnaireResponseToAssessment(ctx context.Context, questionnaireResponse *fhir_dto.QuestionnaireResponse, questionnaireFhir *fhir_dto.Questionnaire, patientResponse *fhir_dto.Patient) *responses.AssessmentResponse {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.mapFHIRQuestionnaireResponseToAssessment called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	participantName := utils.GetFullName(patientResponse.Name)
	assessmentTitle := questionnaireFhir.Title
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

	uc.Log.Info("assessmentResponseUsecase.mapFHIRQuestionnaireResponseToAssessment succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingAssessmentIDKey, questionnaireResponse.ID),
	)
	return &responses.AssessmentResponse{
		ID:              questionnaireResponse.ID,
		ParticipantName: participantName,
		AssessmentTitle: assessmentTitle,
		ResultBrief:     resultBrief,
		ResultTables:    resultTables,
	}
}

func (uc *assessmentResponseUsecase) getQuestionnaireIDFromQuestionnaireResponseDTO(ctx context.Context, questionnaireResponse *fhir_dto.QuestionnaireResponse) (string, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	uc.Log.Info("assessmentResponseUsecase.getQuestionnaireIDFromQuestionnaireResponseDTO called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	parts := strings.Split(questionnaireResponse.Questionnaire, "/")
	if len(parts) < 2 {
		errResponse := fmt.Errorf("invalid URL format: %s", questionnaireResponse.Questionnaire)
		uc.Log.Error("assessmentResponseUsecase.getQuestionnaireIDFromQuestionnaireResponseDTO error", zap.Error(errResponse))
		return "", exceptions.ErrServerProcess(errResponse)
	}

	resourceType := parts[len(parts)-2]
	if resourceType != constvars.ResourceQuestionnaire {
		errResponse := fmt.Errorf("URL does not point to a Questionnaire resource: %s", questionnaireResponse.Questionnaire)
		uc.Log.Error("assessmentResponseUsecase.getQuestionnaireIDFromQuestionnaireResponseDTO error", zap.Error(errResponse))
		return "", exceptions.ErrServerProcess(errResponse)
	}

	questionnaireID := parts[len(parts)-1]
	uc.Log.Info("assessmentResponseUsecase.getQuestionnaireIDFromQuestionnaireResponseDTO succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID))
	return questionnaireID, nil
}
