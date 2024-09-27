package appointments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
)

type appointmentFhirClient struct {
	BaseUrl string
}

func NewAppointmentFhirClient(baseUrl string) AppointmentFhirClient {
	return &appointmentFhirClient{
		BaseUrl: baseUrl + constvars.ResourceAppointment,
	}
}

func (c *appointmentFhirClient) CreateAppointment(ctx context.Context, request *fhir_dto.Appointment) (*fhir_dto.Appointment, error) {
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
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceAppointment)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceAppointment)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceAppointment)
		}
	}

	appointmentFhir := new(fhir_dto.Appointment)
	err = json.NewDecoder(resp.Body).Decode(&appointmentFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceAppointment)
	}

	return appointmentFhir, nil
}
