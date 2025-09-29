package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"

	"go.uber.org/zap"
)

type ServiceRequestStorage struct {
	FhirClient contracts.ServiceRequestFhirClient
	Log        *zap.Logger
}

func NewServiceRequestStorage(fhirClient contracts.ServiceRequestFhirClient, log *zap.Logger) *ServiceRequestStorage {
	return &ServiceRequestStorage{FhirClient: fhirClient, Log: log}
}

// Create stores the payload by creating a FHIR ServiceRequest and returns partner_trx_id and identifiers.
func (s *ServiceRequestStorage) Create(ctx context.Context, input *requests.CreateServiceRequestStorageInput) (*requests.CreateServiceRequestStorageOutput, error) {
	requestID, _ := ctx.Value(constvars.CONTEXT_REQUEST_ID_KEY).(string)
	s.Log.Info("ServiceRequestStorage.Create called",
		zap.String(constvars.LoggingRequestIDKey, requestID),
	)

	// Prepare note payload (raw body, instantiateUri, patientID, uid)
	notePayload := &requests.NoteStorage{
		RawBody:        input.RawBody,
		InstantiateURI: input.InstantiateURI,
		PatientID:      input.PatientID,
		UID:            input.UID,
	}
	serialized, err := json.Marshal(notePayload)
	if err != nil {
		s.Log.Error("ServiceRequestStorage.Create cannot marshal note payload",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	resource := &fhir_dto.CreateServiceRequestInput{
		ResourceType:       constvars.ResourceServiceRequest,
		Status:             "active",
		Intent:             "directive",
		OccurrenceDateTime: input.Occurrence,
		Note: []fhir_dto.Annotation{
			{Text: string(serialized)},
		},
	}
	// Set subject reference from input (Group existence ensured at bootstrap time)
	if input.Subject != "" {
		resource.Subject = fhir_dto.Reference{Reference: input.Subject}
	}

	if input.PatientID != "" {
		resource.Requester = fhir_dto.Reference{Reference: fmt.Sprintf("%s/%s", constvars.ResourcePatient, input.PatientID)}
	}

	created, err := s.FhirClient.CreateServiceRequest(ctx, resource)
	if err != nil {
		s.Log.Error("ServiceRequestStorage.Create CreateServiceRequest failed",
			zap.String(constvars.LoggingRequestIDKey, requestID),
			zap.Error(err),
		)
		return nil, err
	}

	partnerTrxID := fmt.Sprintf("%s-%s", created.ID, created.Meta.VersionId)

	return &requests.CreateServiceRequestStorageOutput{
		ServiceRequestID:      created.ID,
		ServiceRequestVersion: created.Meta.VersionId,
		PartnerTrxID:          partnerTrxID,
		Subject:               created.Subject.Reference,
	}, nil
}
