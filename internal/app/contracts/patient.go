package contracts

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/fhir_dto"
)

type PatientUsecase interface {
	CreateAppointment(ctx context.Context, sessionData string, request *requests.CreateAppointmentRequest) (*responses.CreateAppointment, error)
}

type PatientFhirClient interface {
	CreatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error)
	UpdatePatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error)
	PatchPatient(ctx context.Context, request *fhir_dto.Patient) (*fhir_dto.Patient, error)
	FindPatientByID(ctx context.Context, patientID string) (*fhir_dto.Patient, error)
	FindPatientByIdentifier(ctx context.Context, system, value string) ([]fhir_dto.Patient, error)
	FindPatientByEmail(ctx context.Context, email string) ([]fhir_dto.Patient, error)
}

type PatientRepository interface{}
