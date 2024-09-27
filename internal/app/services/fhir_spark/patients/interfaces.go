package patients

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
)

type PatientUsecase interface{}

type PatientRepository interface{}

type PatientFhirClient interface {
	CreatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error)
	UpdatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error)
	PatchPatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error)
	FindPatientByID(ctx context.Context, patientID string) (*fhir_dto.Patient, error)
}
