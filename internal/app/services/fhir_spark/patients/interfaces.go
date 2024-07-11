package patients

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PatientUsecase interface{}

type PatientRepository interface{}

type PatientFhirClient interface {
	CreatePatient(ctx context.Context, patient *requests.PatientFhir) (*responses.Patient, error)
	UpdatePatient(ctx context.Context, patient *requests.PatientFhir) (*responses.Patient, error)
	GetPatientByID(ctx context.Context, patientID string) (*responses.Patient, error)
}
