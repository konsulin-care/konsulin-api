package organizations

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
	"sync"

	"go.uber.org/zap"
)

var (
	organizationFhirClientInstance contracts.OrganizationFhirClient
	onceOrganizationFhirClient     sync.Once
)

type organizationFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
	client  *fhir_http_client.FHIRHTTPClient
}

func NewOrganizationFhirClient(baseUrl string, logger *zap.Logger) contracts.OrganizationFhirClient {
	onceOrganizationFhirClient.Do(func() {
		client := &organizationFhirClient{
			BaseUrl: baseUrl + constvars.ResourceOrganization,
			Log:     logger,
			client:  fhir_http_client.New(logger),
		}
		organizationFhirClientInstance = client
	})
	return organizationFhirClientInstance
}

func (c *organizationFhirClient) FindAll(ctx context.Context, nameFilter, fetchType string, page, pageSize int) ([]fhir_dto.Organization, int, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("organizationFhirClient.FindAll called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	url := c.BaseUrl

	if nameFilter != "" {
		url += fmt.Sprintf("?name:contains=%s", nameFilter)
	}

	if fetchType == constvars.FhirFetchResourceTypePaged {
		url += fmt.Sprintf("&?page=%d&?_count=%d", page, pageSize)
	}

	c.Log.Info("organizationFhirClient.FindAll built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, url),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindAll FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, 0, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
	}

	var result struct {
		Total int `json:"total"`
		Entry []struct {
			FullUrl  string                `json:"fullUrl"`
			Resource fhir_dto.Organization `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("organizationFhirClient.FindAll error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, 0, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	organizations := make([]fhir_dto.Organization, len(result.Entry))
	for i, entry := range result.Entry {
		organizations[i] = entry.Resource
	}

	c.Log.Info("organizationFhirClient.FindAll succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingOrganizationCountKey, len(organizations)),
	)
	return organizations, result.Total, nil
}

func (c *organizationFhirClient) FindOrganizationByID(ctx context.Context, organizationID string) (*fhir_dto.Organization, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("organizationFhirClient.FindOrganizationByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOrganizationIDKey, organizationID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, organizationID), nil)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindOrganizationByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
	}

	var organization fhir_dto.Organization
	if err := json.Unmarshal(respBody, &organization); err != nil {
		c.Log.Error("organizationFhirClient.FindOrganizationByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	c.Log.Info("organizationFhirClient.FindOrganizationByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOrganizationIDKey, organization.ID),
	)
	return &organization, nil
}

func (c *organizationFhirClient) Update(ctx context.Context, organization fhir_dto.Organization) (*fhir_dto.Organization, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("organizationFhirClient.Update called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOrganizationIDKey, organization.ID),
	)

	bodyBytes, err := json.Marshal(organization)
	if err != nil {
		c.Log.Error("organizationFhirClient.Update error marshaling request body",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}

	url := fmt.Sprintf("%s/%s", c.BaseUrl, organization.ID)
	respBody, err := c.client.Do(ctx, constvars.MethodPut, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.Log.Error("organizationFhirClient.Update FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
	}

	var org fhir_dto.Organization
	if err := json.Unmarshal(respBody, &org); err != nil {
		c.Log.Error("organizationFhirClient.Update error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	c.Log.Info("organizationFhirClient.Update succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOrganizationIDKey, org.ID),
	)
	return &org, nil
}
