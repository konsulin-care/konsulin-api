package persons

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"net/url"
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

	emailEnc := url.QueryEscape(email)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, emailEnc), nil)
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

// Search queries Person resources using supported search parameters.
func (c *personFhirClient) Search(ctx context.Context, params contracts.PersonSearchInput) ([]fhir_dto.Person, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("personFhirClient.Search called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	urlStr := c.BaseUrl
	if enc := params.ToQueryParam().Encode(); enc != "" {
		urlStr += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, urlStr, nil)
	if err != nil {
		c.Log.Error("personFhirClient.Search error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	c.Log.Info("personFhirClient.Search built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, req.URL.String()),
	)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("personFhirClient.Search error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("personFhirClient.Search error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		var outcome fhir_dto.OperationOutcome
		_ = json.Unmarshal(bodyBytes, &outcome)
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("personFhirClient.Search FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePerson)
		}
		return nil, exceptions.ErrGetFHIRResource(fmt.Errorf("status %d", resp.StatusCode), constvars.ResourcePerson)
	}

	var bundle struct {
		Entry []struct {
			Resource fhir_dto.Person `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		c.Log.Error("personFhirClient.Search error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	people := make([]fhir_dto.Person, len(bundle.Entry))
	for i, e := range bundle.Entry {
		people[i] = e.Resource
	}

	c.Log.Info("personFhirClient.Search succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(people)),
	)
	return people, nil
}

func (c *personFhirClient) Create(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("personFhirClient.Create called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(person)
	if err != nil {
		c.Log.Error("personFhirClient.Create error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("personFhirClient.CreatePerson error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("personFhirClient.CreatePerson error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("personFhirClient.Create error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("personFhirClient.Create error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("personFhirClient.Create FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePerson)
		}

		c.Log.Error("personFhirClient.Create unexpected status code",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingStatusCodeKey, resp.StatusCode),
			zap.String("body", string(bodyBytes)),
		)
		return nil, exceptions.ErrCreateFHIRResource(fmt.Errorf("unexpected status code: %d", resp.StatusCode), constvars.ResourcePerson)
	}

	var result fhir_dto.Person
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.Log.Error("personFhirClient.CreatePerson error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}
	return &result, nil
}

func (c *personFhirClient) Update(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("personFhirClient.Update called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(person)
	if err != nil {
		c.Log.Error("personFhirClient.Update error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, person.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("personFhirClient.Update error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("personFhirClient.Update error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("personFhirClient.Update error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("personFhirClient.Update error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
		}
		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("personFhirClient.Update FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePerson)
		}

		c.Log.Error("personFhirClient.Update unexpected status code",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Int(constvars.LoggingStatusCodeKey, resp.StatusCode),
			zap.String("body", string(bodyBytes)),
		)
		return nil, exceptions.ErrUpdateFHIRResource(fmt.Errorf("unexpected status code during update person: %d", resp.StatusCode), constvars.ResourcePerson)
	}

	var result fhir_dto.Person
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.Log.Error("personFhirClient.Update error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}
	return &result, nil
}
