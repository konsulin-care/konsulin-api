package questionnaireResponses

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
	"sync"

	"go.uber.org/zap"
)

var (
	questionnaireResponseFhirClientInstance contracts.QuestionnaireResponseFhirClient
	onceQuestionnaireResponseFhirClient     sync.Once
)

type questionnaireResponseFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewQuestionnaireResponseFhirClient(baseUrl string, logger *zap.Logger) contracts.QuestionnaireResponseFhirClient {
	onceQuestionnaireResponseFhirClient.Do(func() {
		client := &questionnaireResponseFhirClient{
			BaseUrl: baseUrl + constvars.ResourceQuestionnaireResponse,
			Log:     logger,
		}
		questionnaireResponseFhirClientInstance = client
	})
	return questionnaireResponseFhirClientInstance
}

func (c *questionnaireResponseFhirClient) CreateQuestionnaireResponse(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.CreateQuestionnaireResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error reading response body after creation",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrReadBody(err)
	}

	data, err := utils.ParseJSONBody(body)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.CreateQuestionnaireResponse error parsing JSON body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotParseJSON(err)
	}

	c.Log.Info("questionnaireResponseFhirClient.CreateQuestionnaireResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return data, nil
}

func (c *questionnaireResponseFhirClient) FindQuestionnaireResponses(ctx context.Context, request *requests.FindAllAssessmentResponse) ([]fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponses called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	url := c.BaseUrl
	if request.PatientID != "" {
		url += fmt.Sprintf("?subject=%s/%s", constvars.ResourcePatient, request.PatientID)
		if request.AssessmentID != "" {
			url += fmt.Sprintf("&questionnaire=%s/%s", constvars.ResourceQuestionnaire, request.AssessmentID)
		}
	}
	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponses built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, url),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponses error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponses error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponses error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponses error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponses FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                         `json:"fullUrl"`
			Resource fhir_dto.QuestionnaireResponse `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponses error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	questionnaireResponses := make([]fhir_dto.QuestionnaireResponse, len(result.Entry))
	for i, entry := range result.Entry {
		questionnaireResponses[i] = entry.Resource
	}

	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponses succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingQuestionnaireResponseCountKey, len(questionnaireResponses)),
	)
	return questionnaireResponses, nil
}

func (c *questionnaireResponseFhirClient) FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireResponseID), nil)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseFhir.ID),
	)
	return questionnaireResponseFhir, nil
}

func (c *questionnaireResponseFhirClient) DeleteQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodDelete,
		fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireResponseID), nil)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusNoContent {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	c.Log.Info("questionnaireResponseFhirClient.DeleteQuestionnaireResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)
	return nil
}

func (c *questionnaireResponseFhirClient) UpdateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.UpdateQuestionnaireResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse error marshaling JSON",
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
		c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.UpdateQuestionnaireResponse error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	c.Log.Info("questionnaireResponseFhirClient.UpdateQuestionnaireResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseFhir.ID),
	)
	return questionnaireResponseFhir, nil
}

func (c *questionnaireResponseFhirClient) PatchQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.PatchQuestionnaireResponse called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse error marshaling JSON",
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
		c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.PatchQuestionnaireResponse error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	c.Log.Info("questionnaireResponseFhirClient.PatchQuestionnaireResponse succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseFhir.ID),
	)
	return questionnaireResponseFhir, nil
}
