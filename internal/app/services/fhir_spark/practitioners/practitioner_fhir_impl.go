package practitioners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"
)

type practitionerFhirClient struct {
	BaseUrl string
}

func NewPractitionerFhirClient(baseUrl string) PractitionerFhirClient {
	return &practitionerFhirClient{
		BaseUrl: baseUrl + constvars.ResourcePractitioner,
	}
}

func (c *practitionerFhirClient) CreatePractitioner(ctx context.Context, request *requests.Practitioner) (*responses.Practitioner, error) {
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitioner)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitioner)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return practitionerFhir, nil
}

func (c *practitionerFhirClient) FindPractitionerByID(ctx context.Context, PractitionerID string) (*responses.Practitioner, error) {
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitioner)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return practitionerFhir, nil
}

func (c *practitionerFhirClient) UpdatePractitioner(ctx context.Context, request *requests.Practitioner) (*responses.Practitioner, error) {
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return practitionerFhir, nil
}

func (c *practitionerFhirClient) PatchPractitioner(ctx context.Context, request *requests.Practitioner) (*responses.Practitioner, error) {
	// Convert FHIR Practitioner to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	// Send PUT request to FHIR server
	req, err := http.NewRequestWithContext(ctx, constvars.MethodPatch, fmt.Sprintf("%s/%s", c.BaseUrl, request.ID), bytes.NewBuffer(requestJSON))
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
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourcePractitioner)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourcePractitioner)
		}
	}

	practitionerFhir := new(responses.Practitioner)
	err = json.NewDecoder(resp.Body).Decode(&practitionerFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitioner)
	}

	return practitionerFhir, nil
}
