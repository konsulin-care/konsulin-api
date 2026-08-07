package middlewares

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/fhir_dto"

	"github.com/stretchr/testify/assert"
)

// mockPatientClient returns configurable results for PatientFhirClient.
type mockPatientClient struct {
	patients []fhir_dto.Patient
	err      error
}

func (m *mockPatientClient) FindPatientByIdentifier(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return m.patients, m.err
}

func (*mockPatientClient) FindPatientByID(_ context.Context, _ string) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) FindPatientByEmail(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) FindPatientByPhone(_ context.Context, _ string) ([]fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) CreatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) UpdatePatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}
func (*mockPatientClient) PatchPatient(_ context.Context, _ *fhir_dto.Patient) (*fhir_dto.Patient, error) {
	return nil, nil
}

// mockPractitionerClient returns configurable results for PractitionerFhirClient.
type mockPractitionerClient struct {
	practitioners []fhir_dto.Practitioner
	err           error
}

func (m *mockPractitionerClient) FindPractitionerByIdentifier(_ context.Context, _, _ string) ([]fhir_dto.Practitioner, error) {
	return m.practitioners, m.err
}

func (*mockPractitionerClient) FindPractitionerByID(_ context.Context, _ string) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) FindPractitionerByEmail(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) FindPractitionerByPhone(_ context.Context, _ string) ([]fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) CreatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) UpdatePractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}
func (*mockPractitionerClient) PatchPractitioner(_ context.Context, _ *fhir_dto.Practitioner) (*fhir_dto.Practitioner, error) {
	return nil, nil
}

func TestResolveFHIRIdentity_ActiveRolePatient(t *testing.T) {
	// When activeRole is "Patient" and a Patient resource exists, resolveFHIRIdentity
	// should return the Patient role even if a Practitioner resource also exists.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{
				{ID: "prac-1"},
			},
		},
		PatientFhirClient: &mockPatientClient{
			patients: []fhir_dto.Patient{
				{ID: "pat-1"},
			},
		},
	}

	ctx := context.WithValue(context.Background(), keyActiveRole, constvars.KonsulinRolePatient)
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePatient, role)
	assert.Equal(t, "pat-1", id)
}

func TestResolveFHIRIdentity_ActiveRoleEmpty(t *testing.T) {
	// When activeRole is empty (not set), the default Practitioner-first behavior applies.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{
				{ID: "prac-1"},
			},
		},
		PatientFhirClient: &mockPatientClient{
			patients: []fhir_dto.Patient{
				{ID: "pat-1"},
			},
		},
	}

	ctx := context.Background()
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePractitioner, role)
	assert.Equal(t, "prac-1", id)
}

func TestNeedsFHIRResolution(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{"Patient role needs resolution", []string{constvars.KonsulinRolePatient}, true},
		{"Practitioner role needs resolution", []string{constvars.KonsulinRolePractitioner}, true},
		{"Clinic Admin does not need resolution", []string{constvars.KonsulinRoleClinicAdmin}, false},
		{"Researcher does not need resolution", []string{constvars.KonsulinRoleResearcher}, false},
		{"Guest does not need resolution", []string{constvars.KonsulinRoleGuest}, false},
		{"Multiple roles with Patient needs resolution", []string{constvars.KonsulinRoleResearcher, constvars.KonsulinRolePatient}, true},
		{"Multiple roles without Patient/Practitioner", []string{constvars.KonsulinRoleClinicAdmin, constvars.KonsulinRoleResearcher}, false},
		{"Empty roles does not need resolution", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsFHIRResolution(tt.roles)
			if got != tt.want {
				t.Errorf("needsFHIRResolution(%v) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}

func TestResolveFHIRIdentity_ActiveRolePatient_MultiMatch(t *testing.T) {
	// When activeRole is "Patient" and multiple Patient FHIR resources exist,
	// resolveFHIRIdentity should return an error mirroring the Practitioner multi-match guard.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{},
		PatientFhirClient: &mockPatientClient{
			patients: []fhir_dto.Patient{
				{ID: "pat-1"},
				{ID: "pat-2"},
			},
		},
	}

	ctx := context.WithValue(context.Background(), keyActiveRole, constvars.KonsulinRolePatient)
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiple Patient resources")
	assert.Empty(t, role)
	assert.Empty(t, id)
}

func TestOwnsPatientQuery_OwnResourceAllowed(t *testing.T) {
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{},
		PractitionerFhirClient: &mockPractitionerClient{},
	}

	// Patient accessing their own resource via direct path
	u, _ := url.Parse("/Patient/pat-1")
	got := ownsPatientQuery(context.Background(), "pat-1", u, "Patient", mw.PatientFhirClient, mw.PractitionerFhirClient)
	assert.True(t, got, "patient should own their own resource via path ID match")
}

func TestOwnsPatientQuery_CrossPatientIDOR(t *testing.T) {
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{},
		PractitionerFhirClient: &mockPractitionerClient{},
	}

	// Patient A must NOT access /Patient/pat-B directly by ID
	// This is the IDOR guard: when the path targets a different Patient ID,
	// we must NOT fall through to query-based checks.
	u, _ := url.Parse("/Patient/pat-B")
	got := ownsPatientQuery(context.Background(), "pat-A", u, "Patient", mw.PatientFhirClient, mw.PractitionerFhirClient)
	assert.False(t, got, "patient must not access another patient's resource directly by ID")
}

func TestOwnsPatientQuery_IDORViaQueryParamFallthrough(t *testing.T) {
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{},
		PractitionerFhirClient: &mockPractitionerClient{},
	}

	// IDOR attack: Patient A requests /Patient/pat-B?_id=pat-A
	// The path targets pat-B (not the requester), but the _id query param
	// matches fhirID in checkPatientQueryRefs. Currently this falls through
	// and grants access — it must NOT.
	u, _ := url.Parse("/Patient/pat-B?_id=pat-A")
	got := ownsPatientQuery(context.Background(), "pat-A", u, "Patient", mw.PatientFhirClient, mw.PractitionerFhirClient)
	assert.False(t, got, "patient must not access another patient's resource even with matching _id query param")
}

