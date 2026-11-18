package referrals

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mustMarshalEntry marshals a resource into a FHIR bundle entry.
func mustMarshalEntry(t *testing.T, resource any) fhir_dto.Entry {
	t.Helper()
	raw, err := json.Marshal(resource)
	require.NoError(t, err)
	return fhir_dto.Entry{Resource: raw}
}

// partnerBundle wraps entries into a searchset bundle.
func partnerBundle(entries ...fhir_dto.Entry) fhir_dto.FHIRBundle {
	return fhir_dto.FHIRBundle{
		ResourceType: "Bundle",
		Type:         "searchset",
		Total:        len(entries),
		Entry:        entries,
	}
}

// writeBundle encodes a bundle as application/fhir+json.
func writeBundle(t *testing.T, w http.ResponseWriter, bundle fhir_dto.FHIRBundle) {
	t.Helper()
	w.Header().Set("Content-Type", "application/fhir+json")
	require.NoError(t, json.NewEncoder(w).Encode(bundle))
}

// partnerOrg returns an Organization carrying the urn:konsulin:party identifier.
func partnerOrg(id, partyID string) fhir_dto.Organization {
	return fhir_dto.Organization{
		ResourceType: constvars.ResourceOrganization,
		ID:           id,
		Active:       true,
		Identifier: []fhir_dto.Identifier{
			{System: constvars.KonsulinPartyIdentifierSystem, Value: partyID},
		},
		Name: "Partner Clinic",
	}
}

// TestResolvePartner_ResolvesActiveEndpointAddress covers the happy path:
// matching Organization identifier plus an active Endpoint yields its address.
func TestResolvePartner_ResolvesActiveEndpointAddress(t *testing.T) {
	org := partnerOrg("org-1", "party-123")
	endpoint := endpointResource{
		ResourceType:         resourceEndpoint,
		ID:                   "ep-1",
		Status:               endpointStatusActive,
		Address:              "https://partner.example/fhir",
		ManagingOrganization: fhir_dto.Reference{Reference: "Organization/org-1"},
	}

	var gotIdentifier, gotEndpointQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Organization":
			gotIdentifier = r.URL.Query().Get("identifier")
			writeBundle(t, w, partnerBundle(mustMarshalEntry(t, org)))
		case "/" + resourceEndpoint:
			gotEndpointQuery = r.URL.Query().Get("status")
			writeBundle(t, w, partnerBundle(mustMarshalEntry(t, endpoint)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := NewDirectory(fhir_http_client.New(zap.NewNop()), server.URL, zap.NewNop())
	addr, err := dir.ResolvePartner(context.Background(), "party-123")

	require.NoError(t, err)
	assert.Equal(t, "https://partner.example/fhir", addr)
	assert.Equal(t, "urn:konsulin:party|party-123", gotIdentifier)
	assert.Equal(t, endpointStatusActive, gotEndpointQuery)
}

// TestResolvePartner_MissingOrganization covers the no-partner path: an empty
// Organization search yields ErrPartnerNotFound.
func TestResolvePartner_MissingOrganization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Organization":
			writeBundle(t, w, partnerBundle())
		case "/" + resourceEndpoint:
			writeBundle(t, w, partnerBundle())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := NewDirectory(fhir_http_client.New(zap.NewNop()), server.URL, zap.NewNop())
	addr, err := dir.ResolvePartner(context.Background(), "party-unknown")

	assert.ErrorIs(t, err, ErrPartnerNotFound)
	assert.Empty(t, addr)
}

// TestResolvePartner_MissingEndpoint covers the org-without-endpoint path:
// the Organization resolves but no active Endpoint belongs to it.
func TestResolvePartner_MissingEndpoint(t *testing.T) {
	org := partnerOrg("org-1", "party-123")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Organization":
			writeBundle(t, w, partnerBundle(mustMarshalEntry(t, org)))
		case "/" + resourceEndpoint:
			// Endpoint exists but is inactive and belongs to another org.
			inactive := endpointResource{
				ResourceType: resourceEndpoint,
				ID:           "ep-2",
				Status:       "suspended",
				Address:      "https://other.example/fhir",
				ManagingOrganization: fhir_dto.Reference{
					Reference: "Organization/org-other",
				},
			}
			writeBundle(t, w, partnerBundle(mustMarshalEntry(t, inactive)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := NewDirectory(fhir_http_client.New(zap.NewNop()), server.URL, zap.NewNop())
	addr, err := dir.ResolvePartner(context.Background(), "party-123")

	assert.ErrorIs(t, err, ErrPartnerNotFound)
	assert.Empty(t, addr)
}

// TestResolvePartner_EmptyPartyID guards the blank-input shortcut.
func TestResolvePartner_EmptyPartyID(t *testing.T) {
	dir := NewDirectory(fhir_http_client.New(zap.NewNop()), "http://localhost", zap.NewNop())
	_, err := dir.ResolvePartner(context.Background(), "  ")
	assert.True(t, errors.Is(err, ErrPartnerNotFound))
}
