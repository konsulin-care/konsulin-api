package roles

import (
	"context"

	"github.com/casbin/casbin/v2"
)

// CasbinRoleUsecase implements contracts.RoleUsecase using a Casbin enforcer
// to manage role permissions stored in a CSV policy file.
type CasbinRoleUsecase struct {
	enforcer *casbin.Enforcer
}

// NewCasbinRoleUsecase creates a new usecase with the provided enforcer.
func NewCasbinRoleUsecase(e *casbin.Enforcer) *CasbinRoleUsecase {
	return &CasbinRoleUsecase{enforcer: e}
}

// ListRoles returns all roles defined in the policy.
func (u *CasbinRoleUsecase) ListRoles(ctx context.Context) ([]string, error) {
	return u.enforcer.GetAllRoles()
}

// AddPermission adds a permission rule for the given role and reloads the policy.
func (u *CasbinRoleUsecase) AddPermission(ctx context.Context, role, obj, act string) error {
	if _, err := u.enforcer.AddPolicy(role, obj, act); err != nil {
		return err
	}
	if err := u.enforcer.SavePolicy(); err != nil {
		return err
	}
	return u.enforcer.LoadPolicy()
}

// RemovePermission removes a permission rule for the given role and reloads the policy.
func (u *CasbinRoleUsecase) RemovePermission(ctx context.Context, role, obj, act string) error {
	if _, err := u.enforcer.RemovePolicy(role, obj, act); err != nil {
		return err
	}
	if err := u.enforcer.SavePolicy(); err != nil {
		return err
	}
	return u.enforcer.LoadPolicy()
}