func TestOwnsPatientQuery_FHIRPrefixedOwnResourceAllowed(t *testing.T) {
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{},
		PractitionerFhirClient: &mockPractitionerClient{},
	}

	u, _ := url.Parse("/fhir/Patient/pat-1")
	got := ownsPatientQuery(context.Background(), "pat-1", u, "Patient", mw.PatientFhirClient, mw.PractitionerFhirClient)
	assert.True(t, got, "patient should own their own resource via fhir-prefixed path")
}

func TestOwnsPatientQuery_FHIRPrefixedCrossPatientIDOR(t *testing.T) {
	mw := &Middlewares{
		PatientFhirClient:      &mockPatientClient{},
		PractitionerFhirClient: &mockPractitionerClient{},
	}

	u, _ := url.Parse("/fhir/Patient/pat-B")
	got := ownsPatientQuery(context.Background(), "pat-A", u, "Patient", mw.PatientFhirClient, mw.PractitionerFhirClient)
	assert.False(t, got, "patient must not access another patient via fhir-prefixed path")
}

func TestResolveFHIRIdentity_ActiveRolePatient_NoPatientResource(t *testing.T) {
	// When activeRole is "Patient" but no Patient FHIR resource exists,
	// it should fall through to Practitioner lookup.
	mw := &Middlewares{
		PractitionerFhirClient: &mockPractitionerClient{
			practitioners: []fhir_dto.Practitioner{
				{ID: "prac-1"},
			},
		},
		PatientFhirClient: &mockPatientClient{
			patients: nil, // No patient resource
		},
	}

	ctx := context.WithValue(context.Background(), keyActiveRole, constvars.KonsulinRolePatient)
	role, id, err := mw.resolveFHIRIdentity(ctx, "user-123")

	assert.NoError(t, err)
	assert.Equal(t, constvars.KonsulinRolePractitioner, role)
	assert.Equal(t, "prac-1", id)
}

