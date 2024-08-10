package patients

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PatientUsecase interface{}

type PatientRepository interface{}

type PatientFhirClient interface {
	CreatePatient(ctx context.Context, request *requests.PatientFhir) (*responses.Patient, error)
	UpdatePatient(ctx context.Context, request *requests.PatientFhir) (*responses.Patient, error)
	PatchPatient(ctx context.Context, request *requests.PatientFhir) (*responses.Patient, error)
	FindPatientByID(ctx context.Context, patientID string) (*responses.Patient, error)
}
