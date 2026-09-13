package contracts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPractitionerRoleSearchParams_ToQueryParam pins the PractitionerRole search
// query construction, including the code token as system|code.
func TestPractitionerRoleSearchParams_ToQueryParam(t *testing.T) {
	active := true
	params := PractitionerRoleSearchParams{
		Active:         &active,
		PractitionerID: "pr-123",
		OrganizationID: "org-1",
		Code:           "http://terminology.hl7.org/CodeSystem/practitioner-role|researcher",
		Elements:       []string{"code"},
	}
	q := params.ToQueryParam()

	assert.Equal(t, "true", q.Get("active"))
	assert.Equal(t, "Practitioner/pr-123", q.Get("practitioner"))
	assert.Equal(t, "Organization/org-1", q.Get("organization"))
	assert.Equal(t, "http://terminology.hl7.org/CodeSystem/practitioner-role|researcher", q.Get("code"))
	assert.Equal(t, "code", q.Get("_elements"))

	// url.Values encodes the token so the pipe and scheme are safe in a URL.
	encoded := q.Encode()
	assert.Contains(t, encoded, "code=http%3A%2F%2Fterminology.hl7.org%2FCodeSystem%2Fpractitioner-role%7Cresearcher")
}

func TestPractitionerRoleSearchParams_ToQueryParam_CodeOnly(t *testing.T) {
	params := PractitionerRoleSearchParams{Code: "224608005"}
	q := params.ToQueryParam()
	assert.Equal(t, "224608005", q.Get("code"))
}
