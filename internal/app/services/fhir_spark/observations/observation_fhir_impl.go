package observations

import (
	"context"
	"sync"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

var (
	observationFhirClientInstance contracts.ObservationFhirClient
	onceObservationFhirClient     sync.Once
)

type observationFhirClient struct {
	*base.ResourceClient
}

func NewObservationFhirClient(baseUrl string, logger *zap.Logger) contracts.ObservationFhirClient {
	onceObservationFhirClient.Do(func() {
		observationFhirClientInstance = &observationFhirClient{
			ResourceClient: base.New(baseUrl, constvars.ResourceObservation, logger),
		}
	})
	return observationFhirClientInstance
}

func (c *observationFhirClient) CreateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.Client, c.BaseUrl, request,
		constvars.ResourceObservation, constvars.LoggingObservationIDKey)
}

func (c *observationFhirClient) FindObservationByID(ctx context.Context, observationID string) (*fhir_dto.Observation, error) {
	return fhir_http_client.GetResource[fhir_dto.Observation](ctx, c.Log, c.Client, c.BaseUrl, observationID,
		constvars.ResourceObservation, constvars.LoggingObservationIDKey)
}

func (c *observationFhirClient) DeleteObservationByID(ctx context.Context, observationID string) error {
	return fhir_http_client.DeleteResource(ctx, c.Log, c.Client, c.BaseUrl, observationID,
		constvars.ResourceObservation)
}

func (c *observationFhirClient) UpdateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.Client, constvars.MethodPut, c.BaseUrl, request.ID, request,
		constvars.ResourceObservation, constvars.LoggingObservationIDKey)
}

func (c *observationFhirClient) PatchObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.Client, constvars.MethodPatch, c.BaseUrl, request.ID, request,
		constvars.ResourceObservation, constvars.LoggingObservationIDKey)
}
