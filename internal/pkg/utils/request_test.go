package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
)

func TestHTTPQueryParsing(t *testing.T) {
	for _, tc := range []struct {
		query         string
		page, size    int
		search, fetch string
	}{
		{"", 1, 10, "", ""},
		{"?page=2&page_size=25&search=test&fetch_type=upcoming", 2, 25, "test", "upcoming"},
		{"?page=invalid&page_size=-1&fetch_type=other", 1, 10, "", ""},
		{"?page=0&page_size=invalid", 1, 10, "", ""},
	} {
		r := httptest.NewRequest(http.MethodGet, "/fhir/Patient"+tc.query, http.NoBody)
		require.Equal(t, &requests.Pagination{Page: tc.page, PageSize: tc.size}, BuildPaginationRequest(r))
		require.Equal(t, &requests.QueryParams{Search: tc.search, FetchType: tc.fetch}, BuildQueryParamsRequest(r))
	}
	parsed, err := ParseJSONBody([]byte(`{"enabled":true,"count":2}`))
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{"enabled": true, "count": float64(2)}, parsed)
	_, err = ParseJSONBody([]byte(`{`))
	require.Error(t, err)
}

func TestFHIRRegistrationContactMappings(t *testing.T) {
	email := "test+tag@example.invalid"
	phone := "+628123456789"
	patient := BuildFhirPatientRegistrationRequest("user", email)
	require.Equal(t, "Patient", patient.ResourceType)
	require.Equal(t, []fhir_dto.ContactPoint{{System: fhir_dto.ContactPointSystemEmail, Value: email, Use: constvars.FhirAddressUseHome}}, patient.Telecom)
	practitioner := BuildFhirPractitionerRegistrationRequest("user", email)
	require.Equal(t, "Practitioner", practitioner.ResourceType)
	require.Equal(t, []fhir_dto.ContactPoint{{System: fhir_dto.ContactPointSystemEmail, Value: email, Use: constvars.FhirAddressUseWork}}, practitioner.Telecom)
	patientPhone := BuildFhirPatientWhatsAppRegistrationRequest(phone)
	practitionerPhone := BuildFhirPractitionerWhatsAppRegistrationRequest(phone)
	want := []fhir_dto.ContactPoint{{System: fhir_dto.ContactPointSystemPhone, Value: phone, Use: constvars.FhirTelecomUseMobile}}
	require.Equal(t, want, patientPhone.Telecom)
	require.Equal(t, want, practitionerPhone.Telecom)
	require.Equal(t, "Patient", patientPhone.ResourceType)
	require.Equal(t, "Practitioner", practitionerPhone.ResourceType)
}

func TestFHIRProfileUpdateMappings(t *testing.T) {
	input := &requests.UpdateProfile{Fullname: "Test User", Email: "test@example.invalid", BirthDate: "1990-01-02", WhatsAppNumber: "+628123456789", Address: "Street, City", Gender: "female", Educations: []string{"Degree", "Course"}}
	patient := BuildFhirPatientUpdateProfileRequest(input, "patient1")
	practitioner := BuildFhirPractitionerUpdateProfileRequest(input, "pr1")
	require.Equal(t, "patient1", patient.ID)
	require.Equal(t, "pr1", practitioner.ID)
	require.True(t, patient.Active)
	require.True(t, practitioner.Active)
	require.Equal(t, []string{input.Fullname}, patient.Name[0].Given)
	require.Equal(t, input.Fullname, practitioner.Name[0].Family)
	// Patients publish home contact details; practitioners publish work ones.
	for _, tc := range []struct {
		output interface{}
		use    string
	}{{patient, constvars.FhirAddressUseHome}, {practitioner, constvars.FhirAddressUseWork}} {
		body, err := json.Marshal(tc.output)
		require.NoError(t, err)
		var common struct {
			Gender, BirthDate string
			Telecom           []fhir_dto.ContactPoint
			Address           []fhir_dto.Address
			Extension         []fhir_dto.Extension
		}
		require.NoError(t, json.Unmarshal(body, &common))
		require.Equal(t, input.Gender, common.Gender)
		require.Equal(t, input.BirthDate, common.BirthDate)
		require.Equal(t, []fhir_dto.ContactPoint{{System: fhir_dto.ContactPointSystemEmail, Value: input.Email, Use: tc.use}, {System: fhir_dto.ContactPointSystemPhone, Value: input.WhatsAppNumber, Use: constvars.FhirTelecomUseMobile}}, common.Telecom)
		require.Equal(t, []fhir_dto.Address{{Use: tc.use, Line: []string{"Street", "City"}}}, common.Address)
		require.Equal(t, []fhir_dto.Extension{{Url: constvars.FhirEducationExtensionURL, ValueString: "Degree"}, {Url: constvars.FhirEducationExtensionURL, ValueString: "Course"}}, common.Extension)
	}
}

func TestFHIRActivationAndOrganizationRoleMappings(t *testing.T) {
	require.Equal(t, &fhir_dto.Patient{ResourceType: "Patient", ID: "p1", Active: false}, BuildFhirPatientDeactivateRequest("p1"))
	require.Equal(t, &fhir_dto.Patient{ResourceType: "Patient", ID: "p1", Active: true}, BuildFhirPatientReactivateRequest("p1"))
	require.Equal(t, &fhir_dto.Practitioner{ResourceType: "Practitioner", ID: "pr1", Active: false}, BuildFhirPractitionerDeactivateRequest("pr1"))
	require.Equal(t, &fhir_dto.Practitioner{ResourceType: "Practitioner", ID: "pr1", Active: true}, BuildFhirPractitionerReactivateRequest("pr1"))
	bundle := BuildPractitionerRolesBundleRequestByPractitionerID("pr1", []string{"org1", "org2"})
	encoded, err := json.Marshal(bundle)
	require.NoError(t, err)
	var decoded struct {
		ResourceType, Type string
		Entry              []struct {
			Resource fhir_dto.PractitionerRole
			Request  map[string]string
		}
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, "Bundle", decoded.ResourceType)
	require.Equal(t, "transaction", decoded.Type)
	require.Len(t, decoded.Entry, 2)
	for i, entry := range decoded.Entry {
		require.Equal(t, "PractitionerRole", entry.Resource.ResourceType)
		require.True(t, entry.Resource.Active)
		require.Equal(t, "Practitioner/pr1", entry.Resource.Practitioner.Reference)
		require.Equal(t, "Organization/"+[]string{"org1", "org2"}[i], entry.Resource.Organization.Reference)
		require.Equal(t, map[string]string{"method": "POST", "url": "PractitionerRole"}, entry.Request)
	}
}

func TestAvailableTimeMappings(t *testing.T) {
	input := []requests.AvailableTimeRequest{{DaysOfWeek: []string{"mon", "wed"}, AvailableStartTime: "09:00", AvailableEndTime: "17:00"}}
	model := ConvertToModelAvailableTimes(input)
	require.Len(t, model, 1)
	require.Equal(t, input[0].DaysOfWeek, model[0].DaysOfWeek)
	require.Equal(t, "09:00", model[0].AvailableStartTime)
	require.Equal(t, "17:00", model[0].AvailableEndTime)
	response := ConvertToAvailableTimesResponse(model)
	require.Len(t, response, 1)
	require.Equal(t, input[0].DaysOfWeek, response[0].DaysOfWeek)
	require.Equal(t, "09:00", response[0].AvailableStartTime)
	require.Equal(t, "17:00", response[0].AvailableEndTime)
	require.Empty(t, ConvertToModelAvailableTimes(nil))
	require.Empty(t, ConvertToAvailableTimesResponse(nil))
}
