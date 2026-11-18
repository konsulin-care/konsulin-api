// Package services hosts cross-cutting helpers shared by the core usecases:
// resolving a service provider alias to the providing gateway party id.
package services

import (
	"errors"
	"fmt"
	"strings"

	"github.com/konsulin-care/konsulin-api/internal/app/config"
)

// ErrProviderNotFound reports that an alias cannot be resolved to a party id:
// a reserved internal listing name, the bare "automation" alias without a
// configured APP_AUTOMATION_PARTY_ID, an alias configured with an empty party
// id, or a blank alias. Callers surface it as a missing provider, not a
// transport failure.
var ErrProviderNotFound = errors.New("provider not found")

// automationProviderAlias is the alias that maps to APP_AUTOMATION_PARTY_ID.
const automationProviderAlias = "automation"

// reservedProviderAliases are the internal listing names wired for the
// backend's own flows (magic-link delivery, provider notification, federation
// pepper fetch). They are never user-callable: ResolveProvider refuses them
// before any config lookup, even when an operator maps them as aliases.
var reservedProviderAliases = map[string]struct{}{
	"send-magiclink":    {},
	"notify-provider":   {},
	"federation-pepper": {},
}

// ProviderResolver maps a service provider alias to the providing party id.
// It is immutable after construction and safe for concurrent use, so payment
// dependencies can bind one resolver at bootstrap.
type ProviderResolver struct {
	// automationPartyID is APP_AUTOMATION_PARTY_ID, trimmed.
	automationPartyID string
	// aliases maps lowercased alias to trimmed party id, parsed from
	// APP_SERVICE_PROVIDER_ALIASES.
	aliases map[string]string
}

// NewProviderResolver builds a resolver from the gateway configuration:
// APP_AUTOMATION_PARTY_ID for the "automation" alias and
// APP_SERVICE_PROVIDER_ALIASES (comma-separated alias=partyId list) for
// configured aliases.
//
// Parameters:
//   - gw: the gateway config to read; a nil pointer is treated as empty
//     config, so every alias then fails closed or passes through raw.
//
// Returns: a resolver ready to bind. Never panics.
func NewProviderResolver(gw *config.AppGateway) *ProviderResolver {
	r := &ProviderResolver{aliases: map[string]string{}}
	if gw == nil {
		return r
	}
	r.automationPartyID = strings.TrimSpace(gw.AutomationPartyID)
	r.aliases = parseServiceProviderAliases(gw.ServiceProviderAliases)
	return r
}

// ResolveProvider resolves alias to the providing party id:
// "automation" maps to APP_AUTOMATION_PARTY_ID, aliases configured in
// APP_SERVICE_PROVIDER_ALIASES map to their party ids, and anything else
// passes through unchanged as a raw party id. Reserved internal listing names
// (send-magiclink, notify-provider, federation-pepper) are refused first and
// are never user-callable.
//
// Parameters:
//   - alias: the provider alias or raw party id; matched case-insensitively
//     after trimming, while a raw pass-through keeps the trimmed input as-is.
//
// Returns: the party id and nil, or "" with ErrProviderNotFound when the
// alias is blank, reserved, is "automation" with no APP_AUTOMATION_PARTY_ID,
// or is configured with an empty party id. Never panics.
func (r *ProviderResolver) ResolveProvider(alias string) (string, error) {
	if r == nil {
		r = &ProviderResolver{}
	}
	key := strings.ToLower(strings.TrimSpace(alias))
	if key == "" {
		return "", fmt.Errorf("%w: empty alias", ErrProviderNotFound)
	}
	if _, reserved := reservedProviderAliases[key]; reserved {
		return "", fmt.Errorf("%w: %q is an internal listing name", ErrProviderNotFound, key)
	}
	if key == automationProviderAlias {
		if r.automationPartyID == "" {
			return "", fmt.Errorf("%w: %q has no configured party id", ErrProviderNotFound, automationProviderAlias)
		}
		return r.automationPartyID, nil
	}
	if partyID, configured := r.aliases[key]; configured {
		if partyID == "" {
			return "", fmt.Errorf("%w: alias %q has no configured party id", ErrProviderNotFound, key)
		}
		return partyID, nil
	}
	return strings.TrimSpace(alias), nil
}

// parseServiceProviderAliases parses APP_SERVICE_PROVIDER_ALIASES, a
// comma-separated alias=partyId list, into a map keyed by lowercased alias.
// Entries are trimmed; an entry without '=' carries no party id and is
// dropped; an alias mapped to an empty party id is kept so the lookup fails
// closed instead of passing the name through raw.
//
// Parameters:
//   - raw: the raw APP_SERVICE_PROVIDER_ALIASES value.
//
// Returns: the alias map; empty (never nil) for blank input. Never panics.
func parseServiceProviderAliases(raw string) map[string]string {
	aliases := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(entry), "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		aliases[key] = strings.TrimSpace(value)
	}
	return aliases
}
