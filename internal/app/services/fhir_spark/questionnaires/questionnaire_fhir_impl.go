package questionnaires

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/utils"
	"net/http"
	"net/url"
	"sync"

	"go.uber.org/zap"
)

var (
	questionnaireFhirClientInstance contracts.QuestionnaireFhirClient
	onceQuestionnaireFhirClient     sync.Once
)

type questionnaireFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewQuestionnaireFhirClient(baseUrl string, logger *zap.Logger) contracts.QuestionnaireFhirClient {
	onceQuestionnaireFhirClient.Do(func() {
		client := &questionnaireFhirClient{
			BaseUrl: baseUrl + constvars.ResourceQuestionnaire,
			Log:     logger,
		}
		questionnaireFhirClientInstance = client
	})
	return questionnaireFhirClientInstance
}

func (c *questionnaireFhirClient) FindQuestionnaires(ctx context.Context, request *requests.FindAllAssessment) ([]fhir_dto.Questionnaire, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.FindQuestionnaires called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	params := url.Values{}
	if request.SubjectType != "" {
		params.Add("subject-type", request.SubjectType)
	}
	url := fmt.Sprintf("%s?%s", c.BaseUrl, params.Encode())

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindQuestionnaires error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindQuestionnaires error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.FindQuestionnaires error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.FindQuestionnaires error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.FindQuestionnaires FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                 `json:"fullUrl"`
			Resource fhir_dto.Questionnaire `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindQuestionnaires error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaire)
	}

	questionnaires := make([]fhir_dto.Questionnaire, len(result.Entry))
	for i, entry := range result.Entry {
		questionnaires[i] = entry.Resource
	}

	c.Log.Info("questionnaireFhirClient.FindQuestionnaires succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingQuestionnaireCountKey, len(questionnaires)),
	)
	return questionnaires, nil
}

func (c *questionnaireFhirClient) CreateQuestionnaire(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.CreateQuestionnaire called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.CreateQuestionnaire FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error reading response body after creation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrReadBody(err)
	}

	data, err := utils.ParseJSONBody(body)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.CreateQuestionnaire error parsing JSON body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	c.Log.Info("questionnaireFhirClient.CreateQuestionnaire succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return data, nil
}

func (c *questionnaireFhirClient) FindRawQuestionnaireByID(ctx context.Context, questionnaireID string) (map[string]interface{}, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.FindRawQuestionnaireByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireID), nil)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID error reading response body after status check",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrReadBody(err)
	}

	data, err := utils.ParseJSONBody(body)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindRawQuestionnaireByID error parsing JSON body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	c.Log.Info("questionnaireFhirClient.FindRawQuestionnaireByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)
	return data, nil
}

func (c *questionnaireFhirClient) FindQuestionnaireByID(ctx context.Context, questionnaireID string) (*fhir_dto.Questionnaire, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.FindQuestionnaireByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireID), nil)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindQuestionnaireByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindQuestionnaireByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.FindQuestionnaireByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.FindQuestionnaireByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.FindQuestionnaireByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	questionnaireFhir := new(fhir_dto.Questionnaire)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireFhir)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.FindQuestionnaireByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaire)
	}

	c.Log.Info("questionnaireFhirClient.FindQuestionnaireByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireFhir.ID),
	)
	return questionnaireFhir, nil
}

func (c *questionnaireFhirClient) DeleteQuestionnaireByID(ctx context.Context, questionnaireID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.DeleteQuestionnaireByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireID), nil)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.DeleteQuestionnaireByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.DeleteQuestionnaireByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusNoContent {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.DeleteQuestionnaireByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.DeleteQuestionnaireByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.DeleteQuestionnaireByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	c.Log.Info("questionnaireFhirClient.DeleteQuestionnaireByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireID),
	)
	return nil
}

func (c *questionnaireFhirClient) UpdateQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.UpdateQuestionnaire called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut,
		fmt.Sprintf("%s/%s", c.BaseUrl, request.ID),
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	questionnaireFhir := new(fhir_dto.Questionnaire)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireFhir)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.UpdateQuestionnaire error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaire)
	}

	c.Log.Info("questionnaireFhirClient.UpdateQuestionnaire succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireFhir.ID),
	)
	return questionnaireFhir, nil
}

func (c *questionnaireFhirClient) PatchQuestionnaire(ctx context.Context, request *fhir_dto.Questionnaire) (*fhir_dto.Questionnaire, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireFhirClient.PatchQuestionnaire called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Any(constvars.LoggingRequestKey, request),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.PatchQuestionnaire error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPatch,
		fmt.Sprintf("%s/%s", c.BaseUrl, request.ID),
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.PatchQuestionnaire error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.PatchQuestionnaire error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.PatchQuestionnaire error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireFhirClient.PatchQuestionnaire error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaire)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireFhirClient.PatchQuestionnaire FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaire)
		}
	}

	questionnaireFhir := new(fhir_dto.Questionnaire)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireFhir)
	if err != nil {
		c.Log.Error("questionnaireFhirClient.PatchQuestionnaire error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaire)
	}

	c.Log.Info("questionnaireFhirClient.PatchQuestionnaire succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireIDKey, questionnaireFhir.ID),
	)
	return questionnaireFhir, nil
}
