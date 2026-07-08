package practitioners

import (
	"context"
	"fmt"
	"sync"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"
	"net/url"

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
	return fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[fhir_dto.Practitioner]{
		Ctx: ctx, Log: c.Log, Client: c.Client, Method: constvars.MethodPut,
		BaseUrl: c.BaseUrl, ID: request.ID, Resource: request,
		ResourceName: constvars.ResourcePractitioner, IDLogKey: constvars.LoggingPractitionerIDKey,
	})
}

func (c *practitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	identifierEnc := url.QueryEscape(fmt.Sprintf("%s|%s", system, value))
	url := fmt.Sprintf("%s?identifier=%s", c.BaseUrl, identifierEnc)
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitioner)
}

func (c *practitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	url := fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, url.QueryEscape(email))
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitioner)
}

func (c *practitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	url := fmt.Sprintf("%s?phone=%s&_sort=-_lastUpdated", c.BaseUrl, url.QueryEscape(phone))
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.Client, url,
		constvars.ResourcePractitioner)
}

func (c *practitionerFhirClient) PatchPractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[fhir_dto.Practitioner]{
		Ctx: ctx, Log: c.Log, Client: c.Client, Method: constvars.MethodPatch,
		BaseUrl: c.BaseUrl, ID: request.ID, Resource: request,
		ResourceName: constvars.ResourcePractitioner, IDLogKey: constvars.LoggingPractitionerIDKey,
	})
}
