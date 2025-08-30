package persons

import (
	"context"
	"encoding/json"
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
	personFhirClientInstance contracts.PersonFhirClient
	oncePersonFhirClient     sync.Once
)

type personFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewPersonFhirClient(baseUrl string, logger *zap.Logger) contracts.PersonFhirClient {
	oncePersonFhirClient.Do(func() {
		client := &personFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePerson,
			Log:     logger,
		}
		personFhirClientInstance = client
	})
	return personFhirClientInstance
}

// FindPersonByEmail queries Person by email search parameter.
func (c *personFhirClient) FindPersonByEmail(ctx context.Context, email string) ([]fhir_dto.Person, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("personFhirClient.FindPersonByEmail called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?email=%s", c.BaseUrl, email), nil)
	if err != nil {
		c.Log.Error("personFhirClient.FindPersonByEmail error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("personFhirClient.FindPersonByEmail error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("personFhirClient.FindPersonByEmail error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("personFhirClient.FindPersonByEmail error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("personFhirClient.FindPersonByEmail FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePerson)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string          `json:"fullUrl"`
			Resource fhir_dto.Person `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		c.Log.Error("personFhirClient.FindPersonByEmail error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	people := make([]fhir_dto.Person, len(result.Entry))
	for i, entry := range result.Entry {
		people[i] = entry.Resource
	}

	c.Log.Info("personFhirClient.FindPersonByEmail succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(people)),
	)
	return people, nil
}
