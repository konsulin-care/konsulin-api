package patients

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
	patientFhirClientInstance contracts.PatientFhirClient
	oncePatientFhirClient     sync.Once
)

type patientFhirClient struct {
	BaseUrl string
	Log     *zap.Logger
	client  *fhir_http_client.FHIRHTTPClient
}

func NewPatientFhirClient(baseUrl string, logger *zap.Logger) contracts.PatientFhirClient {
	oncePatientFhirClient.Do(func() {
		client := &patientFhirClient{
			BaseUrl: baseUrl + constvars.ResourcePatient,
			Log:     logger,
			client:  fhir_http_client.New(logger),
		}
		patientFhirClientInstance = client
	})
	return patientFhirClientInstance
}

func (c *patientFhirClient) CreatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return fhir_http_client.CreateResource(ctx, c.Log, c.client, c.BaseUrl, request,
		constvars.ResourcePatient, constvars.LoggingPatientIDKey)
}

func (c *patientFhirClient) FindPatientByID(ctx context.Context, patientID string) (*fhir_dto.Patient, error) {
	return fhir_http_client.GetResource[fhir_dto.Patient](ctx, c.Log, c.client, c.BaseUrl, patientID,
		constvars.ResourcePatient, constvars.LoggingPatientIDKey)
}

func (c *patientFhirClient) FindPatientByIdentifier(ctx context.Context, identifier string) ([]fhir_dto.Patient, error) {
	url := fmt.Sprintf("%s?identifier=%s", c.BaseUrl, url.QueryEscape(identifier))
	return fhir_http_client.SearchResources[fhir_dto.Patient](ctx, c.Log, c.client, url,
		constvars.ResourcePatient)
}

func (c *patientFhirClient) UpdatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.client, constvars.MethodPut, c.BaseUrl, request.ID, request,
		constvars.ResourcePatient, constvars.LoggingPatientIDKey)
}

func (c *patientFhirClient) PatchPatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return fhir_http_client.WriteResource(ctx, c.Log, c.client, constvars.MethodPatch, c.BaseUrl, request.ID, request,
		constvars.ResourcePatient, constvars.LoggingPatientIDKey)
}

func (c *patientFhirClient) FindPatientByEmail(ctx context.Context, email string) ([]fhir_dto.Patient, error) {
	emailEnc := url.QueryEscape(email)
	url := fmt.Sprintf("%s?email=%s&_sort=-_lastUpdated", c.BaseUrl, emailEnc)
	return fhir_http_client.SearchResources[fhir_dto.Patient](ctx, c.Log, c.client, url,
		constvars.ResourcePatient)
}

func (c *patientFhirClient) FindPatientByPhone(ctx context.Context, phone string) ([]fhir_dto.Patient, error) {
	phoneEnc := url.QueryEscape(phone)
	url := fmt.Sprintf("%s?phone=%s&_sort=-_lastUpdated", c.BaseUrl, phoneEnc)
	return fhir_http_client.SearchResources[fhir_dto.Patient](ctx, c.Log, c.client, url,
		constvars.ResourcePatient)
}
