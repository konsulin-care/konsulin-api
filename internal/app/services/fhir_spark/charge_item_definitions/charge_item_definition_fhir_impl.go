package charge_item_definitions

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

type chargeItemDefinitionFhirClient struct {
	BaseUrl string
}

func NewChargeItemDefinitionFhirClient(baseUrl string) contracts.ChargeItemDefinitionFhirClient {
	return &chargeItemDefinitionFhirClient{
		BaseUrl: baseUrl + constvars.ResourceChargeItemDefinition,
	}
}

func (c *chargeItemDefinitionFhirClient) CreateChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error) {
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
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) FindChargeItemDefinitionByID(ctx context.Context, chargeItemDefinitionID string) (*fhir_dto.ChargeItemDefinition, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, chargeItemDefinitionID), nil)
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
		if resp.StatusCode == constvars.StatusNotFound {
			return &fhir_dto.ChargeItemDefinition{}, nil
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) FindChargeItemDefinitionByPractitionerRoleID(ctx context.Context, chargeItemDefinitionID string) (*fhir_dto.ChargeItemDefinition, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, chargeItemDefinitionID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) UpdateChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

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
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	return chargeItemDefinitionFhir, nil
}

func (c *chargeItemDefinitionFhirClient) PatchChargeItemDefinition(ctx context.Context, request *fhir_dto.ChargeItemDefinition) (*fhir_dto.ChargeItemDefinition, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

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
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceChargeItemDefinition)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceChargeItemDefinition)
		}
	}

	chargeItemDefinitionFhir := new(fhir_dto.ChargeItemDefinition)
	err = json.NewDecoder(resp.Body).Decode(&chargeItemDefinitionFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceChargeItemDefinition)
	}

	return chargeItemDefinitionFhir, nil
}
