package persons

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
	personFhirClientInstance contracts.PersonFhirClient
	oncePersonFhirClient     sync.Once
)

type personFhirClient struct {
	*base.ResourceClient
}

func NewPersonFhirClient(baseUrl string, logger *zap.Logger) contracts.PersonFhirClient {
	oncePersonFhirClient.Do(func() {
		personFhirClientInstance = &personFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourcePerson, logger),
		}
	})
	return personFhirClientInstance
}

func (c *personFhirClient) Create(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.Client, c.BaseUrl, person,
		constvars.ResourcePerson, "person_id")
}

func (c *personFhirClient) Update(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error) {
	return fhir_http_client.WriteResource(fhir_http_client.WriteResourceInput[fhir_dto.Person]{
		Ctx: ctx, Log: c.Log, Client: c.Client, Method: constvars.MethodPut,
		BaseUrl: c.BaseUrl, ID: person.ID, Resource: person,
		ResourceName: constvars.ResourcePerson, IDLogKey: "person_id",
	})
}

func (c *personFhirClient) Search(ctx context.Context, params contracts.PersonSearchInput) ([]fhir_dto.Person, error) {
	urlStr := c.BaseUrl
	if enc := params.ToQueryParam().Encode(); enc != "" {
		urlStr += "?" + enc
	}
	return fhir_http_client.SearchResources[fhir_dto.Person](ctx, c.Log, c.Client, urlStr,
		constvars.ResourcePerson)
}

func (c *personFhirClient) FindPersonByEmail(ctx context.Context, email string) ([]fhir_dto.Person, error) {
	return c.searchPersonsByParam(ctx, "email", email)
}

func (c *personFhirClient) FindPersonByPhone(ctx context.Context, phone string) ([]fhir_dto.Person, error) {
	return c.searchPersonsByParam(ctx, "phone", phone)
}

func (c *personFhirClient) searchPersonsByParam(ctx context.Context, param, value string) ([]fhir_dto.Person, error) {
	url := fmt.Sprintf("%s?%s=%s", c.BaseUrl, param, url.QueryEscape(value))
	return fhir_http_client.SearchResources[fhir_dto.Person](ctx, c.Log, c.Client, url,
		constvars.ResourcePerson)
}
