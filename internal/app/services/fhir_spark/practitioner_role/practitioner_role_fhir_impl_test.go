package practitionerRoles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/services/fhir_spark/base"
	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/dto/requests"
	"konsulin-service/internal/pkg/fhir_dto"
	"konsulin-service/internal/pkg/fhir_http_client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// sharedFHIRServer returns an httptest.Server that mocks a FHIR PractitionerRole endpoint.
// Each route handler is extracted into a separate function to keep cognitive complexity low.
func sharedFHIRServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")

		pract := r.URL.Query().Get("practitioner")
		org := r.URL.Query().Get("organization")
		hasActive := strings.Contains(r.URL.RawQuery, "active")
		isIDPath := strings.Count(r.URL.Path, "/") == 2

		switch {
		case r.Method == http.MethodGet && pract != "" && org != "":
			writeRoleComboSearchResult(w)
		case r.Method == http.MethodGet && pract != "":
			handleRoleByPractitioner(w, r, pract)
		case r.Method == http.MethodGet && org != "":
			handleRoleByOrganization(w, r, org)
		case r.Method == http.MethodGet && hasActive:
			writeRoleActiveSearchResult(w)
		case r.Method == http.MethodGet && isIDPath:
			handleRoleGetByID(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/PractitionerRole":
			handleRoleCreate(w, r)
		case r.Method == http.MethodPut && isIDPath:
			handleRoleUpdate(w, r)
		case r.Method == http.MethodDelete && isIDPath:
			handleRoleDelete(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/":
			writeRoleTransactionResult(w)
		default:
			writeOperationOutcome(w, http.StatusNotFound,
				fmt.Sprintf("unexpected request: %s %s", r.Method, r.URL.String()))
		}
	}))
}

