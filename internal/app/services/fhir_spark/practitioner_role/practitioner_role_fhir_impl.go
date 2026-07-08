package practitionerRoles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
	"net/url"
	"sync"

	"go.uber.org/zap"
)

var (
	practitionerRoleFhirClientInstance contracts.PractitionerRoleFhirClient
	oncePractitionerRoleFhirClient     sync.Once
)

type practitionerRoleFhirClient struct {
	BaseFhirUrl string
	BaseUrl     string
	Log         *zap.Logger
	client      *fhir_http_client.FHIRHTTPClient
}

func NewPractitionerRoleFhirClient(baseUrl string, logger *zap.Logger) contracts.PractitionerRoleFhirClient {
	oncePractitionerRoleFhirClient.Do(func() {
		client := &practitionerRoleFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePractitionerRole,
			Log:     logger,
			client:  fhir_http_client.New(logger),
		}
		practitionerRoleFhirClientInstance = client
	})
	return practitionerRoleFhirClientInstance
}

func (c *practitionerRoleFhirClient) Search(ctx context.Context, params contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	q := params.ToQueryParam()
	urlStr := c.BaseUrl
	if enc := q.Encode(); enc != "" {
		urlStr += "?" + enc
	}

	respBody, err := c.client.Do(ctx, constvars.MethodGet, urlStr, nil)
	if err != nil {
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var bundle struct {
		Entry []struct {
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &bundle); err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	out := make([]fhir_dto.PractitionerRole, len(bundle.Entry))
	for i, e := range bundle.Entry {
		out[i] = e.Resource
	}

	c.Log.Info("practitionerRoleFhirClient.Search",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(out)))
	return out, nil
}

func (c *practitionerRoleFhirClient) DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.DeletePractitionerRoleByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleID),
	)

	_, err := c.client.Do(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, practitionerRoleID), nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.DeletePractitionerRoleByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	c.Log.Info("practitionerRoleFhirClient.DeletePractitionerRoleByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleID),
	)
	return nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByOrganizationID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingOrganizationIDKey, organizationID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/?organization=Organization/%s", c.BaseUrl, organizationID), nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByOrganizationID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByOrganizationID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByOrganizationID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitionerRoles)),
	)
	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByCustomRequest called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	params := url.Values{}
	params.Add("organization", fmt.Sprintf("Organization/%s", request.ClinicID))
	if request.City != "" {
		params.Add("organization.address-city", request.City)
	}
	if request.PractitionerName != "" {
		params.Add("practitioner.name:contains", request.PractitionerName)
	}

	url := fmt.Sprintf("%s?%s", c.BaseUrl, params.Encode())
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByCustomRequest built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, url),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByCustomRequest FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByCustomRequest error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByCustomRequest succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitionerRoles)),
	)
	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerIDKey, practitionerID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s?practitioner=Practitioner/%s", c.BaseUrl, practitionerID), nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitionerRoles)),
	)
	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	url := fmt.Sprintf("%s?practitioner=Practitioner/%s", c.BaseUrl, request.PractitionerID)
	if request.OrganizationName != "" {
		url += fmt.Sprintf("&organization.name:contains=%s", request.OrganizationName)
	}
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, url),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndName succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitionerRoles)),
	)
	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	url := fmt.Sprintf("%s?practitioner=Practitioner/%s&organization=Organization/%s",
		c.BaseUrl,
		practitionerID,
		organizationID,
	)
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID built URL",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingFhirUrlKey, url),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByPractitionerIDAndOrganizationID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.Int(constvars.LoggingPractitionerRoleCountKey, len(practitionerRoles)),
	)
	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) CreatePractitionerRoles(ctx context.Context, request interface{}) error {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.CreatePractitionerRoles called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.CreatePractitionerRoles error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrCannotMarshalJSON(err)
	}

	_, err = c.client.Do(ctx, constvars.MethodPost, c.BaseFhirUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.CreatePractitionerRoles FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	c.Log.Info("practitionerRoleFhirClient.CreatePractitionerRoles succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)
	return nil
}

func (c *practitionerRoleFhirClient) CreatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.CreatePractitionerRole called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.CreatePractitionerRole error marshaling JSON",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	respBody, err := c.client.Do(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.CreatePractitionerRole FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var practitionerRoleFhir fhir_dto.PractitionerRole
	if err := json.Unmarshal(respBody, &practitionerRoleFhir); err != nil {
		c.Log.Error("practitionerRoleFhirClient.CreatePractitionerRole error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	c.Log.Info("practitionerRoleFhirClient.CreatePractitionerRole succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleFhir.ID),
	)
	return &practitionerRoleFhir, nil
}

func (c *practitionerRoleFhirClient) UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.UpdatePractitionerRole called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	requestJSON, err := json.Marshal(request)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.UpdatePractitionerRole error marshaling JSON",
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
		c.Log.Error("practitionerRoleFhirClient.UpdatePractitionerRole FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var practitionerRoleFhir fhir_dto.PractitionerRole
	if err := json.Unmarshal(respBody, &practitionerRoleFhir); err != nil {
		c.Log.Error("practitionerRoleFhirClient.UpdatePractitionerRole error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	c.Log.Info("practitionerRoleFhirClient.UpdatePractitionerRole succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleFhir.ID),
	)
	return &practitionerRoleFhir, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByID called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRoleID),
	)

	respBody, err := c.client.Do(ctx, constvars.MethodGet,
		fmt.Sprintf("%s/%s", c.BaseUrl, practitionerRoleID), nil)
	if err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByID FHIR error",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
	}

	var practitionerRole fhir_dto.PractitionerRole
	if err := json.Unmarshal(respBody, &practitionerRole); err != nil {
		c.Log.Error("practitionerRoleFhirClient.FindPractitionerRoleByID error decoding response",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	c.Log.Info("practitionerRoleFhirClient.FindPractitionerRoleByID succeeded",
		zap.String(constvars.LoggingRequestIDKey, requestID),
		zap.String(constvars.LoggingPractitionerRoleIDKey, practitionerRole.ID),
	)
	return &practitionerRole, nil
}
