package practitionerRoles

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/fhir_dto"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type practitionerRoleFhirClient struct {
	BaseFhirUrl string
	BaseUrl     string
}

func NewPractitionerRoleFhirClient(baseUrl string) contracts.PractitionerRoleFhirClient {
	return &practitionerRoleFhirClient{
		BaseFhirUrl: baseUrl,
		BaseUrl:     baseUrl + constvars.ResourcePractitionerRole,
	}
}

func (c *practitionerRoleFhirClient) DeletePractitionerRoleByID(ctx context.Context, practitionerRoleID string) error {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, practitionerRoleID), nil)
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
			return exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	return nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/?organization=Organization/%s", c.BaseUrl, organizationID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByCustomRequest(ctx context.Context, request *requests.FindAllCliniciansByClinicID) ([]fhir_dto.PractitionerRole, error) {
	params := url.Values{}

	params.Add("organization", fmt.Sprintf("Organization/%s", request.ClinicID))

	if request.City != "" {
		params.Add("organization.address-city", request.City)
	}
	if request.PractitionerName != "" {
		params.Add("practitioner.name:contains", request.PractitionerName)
	}

	url := fmt.Sprintf("%s?%s", c.BaseUrl, params.Encode())

	req, err := http.NewRequestWithContext(
		ctx,
		constvars.MethodGet,
		url,
		nil,
	)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	desiredDays := strings.Split(request.Days, ",")
	startTimeFilterParsed, _ := time.Parse(constvars.TimeFormatHoursMinutesSeconds, request.StartTime)
	endTimeFilterParsed, _ := time.Parse(constvars.TimeFormatHoursMinutesSeconds, request.EndTime)
	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))

	for i, entry := range result.Entry {
		matched := false
		for _, availableTime := range entry.Resource.AvailableTime {
			for _, availableDay := range availableTime.DaysOfWeek {
				for _, desiredDay := range desiredDays {
					if strings.ToLower(availableDay) == desiredDay {
						startTime, _ := time.Parse(constvars.TimeFormatHoursMinutesSeconds, availableTime.AvailableStartTime)
						endTime, _ := time.Parse(constvars.TimeFormatHoursMinutesSeconds, availableTime.AvailableEndTime)
						if startTime.Hour() >= startTimeFilterParsed.Hour() && endTime.Hour() <= endTimeFilterParsed.Hour() {
							matched = true
							break
						}
					}
				}
				if matched {
					break
				}
			}
			if matched {
				practitionerRoles[i] = entry.Resource
				break
			}
		}
	}

	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerID(ctx context.Context, practitionerID string) ([]fhir_dto.PractitionerRole, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s?practitioner=Practitioner/%s", c.BaseUrl, practitionerID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndName(ctx context.Context, request *requests.FindClinicianByClinicianID) ([]fhir_dto.PractitionerRole, error) {
	url := fmt.Sprintf("%s?practitioner=Practitioner/%s", c.BaseUrl, request.PractitionerID)

	if request.OrganizationName != "" {
		url += fmt.Sprintf("&organization.name:contains=%s", request.OrganizationName)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		constvars.MethodGet,
		url,
		nil,
	)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByPractitionerIDAndOrganizationID(ctx context.Context, practitionerID, organizationID string) ([]fhir_dto.PractitionerRole, error) {
	url := fmt.Sprintf(
		"%s?practitioner=Practitioner/%s&organization=Organization/%s",
		c.BaseUrl,
		practitionerID,
		organizationID,
	)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                    `json:"fullUrl"`
			Resource fhir_dto.PractitionerRole `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	practitionerRoles := make([]fhir_dto.PractitionerRole, len(result.Entry))
	for i, entry := range result.Entry {
		practitionerRoles[i] = entry.Resource
	}

	return practitionerRoles, nil
}

func (c *practitionerRoleFhirClient) CreatePractitionerRoles(ctx context.Context, request interface{}) error {
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return exceptions.ErrCannotMarshalJSON(err)
	}

	req, err := http.NewRequestWithContext(ctx, constvars.MethodPost, c.BaseFhirUrl, bytes.NewBuffer(requestJSON))
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

	if resp.StatusCode != constvars.StatusOK && resp.StatusCode != constvars.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	return nil
}

func (c *practitionerRoleFhirClient) CreatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
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
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	practitionerRoleFhir := new(fhir_dto.PractitionerRole)
	err = json.NewDecoder(resp.Body).Decode(&practitionerRoleFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	return practitionerRoleFhir, nil
}

func (c *practitionerRoleFhirClient) UpdatePractitionerRole(ctx context.Context, request *fhir_dto.PractitionerRole) (*fhir_dto.PractitionerRole, error) {
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

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	practitionerRoleFhir := new(fhir_dto.PractitionerRole)
	err = json.NewDecoder(resp.Body).Decode(&practitionerRoleFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	return practitionerRoleFhir, nil
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByID(ctx context.Context, practitionerRoleID string) (*fhir_dto.PractitionerRole, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, practitionerRoleID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	practitionerRole := new(fhir_dto.PractitionerRole)
	err = json.NewDecoder(resp.Body).Decode(&practitionerRole)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	return practitionerRole, nil
}
