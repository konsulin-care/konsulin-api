package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type ObservationFhirClient interface {
	CreateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error)
	UpdateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error)
	PatchObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error)
	FindObservationByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.Observation, error)
	DeleteObservationByID(ctx context.Context, questionnaireResponseID string) error
}
