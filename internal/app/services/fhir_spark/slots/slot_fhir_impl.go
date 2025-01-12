package slots

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
	"time"
)

type slotFhirClient struct {
	BaseUrl string
}

func NewSlotFhirClient(baseUrl string) contracts.SlotFhirClient {
	return &slotFhirClient{
		BaseUrl: baseUrl + constvars.ResourceSlot,
	}
}

func (c *slotFhirClient) FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]fhir_dto.Slot, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/schedule=Schedule/%s", c.BaseUrl, scheduleID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	slotsFhir := make([]fhir_dto.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slotsFhir[i] = entry.Resource
	}

	return slotsFhir, nil
}

func (c *slotFhirClient) FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]fhir_dto.Slot, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s?schedule=Schedule/%s&status=%s", c.BaseUrl, scheduleID, status), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	slotsFhir := make([]fhir_dto.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slotsFhir[i] = entry.Resource
	}

	return slotsFhir, nil
}

func (c *slotFhirClient) CreateSlot(ctx context.Context, request *fhir_dto.Slot) (*fhir_dto.Slot, error) {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, fmt.Sprintf("%s", c.BaseUrl), bytes.NewBuffer(requestJSON))
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	slotFhir := new(fhir_dto.Slot)
	err = json.NewDecoder(resp.Body).Decode(&slotFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	return slotFhir, nil
}

func (c *slotFhirClient) FindSlotByScheduleAndTimeRange(ctx context.Context, scheduleID string, startTime time.Time, endTime time.Time) ([]fhir_dto.Slot, error) {
	queryURL := fmt.Sprintf(
		"%s?schedule=Schedule/%s&start=eq%s&end=eq%s",
		c.BaseUrl,
		scheduleID,
		startTime.Format(time.RFC3339),
		endTime.Format(time.RFC3339),
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, queryURL, nil)
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
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, exceptions.ErrGetFHIRResource(readErr, constvars.ResourceSlot)
		}

		var outcome fhir_dto.OperationOutcome
		if jsonErr := json.Unmarshal(bodyBytes, &outcome); jsonErr != nil {
			return nil, exceptions.ErrGetFHIRResource(jsonErr, constvars.ResourceSlot)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string        `json:"fullUrl"`
			Resource fhir_dto.Slot `json:"resource"`
		} `json:"entry"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	slots := make([]fhir_dto.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slots[i] = entry.Resource
	}

	return slots, nil
}
