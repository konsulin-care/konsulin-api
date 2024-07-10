package practitioners

import (
	"context"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PractitionerUsecase interface {
	GetPractitionerProfileBySession(ctx context.Context, sessionData string) (*responses.PractitionerProfile, error)
	UpdatePractitionerProfileBySession(ctx context.Context, sessionData string, request *requests.UpdateProfile) (*responses.UpdatePractitionerProfile, error)
}

type PractitionerRepository interface{}

type PractitionerFhirClient interface {
	CreatePractitioner(ctx context.Context, Practitioner *requests.PractitionerFhir) (*models.Practitioner, error)
	UpdatePractitioner(ctx context.Context, Practitioner *requests.PractitionerFhir) (*models.Practitioner, error)
	GetPractitionerByID(ctx context.Context, PractitionerID string) (*models.Practitioner, error)
}