func writeOperationOutcome(w http.ResponseWriter, status int, diagnostics string) {
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(fhir_dto.OperationOutcome{
		ResourceType: "OperationOutcome",
		Issue: []fhir_dto.Issue{
			{Severity: "error", Diagnostics: diagnostics},
		},
	})
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

// writeRoleComboSearchResult writes the practitioner + organization combined search response.
func writeRoleComboSearchResult(w http.ResponseWriter) {
	bundle := fhir_dto.FHIRBundle{
		ResourceType: "Bundle",
		Type:         "searchset",
		Total:        1,
		Entry: []fhir_dto.Entry{
			{Resource: mustMarshal(fhir_dto.PractitionerRole{
				ResourceType: "PractitionerRole",
				ID:           "role-combo",
			})},
		},
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bundle)
}

// handleRoleByPractitioner handles GET with ?practitioner={ref}.
func handleRoleByPractitioner(w http.ResponseWriter, r *http.Request, pract string) {
	if pract == "Practitioner/prac-456" {
		bundle := fhir_dto.FHIRBundle{
			ResourceType: "Bundle",
			Type:         "searchset",
			Total:        1,
			Entry: []fhir_dto.Entry{
				{Resource: mustMarshal(fhir_dto.PractitionerRole{
					ResourceType: "PractitionerRole",
					ID:           "role-789",
					Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-456"},
				})},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(bundle)
		return
	}
	writeOperationOutcome(w, http.StatusNotFound, "no roles found for practitioner")
}

// handleRoleByOrganization handles GET with ?organization={ref}.
func handleRoleByOrganization(w http.ResponseWriter, r *http.Request, org string) {
	if org == "Organization/org-111" {
		bundle := fhir_dto.FHIRBundle{
			ResourceType: "Bundle",
			Type:         "searchset",
			Total:        2,
			Entry: []fhir_dto.Entry{
				{Resource: mustMarshal(fhir_dto.PractitionerRole{
					ResourceType: "PractitionerRole",
					ID:           "role-aaa",
					Organization: fhir_dto.Reference{Reference: "Organization/org-111"},
				})},
				{Resource: mustMarshal(fhir_dto.PractitionerRole{
					ResourceType: "PractitionerRole",
					ID:           "role-bbb",
					Organization: fhir_dto.Reference{Reference: "Organization/org-111"},
				})},
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(bundle)
		return
	}
	writeOperationOutcome(w, http.StatusNotFound, "no roles found for organization")
}

// writeRoleActiveSearchResult writes the search response for ?active&_elements.
func writeRoleActiveSearchResult(w http.ResponseWriter) {
	bundle := fhir_dto.FHIRBundle{
		ResourceType: "Bundle",
		Type:         "searchset",
		Total:        1,
		Entry: []fhir_dto.Entry{
			{Resource: mustMarshal(fhir_dto.PractitionerRole{
				ResourceType: "PractitionerRole",
				ID:           "role-active-1",
				Active:       true,
			})},
		},
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(bundle)
}

// handleRoleGetByID handles GET /PractitionerRole/{id}.
func handleRoleGetByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/PractitionerRole/")
	if id == "role-123" {
		resp := fhir_dto.PractitionerRole{
			ResourceType: "PractitionerRole",
			ID:           "role-123",
			Active:       true,
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}
	writeOperationOutcome(w, http.StatusNotFound, "PractitionerRole/"+id+" not found")
}

// handleRoleCreate handles POST /PractitionerRole.
func handleRoleCreate(w http.ResponseWriter, r *http.Request) {
	var req fhir_dto.PractitionerRole
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOperationOutcome(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.ID = "role-new-1"
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(req)
}

// handleRoleUpdate handles PUT /PractitionerRole/{id}.
func handleRoleUpdate(w http.ResponseWriter, r *http.Request) {
	var req fhir_dto.PractitionerRole
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeOperationOutcome(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(req)
}

// handleRoleDelete handles DELETE /PractitionerRole/{id}.
func handleRoleDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/PractitionerRole/")
	if id == "role-123" || id == "role-to-delete" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeOperationOutcome(w, http.StatusNotFound, "PractitionerRole/"+id+" not found")
}

// writeRoleTransactionResult writes the transaction bundle POST / response.
func writeRoleTransactionResult(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"resourceType": "Bundle", "id": "transaction-1"})
}

// newTestClient creates a fresh practitionerRoleFhirClient for testing, bypassing
// the singleton pattern to allow each test to use its own httptest server.
func newTestClient(srvURL string, logger *zap.Logger) *practitionerRoleFhirClient {
	if srvURL[len(srvURL)-1] != '/' {
		srvURL += "/"
	}
	return &practitionerRoleFhirClient{
		ResourceClient: &base.ResourceClient{
			BaseUrl: srvURL + constvars.ResourcePractitionerRole,
			Log:     logger,
			Client:  fhir_http_client.New(logger),
		},
	}
}

func TestPractitionerRole_FindPractitionerRoleByID_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	role, err := client.FindPractitionerRoleByID(context.Background(), "role-123")
	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "role-123", role.ID)
	assert.Equal(t, "PractitionerRole", role.ResourceType)
	assert.True(t, role.Active)
}

func TestPractitionerRole_FindPractitionerRoleByID_NotFound(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	_, err := client.FindPractitionerRoleByID(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPractitionerRole_FindPractitionerRoleByPractitionerID_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	roles, err := client.FindPractitionerRoleByPractitionerID(context.Background(), "prac-456")
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "role-789", roles[0].ID)
}

func TestPractitionerRole_FindPractitionerRoleByPractitionerID_NotFound(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	_, err := client.FindPractitionerRoleByPractitionerID(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestPractitionerRole_FindPractitionerRoleByOrganizationID_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	roles, err := client.FindPractitionerRoleByOrganizationID(context.Background(), "org-111")
	require.NoError(t, err)
	assert.Len(t, roles, 2)
	assert.Equal(t, "role-aaa", roles[0].ID)
	assert.Equal(t, "role-bbb", roles[1].ID)
}

func TestPractitionerRole_FindPractitionerRoleByOrganizationID_NotFound(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	_, err := client.FindPractitionerRoleByOrganizationID(context.Background(), "org-nonexistent")
	require.Error(t, err)
}

func TestPractitionerRole_FindPractitionerRoleByPractitionerIDAndOrganizationID_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	roles, err := client.FindPractitionerRoleByPractitionerIDAndOrganizationID(context.Background(), "prac-456", "org-111")
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "role-combo", roles[0].ID)
}

func TestPractitionerRole_CreatePractitionerRole_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	req := &fhir_dto.PractitionerRole{
		ResourceType: "PractitionerRole",
		Practitioner: fhir_dto.Reference{Reference: "Practitioner/prac-456"},
		Active:       true,
	}

	role, err := client.CreatePractitionerRole(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "role-new-1", role.ID)
	assert.True(t, role.Active)
	assert.Equal(t, "Practitioner/prac-456", role.Practitioner.Reference)
}

func TestPractitionerRole_UpdatePractitionerRole_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	req := &fhir_dto.PractitionerRole{
		ID:           "role-123",
		ResourceType: "PractitionerRole",
		Active:       true,
	}

	role, err := client.UpdatePractitionerRole(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, role)
	assert.Equal(t, "role-123", role.ID)
	assert.True(t, role.Active)
}

func TestPractitionerRole_DeletePractitionerRoleByID_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	err := client.DeletePractitionerRoleByID(context.Background(), "role-to-delete")
	require.NoError(t, err)
}

func TestPractitionerRole_DeletePractitionerRoleByID_NotFound(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	err := client.DeletePractitionerRoleByID(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestPractitionerRole_Search_ByActive(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	active := true
	params := contracts.PractitionerRoleSearchParams{
		Active:   &active,
		Elements: []string{"id"},
	}

	roles, err := client.Search(context.Background(), params)
	require.NoError(t, err)
	assert.Len(t, roles, 1)
	assert.Equal(t, "role-active-1", roles[0].ID)
}

func TestPractitionerRole_FindPractitionerRoleByCustomRequest_Success(t *testing.T) {
	server := sharedFHIRServer()
	defer server.Close()

	client := newTestClient(server.URL, zap.NewNop())

	req := &requests.FindAllCliniciansByClinicID{
		ClinicID: "org-111",
	}

	roles, err := client.FindPractitionerRoleByCustomRequest(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, roles, 2)
}
