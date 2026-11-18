// Package referrals resolves referral partner directory entries from local
// FHIR resources: a partner is an Organization carrying the urn:konsulin:party
// identifier plus its active Endpoint resource.
package referrals

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/pkg/constvars"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_dto"
	"github.com/konsulin-care/konsulin-api/internal/pkg/fhir_http_client"

	"go.uber.org/zap"
)

// ErrPartnerNotFound reports that a referral partner lookup found neither an
// Organization matching the requested party identifier nor an active Endpoint
// resource belonging to it.
var ErrPartnerNotFound = errors.New("referral partner not found")

// endpointStatusActive is the FHIR Endpoint.status value accepted by the
// directory; inactive endpoints are never resolved.
const endpointStatusActive = "active"

// resourceEndpoint names the FHIR Endpoint resource type for searches and logs.
const resourceEndpoint = "Endpoint"

// endpointResource is the subset of the FHIR Endpoint resource the directory
// reads: lifecycle status, the address referrals are delivered to, and the
// references tying the endpoint back to its partner Organization.
type endpointResource struct {
	ResourceType         string               `json:"resourceType,omitempty"`
	ID                   string               `json:"id,omitempty"`
	Status               string               `json:"status,omitempty"`
	Address              string               `json:"address,omitempty"`
	ManagingOrganization fhir_dto.Reference   `json:"managingOrganization,omitempty"`
	Connection           []endpointConnection `json:"connection,omitempty"`
}

// endpointConnection is the Endpoint.connection backbone element; the target
// reference may point back at the partner Organization.
type endpointConnection struct {
	Target fhir_dto.Reference `json:"target,omitempty"`
}

// Directory resolves referral partners against the local FHIR server through
// the shared FHIR HTTP client.
type Directory struct {
	client  *fhir_http_client.FHIRHTTPClient
	baseURL string
	log     *zap.Logger
}

// NewDirectory constructs a referral partner directory.
//
// client is the shared FHIR HTTP client, baseURL is the FHIR server root
// (trailing slashes are trimmed), and log is the logger used for FHIR traffic.
func NewDirectory(client *fhir_http_client.FHIRHTTPClient, baseURL string, log *zap.Logger) *Directory {
	if log == nil {
		log = zap.NewNop()
	}
	return &Directory{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		log:     log,
	}
}

// ResolvePartner finds the partner Organization whose identifier is
// urn:konsulin:party|partyID and returns the address of its active Endpoint
// resource. It returns ErrPartnerNotFound when either resource is missing or
// when no active endpoint carries an address.
//
// Parameters:
//   - ctx: request context for the FHIR searches
//   - partyID: the party identifier value paired with the urn:konsulin:party system
//
// Returns the endpoint address and nil on success.
func (d *Directory) ResolvePartner(ctx context.Context, partyID string) (string, error) {
	partyID = strings.TrimSpace(partyID)
	if partyID == "" {
		return "", ErrPartnerNotFound
	}

	orgs, err := d.searchPartnerOrganizations(ctx, partyID)
	if err != nil {
		return "", fmt.Errorf("referrals: search partner organization: %w", err)
	}
	if len(orgs) == 0 {
		return "", ErrPartnerNotFound
	}

	orgRef := fmt.Sprintf("%s/%s", constvars.ResourceOrganization, orgs[0].ID)
	endpoints, err := d.searchActiveEndpoints(ctx, orgRef)
	if err != nil {
		return "", fmt.Errorf("referrals: search partner endpoint: %w", err)
	}

	for _, ep := range endpoints {
		if ep.Status != endpointStatusActive || ep.Address == "" {
			continue
		}
		if ep.ManagingOrganization.Reference == orgRef || endpointTargets(ep, orgRef) {
			return ep.Address, nil
		}
	}
	return "", ErrPartnerNotFound
}

// searchPartnerOrganizations searches Organizations whose identifier equals
// urn:konsulin:party|partyID.
func (d *Directory) searchPartnerOrganizations(ctx context.Context, partyID string) ([]fhir_dto.Organization, error) {
	identifier := fmt.Sprintf("%s|%s", constvars.KonsulinPartyIdentifierSystem, partyID)
	searchURL := fmt.Sprintf("%s/Organization?identifier=%s", d.baseURL, url.QueryEscape(identifier))
	return fhir_http_client.SearchResources[fhir_dto.Organization](ctx, d.log, d.client,
		searchURL, constvars.ResourceOrganization)
}

// searchActiveEndpoints searches active Endpoints scoped to orgRef. The
// server-side organization parameter narrows the result when supported; the
// caller still verifies the reference client-side.
func (d *Directory) searchActiveEndpoints(ctx context.Context, orgRef string) ([]endpointResource, error) {
	searchURL := fmt.Sprintf("%s/%s?status=%s&organization=%s", d.baseURL, resourceEndpoint,
		endpointStatusActive, url.QueryEscape(orgRef))
	return fhir_http_client.SearchResources[endpointResource](ctx, d.log, d.client,
		searchURL, resourceEndpoint)
}

// endpointTargets reports whether any Endpoint.connection.target references orgRef.
func endpointTargets(ep endpointResource, orgRef string) bool {
	for _, conn := range ep.Connection {
		if conn.Target.Reference == orgRef {
			return true
		}
	}
	return false
}
