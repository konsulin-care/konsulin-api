package contracts

import (
	"context"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/url"
)

type InvoiceFhirClient interface {
	Search(ctx context.Context, params InvoiceSearchParams) ([]fhir_dto.Invoice, error)
}

type InvoiceSearchParams struct {
	ID string
}

// ToQueryParam converts InvoiceSearchParams into URL query parameters
func (p InvoiceSearchParams) ToQueryParam() url.Values {
	params := url.Values{}

	if p.ID != "" {
		params.Add("_id", p.ID)
	}

	return params
}
