package slots

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

type slotFhirClient struct {
	BaseUrl string
}

func NewSlotFhirClient(baseUrl string) SlotFhirClient {
	return &slotFhirClient{
		BaseUrl: baseUrl + constvars.ResourceSlot,
	}
}

func (c *slotFhirClient) FindSlotByScheduleID(ctx context.Context, scheduleID string) ([]responses.Slot, error) {
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

		var outcome responses.OperationOutcome
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
			FullUrl  string         `json:"fullUrl"`
			Resource responses.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	slotsFhir := make([]responses.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slotsFhir[i] = entry.Resource
	}

	return slotsFhir, nil
}

func (c *slotFhirClient) FindSlotByScheduleIDAndStatus(ctx context.Context, scheduleID, status string) ([]responses.Slot, error) {
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

		var outcome responses.OperationOutcome
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
			FullUrl  string         `json:"fullUrl"`
			Resource responses.Slot `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	slotsFhir := make([]responses.Slot, len(result.Entry))
	for i, entry := range result.Entry {
		slotsFhir[i] = entry.Resource
	}

	return slotsFhir, nil
}

func (c *slotFhirClient) CreateSlot(ctx context.Context, request *requests.Slot) (*responses.Slot, error) {
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

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSlot)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSlot)
		}
	}

	slotFhir := new(responses.Slot)
	err = json.NewDecoder(resp.Body).Decode(&slotFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceSlot)
	}

	return slotFhir, nil
}
