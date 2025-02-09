package questionnaireResponses

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
)

type questionnaireResponseFhirClient struct {
	BaseUrl string
}

func NewQuestionnaireResponseFhirClient(baseUrl string) contracts.QuestionnaireResponseFhirClient {
	return &questionnaireResponseFhirClient{
		BaseUrl: baseUrl + constvars.ResourceQuestionnaireResponse,
	}
}

func (c *questionnaireResponseFhirClient) CreateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
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
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrCreateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrCreateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	return questionnaireResponseFhir, nil
}

func (c *questionnaireResponseFhirClient) FindQuestionnaireResponses(ctx context.Context, request *requests.FindAllAssessmentResponse) ([]fhir_dto.QuestionnaireResponse, error) {
	url := c.BaseUrl

	if request.PatientID != "" {
		url += fmt.Sprintf("?subject=%s/%s", constvars.ResourcePatient, request.PatientID)
		if request.AssessmentID != "" {
			url += fmt.Sprintf("&questionnaire=%s/%s", constvars.ResourceQuestionnaire, request.AssessmentID)
		}
	}

	fmt.Println(url)

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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			fmt.Println("here2")
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			fmt.Println("here3", fhirErrorIssue)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	var result struct {
		Total        int    `json:"total"`
		ResourceType string `json:"resourceType"`
		Entry        []struct {
			FullUrl  string                         `json:"fullUrl"`
			Resource fhir_dto.QuestionnaireResponse `json:"resource"`
		} `json:"entry"`
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	questionnaireResponses := make([]fhir_dto.QuestionnaireResponse, len(result.Entry))
	for i, entry := range result.Entry {
		questionnaireResponses[i] = entry.Resource
	}

	return questionnaireResponses, nil
}

func (c *questionnaireResponseFhirClient) FindQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) (*fhir_dto.QuestionnaireResponse, error) {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireResponseID), nil)
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	return questionnaireResponseFhir, nil
}

func (c *questionnaireResponseFhirClient) DeleteQuestionnaireResponseByID(ctx context.Context, questionnaireResponseID string) error {
	req, err := http.NewRequestWithContext(ctx, constvars.MethodDelete, fmt.Sprintf("%s/%s", c.BaseUrl, questionnaireResponseID), nil)
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
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return exceptions.ErrGetFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	return nil
}

func (c *questionnaireResponseFhirClient) UpdateQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	// Convert FHIR QuestionnaireResponse to JSON
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
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	return questionnaireResponseFhir, nil
}

func (c *questionnaireResponseFhirClient) PatchQuestionnaireResponse(ctx context.Context, request *fhir_dto.QuestionnaireResponse) (*fhir_dto.QuestionnaireResponse, error) {
	// Convert FHIR QuestionnaireResponse to JSON
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
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		var outcome fhir_dto.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrUpdateFHIRResource(err, constvars.ResourceQuestionnaireResponse)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrUpdateFHIRResource(fhirErrorIssue, constvars.ResourceQuestionnaireResponse)
		}
	}

	questionnaireResponseFhir := new(fhir_dto.QuestionnaireResponse)
	err = json.NewDecoder(resp.Body).Decode(&questionnaireResponseFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceQuestionnaireResponse)
	}

	return questionnaireResponseFhir, nil
}
