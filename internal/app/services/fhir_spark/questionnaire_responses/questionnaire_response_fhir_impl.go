package questionnaire_responses

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
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

func (c *questionnaireResponseFhirClient) FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponseByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, questionnaireResponseID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireResponseID), nil)
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
			fhirErrorIssue := errors.New(outcome.Issue[0].Diagnostics)
			c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}

		return nil, exceptions.ErrGetFHIRResource(fmt.Errorf("unexpected status code during find questionnaire response: %d", resp.StatusCode), constvars.ResourceQuestionnaireResponse)
	}

	qr := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&qr)
	if err != nil {
		c.Log.Error("questionnaireResponseFhirClient.FindQuestionnaireResponseByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	c.Log.Info("questionnaireResponseFhirClient.FindQuestionnaireResponseByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingQuestionnaireResponseIDKey, qr.ID),
	)

	return qr, nil
}
