package roles

import (
	"context"

	"github.com/casbin/casbin/v2"
)

// CasbinRoleUsecase manages RBAC policies through a synced Casbin enforcer,
// which guards concurrent reads and policy reloads with an internal RWMutex.
type CasbinRoleUsecase struct {
	enforcer *casbin.SyncedEnforcer
}

// NewCasbinRoleUsecase wraps the given synced enforcer in a role usecase.
// It returns a usecase backed by e for listing and mutating policies.
func NewCasbinRoleUsecase(e *casbin.SyncedEnforcer) *CasbinRoleUsecase {
	return &CasbinRoleUsecase{enforcer: e}
}

// ListRoles returns every role known to the enforcer.
func (u *CasbinRoleUsecase) ListRoles(_ context.Context) ([]string, error) {
	return u.enforcer.GetAllRoles()
}

// AddPermission grants the role access to the method and path, then persists it.
func (u *CasbinRoleUsecase) AddPermission(_ context.Context, role, method, path string) error {
	return u.updatePolicy(func() (bool, error) { return u.enforcer.AddPolicy(role, method, path) })
}

// RemovePermission revokes the role's access to the method and path, then persists it.
func (u *CasbinRoleUsecase) RemovePermission(_ context.Context, role, method, path string) error {
	return u.updatePolicy(func() (bool, error) { return u.enforcer.RemovePolicy(role, method, path) })
}

// updatePolicy applies a policy mutation, persists it, and reloads the enforcer.
func (u *CasbinRoleUsecase) updatePolicy(apply func() (bool, error)) error {
	if _, err := apply(); err != nil {
		return err
	}
	if err := u.enforcer.SavePolicy(); err != nil {
		return err
	}
	return u.enforcer.LoadPolicy()
}
