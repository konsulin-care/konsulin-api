package persons

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

func (c *personFhirClient) Create(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.client, c.BaseUrl, person,
		constvars.ResourcePerson, "person_id")
}

func (c *personFhirClient) Update(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.client, constvars.MethodPut, c.BaseUrl, person.ID, person,
		constvars.ResourcePerson, "person_id")
}

func (c *personFhirClient) Search(ctx context.Context, params contracts.PersonSearchInput) ([]fhir_dto.Person, error) {
	urlStr := c.BaseUrl
	if enc := params.ToQueryParam().Encode(); enc != "" {
		urlStr += "?" + enc
	}
	return fhir_http_client.SearchResources[fhir_dto.Person](ctx, c.Log, c.client, urlStr,
		constvars.ResourcePerson)
}

func (c *personFhirClient) FindPersonByEmail(ctx context.Context, email string) ([]fhir_dto.Person, error) {
	return c.searchPersonsByParam(ctx, "email", email)
}

func (c *personFhirClient) FindPersonByPhone(ctx context.Context, phone string) ([]fhir_dto.Person, error) {
	return c.searchPersonsByParam(ctx, "phone", phone)
}

// searchPersonsByParam is a shared helper for FindPersonByEmail and FindPersonByPhone.
func (c *personFhirClient) searchPersonsByParam(ctx context.Context, param, value string) ([]fhir_dto.Person, error) {
	url := fmt.Sprintf("%s?%s=%s", c.BaseUrl, param, url.QueryEscape(value))
	return fhir_http_client.SearchResources[fhir_dto.Person](ctx, c.Log, c.client, url,
		constvars.ResourcePerson)
}