func TestOwnsPostBody(t *testing.T) {
	tests := []struct {
		name         string
		fhirID       string
		role         string
		resourceType string
		body         string
		want         bool
	}{
		{
			name:         "Patient consents for self",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceConsent,
			body:         `{"resourceType":"Consent","status":"active","patient":{"reference":"Patient/pat-1"}}`,
			want:         true,
		},
		{
			name:         "Patient consenting for another patient is denied",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceConsent,
			body:         `{"resourceType":"Consent","status":"active","patient":{"reference":"Patient/pat-2"}}`,
			want:         false,
		},
		{
			name:         "Patient ResearchSubject for self",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceResearchSubject,
			body:         `{"resourceType":"ResearchSubject","status":"active","individual":{"reference":"Patient/pat-1"}}`,
			want:         true,
		},
		{
			name:         "Patient ResearchSubject for another patient is denied",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceResearchSubject,
			body:         `{"resourceType":"ResearchSubject","status":"active","individual":{"reference":"Patient/pat-2"}}`,
			want:         false,
		},
		{
			name:         "Missing patient field is lenient",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceConsent,
			body:         `{"resourceType":"Consent","status":"active"}`,
			want:         true,
		},
		{
			name:         "Missing individual field is lenient",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceResearchSubject,
			body:         `{"resourceType":"ResearchSubject","status":"active"}`,
			want:         true,
		},
		{
			name:         "Non-Patient reference prefix in patient field is denied",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceConsent,
			body:         `{"resourceType":"Consent","status":"active","patient":{"reference":"Practitioner/prac-1"}}`,
			want:         false,
		},
		{
			name:         "Non-Patient role is allowed",
			fhirID:       "prac-1",
			role:         constvars.KonsulinRolePractitioner,
			resourceType: constvars.ResourceConsent,
			body:         `{"resourceType":"Consent","status":"active","patient":{"reference":"Patient/pat-2"}}`,
			want:         true,
		},
		{
			name:         "Unrelated resource type is allowed",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceObservation,
			body:         `{"resourceType":"Observation","subject":{"reference":"Patient/pat-2"}}`,
			want:         true,
		},
		{
			name:         "Empty body is allowed",
			fhirID:       "pat-1",
			role:         constvars.KonsulinRolePatient,
			resourceType: constvars.ResourceConsent,
			body:         ``,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ownsPostBody(tt.fhirID, tt.role, tt.resourceType, []byte(tt.body))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestOwnsResource_PostConsentOwnership(t *testing.T) {
	// A Patient may POST a Consent only when patient.reference is their own id.
	got := ownsResource(context.Background(), "pat-1", "/fhir/Consent", constvars.KonsulinRolePatient, constvars.MethodPost, nil, nil, nil, nil, nil,
		[]byte(`{"resourceType":"Consent","status":"active","patient":{"reference":"Patient/pat-1"}}`))
	assert.True(t, got, "Patient should own Consent POST referencing themselves")

	got = ownsResource(context.Background(), "pat-1", "/fhir/Consent", constvars.KonsulinRolePatient, constvars.MethodPost, nil, nil, nil, nil, nil,
		[]byte(`{"resourceType":"Consent","status":"active","patient":{"reference":"Patient/pat-2"}}`))
	assert.False(t, got, "Patient must not POST a Consent for another patient")
}

func TestCheckPatientRefs_ResearchSubjectIndividual(t *testing.T) {
	// PUT ownership for ResearchSubject relies on individual.reference.
	assert.True(t, checkPatientRefs(`{"resourceType":"ResearchSubject","status":"active","individual":{"reference":"Patient/pat-1"}}`, "pat-1"),
		"patient should own a ResearchSubject referencing themselves via individual")
	assert.False(t, checkPatientRefs(`{"resourceType":"ResearchSubject","status":"active","individual":{"reference":"Patient/pat-2"}}`, "pat-1"),
		"patient must not own a ResearchSubject referencing another patient")
}

func TestExtractPathResourceID(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		wantResource string
		wantID       string
	}{
		{
			name:         "/Patient/pat-1",
			path:         "/Patient/pat-1",
			wantResource: "Patient",
			wantID:       "pat-1",
		},
		{
			name:         "/fhir/Patient/pat-1",
			path:         "/fhir/Patient/pat-1",
			wantResource: "Patient",
			wantID:       "pat-1",
		},
		{
			name:         "/fhir/Observation/obs-1",
			path:         "/fhir/Observation/obs-1",
			wantResource: "Observation",
			wantID:       "obs-1",
		},
		{
			name:         "/Practitioner/prac-1",
			path:         "/Practitioner/prac-1",
			wantResource: "Practitioner",
			wantID:       "prac-1",
		},
		{
			name:         "empty path",
			path:         "",
			wantResource: "",
			wantID:       "",
		},
		{
			name:         "single segment",
			path:         "/Patient",
			wantResource: "",
			wantID:       "",
		},
		{
			name:         "/fhir only",
			path:         "/fhir",
			wantResource: "",
			wantID:       "",
		},
		{
			name:         "/fhir/Patient (no ID) falls through",
			path:         "/fhir/Patient",
			wantResource: "",
			wantID:       "",
		},
		{
			name:         "case insensitive fhir",
			path:         "/FHIR/Patient/pat-1",
			wantResource: "Patient",
			wantID:       "pat-1",
		},
		{
			name:         "/Organization/org-1",
			path:         "/Organization/org-1",
			wantResource: "Organization",
			wantID:       "org-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, id := extractPathResourceID(tt.path)
			assert.Equal(t, tt.wantResource, res)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestValidateCommunicationSenderInBody(t *testing.T) {
	mw := &Middlewares{}

	t.Run("own sender passes", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","sender":{"reference":"Patient/pat-1"},"recipient":[{"reference":"Patient/pat-2"}]}`
		assert.NoError(t, mw.validateCommunicationSenderInBody([]byte(body), "pat-1"))
	})

	t.Run("other patient sender rejected", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","sender":{"reference":"Patient/pat-2"}}`
		err := mw.validateCommunicationSenderInBody([]byte(body), "pat-1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pat-2")
	})

	t.Run("missing sender is lenient", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","recipient":[{"reference":"Patient/pat-2"}]}`
		assert.NoError(t, mw.validateCommunicationSenderInBody([]byte(body), "pat-1"))
	})

	t.Run("non-patient sender rejected", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","sender":{"reference":"Practitioner/prac-1"}}`
		err := mw.validateCommunicationSenderInBody([]byte(body), "pat-1")
		assert.Error(t, err)
	})
}

func TestValidatePatientCommunicationSender(t *testing.T) {
	t.Run("own sender passes", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","sender":{"reference":"Patient/pat-1"},"recipient":[{"reference":"Patient/pat-2"}]}`
		assert.True(t, validatePatientCommunicationSender(body, "pat-1"))
	})

	t.Run("other patient sender rejected", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","sender":{"reference":"Patient/pat-2"}}`
		assert.False(t, validatePatientCommunicationSender(body, "pat-1"))
	})

	t.Run("missing sender rejected", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","recipient":[{"reference":"Patient/pat-2"}]}`
		assert.False(t, validatePatientCommunicationSender(body, "pat-1"))
	})

	t.Run("non-patient sender rejected", func(t *testing.T) {
		body := `{"resourceType":"Communication","status":"completed","sender":{"reference":"Practitioner/prac-1"}}`
		assert.False(t, validatePatientCommunicationSender(body, "pat-1"))
	})
}

