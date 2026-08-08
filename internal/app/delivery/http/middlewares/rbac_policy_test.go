package middlewares

import (
	"net/http"
	"path/filepath"
	"runtime"
	"testing"

	"konsulin-service/internal/pkg/constvars"
	"konsulin-service/internal/pkg/utils"

	"github.com/casbin/casbin/v2"
	"github.com/stretchr/testify/assert"
)

// testEnforcer builds a Casbin enforcer from the repo-root resources, mirroring
// newEnforcer (custom pathMatch included). Test working dirs differ from the
// binary's, so paths are resolved from this source file's location.
func testEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test source path")
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..")
	enf, err := casbin.NewEnforcer(
		filepath.Join(root, "resources", "rbac_model.conf"),
		filepath.Join(root, "resources", "rbac_policy.csv"),
	)
	if err != nil {
		t.Fatalf("failed to load RBAC policy: %v", err)
	}
	enf.AddFunction("pathMatch", func(args ...interface{}) (interface{}, error) {
		if len(args) != 2 {
			return false, nil
		}
		requestPath, ok1 := args[0].(string)
		policyPath, ok2 := args[1].(string)
		if !ok1 || !ok2 {
			return false, nil
		}
		return utils.PathMatch(requestPath, policyPath), nil
	})
	return enf
}

// TestRBACPolicy_Communication asserts the full Communication access matrix:
// Patient may GET/POST/PUT; Researcher and Superadmin may GET only; every other
// role is denied entirely.
func TestRBACPolicy_Communication(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name   string
		role   string
		method string
		want   bool
	}{
		{"patient get", constvars.KonsulinRolePatient, http.MethodGet, true},
		{"patient post", constvars.KonsulinRolePatient, http.MethodPost, true},
		{"patient put", constvars.KonsulinRolePatient, http.MethodPut, true},
		{"patient delete", constvars.KonsulinRolePatient, http.MethodDelete, false},
		{"researcher get", constvars.KonsulinRoleResearcher, http.MethodGet, true},
		{"researcher post", constvars.KonsulinRoleResearcher, http.MethodPost, false},
		{"researcher put", constvars.KonsulinRoleResearcher, http.MethodPut, false},
		{"superadmin get", constvars.KonsulinRoleSuperadmin, http.MethodGet, true},
		{"superadmin post", constvars.KonsulinRoleSuperadmin, http.MethodPost, false},
		{"superadmin put", constvars.KonsulinRoleSuperadmin, http.MethodPut, false},
		{"practitioner get", constvars.KonsulinRolePractitioner, http.MethodGet, false},
		{"practitioner post", constvars.KonsulinRolePractitioner, http.MethodPost, false},
		{"clinic admin get", constvars.KonsulinRoleClinicAdmin, http.MethodGet, false},
		{"guest get", constvars.KonsulinRoleGuest, http.MethodGet, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, c.method, "/fhir/Communication")
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, %q, /fhir/Communication)", c.role, c.method)
		})
	}
}

// TestRBACPolicy_PrivacyPurge asserts the erasure endpoint matrix: only a
// resolved Patient session may DELETE /privacy/purge; Guest, Practitioner,
// Clinic Admin, Researcher, and Superadmin are denied by policy.
func TestRBACPolicy_PrivacyPurge(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name   string
		role   string
		method string
		want   bool
	}{
		{"patient delete", constvars.KonsulinRolePatient, http.MethodDelete, true},
		{"guest delete", constvars.KonsulinRoleGuest, http.MethodDelete, false},
		{"practitioner delete", constvars.KonsulinRolePractitioner, http.MethodDelete, false},
		{"clinic admin delete", constvars.KonsulinRoleClinicAdmin, http.MethodDelete, false},
		{"researcher delete", constvars.KonsulinRoleResearcher, http.MethodDelete, false},
		{"superadmin delete", constvars.KonsulinRoleSuperadmin, http.MethodDelete, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, c.method, "/privacy/purge")
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, %q, /privacy/purge)", c.role, c.method)
		})
	}
}
