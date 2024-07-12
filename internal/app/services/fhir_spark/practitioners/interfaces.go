package practitioners

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PractitionerUsecase interface{}

type PractitionerRepository interface{}

type PractitionerFhirClient interface {
	CreatePractitioner(ctx context.Context, Practitioner *requests.PractitionerFhir) (*responses.Practitioner, error)
	UpdatePractitioner(ctx context.Context, Practitioner *requests.PractitionerFhir) (*responses.Practitioner, error)
	GetPractitionerByID(ctx context.Context, PractitionerID string) (*responses.Practitioner, error)
}
