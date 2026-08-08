package roles

import (
	"context"

	"github.com/casbin/casbin/v2"
)

type CasbinRoleUsecase struct {
	enforcer *casbin.Enforcer
}

func NewCasbinRoleUsecase(e *casbin.Enforcer) *CasbinRoleUsecase {
	return &CasbinRoleUsecase{enforcer: e}
}

func (u *CasbinRoleUsecase) ListRoles(ctx context.Context) ([]string, error) {
	return u.enforcer.GetAllRoles()
}

func (u *CasbinRoleUsecase) AddPermission(ctx context.Context, role, method, path string) error {
	return u.updatePolicy(func() (bool, error) { return u.enforcer.AddPolicy(role, method, path) })
}

func (u *CasbinRoleUsecase) RemovePermission(ctx context.Context, role, method, path string) error {
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
