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

// TestRBACPolicy_Schedule asserts the Schedule access matrix: Clinic Admin may
// GET/POST, Patient and Superadmin GET, Practitioner and Superadmin PUT (the
// idempotent seed flow PUTs /fhir/Schedule/seed-schedule as superadmin), and
// no role may DELETE via the gateway.
func TestRBACPolicy_Schedule(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name   string
		role   string
		method string
		want   bool
	}{
		{"clinic admin get", constvars.KonsulinRoleClinicAdmin, http.MethodGet, true},
		{"clinic admin post", constvars.KonsulinRoleClinicAdmin, http.MethodPost, true},
		{"clinic admin put", constvars.KonsulinRoleClinicAdmin, http.MethodPut, false},
		{"clinic admin delete", constvars.KonsulinRoleClinicAdmin, http.MethodDelete, false},
		{"patient get", constvars.KonsulinRolePatient, http.MethodGet, true},
		{"patient put", constvars.KonsulinRolePatient, http.MethodPut, false},
		{"practitioner put", constvars.KonsulinRolePractitioner, http.MethodPut, true},
		{"superadmin get", constvars.KonsulinRoleSuperadmin, http.MethodGet, true},
		{"superadmin post", constvars.KonsulinRoleSuperadmin, http.MethodPost, true},
		{"superadmin put", constvars.KonsulinRoleSuperadmin, http.MethodPut, true},
		{"superadmin delete", constvars.KonsulinRoleSuperadmin, http.MethodDelete, false},
		{"researcher put", constvars.KonsulinRoleResearcher, http.MethodPut, false},
		{"guest put", constvars.KonsulinRoleGuest, http.MethodPut, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, c.method, "/fhir/Schedule")
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, %q, /fhir/Schedule)", c.role, c.method)
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

// TestRBACPolicy_OrganizationArchetype asserts the party-identity read surface
// (BK-PARTYID-001): Guest may GET /fhir/Organization/archetype and nothing
// else on that path — the policy row is GET-only.
func TestRBACPolicy_OrganizationArchetype(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name   string
		role   string
		method string
		want   bool
	}{
		{"guest get", constvars.KonsulinRoleGuest, http.MethodGet, true},
		{"guest post", constvars.KonsulinRoleGuest, http.MethodPost, false},
		{"guest put", constvars.KonsulinRoleGuest, http.MethodPut, false},
		{"guest delete", constvars.KonsulinRoleGuest, http.MethodDelete, false},
		{"patient get", constvars.KonsulinRolePatient, http.MethodGet, true},
		{"patient put", constvars.KonsulinRolePatient, http.MethodPut, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, c.method, "/fhir/Organization/archetype")
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, %q, /fhir/Organization/archetype)", c.role, c.method)
		})
	}
}

// TestRBACPolicy_ServiceAutomation asserts the user-facing automation
// listings under /api/v1/services/automation: Patient, Practitioner and Guest
// may POST interpret, modify-profile and update-avatar; Superadmin may POST
// send-wa-link; send-magiclink is not granted on the automation surface
// (login stays on /hook/synchronous/send-magiclink).
func TestRBACPolicy_ServiceAutomation(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name string
		role string
		path string
		want bool
	}{
		{"patient post interpret", constvars.KonsulinRolePatient, "/api/v1/services/automation/interpret", true},
		{"patient post modify-profile", constvars.KonsulinRolePatient, "/api/v1/services/automation/modify-profile", true},
		{"patient post update-avatar", constvars.KonsulinRolePatient, "/api/v1/services/automation/update-avatar", true},
		{"patient post send-magiclink", constvars.KonsulinRolePatient, "/api/v1/services/automation/send-magiclink", false},
		{"patient post send-wa-link", constvars.KonsulinRolePatient, "/api/v1/services/automation/send-wa-link", false},
		{"practitioner post interpret", constvars.KonsulinRolePractitioner, "/api/v1/services/automation/interpret", true},
		{"practitioner post modify-profile", constvars.KonsulinRolePractitioner, "/api/v1/services/automation/modify-profile", true},
		{"practitioner post update-avatar", constvars.KonsulinRolePractitioner, "/api/v1/services/automation/update-avatar", true},
		{"guest post interpret", constvars.KonsulinRoleGuest, "/api/v1/services/automation/interpret", true},
		{"guest post modify-profile", constvars.KonsulinRoleGuest, "/api/v1/services/automation/modify-profile", true},
		{"guest post update-avatar", constvars.KonsulinRoleGuest, "/api/v1/services/automation/update-avatar", true},
		{"guest post send-magiclink", constvars.KonsulinRoleGuest, "/api/v1/services/automation/send-magiclink", false},
		{"superadmin post send-wa-link", constvars.KonsulinRoleSuperadmin, "/api/v1/services/automation/send-wa-link", true},
		{"superadmin post interpret", constvars.KonsulinRoleSuperadmin, "/api/v1/services/automation/interpret", false},
		{"researcher post interpret", constvars.KonsulinRoleResearcher, "/api/v1/services/automation/interpret", false},
		{"clinic admin post interpret", constvars.KonsulinRoleClinicAdmin, "/api/v1/services/automation/interpret", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, http.MethodPost, c.path)
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, POST, %s)", c.role, c.path)
		})
	}
}

