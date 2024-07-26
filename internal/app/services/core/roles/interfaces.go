package roles

import (
	"context"
	"konsulin-service/internal/app/models"
)

type RoleUsecase interface{}

type RoleRepository interface {
	FindAll(ctx context.Context) ([]models.Role, error)
	CreateRole(ctx context.Context, roleEntity *models.Role) (roleID string, err error)
	FindByName(ctx context.Context, rolename string) (*models.Role, error)
	FindRoleByID(ctx context.Context, roleID string) (*models.Role, error)
	UpdateRole(ctx context.Context, roleID string, updateData map[string]interface{}) error
}
