package practitioners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"
)

type practitionerFhirClient struct {
	BaseUrl string
}

func NewPractitionerFhirClient(PractitionerFhirBaseUrl string) PractitionerFhirClient {
	return &practitionerFhirClient{
		BaseUrl: PractitionerFhirBaseUrl,
	}
}

func (c *practitionerFhirClient) CreatePractitioner(ctx context.Context, request *requests.PractitionerFhir) (*responses.Practitioner, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseUrl, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusCreated {
		return nil, exceptions.ErrCreateFHIRResource(nil, constvars.ResourcePractitioner)
	}

	PractitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&PractitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return PractitionerFhir, nil
}

func (c *practitionerFhirClient) GetPractitionerByID(ctx context.Context, PractitionerID string) (*responses.Practitioner, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, PractitionerID), nil)
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		return nil, exceptions.ErrGetFHIRResource(nil, constvars.ResourcePractitioner)
	}

	PractitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&PractitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return PractitionerFhir, nil
}

func (c *practitionerFhirClient) UpdatePractitioner(ctx context.Context, request *requests.PractitionerFhir) (*responses.Practitioner, error) {
	// Convert FHIR Practitioner to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	// Send PUT request to FHIR server
	req, err := http.NewRequestWithContext(ctx, constvars.MethodPut, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		return nil, exceptions.ErrUpdateFHIRResource(nil, constvars.ResourcePractitioner)
	}

	PractitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&PractitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return PractitionerFhir, nil
}
