package schedules

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"
)

type scheduleFhirClient struct {
	BaseUrl string
}

func NewScheduleFhirClient(scheduleFhirBaseUrl string) ScheduleFhirClient {
	return &scheduleFhirClient{
		BaseUrl: scheduleFhirBaseUrl,
	}
}

func (c *scheduleFhirClient) FindScheduleByPractitionerID(ctx context.Context, practitionerID string) ([]responses.Schedule, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s?actor=Practitioner/%s", c.BaseUrl, practitionerID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceSchedule)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceSchedule)
		}
	}

	var result responses.FHIRBundle
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	schedulesFhir := make([]responses.Schedule, len(result.Entry))
	for _, entry := range result.Entry {
		var schedule responses.Schedule
		err := json.Unmarshal(entry.Resource, &schedule)
		if err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		schedulesFhir = append(schedulesFhir, schedule)
	}

	return schedulesFhir, nil
}
