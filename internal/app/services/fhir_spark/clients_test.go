package fhir_spark_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/observations"
	"konsulin-service/internal/app/services/fhir_spark/patients"
	"konsulin-service/internal/app/services/fhir_spark/persons"
	"konsulin-service/internal/app/services/fhir_spark/practitioners"
	"konsulin-service/internal/app/services/fhir_spark/schedules"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// resourceOperation describes one client call and the request it must send.
type resourceOperation struct {
	name, resource, method, id string
	query                      url.Values
	search                     bool
	call                       func(context.Context) (interface{}, error)
}

// resourceServer is a fake FHIR server that validates the request of the
// current operation and answers with either the resource or an
// OperationOutcome, depending on status.
type resourceServer struct {
	t       *testing.T
	current resourceOperation
	status  int
}

func (s *resourceServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	assert.Equal(s.t, s.current.method, r.Method)
	path := "/fhir/" + s.current.resource
	if s.current.id != "" {
		path += "/" + s.current.id
	}
	assert.Equal(s.t, path, r.URL.Path)
	assert.Equal(s.t, "application/fhir+json", r.Header.Get("Content-Type"))
	if len(s.current.query) == 0 {
		assert.Empty(s.t, r.URL.RawQuery)
	} else {
		assert.Equal(s.t, s.current.query, r.URL.Query())
	}
	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		var body map[string]interface{}
		assert.NoError(s.t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(s.t, s.current.resource, body["resourceType"])
	}
	w.Header().Set("Content-Type", "application/fhir+json")
	w.WriteHeader(s.status)
	assert.NoError(s.t, json.NewEncoder(w).Encode(s.responseBody()))
}

func (s *resourceServer) responseBody() map[string]interface{} {
	if s.status >= 400 {
		return map[string]interface{}{
			"resourceType": "OperationOutcome",
			"issue":        []map[string]interface{}{{"diagnostics": "test failure"}},
		}
	}
	resource := map[string]interface{}{"resourceType": s.current.resource, "id": "r1"}
	if s.current.search {
		return map[string]interface{}{"resourceType": "Bundle", "entry": []map[string]interface{}{{"resource": resource}}}
	}
	return resource
}

// runResourceOperation checks the success path (decoded resource or nil for
// deletes) and that a FHIR error response is propagated to the caller.
func runResourceOperation(t *testing.T, srv *resourceServer, op resourceOperation) {
	srv.t, srv.current, srv.status = t, op, http.StatusOK
	if op.method == http.MethodPost {
		srv.status = http.StatusCreated
	}
	got, err := op.call(context.Background())
	require.NoError(t, err)
	if op.method != http.MethodDelete {
		encoded, err := json.Marshal(got)
		require.NoError(t, err)
		require.Contains(t, string(encoded), `"id":"r1"`)
		require.Contains(t, string(encoded), `"resourceType":"`+op.resource+`"`)
	}
	srv.status = http.StatusBadRequest
	_, err = op.call(context.Background())
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "test failure"))
}

