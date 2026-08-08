package practitioners

import (
	"context"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
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
	*base.ResourceClient
}

func NewPractitionerFhirClient(baseUrl string, logger *zap.Logger) contracts.PractitionerFhirClient {
	oncePractitionerFhirClient.Do(func() {
		practitionerFhirClientInstance = &practitionerFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourcePractitioner, logger),
		}
	})
	return practitionerFhirClientInstance
}

func (c *practitionerFhirClient) CreatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.Client, c.BaseUrl, request,
		constvars.ResourcePractitioner, constvars.LoggingPractitionerIDKey)
}

func (c *practitionerFhirClient) FindPractitionerByID(ctx context.Context, practitionerID string) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.GetResource[fhir_dto.Practitioner](ctx, c.Log, c.Client, c.BaseUrl, practitionerID,
		constvars.ResourcePractitioner, constvars.LoggingPractitionerIDKey)
}

func (c *practitionerFhirClient) UpdatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return c.writePractitioner(ctx, constvars.MethodPut, request)
}

func (c *practitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	return c.searchByQuery(ctx, fmt.Sprintf("?identifier=%s", url.QueryEscape(fmt.Sprintf("%s|%s", system, value))))
}

func (c *practitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	return c.searchByQuery(ctx, fmt.Sprintf("?email=%s&_sort=-_lastUpdated", url.QueryEscape(email)))
}

func (c *practitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	return c.searchByQuery(ctx, fmt.Sprintf("?phone=%s&_sort=-_lastUpdated", url.QueryEscape(phone)))
}

func (c *practitionerFhirClient) PatchPractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return c.writePractitioner(ctx, constvars.MethodPatch, request)
}

// searchByQuery executes a Practitioner search against c.BaseUrl plus the
// query suffix.
func (c *practitionerFhirClient) searchByQuery(ctx context.Context, query string) ([]fhir_dto.Practitioner, error) {
	url := fmt.Sprintf("%s%s", c.BaseUrl, query)
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitioner)
}

// writePractitioner writes the practitioner resource via WriteResource with
// the given HTTP method.
func (c *practitionerFhirClient) writePractitioner(ctx context.Context, method string, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[fhir_dto.Practitioner]{
		Ctx: ctx, Log: c.Log, Client: c.Client, Method: method,
		BaseUrl: c.BaseUrl, ID: request.ID, Resource: request,
		ResourceName: constvars.ResourcePractitioner, IDLogKey: constvars.LoggingPractitionerIDKey,
	})
}
