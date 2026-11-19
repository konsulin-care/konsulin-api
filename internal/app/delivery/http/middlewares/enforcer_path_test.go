package middlewares

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/konsulin-care/konsulin-api/internal/app/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// enforcerRepoRoot resolves the repository root. go test runs each test with
// the package directory as its working directory, so the root is five levels
// up from there.
func enforcerRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", ".."))
}

// setupEnforcerTest chdirs to the repository root so the relative default
// paths (resources/rbac_model.conf, resources/rbac_policy.csv) resolve, and
// returns a silent logger for the middleware constructors.
func setupEnforcerTest(t *testing.T) *zap.Logger {
	t.Helper()
	t.Chdir(enforcerRepoRoot(t))
	return zap.NewNop()
}

// newTestConfig builds an InternalConfig with the given RBAC policy path.
func newTestConfig(rbacPolicyPath string) *config.InternalConfig {
	cfg := &config.InternalConfig{}
	cfg.App.RBACPolicyPath = rbacPolicyPath
	return cfg
}

// writePolicyFile writes the given policy rows to path, replacing content.
func writePolicyFile(t *testing.T, path string, rows ...string) {
	t.Helper()
	content := ""
	for _, row := range rows {
		content += row + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// appendPolicyFile appends one policy row to path without truncating it, so
// the fsnotify watcher observes a single Write event carrying full content.
func appendPolicyFile(t *testing.T, path, row string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	require.NoError(t, err)
	defer func() { require.NoError(t, f.Close()) }()
	_, err = f.WriteString(row + "\n")
	require.NoError(t, err)
}

// waitForEnforce polls the enforcer until Enforce(role, method, path) equals
// want or the deadline passes, then fails the test with the last observed value.
func waitForEnforce(t *testing.T, enf policyEnforcer, role, method, path string, want bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	last := false
	for time.Now().Before(deadline) {
		ok, err := enf.Enforce(role, method, path)
		if err == nil && ok == want {
			return
		}
		last = ok
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("Enforce(%q, %q, %q) never reached %v (last observed %v)", role, method, path, want, last)
}

// TestEnforcer_ConfiguredPolicyPath asserts that NewMiddlewares builds the
// enforcer from InternalConfig.App.RBACPolicyPath: rules present only in the
// configured file are loaded, rules from the default file are not.
func TestEnforcer_ConfiguredPolicyPath(t *testing.T) {
	logger := setupEnforcerTest(t)
	policyPath := filepath.Join(t.TempDir(), "mounted_policy.csv")
	writePolicyFile(t, policyPath, "p, Patient, GET, /fhir/DiagnosticReport")

	mw := NewMiddlewares(logger, nil, newTestConfig(policyPath), nil, nil, nil, nil, nil, nil)
	require.NotNil(t, mw.Enforcer)

	ok, err := mw.Enforcer.Enforce("Patient", "GET", "/fhir/DiagnosticReport")
	require.NoError(t, err)
	assert.True(t, ok, "rule from the configured policy file must be loaded")

	ok, err = mw.Enforcer.Enforce("Patient", "DELETE", "/privacy/purge")
	require.NoError(t, err)
	assert.False(t, ok, "rules from the default policy file must not be loaded")
}

// TestEnforcer_DefaultPolicyPath asserts that with no path configured the
// enforcer loads resources/rbac_policy.csv: the resolved default path is
// exactly that file and its rules are enforced.
func TestEnforcer_DefaultPolicyPath(t *testing.T) {
	logger := setupEnforcerTest(t)

	assert.Equal(t, "resources/rbac_policy.csv", resolveRBACPolicyPath(""),
		"empty configuration must fall back to resources/rbac_policy.csv")

	mw := NewMiddlewares(logger, nil, newTestConfig(""), nil, nil, nil, nil, nil, nil)
	require.NotNil(t, mw.Enforcer)

	ok, err := mw.Enforcer.Enforce("Patient", "DELETE", "/privacy/purge")
	require.NoError(t, err)
	assert.True(t, ok, "rules from resources/rbac_policy.csv must be loaded")

	ok, err = mw.Enforcer.Enforce("Patient", "GET", "/fhir/DiagnosticReport")
	require.NoError(t, err)
	assert.False(t, ok, "rules outside the default policy file must not be loaded")
}

// TestEnforcer_HotReloadOnPolicyWrite asserts that the fsnotify watcher
// watches the configured policy file: appending a rule to it reloads the
// enforcer without a restart.
func TestEnforcer_HotReloadOnPolicyWrite(t *testing.T) {
	logger := setupEnforcerTest(t)
	policyPath := filepath.Join(t.TempDir(), "hot_policy.csv")
	writePolicyFile(t, policyPath, "p, Patient, GET, /fhir/DiagnosticReport")

	mw := NewMiddlewares(logger, nil, newTestConfig(policyPath), nil, nil, nil, nil, nil, nil)
	require.NotNil(t, mw.Enforcer)

	ok, err := mw.Enforcer.Enforce("Guest", "POST", "/fhir/DiagnosticReport")
	require.NoError(t, err)
	require.False(t, ok, "rule must not exist before the file is written")

	appendPolicyFile(t, policyPath, "p, Guest, POST, /fhir/DiagnosticReport")

	waitForEnforce(t, mw.Enforcer, "Guest", "POST", "/fhir/DiagnosticReport", true)

	ok, err = mw.Enforcer.Enforce("Patient", "GET", "/fhir/DiagnosticReport")
	require.NoError(t, err)
	assert.True(t, ok, "previously loaded rules must survive the reload")
}