func TestCheckScopedEntryRead(t *testing.T) {
	const patientID = "pat-1"
	tests := []struct {
		name         string
		method       string
		resourceType string
		roles        []string
		fhirID       string
		rawURL       string
		wantErr      bool
	}{
		{
			name:         "patient scoped communication read allowed",
			method:       http.MethodGet,
			resourceType: constvars.ResourceCommunication,
			roles:        []string{constvars.KonsulinRolePatient},
			fhirID:       patientID,
			rawURL:       "http://blaze/fhir/Communication?sender=Patient/pat-1",
		},
		{
			name:         "patient unscoped communication read denied",
			method:       http.MethodGet,
			resourceType: constvars.ResourceCommunication,
			roles:        []string{constvars.KonsulinRolePatient},
			fhirID:       patientID,
			rawURL:       "http://blaze/fhir/Communication?status=completed",
			wantErr:      true,
		},
		{
			name:         "aggregate count stays public",
			method:       http.MethodGet,
			resourceType: constvars.ResourceQuestionnaireResponse,
			roles:        []string{constvars.KonsulinRolePatient},
			fhirID:       patientID,
			rawURL:       "http://blaze/fhir/QuestionnaireResponse?_summary=count",
		},
		{
			name:         "non-GET exempt",
			method:       http.MethodPost,
			resourceType: constvars.ResourceCommunication,
			rawURL:       "http://blaze/fhir/Communication",
		},
		{
			name:         "non-scoped resource exempt",
			method:       http.MethodGet,
			resourceType: constvars.ResourcePatient,
			rawURL:       "http://blaze/fhir/Patient",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkScopedEntryRead(tt.method, tt.resourceType, tt.roles, tt.fhirID, tt.rawURL)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsReferralCommunicationPut(t *testing.T) {
	assert.True(t, isReferralCommunicationPut(http.MethodPut, constvars.ResourceCommunication, "/fhir/Communication/referral-abc123"))
	assert.False(t, isReferralCommunicationPut(http.MethodPost, constvars.ResourceCommunication, "/fhir/Communication/referral-abc123"))
	assert.False(t, isReferralCommunicationPut(http.MethodPut, constvars.ResourceObservation, "/fhir/Observation/referral-abc123"))
	assert.False(t, isReferralCommunicationPut(http.MethodPut, constvars.ResourceCommunication, "/fhir/Communication/comm-1"))
	assert.False(t, isReferralCommunicationPut(http.MethodPut, constvars.ResourceCommunication, "/fhir/Communication"))
}
