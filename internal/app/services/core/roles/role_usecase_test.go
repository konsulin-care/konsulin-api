package roles

import (
	"context"
	"errors"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	"github.com/stretchr/testify/require"
)

type rolePolicyStore struct {
	persist.Adapter
	policies                      [][]string
	saveErr, loadErr, mutationErr error
	saves, loads                  int
}

func (a *rolePolicyStore) LoadPolicy(m model.Model) error {
	a.loads++
	if a.loadErr != nil {
		return a.loadErr
	}
	for _, policy := range a.policies {
		if err := persist.LoadPolicyArray(append([]string{"p"}, policy...), m); err != nil {
			return err
		}
	}
	return nil
}

func (a *rolePolicyStore) SavePolicy(m model.Model) error {
	a.saves++
	if a.saveErr != nil {
		return a.saveErr
	}
	a.policies = nil
	for _, policy := range m["p"]["p"].Policy {
		a.policies = append(a.policies, append([]string(nil), policy...))
	}
	return nil
}
func (a *rolePolicyStore) AddPolicy(string, string, []string) error    { return a.mutationErr }
func (a *rolePolicyStore) RemovePolicy(string, string, []string) error { return a.mutationErr }

func rolesForTest(t *testing.T) (*CasbinRoleUsecase, *rolePolicyStore) {
	t.Helper()
	m, err := model.NewModelFromString(`[request_definition]
r = sub, method, path
[policy_definition]
p = sub, method, path
[role_definition]
g = _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = g(r.sub,p.sub) && r.method == p.method && r.path == p.path`)
	require.NoError(t, err)
	store := &rolePolicyStore{}
	e, err := casbin.NewEnforcer(m, store)
	require.NoError(t, err)
	store.loads = 0
	return NewCasbinRoleUsecase(e), store
}

func TestRolePermissionsPersistAndReload(t *testing.T) {
	u, store := rolesForTest(t)
	ctx := context.Background()
	require.NoError(t, u.AddPermission(ctx, "Patient", "GET", "/fhir/Patient"))
	allowed, err := u.enforcer.Enforce("Patient", "GET", "/fhir/Patient")
	require.NoError(t, err)
	require.True(t, allowed)
	roles, err := u.ListRoles(ctx)
	require.NoError(t, err)
	require.Empty(t, roles)
	_, err = u.enforcer.AddGroupingPolicy("user", "Patient")
	require.NoError(t, err)
	roles, err = u.ListRoles(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"Patient"}, roles)
	require.NoError(t, u.RemovePermission(ctx, "Patient", "GET", "/fhir/Patient"))
	allowed, err = u.enforcer.Enforce("Patient", "GET", "/fhir/Patient")
	require.NoError(t, err)
	require.False(t, allowed)
	require.Empty(t, store.policies)
	require.Equal(t, 2, store.saves)
	require.Equal(t, 2, store.loads)
}

func TestRolePolicyPersistenceFailures(t *testing.T) {
	failure := errors.New("policy storage unavailable")
	for _, phase := range []string{"mutation", "save", "reload"} {
		t.Run(phase, func(t *testing.T) {
			u, store := rolesForTest(t)
			switch phase {
			case "mutation":
				store.mutationErr = failure
			case "save":
				store.saveErr = failure
			case "reload":
				store.loadErr = failure
			}
			require.ErrorIs(t, u.AddPermission(context.Background(), "Patient", "GET", "/fhir/Patient"), failure)
			if phase == "mutation" {
				require.Zero(t, store.saves)
				require.Zero(t, store.loads)
			}
			if phase == "save" {
				require.Equal(t, 1, store.saves)
				require.Zero(t, store.loads)
			}
			if phase == "reload" {
				require.Equal(t, 1, store.saves)
				require.Equal(t, 1, store.loads)
			}
		})
	}
}
