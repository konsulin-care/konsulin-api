package patients

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PatientUsecase interface {
	GetPatientProfileBySession(ctx context.Context, sessionData string) (*responses.PatientProfile, error)
	UpdatePatientProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdateProfile, error)
}

type PatientRepository interface{}

type PatientFhirClient interface {
	CreatePatient(ctx context.Context, patient *requests.PatientFhir) (*models.Patient, error)
	UpdatePatient(ctx context.Context, patient *requests.PatientFhir) (*models.Patient, error)
	GetPatientByID(ctx context.Context, patientID string) (*models.Patient, error)
}
