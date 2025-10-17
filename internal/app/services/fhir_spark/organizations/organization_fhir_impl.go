package organizations

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
	organizationFhirClientInstance contracts.OrganizationFhirClient
	onceOrganizationFhirClient     sync.Once
)

type organizationFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
}

func NewOrganizationFhirClient(baseUrl string, logger *zap.Logger) contracts.OrganizationFhirClient {
	onceOrganizationFhirClient.Do(func() {
		client := &organizationFhirClient{
			BaseUrl: baseUrl + constvars.ResourceOrganization,
			Log:     logger,
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

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindAll error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, 0, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindAll error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, 0, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("organizationFhirClient.FindAll error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, 0, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("organizationFhirClient.FindAll error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, 0, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("organizationFhirClient.FindAll FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, 0, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceOrganization)
		}
	}

	var result struct {
		Total int `json:"total"`
		Entry []struct {
			FullUrl  string                `json:"fullUrl"`
			Resource fhir_dto.Organization `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
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

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, organizationID), nil)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindOrganizationByID error creating HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindOrganizationByID error sending HTTP request",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			c.Log.Error("organizationFhirClient.FindOrganizationByID error reading response body",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			c.Log.Error("organizationFhirClient.FindOrganizationByID error unmarshaling outcome",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(err),
			)
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			c.Log.Error("organizationFhirClient.FindOrganizationByID FHIR error",
				zap.String(constvars.LoggingRequestIDKey, requestID),
				zap.Error(fhirErrorIssue),
			)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceOrganization)
		}
	}

	organizationFhir := new(fhir_dto.Organization)
	err = json.NewDecoder(resp.Body).Decode(&organizationFhir)
	if err != nil {
		c.Log.Error("organizationFhirClient.FindOrganizationByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	c.Log.Info("organizationFhirClient.FindOrganizationByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOrganizationIDKey, organizationFhir.ID),
	)
	return organizationFhir, nil
}
