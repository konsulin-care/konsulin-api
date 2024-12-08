package roles

import (
	"context"
	"database/sql"
	"encoding/json"
	"konsulin-service/internal/app/contracts"
	"konsulin-service/internal/app/models"
	"konsulin-service/internal/pkg/exceptions"
	"konsulin-service/internal/pkg/queries"
)

type rolePostgresRepository struct {
	DB *sql.DB
}

func NewRolePostgresRepository(db *sql.DB) contracts.RoleRepository {
	return &rolePostgresRepository{
		DB: db,
	}
}

func (repo *rolePostgresRepository) FindAll(ctx context.Context) ([]models.Role, error) {
	query := queries.GetAllRolesWithPermissions
	rows, err := repo.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		var permissionsJSON string

		err := rows.Scan(&role.ID, &role.Name, &permissionsJSON)
		if err != nil {
			return nil, exceptions.ErrPostgresDBFindData(err)
		}

		err = json.Unmarshal([]byte(permissionsJSON), &role.Permissions)
		if err != nil {
			return nil, exceptions.ErrPostgresDBFindData(err)
		}

		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	return roles, nil
}

func (repo *rolePostgresRepository) CreateRole(ctx context.Context, entityRole *models.Role) (roleID string, err error) {
	var id string
	query := queries.InsertRole
	err = repo.DB.QueryRowContext(ctx, query, entityRole.Name).Scan(&id)
	if err != nil {
		return "", exceptions.ErrPostgresDBInsertData(err)
	}
	return id, nil
}

func (repo *rolePostgresRepository) FindByName(ctx context.Context, roleName string) (*models.Role, error) {
	query := queries.GetRoleWithPermissionsByName

	var role models.Role
	var permissionsJSON string

	err := repo.DB.QueryRowContext(ctx, query, roleName).Scan(&role.ID, &role.Name, &permissionsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	err = json.Unmarshal([]byte(permissionsJSON), &role.Permissions)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	return &role, nil
}

func (repo *rolePostgresRepository) FindRoleByID(ctx context.Context, roleID string) (*models.Role, error) {
	query := queries.GetRoleWithPermissionsByID

	var role models.Role
	var permissionsJSON string

	err := repo.DB.QueryRowContext(ctx, query, roleID).Scan(&role.ID, &role.Name, &permissionsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	err = json.Unmarshal([]byte(permissionsJSON), &role.Permissions)
	if err != nil {
		return nil, exceptions.ErrPostgresDBFindData(err)
	}

	return &role, nil
}

func (repo *rolePostgresRepository) UpdateRole(ctx context.Context, roleID string, updateData map[string]interface{}) error {
	query := queries.UpdateRole
	_, err := repo.DB.ExecContext(ctx, query, roleID, updateData)
	if err != nil {
		return exceptions.ErrPostgresDBUpdateData(err)
	}
	return nil
}
