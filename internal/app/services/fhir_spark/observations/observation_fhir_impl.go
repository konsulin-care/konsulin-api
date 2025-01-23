package observations

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
)

type observationFhirClient struct {
	BaseUrl string
}

func NewObservationFhirClient(baseUrl string) contracts.ObservationFhirClient {
	return &observationFhirClient{
		BaseUrl: baseUrl + constvars.ResourceObservation,
	}
}

func (c *observationFhirClient) CreateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
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
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceObservation)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceObservation)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceObservation)
		}
	}

	observationFhir := new(fhir_dto.Observation)
	err = json.NewDecoder(resp.Body).Decode(&observationFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	return observationFhir, nil
}

func (c *observationFhirClient) FindObservationByID(ctx context.Context, observationID string) (*fhir_dto.Observation, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, observationID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceObservation)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceObservation)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceObservation)
		}
	}

	observationFhir := new(fhir_dto.Observation)
	err = json.NewDecoder(resp.Body).Decode(&observationFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	return observationFhir, nil
}

func (c *observationFhirClient) DeleteObservationByID(ctx context.Context, observationID string) error {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, observationID), nil)
	if err != nil {
		return exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusNoContent {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceObservation)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceObservation)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceObservation)
		}
	}

	return nil
}

func (c *observationFhirClient) UpdateObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	// Convert FHIR Observation to JSON
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

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceObservation)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceObservation)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceObservation)
		}
	}

	observationFhir := new(fhir_dto.Observation)
	err = json.NewDecoder(resp.Body).Decode(&observationFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	return observationFhir, nil
}

func (c *observationFhirClient) PatchObservation(ctx context.Context, request *fhir_dto.Observation) (*fhir_dto.Observation, error) {
	// Convert FHIR Observation to JSON
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

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceObservation)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceObservation)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceObservation)
		}
	}

	observationFhir := new(fhir_dto.Observation)
	err = json.NewDecoder(resp.Body).Decode(&observationFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceObservation)
	}

	return observationFhir, nil
}
