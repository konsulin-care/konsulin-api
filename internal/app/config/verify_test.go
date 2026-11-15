package config

import "testing"

func has(fs []Finding, lvl, v string) bool {
	for _, f := range fs {
		if f.Level == lvl && f.Var == v {
			return true
		}
	}
	return false
}

func TestVerifyEnvProd(t *testing.T) {
	fs := VerifyEnv(map[string]string{"APP_ENV": "production", "REDIS_HOST": "localhost", "APP_JWT_SECRET": "short-secret-value"}, VerifyStandalone)
	for _, v := range []string{"REDIS_PASSWORD", "SUPERTOKEN_API_KEY", "SUPERADMIN_API_KEY", "REDIS_HOST", "APP_BASE_URL"} {
		if !has(fs, "FAIL", v) {
			t.Errorf("missing FAIL %s", v)
		}
	}
	if !has(fs, "WARN", "APP_JWT_SECRET") {
		t.Error("missing WARN")
	}
	for _, f := range fs {
		if f.Var == "REDIS_HOST" && f.Message != "REDIS_HOST must not point at localhost in a non-dev environment" {
			t.Error("unexpected message")
		}
	}
}

func TestVerifyEnvDev(t *testing.T) {
	fs := VerifyEnv(map[string]string{}, VerifyStandalone)
	if has(fs, "FAIL", "REDIS_PASSWORD") || has(fs, "FAIL", "REDIS_HOST") {
		t.Error("dev should not require")
	}
	if !has(fs, "FAIL", "SUPERADMIN_API_KEY") {
		t.Error("always required")
	}
}
