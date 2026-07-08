package practitioners

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
	practitionerFhirClientInstance contracts.PractitionerFhirClient
	oncePractitionerFhirClient     sync.Once
)

type practitionerFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
	client  *fhir_http_client.FHIRHTTPClient
}

func NewPractitionerFhirClient(baseUrl string, logger *zap.Logger) contracts.PractitionerFhirClient {
	oncePractitionerFhirClient.Do(func() {
		client := &practitionerFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePractitioner,
			Log:     logger,
			client:  fhir_http_client.New(logger),
		}
		practitionerFhirClientInstance = client
	})
	return practitionerFhirClientInstance
}

func (c *practitionerFhirClient) CreatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.CreatePractitioner called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitioner)
	}

	var practitioner fhir_dto.Practitioner
	if err := json.Unmarshal(respBody, &practitioner); err != nil {
		c.Log.Error("practitionerFhirClient.CreatePractitioner error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.CreatePractitioner succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitioner.ID),
	)
	return &practitioner, nil
}

func (c *practitionerFhirClient) FindPractitionerByID(ctx context.Context, practitionerID string) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.FindPractitionerByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/%s", c.BaseUrl, practitionerID), nil)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
	}

	var practitioner fhir_dto.Practitioner
	if err := json.Unmarshal(respBody, &practitioner); err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitioner.ID),
	)
	return &practitioner, nil
}

func (c *practitionerFhirClient) UpdatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.UpdatePractitioner called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPut,
		fmt.Sprintf("%s/%s", c.BaseUrl, request.ID),
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
	}

	var practitioner fhir_dto.Practitioner
	if err := json.Unmarshal(respBody, &practitioner); err != nil {
		c.Log.Error("practitionerFhirClient.UpdatePractitioner error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.UpdatePractitioner succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitioner.ID),
	)
	return &practitioner, nil
}

func (c *practitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.FindPractitionerByIdentifier called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	identifierToken := fmt.Sprintf("%s|%s", system, value)
	identifierEnc := url.QueryEscape(identifierToken)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?identifier=%s", c.BaseUrl, identifierEnc), nil)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByIdentifier FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                `json:"fullUrl"`
			Resource fhir_dto.Practitioner `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByIdentifier error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	practitioners := make([]fhir_dto.Practitioner, len(result.Entry))
	for i, entry := range result.Entry {
		practitioners[i] = entry.Resource
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByIdentifier succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitioners)),
	)
	return practitioners, nil
}

// FindPractitionerByEmail queries Practitioner by email search parameter.
// FindPractitionerByEmail queries Practitioner by email search parameter.
func (c *practitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.FindPractitionerByEmail called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	emailEnc := url.QueryEscape(email)
	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, emailEnc), nil)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByEmail FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByEmail built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, emailEnc)),
	)

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                `json:"fullUrl"`
			Resource fhir_dto.Practitioner `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByEmail error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	practitioners := make([]fhir_dto.Practitioner, len(result.Entry))
	for i, entry := range result.Entry {
		practitioners[i] = entry.Resource
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByEmail succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitioners)),
	)
	return practitioners, nil
}

func (c *practitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.FindPractitionerByPhone called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	phoneEnc := url.QueryEscape(phone)
	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?phone=%s&_sort=-_lastUpdated", c.BaseUrl, phoneEnc), nil)
	if err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByPhone FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByPhone built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, fmt.Sprintf("%s?phone=%s&_sort=-_lastUpdated", c.BaseUrl, phoneEnc)),
	)

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                `json:"fullUrl"`
			Resource fhir_dto.Practitioner `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerFhirClient.FindPractitionerByPhone error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	practitioners := make([]fhir_dto.Practitioner, len(result.Entry))
	for i, entry := range result.Entry {
		practitioners[i] = entry.Resource
	}

	c.Log.Info("practitionerFhirClient.FindPractitionerByPhone succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitioners)),
	)
	return practitioners, nil
}

func (c *practitionerFhirClient) PatchPractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerFhirClient.PatchPractitioner called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPatch,
		fmt.Sprintf("%s/%s", c.BaseUrl, request.ID),
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
	}

	var practitioner fhir_dto.Practitioner
	if err := json.Unmarshal(respBody, &practitioner); err != nil {
		c.Log.Error("practitionerFhirClient.PatchPractitioner error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	c.Log.Info("practitionerFhirClient.PatchPractitioner succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitioner.ID),
	)
	return &practitioner, nil
}
