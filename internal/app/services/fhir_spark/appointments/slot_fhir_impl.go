package appointments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/responses"
	"konsulin-service/internal/pkg/exceptions"
	"net/http"
	"time"
)

type appointmentFhirClient struct {
	BaseUrl string
}

func NewAppointmentFhirClient(appointmentFhirBaseUrl string) AppointmentFhirClient {
	return &appointmentFhirClient{
		BaseUrl: appointmentFhirBaseUrl,
	}
}

func (c *appointmentFhirClient) CheckClinicianAvailability(ctx context.Context, clinicianId string, startTime, endTime time.Time) (bool, error) {
	url := fmt.Sprintf("%s?practitioner=Practitioner/%s&date=%s", c.BaseUrl, clinicianId, startTime.Format("2006-01-02"))
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		return false, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return false, exceptions.ErrGetFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return false, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	var bundle struct {
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			Resource struct {
				Start string `json:"start"`
				End   string `json:"end"`
			} `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&bundle)
	if err != nil {
		return false, fmt.Errorf("error decoding response: %w", err)
	}

	for _, entry := range bundle.Entry {
		existingStart, _ := time.Parse(time.RFC3339, entry.Resource.Start)
		existingEnd, _ := time.Parse(time.RFC3339, entry.Resource.End)

		if startTime.Before(existingEnd) && endTime.After(existingStart) {
			return false, nil
		}
	}

	return true, nil
}
