package practitionerRoles

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

type practitionerRoleFhirClient struct {
	BaseUrl string
}

func NewPractitionerRoleFhirClient(practitionerRoleFhirBaseUrl string) PractitionerRoleFhirClient {
	return &practitionerRoleFhirClient{
		BaseUrl: practitionerRoleFhirBaseUrl,
	}
}

func (c *practitionerRoleFhirClient) FindPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) ([]responses.PractitionerRole, error) {
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
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourceOrganization)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourceOrganization)
		}
	}

	var result responses.FHIRBundle
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourceOrganization)
	}

	practitionerRoles := make([]responses.PractitionerRole, len(result.Entry))
	for _, entry := range result.Entry {
		var practitionerRole responses.PractitionerRole
		err := json.Unmarshal(entry.Resource, &practitionerRole)
		if err != nil {
			return nil, exceptions.ErrCannotParseJSON(err)
		}
		practitionerRoles = append(practitionerRoles, practitionerRole)
	}

	return practitionerRoles, nil
}
