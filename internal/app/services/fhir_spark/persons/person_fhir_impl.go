package persons

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
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
	client  *fhir_http_client.FHIRHTTPClient
}

func NewPersonFhirClient(baseUrl string, logger *zap.Logger) contracts.PersonFhirClient {
	oncePersonFhirClient.Do(func() {
		client := &personFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePerson,
			Log:     logger,
			client:  fhir_http_client.New(logger),
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
	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?email=%s", c.BaseUrl, emailEnc), nil)
	if err != nil {
		c.Log.Error("personFhirClient.FindPersonByEmail FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
	}

	var result fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("personFhirClient.FindPersonByEmail error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	persons := make([]fhir_dto.Person, 0, len(result.Entry))
	for _, entry := range result.Entry {
		var p fhir_dto.Person
		if err := json.Unmarshal(entry.Resource, &p); err != nil {
			c.Log.Error("personFhirClient.FindPersonByEmail error unmarshaling person resource",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		persons = append(persons, p)
	}

	c.Log.Info("personFhirClient.FindPersonByEmail succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(persons)),
	)
	return persons, nil
}
func (c *personFhirClient) FindPersonByPhone(ctx context.Context, phone string) ([]fhir_dto.Person, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("personFhirClient.FindPersonByPhone called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	phoneEnc := url.QueryEscape(phone)
	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?phone=%s", c.BaseUrl, phoneEnc), nil)
	if err != nil {
		c.Log.Error("personFhirClient.FindPersonByPhone FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
	}

	var result fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("personFhirClient.FindPersonByPhone error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	persons := make([]fhir_dto.Person, 0, len(result.Entry))
	for _, entry := range result.Entry {
		var p fhir_dto.Person
		if err := json.Unmarshal(entry.Resource, &p); err != nil {
			c.Log.Error("personFhirClient.FindPersonByPhone error unmarshaling person resource",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		persons = append(persons, p)
	}

	c.Log.Info("personFhirClient.FindPersonByPhone succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(persons)),
	)
	return persons, nil
}
func (c *personFhirClient) Search(ctx context.Context, params contracts.PersonSearchInput) ([]fhir_dto.Person, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("personFhirClient.Search called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	urlStr := c.BaseUrl
	if enc := params.ToQueryParam().Encode(); enc != "" {
		urlStr += "?" + enc
	}

	respBody, err := c.client.Do(ctx, constvars.MethodGet, urlStr, nil)
	if err != nil {
		c.Log.Error("personFhirClient.Search FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePerson)
	}

	var bundle fhir_dto.FHIRBundle
	if err := json.Unmarshal(respBody, &bundle); err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	persons := make([]fhir_dto.Person, 0, len(bundle.Entry))
	for _, e := range bundle.Entry {
		var p fhir_dto.Person
		if err := json.Unmarshal(e.Resource, &p); err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		persons = append(persons, p)
	}

	c.Log.Info("personFhirClient.Search succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingResponseCountKey, len(persons)),
	)
	return persons, nil
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

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("personFhirClient.Create FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePerson)
	}

	var p fhir_dto.Person
	if err := json.Unmarshal(respBody, &p); err != nil {
		c.Log.Error("personFhirClient.Create error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	c.Log.Info("personFhirClient.Create succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("person_id", p.ID),
	)
	return &p, nil
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

	respBody, err := c.client.Do(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, person.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("personFhirClient.Update FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePerson)
	}

	var p fhir_dto.Person
	if err := json.Unmarshal(respBody, &p); err != nil {
		c.Log.Error("personFhirClient.Update error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePerson)
	}

	c.Log.Info("personFhirClient.Update succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String("person_id", p.ID),
	)
	return &p, nil
}
