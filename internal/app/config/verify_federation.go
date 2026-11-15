package config

import (
	"crypto/ecdh"
	"encoding/base64"

	"github.com/konsulin-care/gateway-sdk-go/auth"
)

const (
	levelWarn = "WARN"

	varGatewayURL    = "APP_GATEWAY_URL"
	varPartyPrivKey  = "APP_PARTY_PRIVATE_KEY"
	varPartyEncKey   = "APP_PARTY_ENC_PRIVATE_KEY"
	varPartyID       = "APP_PARTY_ID"
	varAutomationPID = "APP_AUTOMATION_PARTY_ID"
)

// verifyFederationRules checks the federation variables.
//
// Parameters:
//   - get: trimmed env lookup.
//   - appEnv: lower-cased APP_ENV.
//
// Returns FAIL findings naming variables only.
func verifyFederationRules(get func(string) string, appEnv string) []Finding {
	var out []Finding
	for _, k := range []string{varGatewayURL, varPartyID, varAutomationPID} {
		if get(k) == "" {
			out = append(out, Finding{levelFail, k, k + " is required in federation mode"})
		}
	}
	if err := validateGatewayURLScheme(get(varGatewayURL), appEnv); err != nil {
		out = append(out, Finding{levelFail, varGatewayURL, varGatewayURL + " must use https:// when APP_ENV is not a dev environment"})
	}
	if _, err := auth.ParseEd25519PrivateKey(get(varPartyPrivKey)); err != nil {
		out = append(out, Finding{levelFail, varPartyPrivKey, varPartyPrivKey + " is missing or not a valid Ed25519 seed"})
	}
	raw, err := base64.StdEncoding.DecodeString(get(varPartyEncKey))
	if err == nil {
		_, err = ecdh.X25519().NewPrivateKey(raw)
	}
	if err != nil {
		out = append(out, Finding{levelFail, varPartyEncKey, varPartyEncKey + " is missing or not a base64 raw 32-byte X25519 key"})
	}
	return out
}

// verifyStandaloneWarnings warns about gateway variables set in standalone mode.
//
// Parameters:
//   - get: trimmed env lookup.
//
// Returns WARN findings only.
func verifyStandaloneWarnings(get func(string) string) []Finding {
	var out []Finding
	for _, k := range []string{
		varGatewayURL, varPartyID, varPartyPrivKey, varPartyEncKey, varAutomationPID,
	} {
		if get(k) != "" {
			out = append(out, Finding{levelWarn, k, k + " is set but standalone mode ignores it"})
		}
	}
	return out
}
