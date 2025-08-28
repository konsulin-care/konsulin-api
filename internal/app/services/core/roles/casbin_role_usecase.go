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
	if _, err := u.enforcer.AddPolicy(role, method, path); err != nil {
		return err
	}
	if err := u.enforcer.SavePolicy(); err != nil {
		return err
	}
	return u.enforcer.LoadPolicy()
}

func (u *CasbinRoleUsecase) RemovePermission(ctx context.Context, role, method, path string) error {
	if _, err := u.enforcer.RemovePolicy(role, method, path); err != nil {
		return err
	}
	if err := u.enforcer.SavePolicy(); err != nil {
		return err
	}
	return u.enforcer.LoadPolicy()
}
