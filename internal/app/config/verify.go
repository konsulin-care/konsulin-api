package config

import "strings"

const levelFail = "FAIL"

// VerifyMode selects which rule set VerifyEnv applies.
type VerifyMode int

const (
	// VerifyStandalone checks a backend without a gateway.
	VerifyStandalone VerifyMode = iota
	// VerifyFederation checks a backend federated with the gateway.
	VerifyFederation
)

// Finding is one verification result. Level is FAIL or WARN.
type Finding struct {
	Level   string
	Var     string
	Message string
}

// VerifyEnv applies the offline base rules to a plain env map.
//
// Parameters:
//   - values: env variable name to value (never read from os.Getenv).
//   - mode: standalone or federation (federation rules are added later).
//
// Returns findings; messages name variables only, never values.
func VerifyEnv(values map[string]string, mode VerifyMode) []Finding {
	get := func(k string) string { return strings.TrimSpace(values[k]) }
	appEnv := strings.ToLower(get("APP_ENV"))
	if appEnv == "" {
		appEnv = "local"
	}
	dev := isDevLikeEnv(appEnv)
	var out []Finding
	fail := func(v, m string) { out = append(out, Finding{levelFail, v, m}) }

	if !dev {
		for _, k := range []string{"APP_JWT_SECRET", "REDIS_PASSWORD", "SUPERTOKEN_API_KEY"} {
			if get(k) == "" {
				fail(k, k+" is required when APP_ENV is not a dev environment")
			}
		}
	}
	for _, k := range []string{"SUPERADMIN_API_KEY", "APP_SUPERTOKEN_KONSULIN_DASHBOARD_ADMIN_EMAIL"} {
		if get(k) == "" {
			fail(k, k+" is required")
		}
	}
	if s := get("APP_JWT_SECRET"); s != "" && len(s) < 32 {
		out = append(out, Finding{"WARN", "APP_JWT_SECRET", "APP_JWT_SECRET is shorter than 32 characters"})
	}
	if !dev {
		out = append(out, verifyHosts(get)...)
	}
	if mode == VerifyFederation {
		out = append(out, verifyFederationRules(get, appEnv)...)
	} else {
		out = append(out, verifyStandaloneWarnings(get)...)
	}
	return out
}

// verifyHosts checks that non-dev endpoints are set and not localhost.
func verifyHosts(get func(string) string) []Finding {
	var out []Finding
	for _, k := range []string{
		"REDIS_HOST", "SUPERTOKEN_CONNECTION_URI", "APP_FHIR_BASE_URL", "APP_BASE_URL",
		"APP_FRONTEND_DOMAIN", "SUPERTOKEN_API_DOMAIN", "SUPERTOKEN_WEBSITE_DOMAIN",
	} {
		v := strings.ToLower(get(k))
		switch {
		case v == "":
			out = append(out, Finding{levelFail, k, k + " must be set when APP_ENV is not a dev environment"})
		case strings.Contains(v, "localhost") || strings.Contains(v, "127.0.0.1"):
			out = append(out, Finding{levelFail, k, k + " must not point at localhost in a non-dev environment"})
		}
	}
	return out
}
