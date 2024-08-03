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

func (c *practitionerRoleFhirClient) GetPractitionerRoleByOrganizationID(ctx context.Context, organizationID string) (*responses.PractitionerRole, error) {
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

		var outcome responses.OperationOutcome
		err = json.Unmarshal(bodyBytes, &outcome)
		if err != nil {
			return nil, exceptions.ErrGetFHIRResource(err, constvars.ResourcePractitionerRole)
		}

		if len(outcome.Issue) > 0 {
			fhirErrorIssue := fmt.Errorf(outcome.Issue[0].Diagnostics)
			return nil, exceptions.ErrGetFHIRResource(fhirErrorIssue, constvars.ResourcePractitionerRole)
		}
	}

	practitionerRoleFhir := new(responses.PractitionerRole)
	err = json.NewDecoder(resp.Body).Decode(&practitionerRoleFhir)
	if err != nil {
		return nil, exceptions.ErrDecodeResponse(err, constvars.ResourcePractitionerRole)
	}

	return practitionerRoleFhir, nil
}
