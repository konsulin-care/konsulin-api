package practitionerRoles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
	"net/url"
	"strings"
	"sync"

	"go.uber.org/zap"
)

var (
	practitionerRoleFhirClientInstance contracts.PractitionerRoleFhirClient
	oncePractitionerRoleFhirClient     sync.Once
)

type practitionerRoleFhirClient struct {
	*base.ResourceClient
	// BaseFhirUrl is removed; use strings.TrimSuffix(c.BaseUrl, constvars.ResourcePractitionerRole)
	// to derive the base FHIR server URL when needed.
}

func NewPractitionerRoleFhirClient(baseUrl string, logger *zap.Logger) contracts.PractitionerRoleFhirClient {
	oncePractitionerRoleFhirClient.Do(func() {
		practitionerRoleFhirClientInstance = &practitionerRoleFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourcePractitionerRole, logger),
		}
	})
	return practitionerRoleFhirClientInstance
}

func (c *practitionerRoleFhirClient) Search(ctx context.Context, params contracts.PractitionerRoleSearchParams) ([]fhir_dto.PractitionerRole, error) {
	urlStr := c.BaseUrl
	if q := params.ToQueryParam(); q != nil {
		if enc := q.Encode(); enc != "" {
			urlStr += "?" + enc
		}
	}
	return fhir_http_client.SearchResources[fhir_dto.PractitionerRole](ctx, c.Log, c.Client, urlStr,
		constvars.ResourcePractitionerRole)
}

func (c *practitionerRoleFhirClient) DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error {
	return fhir_http_client.DeleteResource(ctx, c.Log, c.Client, c.BaseUrl, practitionerRoleID,
		constvars.ResourcePractitionerRole)
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	return c.searchBySuffix(ctx, fmt.Sprintf("/?organization=Organization/%s", organizationID))
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	params := url.Values{}
	params.Add("organization", fmt.Sprintf("Organization/%s", request.ClinicID))
	if request.City != "" {
		params.Add("organization.address-city", request.City)
	}
	if request.PractitionerName != "" {
		params.Add("practitioner.name:contains", request.PractitionerName)
	}
	url := fmt.Sprintf("%s?%s", c.BaseUrl, params.Encode())
	return fhir_http_client.SearchResources[fhir_dto.PractitionerRole](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitionerRole)
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error) {
	return c.searchBySuffix(ctx, fmt.Sprintf("?practitioner=Practitioner/%s", practitionerID))
}

// searchBySuffix executes a PractitionerRole search against c.BaseUrl plus the
// given URL suffix.
func (c *practitionerRoleFhirClient) searchBySuffix(ctx context.Context, suffix string) ([]fhir_dto.PractitionerRole, error) {
	url := fmt.Sprintf("%s%s", c.BaseUrl, suffix)
	return fhir_http_client.SearchResources[fhir_dto.PractitionerRole](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitionerRole)
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	url := fmt.Sprintf("%s?practitioner=Practitioner/%s", c.BaseUrl, request.PractitionerID)
	if request.OrganizationName != "" {
		url += fmt.Sprintf("&organization.name:contains=%s", request.OrganizationName)
	}
	return fhir_http_client.SearchResources[fhir_dto.PractitionerRole](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitionerRole)
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	url := fmt.Sprintf("%s?practitioner=Practitioner/%s&organization=Organization/%s",
		c.BaseUrl, practitionerID, organizationID)
	return fhir_http_client.SearchResources[fhir_dto.PractitionerRole](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitionerRole)
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

	// Post to the base FHIR server URL (not the resource-specific endpoint)
	baseURL := strings.TrimSuffix(c.BaseUrl, constvars.ResourcePractitionerRole)
	_, err = c.Client.Do(ctx, constvars.MethodPost, baseURL, bytes.NewBuffer(requestJSON))
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
	return fhir_http_client.CreateResource(ctx, c.Log, c.Client, c.BaseUrl, request,
		constvars.ResourcePractitionerRole, constvars.LoggingPractitionerRoleIDKey)
}

func (c *practitionerRoleFhirClient) UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
	return fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[fhir_dto.PractitionerRole]{
		Ctx: ctx, Log: c.Log, Client: c.Client, Method: constvars.MethodPut,
		BaseUrl: c.BaseUrl, ID: request.ID, Resource: request,
		ResourceName: constvars.ResourcePractitionerRole, IDLogKey: constvars.LoggingPractitionerRoleIDKey,
	})
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error) {
	return fhir_http_client.GetResource[fhir_dto.PractitionerRole](ctx, c.Log, c.Client, c.BaseUrl, practitionerRoleID,
		constvars.ResourcePractitionerRole, constvars.LoggingPractitionerRoleIDKey)
}
