package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"konsulin-service/internal/app/contracts"
)

// partyRegistryGuard proves at compile time that the adapter satisfies
// contracts.PartyRegistry — the interface and the implementation must land
// together so the package compiles at every commit.
var _ contracts.PartyRegistry = (*Client)(nil)

// partyJWKS is the minimal party JWKS document IsEnrolled decodes: only the
// kid list is read — key material never enters this package or any log.
type partyJWKS struct {
	Keys []struct {
		Kid string `json:"kid"`
	} `json:"keys"`
}

// IsEnrolled fetches GET /v1/parties/{id}/jwks for the adapter's own party
// through the SDK client and reports whether the served JWKS contains the
// adapter's own kid (client.go Kid(), RFC 7638) — the only enrollment check,
// with the gateway as the source of truth (BK-PARTYID-002). It fails closed:
// a disabled adapter returns contracts.ErrGatewayNotConfigured before any
// I/O, a transport or decode failure returns false with the error, and a JWKS
// without the adapter's kid returns (false, nil). Implements
// contracts.PartyRegistry.
// Parameters: ctx bounds the JWKS fetch.
// Returns: whether the party JWKS serves the adapter's own kid, and the
// error explaining a failed probe. Never panics; never logs key values.
func (c *Client) IsEnrolled(ctx context.Context) (bool, error) {
	if !c.enabled || c.kid == "" {
		return false, contracts.ErrGatewayNotConfigured
	}
	resp, err := c.do(ctx, http.MethodGet,
		"/v1/parties/"+url.PathEscape(c.partyID)+"/jwks", nil, false, "")
	if err != nil {
		return false, fmt.Errorf("gateway: party jwks: fetch: %w", err)
	}
	var jwks partyJWKS
	if err := json.Unmarshal(resp.Body, &jwks); err != nil {
		return false, fmt.Errorf("gateway: party jwks: decode: %w", err)
	}
	for _, k := range jwks.Keys {
		if k.Kid == c.kid {
			return true, nil
		}
	}
	return false, nil
}
