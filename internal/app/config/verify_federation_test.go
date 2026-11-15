package config

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func fedValues(t *testing.T, env, url string) map[string]string {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	_, _ = rand.Read(seed)
	x, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"APP_ENV": env, "APP_GATEWAY_URL": url, "APP_PARTY_ID": "p", "APP_AUTOMATION_PARTY_ID": "a",
		"APP_PARTY_PRIVATE_KEY":     base64.StdEncoding.EncodeToString(seed),
		"APP_PARTY_ENC_PRIVATE_KEY": base64.StdEncoding.EncodeToString(x.Bytes()),
	}
}

func hasFail(fs []Finding, v string) bool {
	for _, f := range fs {
		if f.Var == v && f.Level == levelFail {
			return true
		}
	}
	return false
}

func TestFederationRules(t *testing.T) {
	v := fedValues(t, "local", "http://gw")
	fs := VerifyEnv(v, VerifyFederation)
	for _, k := range []string{"APP_GATEWAY_URL", "APP_PARTY_PRIVATE_KEY", "APP_PARTY_ENC_PRIVATE_KEY"} {
		if hasFail(fs, k) {
			t.Errorf("unexpected fail %s", k)
		}
	}
	p := fedValues(t, "production", "http://gw")
	if !hasFail(VerifyEnv(p, VerifyFederation), "APP_GATEWAY_URL") {
		t.Error("http in production should fail")
	}
	v["APP_PARTY_PRIVATE_KEY"] = "bad"
	v["APP_PARTY_ENC_PRIVATE_KEY"] = "bad"
	delete(v, "APP_PARTY_ID")
	fs = VerifyEnv(v, VerifyFederation)
	for _, k := range []string{"APP_PARTY_PRIVATE_KEY", "APP_PARTY_ENC_PRIVATE_KEY", "APP_PARTY_ID"} {
		if !hasFail(fs, k) {
			t.Errorf("expected fail %s", k)
		}
	}
}

func TestStandaloneGatewayVarsWarnOnly(t *testing.T) {
	v := fedValues(t, "local", "https://gw")
	for _, f := range VerifyEnv(v, VerifyStandalone) {
		if f.Var == "APP_GATEWAY_URL" && f.Level != levelWarn {
			t.Errorf("want WARN, got %s", f.Level)
		}
	}
}