// TestRBACPolicy_Referrals asserts the referral workflow matrix: only
// Practitioner and Clinic Admin may send, list, confirm and reject referrals;
// Patient and Guest are denied on every operation. The controller enforces
// the live Task id on confirm/reject, so those cases carry one.
func TestRBACPolicy_Referrals(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name   string
		role   string
		method string
		path   string
		want   bool
	}{
		{"practitioner send", constvars.KonsulinRolePractitioner, http.MethodPost, "/api/v1/referrals", true},
		{"practitioner list", constvars.KonsulinRolePractitioner, http.MethodGet, "/api/v1/referrals/incoming", true},
		{"practitioner confirm", constvars.KonsulinRolePractitioner, http.MethodPost, "/api/v1/referrals/incoming/task-42/confirm", true},
		{"practitioner reject", constvars.KonsulinRolePractitioner, http.MethodPost, "/api/v1/referrals/incoming/task-42/reject", true},
		{"clinic admin send", constvars.KonsulinRoleClinicAdmin, http.MethodPost, "/api/v1/referrals", true},
		{"clinic admin list", constvars.KonsulinRoleClinicAdmin, http.MethodGet, "/api/v1/referrals/incoming", true},
		{"clinic admin confirm", constvars.KonsulinRoleClinicAdmin, http.MethodPost, "/api/v1/referrals/incoming/task-42/confirm", true},
		{"clinic admin reject", constvars.KonsulinRoleClinicAdmin, http.MethodPost, "/api/v1/referrals/incoming/task-42/reject", true},
		{"patient send", constvars.KonsulinRolePatient, http.MethodPost, "/api/v1/referrals", false},
		{"patient list", constvars.KonsulinRolePatient, http.MethodGet, "/api/v1/referrals/incoming", false},
		{"patient confirm", constvars.KonsulinRolePatient, http.MethodPost, "/api/v1/referrals/incoming/task-42/confirm", false},
		{"patient reject", constvars.KonsulinRolePatient, http.MethodPost, "/api/v1/referrals/incoming/task-42/reject", false},
		{"guest send", constvars.KonsulinRoleGuest, http.MethodPost, "/api/v1/referrals", false},
		{"guest list", constvars.KonsulinRoleGuest, http.MethodGet, "/api/v1/referrals/incoming", false},
		{"guest confirm", constvars.KonsulinRoleGuest, http.MethodPost, "/api/v1/referrals/incoming/task-42/confirm", false},
		{"guest reject", constvars.KonsulinRoleGuest, http.MethodPost, "/api/v1/referrals/incoming/task-42/reject", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, c.method, c.path)
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, %q, %s)", c.role, c.method, c.path)
		})
	}
}

// TestRBACPolicy_ResearchDatabank asserts the research databank matrix:
// only a Patient may POST the study withdrawal (the handler binds the
// session uid as the withdrawing patient), only a Researcher may create
// and list export requests, and Patient and Guest are denied on the
// export-request surface entirely.
func TestRBACPolicy_ResearchDatabank(t *testing.T) {
	enf := testEnforcer(t)

	cases := []struct {
		name   string
		role   string
		method string
		path   string
		want   bool
	}{
		{"patient post withdraw", constvars.KonsulinRolePatient, http.MethodPost, "/api/v1/research/studies/st-1/withdraw", true},
		{"researcher post withdraw", constvars.KonsulinRoleResearcher, http.MethodPost, "/api/v1/research/studies/st-1/withdraw", false},
		{"practitioner post withdraw", constvars.KonsulinRolePractitioner, http.MethodPost, "/api/v1/research/studies/st-1/withdraw", false},
		{"guest post withdraw", constvars.KonsulinRoleGuest, http.MethodPost, "/api/v1/research/studies/st-1/withdraw", false},
		{"researcher post requests", constvars.KonsulinRoleResearcher, http.MethodPost, "/api/v1/research/requests", true},
		{"researcher get requests", constvars.KonsulinRoleResearcher, http.MethodGet, "/api/v1/research/requests", true},
		{"patient post requests", constvars.KonsulinRolePatient, http.MethodPost, "/api/v1/research/requests", false},
		{"patient get requests", constvars.KonsulinRolePatient, http.MethodGet, "/api/v1/research/requests", false},
		{"guest post requests", constvars.KonsulinRoleGuest, http.MethodPost, "/api/v1/research/requests", false},
		{"superadmin post requests", constvars.KonsulinRoleSuperadmin, http.MethodPost, "/api/v1/research/requests", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, err := enf.Enforce(c.role, c.method, c.path)
			assert.NoError(t, err)
			assert.Equal(t, c.want, ok, "Enforce(%q, %q, %s)", c.role, c.method, c.path)
		})
	}
}
