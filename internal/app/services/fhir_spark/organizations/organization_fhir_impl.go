package organizations

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

type organizationFhirClient struct {
	BaseUrl string
}

func NewOrganizationFhirClient(OrganizationFhirBaseUrl string) OrganizationFhirClient {
	return &organizationFhirClient{
		BaseUrl: OrganizationFhirBaseUrl,
	}
}

func (c *organizationFhirClient) ListOrganizations(ctx context.Context, page, row int) ([]responses.Organization, int, error) {
	url := fmt.Sprintf(constvars.FhirFetchResourceWithPagination, c.BaseUrl, page, row)

	req, err := http.NewRequestWithContext(ctx, constvars.MethodGet, url, nil)
	if err != nil {
		return nil, 0, exceptions.ErrCreateHTTPRequest(err)
	}
	req.Header.Set(constvars.HeaderContentType, constvars.MIMEApplicationFHIRJSON)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, exceptions.ErrSendHTTPRequest(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != constvars.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, 0, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, 0, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, 0, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceOrganization)
		}
	}

	var result struct {
		Total int `json:"total"`
		Entry []struct {
			FullUrl  string                 `json:"fullUrl"`
			Resource responses.Organization `json:"resource"`
		} `json:"entry"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, 0, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	organizations := make([]responses.Organization, len(result.Entry))
	for i, entry := range result.Entry {
		organizations[i] = entry.Resource
	}

	return organizations, result.Total, nil
}
