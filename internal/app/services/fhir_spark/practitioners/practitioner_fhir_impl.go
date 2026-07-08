package practitioners

import (
	"context"
	"fmt"
	"sync"

	"konsulin-service/internal/app/contracts"
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
	return fhir_http_client.CreateResource(ctx, c.Log, c.client, c.BaseUrl, request,
		constvars.ResourcePractitioner, constvars.LoggingPractitionerIDKey)
}

func (c *practitionerFhirClient) FindPractitionerByID(ctx context.Context, practitionerID string) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.GetResource[fhir_dto.Practitioner](ctx, c.Log, c.client, c.BaseUrl, practitionerID,
		constvars.ResourcePractitioner, constvars.LoggingPractitionerIDKey)
}

func (c *practitionerFhirClient) UpdatePractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.client, constvars.MethodPut, c.BaseUrl, request.ID, request,
		constvars.ResourcePractitioner, constvars.LoggingPractitionerIDKey)
}

func (c *practitionerFhirClient) FindPractitionerByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Practitioner, error) {
	identifierToken := fmt.Sprintf("%s|%s", system, value)
	identifierEnc := url.QueryEscape(identifierToken)
	url := fmt.Sprintf("%s?identifier=%s", c.BaseUrl, identifierEnc)
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.client, url,
		constvars.ResourcePractitioner)
}

func (c *practitionerFhirClient) FindPractitionerByEmail(ctx context.Context, email string) ([]fhir_dto.Practitioner, error) {
	emailEnc := url.QueryEscape(email)
	url := fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, emailEnc)
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.client, url,
		constvars.ResourcePractitioner)
}

func (c *practitionerFhirClient) FindPractitionerByPhone(ctx context.Context, phone string) ([]fhir_dto.Practitioner, error) {
	phoneEnc := url.QueryEscape(phone)
	url := fmt.Sprintf("%s?phone=%s&_sort=-_lastUpdated", c.BaseUrl, phoneEnc)
	return fhir_http_client.SearchResources[fhir_dto.Practitioner](ctx, c.Log, c.client, url,
		constvars.ResourcePractitioner)
}

func (c *practitionerFhirClient) PatchPractitioner(ctx context.Context, request *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.client, constvars.MethodPatch, c.BaseUrl, request.ID, request,
		constvars.ResourcePractitioner, constvars.LoggingPractitionerIDKey)
}
