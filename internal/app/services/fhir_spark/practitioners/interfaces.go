package practitioners

import (
	"context"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
)

type PractitionerUsecase interface{}

type PractitionerRepository interface{}

type PractitionerFhirClient interface {
	CreatePractitioner(ctx context.Context, request *requests.Practitioner) (*responses.Practitioner, error)
	UpdatePractitioner(ctx context.Context, request *requests.Practitioner) (*responses.Practitioner, error)
	PatchPractitioner(ctx context.Context, request *requests.Practitioner) (*responses.Practitioner, error)
	FindPractitionerByID(ctx context.Context, PractitionerID string) (*responses.Practitioner, error)
}
