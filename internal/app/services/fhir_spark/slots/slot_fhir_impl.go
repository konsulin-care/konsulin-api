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
	"time"
)

type slotFhirClient struct {
	BaseUrl string
}

func NewSlotFhirClient(slotFhirBaseUrl string) SlotFhirClient {
	return &slotFhirClient{
		BaseUrl: slotFhirBaseUrl,
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

	var result responses.FHIRBundle
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	slotsFhir := make([]responses.Slot, len(result.Entry))
	for _, entry := range result.Entry {
		var schedule responses.Slot
		err := json.Unmarshal(entry.Resource, &schedule)
		if err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		slotsFhir = append(slotsFhir, schedule)
	}

	return slotsFhir, nil
}

func (c *slotFhirClient) CreateSlotOnDemand(ctx context.Context, clinicianId, date, startTime string, endTime time.Time) (*responses.Slot, error) {
	slotRequest := &requests.Slot{
		Schedule: requests.Reference{
			Reference: fmt.Sprintf("PractitionerRole/%s", clinicianId),
		},
		Status: "free",
		Start:  fmt.Sprintf("%sT%s:00Z", date, startTime),
		End:    endTime.Format(time.RFC3339),
	}

	requestJSON, err := json.Marshal(slotRequest)
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