// Exercise public resource clients through their shared FHIR HTTP transport.
// Query assertions include reserved characters to detect accidental raw URL
// concatenation, and each operation verifies error responses are propagated.
func TestFHIRResourceClients(t *testing.T) {
	srv := &resourceServer{t: t}
	server := httptest.NewServer(srv)
	defer server.Close()
	logger := zap.NewNop()
	base := server.URL + "/fhir/"
	patient := patients.NewPatientFhirClient(base, logger)
	practitioner := practitioners.NewPractitionerFhirClient(base, logger)
	person := persons.NewPersonFhirClient(base, logger)
	schedule := schedules.NewScheduleFhirClient(base, logger)
	observation := observations.NewObservationFhirClient(base, logger)
	require.Same(t, patient, patients.NewPatientFhirClient("ignored", logger))
	require.Same(t, practitioner, practitioners.NewPractitionerFhirClient("ignored", logger))
	require.Same(t, person, persons.NewPersonFhirClient("ignored", logger))
	require.Same(t, schedule, schedules.NewScheduleFhirClient("ignored", logger))
	require.Same(t, observation, observations.NewObservationFhirClient("ignored", logger))
	patientResource := &fhir_dto.Patient{ResourceType: "Patient", ID: "r1"}
	practitionerResource := &fhir_dto.Practitioner{ResourceType: "Practitioner", ID: "r1"}
	personResource := &fhir_dto.Person{ResourceType: "Person", ID: "r1"}
	scheduleResource := &fhir_dto.Schedule{ResourceType: "Schedule", ID: "r1"}
	observationResource := &fhir_dto.Observation{ResourceType: "Observation", ID: "r1"}
	identifier, email, phone := "https://example.invalid/id|user&1", "user+test@example.invalid", "+6281234567890"
	operations := []resourceOperation{
		{name: "create patient", resource: "Patient", method: http.MethodPost, call: func(ctx context.Context) (interface{}, error) { return patient.CreatePatient(ctx, patientResource) }},
		{name: "get patient", resource: "Patient", method: http.MethodGet, id: "r1", call: func(ctx context.Context) (interface{}, error) { return patient.FindPatientByID(ctx, "r1") }},
		{name: "update patient", resource: "Patient", method: http.MethodPut, id: "r1", call: func(ctx context.Context) (interface{}, error) { return patient.UpdatePatient(ctx, patientResource) }},
		{name: "patch patient", resource: "Patient", method: http.MethodPatch, id: "r1", call: func(ctx context.Context) (interface{}, error) { return patient.PatchPatient(ctx, patientResource) }},
		{name: "patient identifier", resource: "Patient", method: http.MethodGet, search: true, query: url.Values{"identifier": {identifier}}, call: func(ctx context.Context) (interface{}, error) {
			return patient.FindPatientByIdentifier(ctx, identifier)
		}},
		{name: "patient email", resource: "Patient", method: http.MethodGet, search: true, query: url.Values{"email": {email}, "_sort": {"-_lastUpdated"}}, call: func(ctx context.Context) (interface{}, error) { return patient.FindPatientByEmail(ctx, email) }},
		{name: "patient phone", resource: "Patient", method: http.MethodGet, search: true, query: url.Values{"phone": {phone}, "_sort": {"-_lastUpdated"}}, call: func(ctx context.Context) (interface{}, error) { return patient.FindPatientByPhone(ctx, phone) }},
		{name: "create practitioner", resource: "Practitioner", method: http.MethodPost, call: func(ctx context.Context) (interface{}, error) {
			return practitioner.CreatePractitioner(ctx, practitionerResource)
		}},
		{name: "get practitioner", resource: "Practitioner", method: http.MethodGet, id: "r1", call: func(ctx context.Context) (interface{}, error) { return practitioner.FindPractitionerByID(ctx, "r1") }},
		{name: "update practitioner", resource: "Practitioner", method: http.MethodPut, id: "r1", call: func(ctx context.Context) (interface{}, error) {
			return practitioner.UpdatePractitioner(ctx, practitionerResource)
		}},
		{name: "patch practitioner", resource: "Practitioner", method: http.MethodPatch, id: "r1", call: func(ctx context.Context) (interface{}, error) {
			return practitioner.PatchPractitioner(ctx, practitionerResource)
		}},
		{name: "practitioner identifier", resource: "Practitioner", method: http.MethodGet, search: true, query: url.Values{"identifier": {"https://example.invalid/id|user&1"}}, call: func(ctx context.Context) (interface{}, error) {
			return practitioner.FindPractitionerByIdentifier(ctx, "https://example.invalid/id", "user&1")
		}},
		{name: "practitioner email", resource: "Practitioner", method: http.MethodGet, search: true, query: url.Values{"email": {email}, "_sort": {"-_lastUpdated"}}, call: func(ctx context.Context) (interface{}, error) {
			return practitioner.FindPractitionerByEmail(ctx, email)
		}},
		{name: "practitioner phone", resource: "Practitioner", method: http.MethodGet, search: true, query: url.Values{"phone": {phone}, "_sort": {"-_lastUpdated"}}, call: func(ctx context.Context) (interface{}, error) {
			return practitioner.FindPractitionerByPhone(ctx, phone)
		}},
		{name: "create person", resource: "Person", method: http.MethodPost, call: func(ctx context.Context) (interface{}, error) { return person.Create(ctx, personResource) }},
		{name: "update person", resource: "Person", method: http.MethodPut, id: "r1", call: func(ctx context.Context) (interface{}, error) { return person.Update(ctx, personResource) }},
		{name: "person identifier", resource: "Person", method: http.MethodGet, search: true, query: url.Values{"identifier": {identifier}}, call: func(ctx context.Context) (interface{}, error) {
			return person.Search(ctx, contracts.PersonSearchInput{Identifier: identifier})
		}},
		{name: "all persons", resource: "Person", method: http.MethodGet, search: true, call: func(ctx context.Context) (interface{}, error) {
			return person.Search(ctx, contracts.PersonSearchInput{})
		}},
		{name: "person email", resource: "Person", method: http.MethodGet, search: true, query: url.Values{"email": {email}}, call: func(ctx context.Context) (interface{}, error) { return person.FindPersonByEmail(ctx, email) }},
		{name: "person phone", resource: "Person", method: http.MethodGet, search: true, query: url.Values{"phone": {phone}}, call: func(ctx context.Context) (interface{}, error) { return person.FindPersonByPhone(ctx, phone) }},
		{name: "create schedule", resource: "Schedule", method: http.MethodPost, call: func(ctx context.Context) (interface{}, error) { return schedule.CreateSchedule(ctx, scheduleResource) }},
		{name: "practitioner schedule", resource: "Schedule", method: http.MethodGet, search: true, query: url.Values{"actor": {"Practitioner/r1"}}, call: func(ctx context.Context) (interface{}, error) {
			return schedule.FindScheduleByPractitionerID(ctx, "r1")
		}},
		{name: "practitioner role schedule", resource: "Schedule", method: http.MethodGet, search: true, query: url.Values{"actor": {"PractitionerRole/r1"}}, call: func(ctx context.Context) (interface{}, error) {
			return schedule.FindScheduleByPractitionerRoleID(ctx, "r1")
		}},
		{name: "schedule ID", resource: "Schedule", method: http.MethodGet, search: true, query: url.Values{"_id": {"r1"}}, call: func(ctx context.Context) (interface{}, error) {
			return schedule.Search(ctx, contracts.ScheduleSearchParams{ID: "r1"})
		}},
		{name: "all schedules", resource: "Schedule", method: http.MethodGet, search: true, call: func(ctx context.Context) (interface{}, error) {
			return schedule.Search(ctx, contracts.ScheduleSearchParams{})
		}},
		{name: "create observation", resource: "Observation", method: http.MethodPost, call: func(ctx context.Context) (interface{}, error) {
			return observation.CreateObservation(ctx, observationResource)
		}},
		{name: "get observation", resource: "Observation", method: http.MethodGet, id: "r1", call: func(ctx context.Context) (interface{}, error) { return observation.FindObservationByID(ctx, "r1") }},
		{name: "update observation", resource: "Observation", method: http.MethodPut, id: "r1", call: func(ctx context.Context) (interface{}, error) {
			return observation.UpdateObservation(ctx, observationResource)
		}},
		{name: "patch observation", resource: "Observation", method: http.MethodPatch, id: "r1", call: func(ctx context.Context) (interface{}, error) {
			return observation.PatchObservation(ctx, observationResource)
		}},
		{name: "delete observation", resource: "Observation", method: http.MethodDelete, id: "r1", call: func(ctx context.Context) (interface{}, error) {
			return nil, observation.DeleteObservationByID(ctx, "r1")
		}},
	}
	for _, op := range operations {
		t.Run(op.name, func(t *testing.T) { runResourceOperation(t, srv, op) })
	}
}
