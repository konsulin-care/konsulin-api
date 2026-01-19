package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/url"
)

type PersonFhirClient interface {
	FindPersonByEmail(ctx context.Context, email string) ([]fhir_dto.Person, error)
	FindPersonByPhone(ctx context.Context, phone string) ([]fhir_dto.Person, error)
	Create(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error)
	Search(ctx context.Context, params PersonSearchInput) ([]fhir_dto.Person, error)
	Update(ctx context.Context, person *fhir_dto.Person) (*fhir_dto.Person, error)
}

// PersonSearchInput captures supported Person search parameters.
// Currently supports only identifier search per FHIR Person search parameters.
// See: https://hl7.org/fhir/R4/person.html#search
type PersonSearchInput struct {
	// Identifier is a token per FHIR (system|value or just value)
	// Example: "https://login.konsulin.care/userid|d554b5e8-cbaf-4027-8ed9-860388cd3bfa"
	Identifier string
}

// ToQueryParam converts supplied fields into URL query parameters.
func (p PersonSearchInput) ToQueryParam() url.Values {
	q := url.Values{}
	if p.Identifier != "" {
		q.Add("identifier", p.Identifier)
	}
	return q
}
